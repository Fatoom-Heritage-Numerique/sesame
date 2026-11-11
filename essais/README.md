<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), essais restants de Confidential Space ; décision 0075 -->

# Essais sur Confidential Space

Programme jetable d'une **séance unique** sur un projet Google Cloud d'essai, créé pour la séance et supprimé à la fin (décision 0075). Ce n'est ni Sésame, ni un adaptateur de Sésame : aucun binaire publié ne lie ce dossier, et son image n'est jamais une version.

- **Durée attendue** : environ 1 h 30 à 2 h, dont 30 à 40 minutes d'attente entre deux relevés d'horloge.
- **Coût** : les trois VM sont facturées **tant qu'elles existent**, même sans charge ; Firestore, Cloud Build, Artifact Registry et Cloud Logging au volume. Ce dépôt n'écrit aucun prix : le budget d'alerte (étape 4) prévient, il n'arrête rien. La dernière étape supprime le projet, et avec lui tout ce qui est facturé.
- **Où** : tout se colle dans Cloud Shell (`https://shell.cloud.google.com`), qui a déjà `gcloud`, `git`, `curl`, `openssl` et `basenc` ; rien à installer. Une commande par bloc, dans l'ordre.

## Ce que mesure chaque essai

Le binaire `essais/cmd/essai` tourne dans un conteneur Confidential Space sur l'image de **production** (`confidential-space`), sur trois VM `n2d-highcpu-2` AMD SEV, une par zone. Il sert en HTTP sur le port 8080, joint seulement par un tunnel IAP depuis Cloud Shell, et rend du JSON :

| Route | Mesure | Critère de réussite (spécification de l'opérateur) |
|---|---|---|
| `GET /horloge?mesures=16&intervalle=250&reference=nom` | Écart de l'horloge de la VM sur le serveur de métadonnées (`metadata.google.internal`, NTP, UDP 123 ; `reference=ip` : `169.254.169.254`), en SNTP (RFC 4330) : écart et délai aller-retour de chaque mesure, min, médiane, max. Le programme écrit aussi une mesure par minute dans son journal, pour suivre une migration à chaud. `demarreLe` montre que le conteneur n'a pas redémarré. | La dérive, avant et après une migration à chaud, tient largement le ± 2 min du contrôle de fraîcheur (`ARCHITECTURE.md` §2.2, « Heure », et §6.3) ; elle est reportée dans la spécification. |
| `GET /attestation?audience=…&nonce=…&nonce=…` | Jeton d'attestation **PKI** réel demandé au lanceur (`POST /v1/token` sur `/run/container_launcher/teeserver.sock`, `token_type` `PKI`), rendu brut. | Le client de référence (`verifierAttestation`) l'accepte avec la racine Confidential Space épinglée, une liste des versions d'essai portant l'empreinte de l'image et son `envDigest`, le numéro du projet d'essai et son dépôt d'images ; il le refuse pour une autre `sessionPub` (motif `eat_nonce`) et pour une liste sans son empreinte (`image_digest`). Un écart de forme avec `ARCHITECTURE.md` §3 est reporté. |
| `GET /firestore?base=essai&duree=60&debit=10&concurrence=16&taille=512` | Pendant `duree` secondes, `debit` transactions par seconde : chacune relit le document unique `chaine/tete`, le réécrit (époque + 1, empreinte chaînée) et crée un document de ligne en création seule, comme le registre (§6.2). Rend les transactions lancées, validées, rejouées (`ABORTED`), en échec, les collisions (deux lignes de même époque : doit rester 0), le débit tenu et les latences. | Le débit tenu sans contention, depuis trois clients à débit croissant, et le taux de transactions rejouées à ce débit, sont reportés dans la spécification. |

Les journaux du programme ne portent que des compteurs et des mesures : ni jeton d'attestation, ni jeton d'accès, ni contenu.

## Ce qui est vérifié, et ce qui ne l'est pas

Rien de ce qui suit n'a été exécuté sur Google Cloud avant la séance : le dépôt n'a pas de projet. Le programme est testé sans Google (`go test ./essais/...` : SNTP contre un serveur local, socket du lanceur simulée, Firestore simulé) et son image construite par Docker. Pour chaque commande, la colonne dit d'où vient sa forme ; **« à ajuster »** signale où la séance peut demander une retouche.

| Étape | Commande | Source |
|---|---|---|
| 1 à 3 | `gcloud billing accounts list`, `projects create`, `billing projects link`, `config set` | usage courant de `gcloud`, non relu pour cette séance |
| 4 | `gcloud billing budgets create` | page de référence de `gcloud billing budgets create` (forme de `--budget-amount`, `--filter-projects`, `--threshold-rule`) |
| 5 | `gcloud services enable` | noms d'API non relus un à un : **à ajuster** si l'un est refusé |
| 6 | `gcloud artifacts repositories create` | usage courant, non relu |
| 7 et 8 | `git clone`, droits du compte de Cloud Build | page « Cloud Build service account updates » : un projet neuf construit avec le compte de service Compute Engine par défaut, sans rôle ; le rôle donné ici (`roles/cloudbuild.builds.builder`) n'est pas relu dans la documentation : **à ajuster** si la construction échoue faute de droit (repli : étape 9 bis) |
| 9 | `gcloud builds submit --config essais/cloudbuild.yaml` | forme du fichier de construction non essayée sur Cloud Build |
| 10 | empreinte de l'image (`gcloud artifacts docker images describe`) | champ `image_summary.digest` non relu : **à ajuster** |
| 11 et 12 | compte de service et rôles | rôles de la page « Deploy workloads » de Confidential Space (`confidentialcomputing.workloadUser`, `artifactregistry.reader`, `logging.logWriter`) ; `roles/datastore.user` pour Firestore, non relu |
| 13 | `gcloud firestore databases create` | page de référence (`--database`, `--location`, type `firestore-native` et édition `standard` par défaut) |
| 14 à 16 | réseau, sous-réseau avec accès privé à Google, pare-feu IAP | plage `35.235.240.0/20` et `start-iap-tunnel` : page « Using IAP for TCP forwarding » ; le tirage de l'image et les API de Google par l'accès privé, sans adresse IP publique, non essayés : **à ajuster** (repli : retirer `--no-address`) |
| 17 | `gcloud compute instances create` | options de la page « Deploy workloads » (`--confidential-compute-type`, `--maintenance-policy=MIGRATE` pour N2D sous SEV, `--shielded-secure-boot`, `--image-project=confidential-space-images`, `--image-family`, `tee-image-reference`, `tee-container-log-redirect`) ; politique `log_redirect=always` et ports par `EXPOSE` : page « Create and customize workloads » |
| 18 | sortie série de la VM | lecture du démarrage du lanceur, forme du journal non relue |
| 20 et 21 | tunnels IAP et `curl` des routes | tunnel : page IAP ; lecture de la socket par un conteneur sans privilège (utilisateur 65532) non essayée : **à ajuster** si `/attestation` rend `permission denied` (c'est aussi une réponse utile pour l'image de Sésame) |
| 21 | mode de concurrence de Firestore (`gcloud firestore databases describe`), puis passage en `OPTIMISTIC` | champ `concurrencyMode` de la ressource `projects.databases` de l'API REST (défaut `PESSIMISTIC` en édition `standard`) ; `describe --database` et la modification par `PATCH` ne sont pas relus : **à ajuster** |
| 22 | `gcloud compute instances simulate-maintenance-event` | page « Simulate a host maintenance event » ; son effet sur une VM SEV n'y est pas décrit (l'essai du lot précédent a vu la migration à chaud) |
| 24 | `gcloud logging read` des mesures d'horloge | filtre non essayé : **à ajuster** si rien ne sort |
| 25 à 27 | archive, `cloudshell download`, suppression du projet | usage courant, non relu |

## Marche à suivre

### 0. Variables

Remplacez les valeurs entre `<…>` ; rien de réel n'est écrit dans ce dépôt.

```bash
export PROJET="<identifiant du projet d'essai, à choisir, unique dans Google Cloud>"
```

```bash
export FACTURATION="<identifiant du compte de facturation, étape 1>"
```

```bash
export COMMIT="<commit de main du dépôt sesame à essayer>"
```

```bash
export REGION="europe-west9"
```

```bash
export IMAGE="${REGION}-docker.pkg.dev/${PROJET}/essais/essai:${COMMIT}"
```

```bash
export COMPTE="essai-vm@${PROJET}.iam.gserviceaccount.com"
```

Si Cloud Shell se ferme pendant la séance, recollez ces blocs (et ceux des étapes 10 et 20) avant de reprendre.

### 1. Compte de facturation

```bash
gcloud billing accounts list
```

### 2. Projet d'essai

Ajoutez `--organization=<…>` ou `--folder=<…>` si votre compte l'exige.

```bash
gcloud projects create "${PROJET}"
```

```bash
gcloud billing projects link "${PROJET}" --billing-account="${FACTURATION}"
```

### 3. Projet courant

```bash
gcloud config set project "${PROJET}"
```

### 4. Budget d'alerte

Montant dans la devise du compte de facturation ; alertes par email aux administrateurs du compte à 50 %, 90 % et 100 %.

```bash
gcloud services enable billingbudgets.googleapis.com
```

```bash
gcloud billing budgets create --billing-account="${FACTURATION}" --display-name="essais-confidential-space" --budget-amount=20 --filter-projects="projects/${PROJET}" --threshold-rule=percent=0.5 --threshold-rule=percent=0.9 --threshold-rule=percent=1.0
```

### 5. API

```bash
gcloud services enable compute.googleapis.com confidentialcomputing.googleapis.com artifactregistry.googleapis.com cloudbuild.googleapis.com firestore.googleapis.com iap.googleapis.com logging.googleapis.com
```

### 6. Dépôt d'images

```bash
gcloud artifacts repositories create essais --repository-format=docker --location="${REGION}"
```

### 7. Clone du dépôt public

```bash
git clone https://github.com/Fatoom-Heritage-Numerique/sesame.git ~/sesame-essais
```

```bash
cd ~/sesame-essais
```

```bash
git checkout "${COMMIT}"
```

### 8. Droit de Cloud Build sur le projet

```bash
export NUMERO="$(gcloud projects describe "${PROJET}" --format='value(projectNumber)')"
```

```bash
gcloud projects add-iam-policy-binding "${PROJET}" --member="serviceAccount:${NUMERO}-compute@developer.gserviceaccount.com" --role=roles/cloudbuild.builds.builder
```

### 9. Construction de l'image

```bash
gcloud builds submit --config=essais/cloudbuild.yaml --substitutions=_IMAGE="${IMAGE}" .
```

**9 bis, seulement si l'étape 9 échoue** : construction dans Cloud Shell, qui a Docker.

```bash
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet
```

```bash
docker build -f essais/Dockerfile -t "${IMAGE}" .
```

```bash
docker push "${IMAGE}"
```

### 10. Empreinte de l'image

Les VM lancent l'image par son empreinte, que porte le jeton d'attestation (`image_digest`).

```bash
export EMPREINTE="$(gcloud artifacts docker images describe "${IMAGE}" --format='value(image_summary.digest)')"
```

```bash
export IMAGE_REF="${REGION}-docker.pkg.dev/${PROJET}/essais/essai@${EMPREINTE}"
```

```bash
echo "${IMAGE_REF}"
```

La ligne doit finir par `@sha256:` suivi de 64 caractères hexadécimaux.

### 11. Compte de service des VM

```bash
gcloud iam service-accounts create essai-vm --display-name="VM des essais"
```

### 12. Droits minimaux du compte

Jeton d'attestation, tirage de l'image, journaux, lecture et écriture de Firestore ; rien d'autre.

```bash
for ROLE in roles/confidentialcomputing.workloadUser roles/artifactregistry.reader roles/logging.logWriter roles/datastore.user; do gcloud projects add-iam-policy-binding "${PROJET}" --member="serviceAccount:${COMPTE}" --role="${ROLE}" --format=none; done
```

### 13. Base Firestore de l'essai

```bash
gcloud firestore databases create --database=essai --location="${REGION}"
```

### 14. Réseau de l'essai

Un réseau à part, sans règle par défaut.

```bash
gcloud compute networks create essais --subnet-mode=custom
```

### 15. Sous-réseau, accès privé aux API de Google

Les VM n'ont pas d'adresse IP publique ; elles joignent Artifact Registry, Firestore et le service d'attestation par l'accès privé à Google.

```bash
gcloud compute networks subnets create essais --network=essais --region="${REGION}" --range=10.20.0.0/24 --enable-private-ip-google-access
```

### 16. Pare-feu : le port 8080 pour le seul tunnel IAP

```bash
gcloud compute firewall-rules create essais-iap-8080 --network=essais --direction=INGRESS --allow=tcp:8080 --source-ranges=35.235.240.0/20
```

### 17. Trois VM Confidential Space, une par zone

Image de **production** pour les trois essais : le jeton doit porter `dbgstat` `disabled-since-boot`, que l'image de débogage n'a pas. L'image de débogage (`--image-family=confidential-space-debug`) ne servirait qu'à comprendre un conteneur qui ne démarre pas, jamais pour l'essai du jeton.

```bash
for Z in a b c; do gcloud compute instances create "essai-${Z}" --zone="${REGION}-${Z}" --machine-type=n2d-highcpu-2 --confidential-compute-type=SEV --maintenance-policy=MIGRATE --shielded-secure-boot --image-project=confidential-space-images --image-family=confidential-space --network=essais --subnet=essais --no-address --service-account="${COMPTE}" --scopes=cloud-platform --metadata="tee-image-reference=${IMAGE_REF},tee-container-log-redirect=true"; done
```

### 18. Démarrage du conteneur

Attendez 2 à 3 minutes, puis cherchez la ligne `essai : écoute sur :8080` (journal du conteneur recopié sur la sortie série).

```bash
for Z in a b c; do echo "== essai-${Z}"; gcloud compute instances get-serial-port-output "essai-${Z}" --zone="${REGION}-${Z}" 2>/dev/null | grep -E "essai :|horloge :|rror" | tail -5; done
```

### 19. Dossier des relevés

```bash
mkdir -p ~/resultats-essais
```

```bash
cd ~/resultats-essais
```

### 20. Tunnels IAP (ports 8081, 8082, 8083 de Cloud Shell)

```bash
P=8081; for Z in a b c; do gcloud compute start-iap-tunnel "essai-${Z}" 8080 --local-host-port="localhost:${P}" --zone="${REGION}-${Z}" > "tunnel-${Z}.log" 2>&1 & P=$((P+1)); done; sleep 10
```

```bash
for P in 8081 8082 8083; do curl -sS "http://localhost:${P}/"; done
```

### 21. Relevés

#### Horloge, avant la maintenance

```bash
for P in 8081 8082 8083; do curl -sS "http://localhost:${P}/horloge?mesures=32" > "horloge-${P}-avant.json"; curl -sS "http://localhost:${P}/horloge?mesures=32&reference=ip" > "horloge-${P}-avant-ip.json"; done
```

```bash
for Z in a b c; do gcloud compute instances describe "essai-${Z}" --zone="${REGION}-${Z}" --format='value(status,lastStartTimestamp)' > "etat-${Z}-avant.txt"; done
```

#### Jeton d'attestation

Clés d'essai aléatoires, sans valeur : le jeton lie leurs empreintes (`eat_nonce`, `ARCHITECTURE.md` §3) ; gardez les deux fichiers, la vérification en a besoin.

```bash
head -c 32 /dev/urandom > sessionPub.bin
```

```bash
head -c 1184 /dev/urandom > kemPub.bin
```

```bash
export NONCE_SESSION="$(openssl dgst -sha256 -binary sessionPub.bin | basenc --base64url | tr -d '=')"
```

```bash
export NONCE_KEM="$(openssl dgst -sha256 -binary kemPub.bin | basenc --base64url | tr -d '=')"
```

```bash
curl -sS -G "http://localhost:8081/attestation" --data-urlencode "audience=Fatoom-Heritage-Numerique/sesame" --data-urlencode "nonce=${NONCE_SESSION}" --data-urlencode "nonce=${NONCE_KEM}" > attestation-a.json
```

```bash
gcloud projects describe "${PROJET}" --format='value(projectNumber)' > numero-projet.txt
```

```bash
echo "${IMAGE_REF}" > image.txt
```

Le jeton vaut environ une heure : la vérification par le client se fait dans l'heure, ou à l'heure du relevé (`attestation-a.json` est daté par son fichier).

#### Débit de `chaine/tete`, concurrence par défaut

Trois clients ensemble, à débit croissant (`debit` est par client : 3 × `debit` en tout), une minute par palier, environ 5 minutes.

```bash
gcloud firestore databases describe --database=essai --format='value(concurrencyMode)' > firestore-mode-1.txt
```

```bash
for D in 2 5 10 20 40; do for P in 8081 8082 8083; do curl -sS "http://localhost:${P}/firestore?duree=60&debit=${D}" > "firestore-1-${D}-${P}.json" & done; wait; done
```

#### Débit de `chaine/tete`, concurrence optimiste

La spécification compte sur la concurrence optimiste de Firestore ; une base `standard` est en concurrence pessimiste par défaut. Passage en `OPTIMISTIC`, puis les mêmes paliers.

```bash
curl -sS -X PATCH -H "Authorization: Bearer $(gcloud auth print-access-token)" -H "Content-Type: application/json" "https://firestore.googleapis.com/v1/projects/${PROJET}/databases/essai?updateMask=concurrencyMode" -d '{"concurrencyMode":"OPTIMISTIC"}' > firestore-passage-optimiste.json
```

Attendez une minute, puis vérifiez que le mode a changé :

```bash
gcloud firestore databases describe --database=essai --format='value(concurrencyMode)' > firestore-mode-2.txt; cat firestore-mode-2.txt
```

```bash
for D in 2 5 10 20 40; do for P in 8081 8082 8083; do curl -sS "http://localhost:${P}/firestore?duree=60&debit=${D}" > "firestore-2-${D}-${P}.json" & done; wait; done
```

### 22. Maintenance simulée sur `essai-a`

```bash
gcloud compute instances simulate-maintenance-event essai-a --zone="${REGION}-a"
```

```bash
gcloud compute instances describe essai-a --zone="${REGION}-a" --format='value(status,lastStartTimestamp)' > etat-a-apres.txt; cat etat-a-avant.txt etat-a-apres.txt
```

Même `lastStartTimestamp` avant et après : la VM a migré sans redémarrer. Le tunnel de `essai-a` a pu tomber ; relancez-le :

```bash
pkill -f "start-iap-tunnel essai-a"; gcloud compute start-iap-tunnel essai-a 8080 --local-host-port=localhost:8081 --zone="${REGION}-a" > tunnel-a-2.log 2>&1 & sleep 10
```

### 23. Horloge, juste après la maintenance

```bash
curl -sS "http://localhost:8081/horloge?mesures=32" > horloge-8081-apres.json
```

### 24. Horloge, 30 minutes après

Attendez 30 minutes (les VM restent facturées), puis :

```bash
for P in 8081 8082 8083; do curl -sS "http://localhost:${P}/horloge?mesures=32" > "horloge-${P}-apres-30min.json"; done
```

Mesures écrites chaque minute par les trois VM pendant la séance (dérive pendant la migration) :

```bash
gcloud logging read '"horloge :"' --freshness=3h --format=json > journal-horloge.json; wc -c journal-horloge.json
```

### 25. Fermeture des tunnels

```bash
pkill -f start-iap-tunnel
```

### 26. Fichiers à renvoyer

```bash
cd ~ && tar czf resultats-essais.tar.gz resultats-essais
```

```bash
cloudshell download resultats-essais.tar.gz
```

L'archive contient les relevés JSON, le jeton (public par nature : Sésame le sert à chacun), les deux clés d'essai, l'empreinte et le chemin de l'image et le numéro du projet d'essai, qui ne valent que pour ce projet jetable. Elle ne contient aucun secret.

### 27. Suppression du projet

Arrête toute facturation du projet (VM, base, images) ; Google garde le projet 30 jours en suppression, sans rien facturer.

```bash
gcloud projects delete "${PROJET}"
```

Le budget d'alerte reste sur le compte de facturation ; pour le retirer :

```bash
gcloud billing budgets list --billing-account="${FACTURATION}" --filter="displayName=essais-confidential-space" --format='value(name)'
```

```bash
gcloud billing budgets delete "<nom rendu par la commande précédente>"
```
