<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), fichiers de Sésame : « principe et modèle de menace », « l'enclave », « registre, journaux et surveillance des clés », « Protocole de PIN », « Clés et formats », « Protocoles », « Versions et alertes » -->

# Architecture de Sésame

Sésame est un service de garde de clés de transmission : il scelle, pour chaque titulaire d'un coffre-fort chiffré de bout en bout, une clé de transmission, et il ne la rouvre que dans trois cas vérifiés par lui seul : la transmission après un décès dont il tient lui-même le compte des 7 jours sans annulation, la transmission par inactivité dont il mesure aussi le délai, ou l'ouverture par le contact de confiance qui a prouvé auprès de lui son code d'accès et son second facteur. Il tourne dans une enclave Google Cloud Confidential Space, au code public, et les clés qu'il rouvre repartent chiffrées pour le navigateur du proche.

Ce document décrit le service indépendamment de son client. Fatoom est le premier client ; quand une valeur lui est propre, elle est donnée en exemple et signalée comme telle. La spécification de référence est tenue par l'opérateur ; ce dépôt en est la reprise publique et fait foi pour le code. Les routes, leurs corps et leurs réponses sont décrits dans `API.md` ; les adversaires et les limites assumées dans `THREAT_MODEL.md` ; la publication d'une version dans `RELEASE.md`.

Conventions : « app » = le téléphone du titulaire, coffre déverrouillé ; « portail » = le navigateur d'un proche ; « client » = le service qui range les objets chiffrés et les transporte (pour Fatoom : ses Cloud Functions et sa base) ; « passerelle » = les routes que le client expose à Sésame (`API.md`) ; « Sésame » = l'enclave, jointe après attestation vérifiée et par le canal chiffré du §5 ; « exploitant » = qui administre le projet cloud de Sésame (pour Fatoom : l'opérateur de Fatoom lui-même, un seul administrateur).

## 1. Principe et vocabulaire

- **Le téléphone décide de tout.** Le coffre déverrouillé produit lui-même les clés chiffrées pour les proches, les droits de chaque proche, la preuve que le titulaire est vivant et l'annulation d'un décès signalé. Le client range et transporte des objets qu'il ne sait ni lire ni fabriquer.
- **Sésame ouvre, et seulement lui.** Il n'ouvre une clé que dans trois cas : transmission après un décès dont il tient lui-même le compte des 7 jours sans annulation ; transmission par inactivité (signe de vie, §11.4) dont il mesure aussi le délai ; ouverture par le contact de confiance qui a prouvé auprès de Sésame son code d'accès et son second facteur. Les clés ouvertes repartent chiffrées pour le navigateur du proche ; le client ne les voit pas.
- **Ce qui n'est destiné à personne ne traverse jamais Sésame** : aucune enveloppe n'existe pour un contenu « gardé pour vous ».
- **Rien ne coûte à l'utilisateur.** Destiner un contenu, changer ses destinataires, créer un proche, créer, modifier ou afficher un mot de passe sont des opérations locales ou un simple appel au client, sans appel à Sésame. Sésame n'est joint qu'à la création du coffre (enrôlement et dépôt du PIN), à l'ouverture du coffre sur un nouvel appareil et au changement de PIN (protocole de PIN, §9), en arrière-plan pour la preuve de vie (au plus une fois par jour) et la vérification quotidienne, pour annuler un décès signalé, lors d'un changement de droits d'un proche (révocation, rôle, coordonnées : rare) ou de la clé de transmission, sur le portail et pour la transmission.

| Terme | Définition |
|---|---|
| **Sésame** | Le programme (ce dépôt) et son enclave. Projet cloud distinct de celui du client (pour Fatoom : un projet dédié, dont le nom n'est pas publié ; `<projet-sesame>` dans ce dépôt), joignable directement par l'app et le portail (pour Fatoom : `sesame.fatoom.fr`). |
| **Empreinte** | `sha256` de l'image de conteneur de Sésame (`image_digest`). C'est elle qui est publiée, attestée et autorisée dans KMS. |
| **Attestation** | Jeton signé par Google pour une enclave Confidential Space, portant l'empreinte de l'image en cours d'exécution et les nonces choisis par Sésame (§3). |
| **Liste des versions** | `versions.json` de ce dépôt (empreintes, matériel admis, dates, statut), compilée dans l'app, le portail et la passerelle (§11.6, `RELEASE.md`). |
| **KTS** (clé de transmission du titulaire) | 32 octets tirés sur le téléphone, protégés par la clé maître du coffre (KMU), confiés une fois à Sésame qui les scelle sous KMS. Tout ce que le titulaire destine est chiffré sous une clé dérivée de KTS. |
| **Clé de vie** (`cleVie`) | Paire Ed25519 dont la graine est tirée au hasard sur le téléphone et gardée sous la KMU. Elle signe la preuve de vie, l'annulation, la publication de génération, la preuve d'enrôlement, les dépôts du PIN et la fiche d'alerte. Sésame n'en connaît que la clé publique (`cleViePub`) et ne peut pas signer à la place du titulaire (§10.2). |
| **Enveloppe** | La clé d'un contenu (clé de fichier, de mot de passe, d'un mot pour un proche ou des dernières volontés) chiffrée sous KTS pour **un** proche, liée au contenu et au proche (§10.3). Rangée par le client à côté du contenu. |
| **Charte** | Les droits d'un proche (rôle, code d'accès, coordonnées du second facteur, identité, adresse, génération), chiffrés et authentifiés sous KTS par le téléphone (§10.4). Seuls le téléphone et Sésame peuvent la lire ou l'écrire. |
| **Registre** | Base minimale tenue par Sésame dans son propre projet, en ajout seul : signalements (décès et inactivité), annulations, preuves de vie, génération courante des chartes et de KTS, clé publique de vie, grappe du dernier dépôt du PIN, fiche d'alerte scellée (§6). Aucune clé secrète, aucun contenu. |
| **Borne du registre** | Fichier déposé toutes les 10 minutes par chaque instance dans le bucket d'audit verrouillé, signé par la clé KMS `session-sign` : époque et empreinte de la chaîne du registre, identité de la grappe et événements. Sert au contrôle anti-retour du registre et au script public (§8). |
| **Défi** | Nonce aléatoire de 32 octets tiré par Sésame à la demande de l'app, à usage unique, valable 5 minutes, lié au canal ; à inclure dans une preuve de vie, une annulation, une preuve d'enrôlement ou un dépôt du PIN (§10.5). |
| **Protocole de PIN** | Vérification du PIN d'un nouvel appareil par Sésame, avec un compteur de 10 essais, qui permet à l'app de rouvrir la copie serveur de la KMU. Tout ce qui sert à vérifier le PIN vit en mémoire des trois VM de la grappe, jamais ailleurs (§9). |
| **Preuve de vie** | Message signé par `cleVie`, en réponse à un défi, envoyé par l'app à chaque déverrouillage (au plus une fois par jour, ou immédiatement pendant une relance du signe de vie) : « ce titulaire est vivant ». Remet à zéro le délai d'inactivité, annule un signalement d'inactivité et coupe les ouvertures après une transmission par inactivité. **N'annule pas un décès signalé.** |
| **Annulation** | Message signé par `cleVie`, lié à un signalement précis et à un défi, envoyé quand le titulaire touche « Je suis en vie, annuler la transmission » : seul moyen d'annuler un décès signalé, depuis n'importe quel appareil où il ouvre son coffre (§10.5). |
| **Fiche d'alerte** | Téléphone et email du titulaire, signés par `cleVie`, scellés par Sésame sous la clé KMS `alerte-seal` et gardés dans son registre : Sésame s'en sert pour alerter le titulaire sans passer par le client (§11.10). |
| **Jeton de session** | Jeton signé par Sésame (clé KMS P-256) remis au navigateur d'un proche authentifié ; le client le vérifie pour lister les contenus et émettre les URL signées (§11.2). |
| **Code d'invitation** | Terme technique ; l'interface, la lettre et le portail disent « code d'accès ». Code remis par lettre au proche sans téléphone ni email : tiré par Sésame, imprimé et posté par Sésame, connu de lui seul ; ouvre le portail après une vérification d'identité menée par Sésame (§11.9). |
| **Scellé KTS** | `sealedKts` : KTS chiffrée par la clé KMS `kts-seal` de Sésame (AES-256, HSM), rangée par le client, inutilisable sans l'enclave attestée. |

## 2. L'enclave

### 2.1 Confidential Space

Sésame est un conteneur OCI exécuté par l'image **Confidential Space** de Google (famille `confidential-space`, jamais `confidential-space-debug`) dans le projet cloud de Sésame, distinct de celui du client, région `europe-west9` (Paris).

- **Matériel en service (option B)** : VM confidentielles **AMD SEV**, trois VM `n2d-highcpu-2` (2 vCPU, 2 Go), **une par zone** (`europe-west9-a`, `-b`, `-c`), chacune seule dans son groupe d'instances zonal, maintenance `MIGRATE` (migration à chaud, sans arrêt), sans adresse IP publique : les VM ne sont joignables que par le répartiteur régional et sortent par Cloud NAT. Confidential Space n'accepte que AMD SEV et Intel TDX ; **SEV-SNP n'est pas utilisé** (il n'existe pas à Paris). SEV chiffre la mémoire mais ne protège pas son intégrité contre qui contrôle l'hyperviseur (`THREAT_MODEL.md`, limite 1).
- **Option A (montée en gamme)** : Intel TDX (`c3-standard-4`, zones `-a` et `-b` seulement, maintenance `TERMINATE` : arrêt à chaque maintenance), qui protège aussi l'intégrité de la mémoire. `GCP_AMD_SEV` et `GCP_INTEL_TDX` sont admis **dès la première version** (liste des versions, condition KMS) : le passage de B à A remplace les VM une à la fois, sans nouvelle version ni redépôt ; `GCP_AMD_SEV` est retiré ensuite par une version suivante. Avec deux zones TDX seulement, une zone porte deux VM et sa perte arrête la grappe ; le passage à cinq VM se réexamine au passage à l'option A, comme une troisième zone TDX si Google en ouvre une (§9.3).
- Le lanceur de Confidential Space mesure l'image, démarre le conteneur et expose le service d'attestation sur une socket locale (`/run/container_launcher/teeserver.sock`). Le conteneur n'a ni shell, ni accès SSH, ni port de débogage ; `dbgstat` vaut `disabled-since-boot`. Le conteneur dispose d'environ 1,38 Gio sur les 2 Go de la VM.
- **Politique de lancement**, inscrite dans l'image par ses étiquettes : `tee.launch_policy.allow_env_override` = la liste exacte des variables du §2.3 (une variable non listée fait échouer le lancement), `tee.launch_policy.allow_cmd_override=false`, `tee.launch_policy.log_redirect=never`, surveillance de la mémoire interdite (`tee.launch_policy.hardened_monitoring=none` et `tee.launch_policy.debug_monitoring=none`), aucune capacité ni point de montage ajouté ; les ports servis sont déclarés par `EXPOSE`. Variable non listée, commande, redirection des journaux ou surveillance de la mémoire font échouer le lancement et arrêtent la VM.
- Le **compte de service attaché à la VM n'est pas attesté** (l'exploitant le choisit et peut s'en servir hors de l'enclave) : il n'a que `roles/confidentialcomputing.workloadUser`, la lecture d'Artifact Registry et l'écriture des journaux. Tout droit sensible (KMS, Secret Manager, registre, écriture des bornes et des marqueurs d'inscription) passe par l'identité fédérée conditionnée à l'attestation (§4).
- L'attribut `STABLE` d'une image Confidential Space dure environ 6 mois et peut être retiré plus tôt en cas de faille : les VM sont recréées sur l'image Confidential Space courante au moins tous les 6 mois, une à la fois (§9).

### 2.2 Code minimal et réseau sortant

Le code de Sésame est **minimal** : un serveur HTTPS, un client KMS, un client Firestore (registre), un client Cloud Storage (bornes et marqueurs d'inscription), un client HTTP vers la passerelle du client, des clients REST sans SDK (Twilio Verify, Twilio Messaging et SendGrid pour les alertes, Stripe Identity, Pingen : quelques appels chacun), la grappe du protocole de PIN (§9), un générateur de lettre PDF texte d'une page sans dépendance (§11.9) et les primitives cryptographiques de la plateforme.

**Budget** : de l'ordre de 5 000 à 6 000 lignes de Go visées, **plafond dur de 6 000 lignes** hors tests et hors bibliothèques épinglées (Raft, §9) ; la CI de ce dépôt échoue au-delà. Répartition estimée : socle (attestation, canal, KMS, registre, sessions, OPRF) 2 000 à 3 000 lignes ; grappe 1 000 à 1 500 ; alertes et lettre environ 800 ; borne du registre et clients REST environ 500. Le chiffre réel est remesuré sur le Sésame factice, qui partage le code des décisions, puis sur l'image, et remplace alors cette estimation.

**Réseau sortant** : la politique de la VM n'autorise que KMS, Secret Manager, Firestore, Cloud Storage (bucket d'audit et bucket des marqueurs), le service d'attestation, les clés publiques des émetteurs de jetons que Sésame vérifie (pour Fatoom : Firebase Auth pour l'`idToken`, Google pour le jeton OIDC du compte de service), la passerelle du client, les autres VM de la grappe, Twilio, SendGrid, Stripe et Pingen. Aucune autre dépendance réseau.

**Heure** : l'horloge des VM vient du serveur de métadonnées de Google (`metadata.google.internal`, NTP), que l'image Confidential Space interroge d'elle-même ; c'est un lien local, hors du VPC, que le pare-feu et Cloud NAT ne voient pas : il ne doit jamais être bloqué. La dérive est mesurée sur les VM d'un essai, avant et après une migration à chaud, à l'étape du contrat exécutable (`README.md`, « État du projet »), pour tenir le ± 2 min du contrôle de fraîcheur (§6.3) et l'échéance des signalements (`THREAT_MODEL.md`, limite 10).

### 2.3 Fournisseurs et réglages

Les fournisseurs (SMS et email, identité, courrier) sont appelés par leur API dans le code public : leurs adresses de base y sont écrites en dur, jamais lues dans une variable au démarrage, car une adresse modifiable permettrait à qui contrôle la VM de détourner l'envoi du dernier code vers son propre serveur.

Ce qui est propre au client va :

- dans des **variables d'environnement** passées par les métadonnées `tee-env-*`, limitées à des identifiants et adresses courts : identifiant du service de vérification, identifiant d'organisation du courrier, nom et adresse de l'expéditeur, adresse de la passerelle, `SESAME_AUDIENCE` (adresse publique de Sésame, audience attendue des jetons OIDC), `SMS_EXPEDITEUR` (identifiant alphanumérique ou numéro qui envoie les SMS), identifiant du projet émetteur des `idToken` du client, adresse du compte de service du client autorisé à signaler un décès, adresse du compte d'exploitation de Sésame autorisé à amorcer une grappe neuve ;
- dans des **fichiers de l'image** pour les textes longs (SMS, alertes, gabarits de lettre), attestés par l'empreinte de l'image ; la condition KMS est bornée à 4 096 caractères et ne peut pas porter ces textes (§4) ;
- dans **Secret Manager** pour les clés d'API (Twilio, SendGrid, Stripe, Pingen).

Les variables figurent dans le jeton d'attestation, seules, dans `submods.container.env_override` (`submods.container.env` contient en plus `HOSTNAME` et les `ENV` de l'image, différents d'une VM à l'autre). Sésame refuse de démarrer si l'une d'elles manque ou est vide, et la liste des versions publie `SHA-256` du JSON canonique de `env_override` (`envDigest`), que les clients comparent au jeton : un expéditeur ou une adresse de passerelle ne se change donc pas en douce. Toute modification d'une de ces variables change l'`envDigest` : elle passe par la publication d'une nouvelle version. Ce dépôt ne contient ni identifiant de compte, ni texte de marque.

### 2.4 Image reproductible et dépôt public

- Ce dépôt contient le code, le `Dockerfile`, le script de build reproductible, la liste des versions (`versions.json`), les vecteurs de test recopiés depuis le client (pour Fatoom : `packages/crypto/vectors/` du dépôt `Fatoom-Heritage-Numerique/Fatoom`) et la procédure de vérification en 4 étapes (`README.md`).
- Build reproductible : binaire statique (`-trimpath`, dates figées par `SOURCE_DATE_EPOCH`), image de base `gcr.io/distroless/static-debian12` épinglée par digest, dépendances verrouillées par somme de contrôle, construction par Dockerfile multi-étapes dans un conteneur de build lui-même épinglé. La CI construit l'image deux fois sur deux exécuteurs et échoue si les digests diffèrent.
- Langage : **Go** (dernière version stable épinglée dans `go.mod` et dans la CI), module unique. Cryptographie : bibliothèque standard (`crypto/ecdh`, `crypto/aes`, `crypto/ed25519`, `crypto/hkdf`, `crypto/mlkem`, et, pour la vérification de l'attestation et des signatures du registre, `crypto/ecdsa`, `crypto/rsa` et `crypto/x509`) ; **deux exceptions** : `github.com/cloudflare/circl` pour l'OPRF du protocole de PIN, épinglée (version et paquet fixés à l'étude du lot 1 du client), et `golang.org/x/crypto/argon2` (extension officielle de Go) pour l'Argon2id de l'index des lettres d'invitation, épinglée. Consensus : `go.etcd.io/raft/v3`, épinglée (§9). SDK Google officiels (KMS, Firestore, Secret Manager, Cloud Storage) ; `net/http` brut pour les fournisseurs.
- Chaque version porte un tag Git signé (`vX.Y.Z`, signé par la clé SSH GitHub du porteur du produit) et une ligne de `versions.json` (`RELEASE.md`).
- Tant qu'il n'y a qu'un administrateur, une PR qui touche une zone critique cite une décision écrite et reste publique au moins 7 jours avant sa fusion (`CONTRIBUTING.md`).
- **Tests sans enclave** : un lanceur factice (`SESAME_FAKE_LAUNCHER=1`, jamais dans l'image publiée : le build échoue si la variable est lue hors du paquet de test) émet un jeton signé par une clé de test avec les claims choisis par le test ; KMS, Twilio et Firestore sont remplacés par des doubles en mémoire. Le même binaire tourne en trois exemplaires (`docker compose`) pour la grappe du §9. Côté client, le crochet qui injecte une clé de test pour vérifier ces jetons est absent du code déployé (pour Fatoom : `SESAME_TEST_JWKS`, contrôlé en CI).

## 3. Attestation : jeton, claims et vérification

Sésame demande au lanceur (`POST /v1/token` sur la socket locale) un jeton d'attestation **de type PKI** (`token_type: PKI`, émetteur `https://confidentialcomputing.googleapis.com`), signé par une clé dont le certificat remonte, par l'en-tête `x5c`, à la racine Confidential Space de Google (`confidential_space_root.crt`, publiée à `https://confidentialcomputing.googleapis.com/.well-known/confidential_space_root.crt` ; `CN=Confidential Space Root CA`, valide du 2024-01-19 au 2034-01-16, empreinte SHA-256 `148B293821BB0C6A317F413C8BA475814091CB22D49B9E3C94198DB8E8F86C39` ; chaîne feuille, intermédiaire, racine, `alg` `RS256`, la racine en dernier dans `x5c`). L'app, le portail et la passerelle **épinglent cette racine** par son empreinte et ne téléchargent aucune clé. Google la renouvelle tous les 10 ans : la racine suivante entre dans les clients par une version, avant l'échéance.

Le jeton porte :

- `audience` = `Fatoom-Heritage-Numerique/sesame` ;
- `eat_nonce` = deux valeurs : `base64url(SHA-256(sessionPub))`, où `sessionPub` est la clé publique X25519 de session de l'instance, et `base64url(SHA-256(kemPub))`, où `kemPub` est sa clé publique ML-KEM-768 (§5). Le jeton lie donc les deux clés du canal à l'enclave attestée.

Il est renouvelé toutes les 10 min (validité du jeton 1 h) et servi à chaque client sur `GET /attestation`, qui renvoie `{jwt, sessionPub, kemPub, instanceId}`.

Un client (app, portail, passerelle) **n'accepte** le jeton que si toutes les conditions suivantes sont vraies :

| Claim | Valeur exigée |
|---|---|
| signature | chaîne `x5c` valide jusqu'à la racine Confidential Space épinglée |
| `iss` | `https://confidentialcomputing.googleapis.com` |
| `aud` | `Fatoom-Heritage-Numerique/sesame` |
| `exp`, `iat` | `iat ≤ now + 5 min`, `exp > now` |
| `eat_nonce` | chaîne ou tableau (les deux formes sont admises) ; contient exactement `base64url(SHA-256(sessionPub))` et `base64url(SHA-256(kemPub))` pour les clés reçues ; au plus 6 valeurs de 10 à 74 octets |
| `swname` | `CONFIDENTIAL_SPACE` (vaut aussi pour l'image de débogage : `dbgstat` et `STABLE` l'excluent) |
| `submods.confidential_space.support_attributes` | contient `STABLE` |
| `hwmodel` | une des valeurs `hwmodels` de cette version dans la liste des versions : `GCP_AMD_SEV` (option B, en service) ou `GCP_INTEL_TDX` (option A) |
| `submods.gce.project_number` | numéro du projet de Sésame, sous forme de **chaîne** (sans cette clause, l'image publiée lancée dans un autre projet serait acceptée) |
| `dbgstat` | `disabled-since-boot` |
| `secboot` | `true` |
| `submods.container.image_digest` | figure dans la liste des versions compilée, statut `en_service` (§11.6) |
| `submods.container.image_reference` | commence par le préfixe du dépôt d'images de l'exploitant, gravé dans les clients (pour Fatoom : `europe-west9-docker.pkg.dev/<projet-sesame>/sesame/`) |
| `submods.container.env_override` | `SHA-256` du JSON canonique de `env_override` égal à l'`envDigest` de la version (§2.3) |

Toute condition fausse ⇒ aucune donnée n'est envoyée ; le client affiche un échec d'attestation (§11.7). Les clients ne vérifient rien d'autre que l'attestation : aucune politique publiée par Sésame n'entre dans la décision.

## 4. Libération des clés : Cloud KMS conditionné à l'attestation

Deux options ont été comparées : une clé privée scellée et gardée en mémoire de l'enclave (plus rapide, mais une faille mémoire expose toutes les enveloppes de tous les titulaires jusqu'à rotation), et une clé KMS HSM que l'enclave ne détient jamais. **Décision : KMS HSM, avec une clé de transmission par titulaire** (§10.2), ce qui ne change rien pour l'utilisateur et borne l'exposition d'une faille mémoire aux titulaires ouverts pendant la session.

Dans le projet de Sésame, trousseau `sesame`, même région que l'enclave :

| Clé | Type | Usage |
|---|---|---|
| `kts-seal` | symétrique AES-256-GCM, niveau de protection **HSM**, non exportable, rotation annuelle (les anciennes versions restent déchiffrables) | Scelle et descelle KTS (`sealedKts`, AAD = `uid ‖ "\|" ‖ ktsGeneration`) |
| `session-sign` | asymétrique EC P-256 (ECDSA SHA-256), HSM | Signe les jetons de session du portail, les reçus, les réponses de `/etat`, les requêtes à la passerelle et les bornes du registre ; la clé publique `session-sign` est lue par `kid` dans `clesSession` de la liste des versions compilée (§11.6) |
| `registre-sign` | asymétrique EC P-256 (ECDSA SHA-256), HSM | Signe chaque ligne du registre à l'écriture (`sig`, §6) ; vérification **locale** avec les clés publiques lues dans KMS au démarrage ; aucun secret du registre en mémoire |
| `alerte-seal` | symétrique AES-256-GCM, HSM, non exportable, rotation annuelle | Scelle la fiche d'alerte du titulaire, gardée au registre (AAD `"fatoom-sesame-alerte\|1\|" ‖ uid ‖ "\|" ‖ génération`, §11.10) |

L'AAD de `sealedKts` contient l'`uid` en clair : un essai a vérifié que l'entrée de journal d'un `Decrypt` KMS ne porte ni l'AAD ni le chiffré, seulement le nom de la clé.

**Accès** : un **pool d'identités de charge de travail** (`sesame-pool`, fournisseur OIDC Confidential Space). Le **fournisseur** du pool porte toute la **condition d'attribut**, qui décide quelles charges obtiennent l'identité munie de `roles/cloudkms.cryptoKeyEncrypterDecrypter` (et `cryptoKeySignerVerifier` pour `session-sign` et `registre-sign`) :

```
assertion.swname == "CONFIDENTIAL_SPACE"
&& "STABLE" in assertion.submods.confidential_space.support_attributes
&& assertion.dbgstat == "disabled-since-boot"
&& assertion.submods.container.image_digest in ["sha256:<empreinte publiée>", …]
&& assertion.submods.container.env_override == {"<VARIABLE>": "<valeur publiée>", …}   # tout le dictionnaire, une seule clause
&& assertion.submods.gce.project_number == "<numéro du projet de Sésame>"
&& assertion.hwmodel in ["GCP_AMD_SEV", "GCP_INTEL_TDX"]                              # valeurs hwmodels de la liste des versions
```

- Mappage `google.subject = assertion.sub` (URL complète de la VM, qui la désigne par son **nom** ; 127 octets au plus, d'où des noms de VM de 29 caractères au plus). La liaison `roles/iam.workloadIdentityUser` du compte de service de la charge de travail (`<compte-charge>`) vise ce seul pool ; aucune liaison `principalSet://…/*` ni `principal://…` n'existe sur les clés ou le compte de service.
- **`env_override` contraint en entier** : une clause par variable ne suffit pas (la même image lancée avec une variable de plus, listée par la politique de lancement mais absente de la condition, a déchiffré la clé à l'essai). L'égalité de tout le dictionnaire refuse une valeur changée comme une variable ajoutée ou retirée, et correspond mot pour mot à l'ensemble dont `envDigest` est l'empreinte. Le langage des conditions n'a pas de fonction de hachage ; la condition est bornée à 4 096 caractères, d'où les textes longs dans l'image (§2.3).
- Une image publiée lancée en mode débogage, avec d'autres réglages, sur un autre matériel ou dans un autre projet ne descelle rien, même avant que les clients refusent son attestation.
- Une modification de la condition a été vue en vigueur environ 4 minutes après l'écriture ; Google annonce « typiquement 2 minutes, parfois 7 minutes ou plus » pour l'IAM : une révocation n'est tenue pour effective qu'après au moins 7 minutes et une vérification.

La liste des empreintes autorisées dans la condition est **identique** à celle des versions `en_service` de `versions.json` ; un contrôle CI de ce dépôt compare les deux, ainsi que `dbgstat` et le dictionnaire `env_override` à l'`envDigest` publié (`RELEASE.md`). Aucun compte humain n'a `cryptoKeyEncrypterDecrypter` sur `kts-seal` ni sur `alerte-seal` : seule la charge attestée descelle.

**Aucune version importée.** Cloud KMS permet d'importer du matériel de clé dans une nouvelle version d'une clé existante ; une version de `registre-sign` ou de `session-sign` dont l'administrateur tiendrait la clé privée signerait des lignes du registre et des bornes hors de l'enclave. Sésame lit au démarrage la liste des versions de ces deux clés et **refuse de servir** si l'une d'elles porte `importJob` (échec fermé, journalisé). Des politiques de refus interdisent `cloudkms.importJobs.create` et `cloudkms.cryptoKeyVersions.import` à tout principal, et le script public signale `CreateImportJob`, `ImportCryptoKeyVersion` et `CreateCryptoKeyVersion` sur le trousseau (§8.3).

**Secrets des fournisseurs** : Twilio, SendGrid, Stripe (clé **restreinte** aux seules fonctions Identity) et Pingen (**compte** Pingen distinct de celui du client, pas seulement une organisation : un accès API voit tout son compte, et le client ne doit pas pouvoir télécharger une lettre) sont dans Secret Manager du même projet, avec la même condition d'attribut.

**Identité de la charge de travail** : le compte de service `<compte-charge>` n'est utilisé qu'à travers le pool (usurpation conditionnée à l'attestation) ; il a `datastore.user` sur le registre, `secretmanager.secretAccessor` sur les secrets des fournisseurs, `storage.objectCreator` limité au préfixe `bornes/` du bucket d'audit (§8), `storage.objectCreator` et `storage.objectViewer` sur le bucket privé des marqueurs d'inscription (§6.2), et rien d'autre. Sésame ne lit aucune politique IAM.

## 5. Clé de session et canal chiffré

Au démarrage, chaque instance tire, en mémoire seulement, une paire X25519 **de session** (`sessionPriv`, `sessionPub`) et une paire **ML-KEM-768** (`kemPriv`, `kemPub`), et inscrit les deux empreintes dans le nonce de son attestation (§3). Un client :

1. `GET /attestation` → `{jwt, sessionPub, kemPub, instanceId}` ; vérifie le jeton (§3) et que `eat_nonce` contient `SHA-256(sessionPub)` et `SHA-256(kemPub)`. Le préfixe de la VM (`a`, `b` ou `c`) se déduit d'`instanceId` (`API.md`, §1.2).
2. Tire sa propre paire X25519 (`cliPriv`, `cliPub`), calcule `shared = X25519(cliPriv, sessionPub)` (refusé s'il est entièrement nul), encapsule vers `kemPub` (`kemCt`, `ssKem`) et dérive **une clé par sens** :
   ```
   K_req = HKDF-SHA256(ikm = ssKem ‖ shared, salt = sessionPub ‖ cliPub ‖ kemPub ‖ kemCt, info = "fatoom-sesame-canal-req", L = 32)
   K_res = HKDF-SHA256(ikm = ssKem ‖ shared, salt = sessionPub ‖ cliPub ‖ kemPub ‖ kemCt, info = "fatoom-sesame-canal-res", L = 32)
   ```
3. Chaque requête est chiffrée sous `K_req` et sa réponse sous `K_res` ; l'enveloppe, l'IV (`ivPrefixe` de 4 octets tirés à la création du canal ‖ compteur `n` sur 8 octets), les aad, les bornes de `n` et la vie d'un canal sont fixés par `API.md` (§1.2). Une requête hors instance (`instanceId` inconnu après redémarrage) reçoit `410` et le client recommence à l'étape 1.

**Une clé par sens** : avec une seule clé, la requête et la réponse `n` seraient chiffrées sous la même clé et le même IV, ce qui sous AES-GCM révèle le XOR des deux clairs et permet de forger des messages. Chaque couple (clé, IV) ne sert qu'une fois.

**Hybride post-quantique** : le canal transporte KTS à l'enrôlement, les clés ouvertes pour les proches et le protocole de PIN, alors que les enveloppes restent stockées des décennies ; un trafic enregistré aujourd'hui (TLS terminé hors de l'enclave) ne doit pas pouvoir être déchiffré par un futur calculateur quantique. La combinaison X25519 + ML-KEM-768 est de type X-Wing : sûre si l'un des deux secrets l'est. Bibliothèques : Go `crypto/mlkem` côté Sésame ; côté app et navigateur, au choix du client, avec des vecteurs communs exigés (pour Fatoom : module natif de `react-native-quick-crypto` dans l'app, `@noble/post-quantum` dans le navigateur seul).

**Routage par instance et chemin unique** : chaque VM est seule dans son groupe d'instances zonal, et la table d'URL du répartiteur envoie `/n/{a|b|c}/…` à la VM de ce préfixe, dernier caractère de son nom (`API.md`, §1.2) ; `GET /attestation` est servie sans préfixe, par n'importe quelle VM. Toutes les requêtes du canal vont ensuite à **un seul chemin**, `POST /n/{a|b|c}/canal` ; le nom de la route (`enroler`, `vie`, `deces/annuler`, `session/ouvrir`, `pin/evaluer`…) est dans le corps chiffré (`API.md`). Le répartiteur, qui termine TLS hors de l'enclave et que l'exploitant règle, ne distingue donc pas une annulation d'une ouverture par un proche : bloquer l'une bloque l'autre, et la manœuvre devient une panne générale, visible. L'affinité de session du répartiteur, « best effort » selon Google, n'est pas utilisée.

Ce canal est **obligatoire** par-dessus TLS : TLS est terminé par le répartiteur de charge Google, hors de l'enclave. Les chaînes de domaine (`fatoom-sesame-…`) portent le nom du premier client ; elles font partie du format et des vecteurs de test et ne changent pas d'un client à l'autre. Les erreurs du canal lui-même (`410`, `400 canal_invalide`) sont renvoyées en clair.

## 6. Registre : l'état minimal de Sésame

Sésame est sans état pour tout ce qui est clé ou contenu. Il tient un **registre** (Firestore natif dans son projet, accessible uniquement à la charge attestée) parce que trois garanties l'exigent : la mesure des délais (7 jours après un signalement, inactivité), l'annulation explicite par le titulaire, et le refus d'une charte révoquée.

Chaque ligne porte une signature :

```
sig = ECDSA(registre-sign, "fatoom-sesame-registre|1|" ‖ UTF-8(JSON canonique sans sig))
```

calculée par KMS à l'écriture (un appel par écriture, écritures rares) et vérifiée **localement** avec la clé publique de `registre-sign` lue dans KMS au démarrage (`kid` = version de la clé ; toutes les versions activées générées par KMS sont acceptées, une version importée fait refuser de servir, §4). Une ligne modifiée hors de l'enclave est ignorée.

### 6.1 Collections

| Collection | Document | Champs |
|---|---|---|
| `titulaires/{uid}` | un par titulaire enrôlé | `etat` ∈ `actif` \| `supprime` ; `ktsGeneration` (int) ; `enroleLe` (date du premier enrôlement) ; `vivantLe` (dernière preuve de vie, horloge Sésame ; **initialisé à `enroleLe`**) ; `chartes.{contactId}` = génération courante (int ; `-1` = proche supprimé ou rôle refusé) ; `chartesDerniere.{contactId}` = dernière génération positive, écrite au passage à `-1` ; `cleViePub` (reçue de l'app à l'enrôlement) ; `grappePin` = `{grappeId, depuisLe}` (grappe du dernier dépôt du PIN, §9) ; `ficheGeneration`, `ficheScellee` (fiche d'alerte sous `alerte-seal`, §11.10) ; `ecritLe` ; `sig` |
| `titulaires/{uid}/signalements/{id}` | un par signalement (décès ou inactivité) | `signaleLe` (horloge Sésame) ; `source` ∈ `insee` \| `acte` \| `inactivite` ; `deces` (vrai sauf pour `inactivite`) ; `dateDeces` (fournie par le client, absente pour `inactivite`) ; `empreinteActe` (`SHA-256` du fichier de l'acte, source `acte` seulement) ; `transmissibleLe` ; `annuleLe?` ; `annulePar` ∈ `annulation` (décès) \| `preuve_de_vie` (inactivité seulement) ; `lettresEnvoyeesLe?` ; `alertesEnvoyeesLe[]` (J0, J3, J6) ; `ecritLe` ; `sig` |
| `titulaires/{uid}/ouvertures/{id}` | une par session de proche réussie | `contactId`, `role`, `ouvertLe`, `nbCles`, `contenus[]` (identifiants des contenus dont la clé a été ouverte), `ecritLe`, `sig` |
| `titulaires/{uid}/lettres/{contactId}` | une par proche prévenu par lettre (§11.9) | `invitationLookup`, `pingenDocumentId`, `statut` ∈ `commandee` \| `imprimee` \| `en_distribution` \| `distribuee` \| `erreur`, `envoyeeLe`, `renvoyeeLe[]`, `ouverteLe?`, `identiteVerifieeLe?`, `essaisIdentite`, `ecritLe`, `sig` |
| `invitations/{lookup}` | une par code d'invitation (`lookup` étiré par Argon2id comme l'index du code d'accès, §10.4) | `uid`, `contactId`, `codeVerifier` (même forme que celui de la charte : Sésame vérifie le code reçu sans recalculer l'index), `expireLe` (12 mois), `ecritLe`, `sig` |
| `instances/{instanceId}` | une par instance en vie | `imageDigest`, `demarreLe`, `vuLe` (toutes les 60 s) |
| `chaine/tete` | un seul | `epoque` (entier croissant à chaque écriture), `empreinte` (tête de la chaîne d'empreintes), `ecritLe`, `sig` |

**Échéance d'un signalement** : `transmissibleLe` = 23 h 59 min 59 s, heure de Paris (`Europe/Paris`), du 7ᵉ jour calendaire après la date de `signaleLe` à Paris (par exemple : signalé le 9 octobre ⇒ 16 octobre à 23 h 59). Le fuseau est embarqué dans le binaire (`time/tzdata`), changements d'heure compris. Même règle pour toutes les sources. L'échéance ne change plus après l'inscription.

Les **défis** (§10.5) ne sont pas au registre : chaque instance les garde en mémoire, liés au canal du client qui les a demandés, jusqu'à leur usage ou 5 minutes ; un redémarrage les perd (le client reçoit `410` et recommence). Les données du protocole de PIN n'y sont pas non plus (§9).

Taille : quelques Ko par titulaire. Aucune coordonnée en clair, aucun nom, aucune clé secrète (seulement des clés publiques).

### 6.2 Registre en ajout seul

La `sig` empêche de modifier une ligne ; elle n'empêche ni de restaurer une sauvegarde ancienne, ni d'effacer un titulaire, ni de recopier une ligne ancienne. Les règles suivantes ferment ces cas.

- **Pierres tombales** : aucune ligne n'est jamais retirée. Supprimer un titulaire (§11.3) passe sa ligne à `supprime` : plus d'enrôlement, d'ouverture ni de PIN pour cet `uid`, définitivement ; un proche supprimé garde sa génération `-1`. Un `uid` sans ligne n'est jamais « ouvert » : toute ouverture est refusée (échec fermé), seul un premier enrôlement est possible.
- **Marqueur d'inscription** : au premier enrôlement, Sésame crée `inscrits/<hex(SHA-256(uid))>` dans un bucket privé (pour Fatoom : `gs://<bucket-inscrits>`, `europe-west9`, rétention verrouillée de 10 ans, création seule avec `ifGenerationMatch=0`, aucun écrasement ni suppression) **avant** d'écrire la ligne. Avant tout premier enrôlement, et à toute lecture qui trouve une ligne absente, Sésame lit ce marqueur : marqueur présent et ligne absente ⇒ échec fermé pour cet `uid`, comme une ligne rejetée (ci-dessous), et événement `ligne_absente` dans la borne du registre (§8) ; bucket illisible ⇒ aucun premier enrôlement. Une ligne effacée ne fait donc pas passer le titulaire pour jamais enrôlé. Le bucket est privé : il ne publie pas le nombre de titulaires.
- **Époque et chaîne** : chaque écriture fait avancer `epoque` et la chaîne `empreinte = SHA-256(empreinte précédente ‖ JSON canonique de l'écriture)`. Chaque borne du registre porte `registre: {epoque, empreinte}` (§8).

### 6.3 Contrôles anti-retour

- **Contrôle d'époque** : au démarrage, avant toute ouverture et avant tout changement de grappe du protocole de PIN, une instance vérifie que le registre lu n'est pas plus ancien que l'époque de la **dernière borne** du bucket d'audit, ni que la dernière époque qu'elle a elle-même écrite (gardée en mémoire). « La dernière borne » est l'**époque la plus haute** parmi les bornes valides (§8.1) de toutes les instances déposées dans les **3 dernières heures** : une borne ancienne recopiée sous un nom récent n'abaisse jamais ce maximum. La fenêtre de 3 heures et le seuil de 2 heures se mesurent sur le `timeCreated` de l'objet, posé par Cloud Storage, jamais sur `borneLe` ni sur le nom du fichier. Si la plus récente de ces bornes a plus de **2 heures**, s'il n'y en a aucune, si le registre est plus ancien ou si le bucket est illisible, l'instance **refuse d'ouvrir, de transmettre et de changer de grappe** ; elle accepte encore preuves de vie et annulations, et l'alerte part à l'exploitant (§11.10) ; une ouverture refusée renvoie `reessayer_plus_tard`, sans détail ni essai décompté. Un arrêt de toutes les instances de plus de 2 heures se termine donc par une reprise publiée, seule sortie de cet état.
- **Fraîcheur de chaque ligne** : la `sig` ne couvre que le contenu d'une ligne et le contrôle d'époque ne regarde que `chaine/tete` ; une ligne ancienne à la `sig` valide, recopiée depuis une sauvegarde, effacerait une annulation, ferait revenir une génération de charte en arrière ou reculerait `grappePin.depuisLe`. Chaque document écrit par Sésame porte donc `ecritLe` (horloge de Sésame, ISO 8601 UTC à la milliseconde), couvert par la `sig` et recalculé à chaque tentative d'une transaction. À chaque lecture, Sésame compare `ecritLe` à l'`update_time` du document, que Firestore attribue seul : un écart de plus de 2 minutes, dans un sens ou dans l'autre, **rejette la ligne**.
- **Ligne rejetée** : elle n'est jamais réécrite à partir de son contenu. Pour cet `uid`, Sésame refuse ouverture, transmission, lettre, inscription d'un signalement, publication de génération, enrôlement, PIN et changement de grappe ; il accepte encore la preuve de vie (répondue, non inscrite) et l'annulation (vérifiée avec la `cleViePub` de la ligne, dont la `sig` reste valide ; une annulation peut réécrire un signalement rejeté, car elle ne fait que le clore). Il publie `ligne_rejetee` dans sa borne et alerte le titulaire (§11.10). La ligne n'est rétablie que par une reprise publiée. L'exploitant garde le pouvoir de bloquer et perd celui de faire oublier un fait à Sésame. Le comportement d'`update_time` après un import, une restauration ou un clonage de base est à vérifier par essai sur le Sésame factice ; tout import ou restauration non suivi d'une reprise publiée est signalé par le script public (§8.3).
- **Reçus signés** : chaque annulation et chaque génération publiée donne à l'app un reçu signé par `session-sign`, gardé sur l'appareil : un retour arrière devient prouvable par le titulaire.

### 6.4 Sauvegarde et reprise

- Le registre est exporté **chaque heure**, à un instant donné (`snapshot_time`, restauration à un instant donné activée sur la base), vers un bucket du projet de Sésame.
- **Toute restauration passe par une reprise publiée** : une nouvelle version de Sésame dont l'image porte l'époque et l'empreinte de reprise (donc attestée, dans `versions.json` et ce dépôt). Au démarrage, cette version vérifie les `sig` de la sauvegarde restaurée (localement, avec les clés publiques de toutes les versions de `registre-sign`) et sa chaîne jusqu'à l'empreinte de reprise, puis **réécrit chaque ligne** (nouvel `ecritLe`), recrée les marqueurs d'inscription manquants et publie `reprise` dans sa borne. Les écritures postérieures à la sauvegarde sont perdues, cas assumé (`THREAT_MODEL.md`, limite 11) ; aucune remise en arrière silencieuse n'est possible.
- La perte du registre ne détruit aucune clé. Les compteurs du PIN n'y sont pas : une restauration ne peut pas les remettre à zéro.
- **Perte du registre sans aucune sauvegarde** : signalements et générations repartent de zéro par une reprise publiée ; la reprise est marquée `sansSauvegarde`. Le marqueur d'inscription d'un titulaire existe sans sa ligne ; pendant la reprise, `/enroler` admet alors un enrôlement dont la preuve se vérifie contre `cleViePubAttendue`, que le client fournit (pour Fatoom : lue dans `users/{uid}.sesame.cleViePub`). Le client pourrait y substituer une autre clé : limite assumée (limite 54 de `THREAT_MODEL.md`). Le client alerte le titulaire du ré-enrôlement (pour Fatoom : email, SMS, notification et message court), et les contenus déjà destinés se republient depuis l'app.

## 7. Journalisation sans fuite

Sésame journalise en JSON structuré : `event`, `uidMasque` (8 premiers hexadécimaux de `SHA-256(uid)`), `contactMasque` (idem), `instanceId`, `imageDigest`, `resultat`, `codeErreur`. Jamais de clé, de code d'accès, de code SMS, de numéro, d'email, de nom, de libellé ni de corps de requête. Les journaux vont dans Cloud Logging du projet de Sésame, buckets dans l'Union européenne (en `europe-west9` si Google le permet), avec une rétention de 90 jours. Le motif de chaque refus est journalisé (`refus par motif`) ; il n'est pas publié. Sésame ne publie aucun compteur : les appels KMS se lisent dans les journaux « Data Access » exportés au bucket public (§8.3).

## 8. Borne publique du registre et surveillance des clés

Il n'y a pas de tiers séquestre : l'IAM des clés KMS est administrée par l'exploitant (`THREAT_MODEL.md`, limite 2). Menace : l'administrateur du projet de Sésame, ou qui lui vole ses accès, ajoute l'empreinte d'une autre image à la condition KMS ; cette image n'a besoin d'aucun client (elle lit `sealedKts` chez le client et les fait desceller), puis l'empreinte est retirée. La liste des versions, ce dépôt et la vérification des clients ne voient rien : ils ne protègent que les échanges avec les clients. Objectif : **une telle opération reste possible, mais elle laisse une trace publique que personne, l'administrateur compris, ne peut effacer.** La trace vient des journaux d'administration de Google, que Sésame ne peut ni produire ni effacer, et du script public ; Sésame ne lit aucune politique et ne dépose qu'une borne du registre.

### 8.1 La borne du registre

Toutes les **10 minutes**, chaque instance dépose au bucket d'audit (pour Fatoom : `gs://<bucket-audit>`) un fichier `bornes/AAAA/MM/JJ/HH-mm-<instanceId>.json`, en création seule (`ifGenerationMatch=0`) :

```
borne = {"v":1, "instanceId", "imageDigest",
         "gceInstanceId",                                     # submods.gce.instance_id du jeton : distingue deux VM successives du même nom
         "borneLe",                                           # horloge de Sésame
         "grappe": {"grappeId", "votants": <nombre>},         # chaque grappe neuve se voit (§9)
         "registre": {"epoque", "empreinte"},                 # §6.2
         "evenements": [{"type": "ligne_rejetee" | "ligne_absente" | "reprise" | "alerte_refusee",
                         "ressource", "vuLe"}]}
                         # ressource = uidMasque ‖ "/" ‖ collection (ligne_rejetee, ligne_absente, reprise)
                         #          ou uidMasque ‖ "/" ‖ signalementId (alerte_refusee) ; jamais le chemin du document, qui contient l'uid
message   = UTF-8("fatoom-sesame-borne|1|" ‖ instanceId ‖ JSON canonique de borne)
signature = ECDSA P-256 / SHA-256 par session-sign (KMS)
fichier   = {borne, signature, kid: "session-sign/<version KMS>"}
```

Le préfixe de domaine empêche de confondre une borne avec un jeton de session, un reçu, une réponse `/etat` ou une requête à la passerelle, signés par la même clé. La clé n'étant utilisable que par la charge attestée, **la signature vaut attestation**, tant qu'aucune version de la clé n'a été importée (§4). Une borne n'est liée à aucune clé par instance : elle vaut ce que vaut la condition KMS ; une image autorisée en douce peut en signer une, mais ne peut qu'élever le maximum (échec fermé pour tous) ou, après 3 heures sans aucune instance honnête, remplir le trou, ce qui ne lui donne rien qu'elle n'ait déjà par `registre-sign`.

**Vérification** (par l'instance au démarrage, §6.3, et par le script public, §8.3) : signature valide pour la clé publique de **toute version activée** de `session-sign` générée par KMS (sans quoi une rotation de la clé invaliderait les bornes des 3 dernières heures et provoquerait un échec fermé) ; `instanceId` du message égal à celui de la borne ; nom de fichier conforme à `borneLe` et `instanceId` ; `borneLe` et le nom à moins de 5 minutes du `timeCreated` de l'objet, seule horloge de la fraîcheur.

### 8.2 Des journaux qu'on ne peut plus effacer en silence

- Le bucket d'audit (`europe-west9`, lecture publique par `allUsers`) porte une **politique de rétention de 10 ans verrouillée** (Bucket Lock) : aucun objet ne peut être supprimé ni remplacé avant 10 ans, par personne, et la rétention ne peut plus être retirée ni raccourcie.
- **Ce que le verrou n'empêche pas** : le propriétaire peut retirer le « lien » posé sur le projet, puis supprimer le projet entier en 30 jours ; couper la facturation peut faire perdre des données ; la lecture publique n'est qu'une liaison IAM, révocable. Le bucket reste donc **dans le projet de Sésame** : effacer les preuves oblige à supprimer Sésame lui-même et ses clés, donc toutes les transmissions, ce qui se voit de partout. Il n'y a pas de témoin hors de Google ; il reste la copie privée des journaux d'administration chez Google (`_Required`, 400 jours, non modifiable).
- Le bucket reçoit : (a) par un **récepteur d'export agrégé au niveau de l'organisation**, les journaux « Admin Activity » et « Data Access » des services du §8.3, de Compute Engine (création de VM, métadonnées `tee-*`), de Cloud Logging (récepteurs, exclusions), de Resource Manager (dont le lien de projet), des contraintes d'organisation, des politiques de refus, de l'IAM et de la rétention de ce bucket, et les journaux « Admin Activity » de Firestore (import, export, restauration, création et suppression de base) du projet de Sésame ; (b) **directement de Sésame**, les bornes du registre.
- Le **trou** se mesure sur `bornes/` (un fichier par instance toutes les 10 minutes, même sans événement ; tolérance de 3 heures), pas sur les fichiers du récepteur, qui arrivent par lots horaires avec 2 à 3 heures de retard et manquent quand il n'y a rien à écrire. Les signatures des bornes (`AsymmetricSign` toutes les 10 minutes par instance) garantissent des entrées KMS chaque heure : une heure de bornes sans fichier du récepteur signale un récepteur coupé.
- Contraintes d'organisation : `storage.publicAccessPrevention` n'est pas appliquée à ce projet, et `iam.allowedPolicyMemberDomains` (ou `iam.managed.allowedPolicyMembers`) reçoit une exception par tag pour lui seul. Les bornes et les journaux ne contiennent ni clé ni contenu.

### 8.3 Chaque descellement journalisé et vérifié par un script public

Les journaux « Data Access » sont activés dans la politique IAM de l'**organisation** : `cloudkms.googleapis.com` (`DATA_READ` pour chiffrer, déchiffrer et signer ; `ADMIN_READ`), `secretmanager.googleapis.com` (`DATA_READ`, `ADMIN_READ`), `datastore.googleapis.com` (`DATA_WRITE`), `iam.googleapis.com` et `sts.googleapis.com` (`ADMIN_READ`, qui couvre l'échange et la création des jetons fédérés). La contrainte `iam.disableAuditLoggingExemption` est posée avant toute autre configuration ; toute exemption ou désactivation est une opération d'administration journalisée et exportée.

Les **écritures du registre** (`datastore.googleapis.com` en `DATA_WRITE`) sont exportées par un récepteur limité au projet de Sésame vers un second bucket (pour Fatoom : `gs://<bucket-ecritures>`, `europe-west9`), à rétention de 10 ans verrouillée mais **non public** : ces entrées portent le chemin des documents, donc l'`uid`. Elles servent de preuve en cas de contestation (une écriture qui ne vient pas de `<compte-charge>` signale une manœuvre) ; la garantie elle-même vient du contrôle de fraîcheur (§6.3).

Le script public `build/verifier-journaux.sh` de ce dépôt lit le bucket d'audit (journaux exportés et bornes) et vérifie :

- que **chaque** appel aux clés de Sésame (`Decrypt`, `AsymmetricSign`) vient d'un sujet fédéré dont l'échange STS a réussi, donc d'une empreinte qui figurait **à cet instant** dans la condition du fournisseur (la suite des `UpdateWorkloadIdentityPoolProvider` du journal d'administration donne la condition à chaque instant), et que cette empreinte est `en_service` dans l'historique de `versions.json` à cette date ; le script ne se fie ni à l'`imageDigest` déclaré d'une borne ni à la métadonnée `tee-image-reference`, que l'exploitant choisit. Le journal donne le sujet fédéré (`serviceAccountDelegationInfo.principalSubject`, exactement le `sub` du jeton, qui désigne la VM par son nom) ; le script rapproche chaque appel d'une instance par le couple (sujet, heure), et `gceInstanceId` distingue deux VM successives du même nom ; un appel d'un sujet qui n'a déposé aucune borne dans l'heure est signalé ;
- que chaque borne correspond à un `AsymmetricSign` de `session-sign` du même sujet dans la même minute, et que tout `AsymmetricSign` hors d'une borne, d'un jeton de session, d'un reçu, d'une réponse `/etat`, d'une requête à la passerelle ou d'une ligne du registre est signalé ;
- que chaque modification d'IAM, de pool ou de fournisseur, de rôle, de clé de compte de service, de journalisation, de récepteur ou de verrou correspond à un commit de `versions.json` ; que tout `CreateImportJob`, `ImportCryptoKeyVersion` et `CreateCryptoKeyVersion` sur le trousseau `sesame` est signalé et correspond à un commit ;
- qu'aucune instance n'a de trou de plus de 3 heures dans `bornes/` ;
- tout import, restauration ou clonage de la base du registre non suivi d'une reprise publiée ; toute modification du répartiteur (table d'URL, services de backend), des règles de pare-feu, de Cloud NAT et des routes du projet ; toute version nouvelle ou désactivée des secrets Twilio et SendGrid de Sésame (`AddSecretVersion`, `DisableSecretVersion`) ;
- tout changement de la version minimale de l'app du client, avec sa date (pour Fatoom : fichier statique `fatoom.fr/.well-known/fatoom-version-minimale.json`, versions minimales de l'app et du système par plateforme, au format tenu par le client, généré au build et versionné dans Git ; c'est la source unique de la version minimale, que l'app lit aussi).

Il signale tout appel d'une identité inconnue, tout surplus, toute modification inattendue et tout trou. La CI publique de ce dépôt l'exécute chaque jour, et n'importe qui peut l'exécuter.

### 8.4 Ce qui reste possible

Un propriétaire unique de l'organisation peut toujours tout défaire dans Google : autoriser une autre image, couper un récepteur, rendre le bucket privé, retirer le verrou et supprimer le projet. La garantie n'est pas une impossibilité (ce serait le rôle d'un tiers séquestre, écarté) : c'est l'impossibilité de le faire **sans le montrer**. La modification de l'IAM, chaque échange STS et chaque descellement de l'image inconnue sont au journal public, et ce qui a été écrit ne s'efface pas avant 10 ans, sauf à supprimer Sésame entier. Sésame ne publie pas lui-même sa politique : une image autorisée en douce ne se voit que dans les journaux de Google, et un attaquant qui tient à la fois Google Cloud et ce dépôt peut y faire correspondre un commit (`THREAT_MODEL.md`, limites 2 et 12).

## 9. Protocole de PIN et grappe

**Principe** (celui de Signal SVR2 et des séquestres d'Apple et de Google : l'opérateur ne peut pas changer le code sans perdre l'état) : tout ce qui sert à vérifier un PIN vit **uniquement dans la mémoire** des VM de Sésame, chiffrée par le processeur, répliqué entre les VM de la grappe, jamais écrit ni scellé, jamais transmis à une autre version. Il n'existe rien qu'une image autorisée en douce puisse desceller, ni de sauvegarde à restaurer.

### 9.1 Ce que Sésame garde et ce que fait l'app

**Par titulaire** (environ 80 octets, en mémoire seulement) : `cleOprf` (clé OPRF de 32 octets tirée par Sésame, RFC 9497, suite ristretto255 seule, avec `cloudflare/circl/oprf` côté Go), `essaisRestants` (10 au dépôt), `prochainEssaiLe` (délai croissant), `creeLe`. Rien sur disque, dans KMS, au registre, dans les journaux ni chez le client.

**Côté app** : l'app « aveugle » le PIN, Sésame évalue l'OPRF sans voir ni le PIN ni une empreinte du PIN, l'app retire l'aveuglement et obtient `y`. La clé de la copie serveur de la KMU est `Argon2id(y, sel)` et la copie est `AES-256-GCM(clé, KMU, aad "fatoom-kmu-pin|" ‖ uid)` (dérivation exacte et vecteurs tenus par le client, recopiés dans ce dépôt). Argon2id ne sert qu'en cas de faille de l'enclave, où la clé OPRF fuirait. À la création du coffre, la copie serveur n'est écrite chez le client qu'**après** l'enrôlement (§11.8) et le dépôt `nouveau_pin` : Sésame injoignable ou en pause ⇒ « Réessayer », rien n'est écrit.

### 9.2 Opérations

Routes sous le canal (§5) ; corps et réponses dans `API.md`.

**Dépôt** (`/pin/deposer`) : à la création du coffre, au changement de PIN, après une récupération, juste après une ouverture réussie sur un nouvel appareil, et pour un redépôt après un effacement ou dans une nouvelle grappe ; coffre ouvert sur l'appareil.

- Sésame exige une ligne `titulaires/{uid}` à l'état `actif` (contrôle de fraîcheur, §6.3) et une `signature` par `cleVie` sur :
  ```
  "fatoom-pin-depot|1|" ‖ uid ‖ "|" ‖ grappeId ‖ JSON canonique {"v":1, "uid", "grappeId", "elementAveugle": hex(SHA-256(élément)), "defi", "motif"}
  ```
  vérifiée avec `cleViePub` du registre ; l'`idToken` est vérifié en plus, jamais seul. Le client, qui peut fabriquer un `idToken`, et le voleur d'un compte ne peuvent donc ni déposer, ni remplacer, ni prendre la place d'un enregistrement effacé.
- `motif` ∈ `nouveau_pin` (création du coffre, changement de PIN, nouveau PIN après une récupération) | `ouverture` (juste après une évaluation sur **le même canal**, que Sésame a notée) | `redepot` (même PIN, après un effacement ou dans une nouvelle grappe).
- **Compteur par code PIN** : `essaisRestants` ne repasse à 10 que pour `nouveau_pin` et `ouverture` ; un `redepot` sur un enregistrement présent garde `essaisRestants` ; au plafond (`essaisRestants = 0`), seul `nouveau_pin` est admis (sinon `pin_bloque`). Les échecs d'un tiers ne sont jamais effacés par un redépôt automatique du même PIN. Il n'y a **pas de plafond cumulé** au-delà de ces 10 essais par code.
- Pour `nouveau_pin` et `redepot` : Sésame tire une nouvelle `cleOprf`, fixe `essaisRestants`, fait valider le changement par la majorité, évalue `elementAveugle` et renvoie l'évaluation ; l'app calcule la nouvelle copie serveur de la KMU et l'écrit chez le client.
- Pour `ouverture` : Sésame **garde** `cleOprf`, remet `essaisRestants` à 10, fait valider par la majorité et répond sans évaluation ; `elementAveugle` doit être celui de l'évaluation notée sur ce canal, ce qui lie la remise à 10 à l'évaluation réussie. `grappePin` n'est pas réécrit, rien n'est écrit chez le client.

**Évaluation** (`/pin/evaluer`, `idToken` seul) : ouverture sur un nouvel appareil, ou sur un appareil connu après une déconnexion.

- Le leader lit d'abord la ligne `titulaires/{uid}` (contrôle de fraîcheur) : absente, rejetée ou `supprime` ⇒ `pin_indisponible` **sans décompte** ; registre illisible ⇒ `pin_indisponible_temporaire`. Sans enregistrement en mémoire : `pin_indisponible` (même réponse pour un compte inconnu et après un effacement). `essaisRestants = 0` : `pin_bloque`. `prochainEssaiLe` non passé : `trop_d_essais` avec l'heure.
- Sinon Sésame **décompte d'abord** un essai, validé par la majorité, puis évalue et répond. Pas de `pin_refuse` : avec l'OPRF, Sésame ne sait pas si le PIN est juste.
- **Chaque essai décompté déclenche une alerte au titulaire** (§11.10, au plus une par heure), réussi ou non : un dépôt `ouverture` ne doit pas pouvoir faire taire l'alerte.
- Délai croissant : aucun pour les 3 premiers essais, puis 1 min, 5 min, 15 min, 1 h, 4 h, 12 h, 24 h. Le 10ᵉ essai est évalué et répondu comme les autres, puis Sésame efface `cleOprf` et garde seulement `essaisRestants = 0` : un 10ᵉ PIN juste n'ouvre pas, un dépôt `ouverture` est refusé (`pin_bloque`) ; seul un dépôt `nouveau_pin` signé par `cleVie` en sort (Sésame contrôle la signature, pas l'appareil).
- Les essais ne sont bornés que par titulaire, comme chez Apple et Google ; il n'y a ni disjoncteur global ni compteur public du PIN (`THREAT_MODEL.md`, limite 19).

**État** (réponses de `/vie` et de `/etat`) : `pinPresent: false` (effacement, nouvelle grappe) ⇒ l'app redemande le PIN une fois, à la prochaine ouverture, et redépose (`redepot`) ; `pinBloque: true` ⇒ jamais de redépôt silencieux, l'app fait choisir un nouveau PIN (`nouveau_pin`) ; `evaluationsDepuis` (réponse de `/etat` seulement) : nombre d'évaluations depuis le repère envoyé par l'app ; non nul ⇒ l'app du téléphone actif signale que le PIN a été utilisé sur un autre appareil, sans dépendre du client.

Il n'existe pas de route d'effacement : une ancienne grappe est arrêtée par l'exploitant (§11.6), et la suppression d'un compte est couverte par la pierre tombale, que l'évaluation et le dépôt lisent avant de répondre.

### 9.3 Grappe de trois VM à majorité

- **Composition** : les trois VM de Sésame, une par zone (§2.1). Chaque VM tire à chaque démarrage, en mémoire, un **identifiant de nœud** neuf, jamais réutilisé, lié à son attestation (`eat_nonce` de son canal entre VM).
- **Consensus** : Raft, par `go.etcd.io/raft/v3` (Apache-2.0, épinglée : la version retenue après un nouvel essai, à l'étape du contrat exécutable), avec son seul stockage en mémoire (`MemoryStorage`), `PreVote` et `CheckQuorum` : rien n'est écrit sur disque. Le transport est un canal TLS mutuellement attesté entre VM (mêmes conditions que les clients, §3, **même** `image_digest`, `hwmodel` admis par la liste des versions). Le leader traite les routes du protocole de PIN ; les autres VM lui transmettent celles qu'elles reçoivent.
- **Aucune réponse à un dépôt ou une évaluation tant que la majorité (deux VM sur trois) n'a pas enregistré le changement.** Sans majorité, Sésame répond `pin_indisponible_temporaire` : c'est une **pause**, l'app propose « Réessayer » et le code de récupération, et l'essai n'est pas décompté. Une VM sans grappe ne sert aucune route du protocole de PIN (même réponse) et n'admet personne. Seul cas où un essai est décompté sans réponse : le leader tombe entre la validation et sa réponse (sens sûr, rare).
- **Liste des membres** répliquée : un membre n'entre (d'abord comme apprenant, puis comme votant) ou ne sort que par une décision de la majorité des membres en place, une modification à la fois. Une VM qui redémarre est **vide** et porte un nouvel identifiant : elle ne vote jamais sous l'ancien. Une coupure du réseau laisse au plus un côté majoritaire ; le côté minoritaire se met en pause et ne peut admettre personne : il est impossible d'obtenir deux copies de l'état, quelle que soit la façon dont l'exploitant coupe le réseau ou crée des VM.
- **Remplacement automatique** : une VM qui démarre sans état demande à entrer (`/grappe/rejoindre`, servie seulement sur le canal entre VM, à l'adresse DNS interne zonale de chaque membre). Le leader l'admet sans intervention humaine si, et seulement si : même `image_digest` que lui, `hwmodel` admis ; sa zone (`submods.gce.zone` du jeton) est celle d'un membre votant injoignable depuis au moins 2 minutes ; aucune autre modification de membres n'est en cours. Il l'ajoute comme apprenant, la promeut une fois le journal rattrapé, puis retire l'ancien membre de cette zone. La grappe ne dépasse jamais trois votants, un par zone. Le passage à l'option A, le déplacement d'une place pendant l'indisponibilité longue d'une zone et toute autre modification de membres restent commandés (`grappe/membres`, route d'exploitation sous le même canal, validée par la majorité, conçue avec le prototype Raft et écrite dans `API.md` avant d'être servie).
- **Pannes**, trois cas à ne pas confondre (seules les routes du protocole de PIN passent par la grappe ; les autres routes, dont la preuve de vie, l'annulation, l'ouverture par un proche et l'enrôlement, sont servies par chaque VM joignable) : **une VM perdue** (hôte, zone, mémoire effacée) ne suspend rien, les deux autres continuent et ajoutent sa remplaçante ; **deux VM injoignables mais vivantes** (réseau coupé) : plus de majorité, donc pause (`pin_indisponible_temporaire`, « Réessayer »), et la grappe reprend seule au retour de la majorité, sans perte d'état ; **deux VM perdues en même temps** (mémoire effacée) : la grappe s'arrête pour toujours, c'est un **effacement total** ; l'exploitant remplace les trois VM, la restante comprise, puis amorce une grappe neuve et vide (ci-dessous), et le titulaire rouvre son coffre sur un appareil connu ou par le code de récupération, puis redépose. Le registre et les clés scellées par KMS ne sont pas touchés : les transmissions reprennent dès que le service revient. Sont aussi des effacements totaux, qu'aucune majorité ne couvre : une entrée fautive appliquée par les trois répliques, le retrait de `STABLE` à l'image Confidential Space avant le remplacement des VM, l'arrêt de l'ancienne version par l'exploitant.
- **Ce que fait l'app pendant une panne** : toute réponse hors canal (erreur du répartiteur, statut ou forme hors contrat) ou absence de réponse ⇒ une reprise à `GET /attestation`, puis « Réessayer » ; `pin_indisponible_temporaire` et `reessayer_plus_tard` donnent le même « Réessayer ». Un dépôt `ouverture` qui échoue pendant une pause ne se rattrape pas : l'essai reste décompté jusqu'à la prochaine ouverture réussie ou au prochain `nouveau_pin`, car un dépôt `ouverture` ne vaut que juste après l'évaluation, sur le même canal. Un redépôt silencieux (`pinPresent: false`) qui reçoit `grappe_recente` est retenté à chaque ouverture de l'app jusqu'à l'échéance des 30 jours, sans écran.
- **Alertes à l'exploitant** : Sésame journalise, sans donnée de titulaire, `grappe_votant_absent` (le leader voit un votant injoignable depuis 2 minutes), `grappe_admission_refusee` (une demande `/grappe/rejoindre` refusée, par le leader) et `grappe_sans_majorite` (une VM ne voit plus de majorité) ; une alerte Cloud Monitoring sur ces événements part **immédiatement** par email à l'exploitant. Avec trois VM, le temps où une VM manque est celui où une seconde perte effacerait tout : l'exploitant doit le savoir sans attendre la lecture quotidienne du script public (§8.3). L'alerte sert la disponibilité, pas la confidentialité : l'exploitant peut la couper.
- **Zone indisponible longtemps** : la grappe tourne avec deux votants, sans rien suspendre, mais une seconde perte serait un effacement total. Si la zone ne revient pas, l'exploitant peut déplacer la place de la VM absente : une VM de même nom (donc de même préfixe de routage) dans une autre zone de Paris, admise par `grappe/membres`, commandée et validée par la majorité (le remplacement automatique ne l'admet pas : sa zone n'est pas celle du votant absent) ; la place revient de la même façon au retour de la zone. Pendant ce temps, une zone porte deux VM, comme en option A.
- **Amorçage d'une grappe neuve** (nouvelle version, effacement total) : **commandé par l'exploitant**, jamais spontané ; une coupure du réseau ne doit pas suffire à faire naître une grappe parallèle. La forme reprend l'amorçage statique de Raft (même liste initiale de membres donnée à chaque nœud, état `new`) et l'attestation mutuelle des pairs avant de former un groupe ; elle est conçue avec le prototype Raft, à l'étape du contrat exécutable, et écrite dans `API.md` (§4.3) avant d'être servie. L'exploitant remplace les trois VM, une par zone, puis appelle `grappe/amorcer` sur l'une d'elles, avec, en plus du canal, le jeton OIDC du compte d'exploitation de Sésame, dont l'adresse est une variable attestée (§2.3). Servie seulement par une VM qui n'est dans aucune grappe, la commande exige exactement trois VM attestées de même `image_digest`, `hwmodel` admis, une par zone, aucune dans une grappe. La VM compose une **configuration initiale** identique pour les trois (`{"v":1, "imageDigest", "membres": [{"nodeId", "zone", "cle"}, …]}`, JSON canonique, membres triés par zone, `cle` liée à l'attestation du membre par `eat_nonce`), la fait réserver par les deux autres (`grappe/preparer` : chacune vérifie sur ses propres canaux que la configuration décrit les VM qu'elle voit, puis la réserve 2 minutes), puis ordonne le démarrage (`grappe/demarrer`) : les trois démarrent Raft en état `new`, les trois membres votants. Une réservation non suivie du démarrage est oubliée. La grappe neuve se voit dans chaque borne (§8.1) et l'amorçage est journalisé (`grappe_amorcee`, avec l'adresse du compte qui l'a commandé). La commande ne donne ni état, ni essai, ni clé : amorcer une grappe parallèle demande de tenir trois VM sans grappe, ce que seul l'exploitant peut faire, et revient à « démarrer une nouvelle grappe » (ci-dessous, `THREAT_MODEL.md`, limite 19 (f)).
- **Mémoire et robustesse** : le journal Raft est borné (instantané et compaction à intervalle fixe et au-delà d'un seuil de taille), si bien que la mémoire reste proportionnelle au nombre de titulaires ; la mémoire du processus a un plafond suivi, avec une alerte à l'exploitant au-delà d'un seuil et, à 90 %, le refus des nouveaux dépôts (`pin_indisponible_temporaire`), jamais un arrêt ; toute entrée est validée avant d'être proposée à Raft (forme, tailles, signature, ligne du registre) et l'application d'une entrée ne peut pas faire paniquer le processus (une entrée fautive appliquée par les trois répliques les arrêterait ensemble).
- **Identité de la grappe** : `grappeId = hex(SHA-256(JSON canonique de la configuration initiale de l'amorçage : identifiants de nœud, zones et clés attestées des trois membres, image_digest))`, publiée dans chaque borne (§8.1). Les identifiants de nœud étant neufs, deux amorçages ne donnent jamais le même `grappeId`.
- **Changement de grappe limité** : Sésame inscrit au registre, pour chaque titulaire, `grappePin = {grappeId, depuisLe}`. Un dépôt dans la grappe inscrite est toujours admis ; un dépôt dans **une autre** grappe n'est admis que si `depuisLe` date d'au moins 30 jours, sinon `grappe_recente`. Le leader fait le contrôle anti-retour (§6.3), lit la ligne avec son contrôle de fraîcheur, lit puis écrit `grappePin` dans **une seule transaction** Firestore, et seulement ensuite fait valider le dépôt par la majorité ; rien n'est déposé si le registre est illisible ou la transaction refusée (`pin_indisponible_temporaire`), si le contrôle anti-retour échoue (`reessayer_plus_tard`, sans détail) ou si la ligne est rejetée (`pin_indisponible`). Aucune exception : d'où la règle d'exploitation **jamais deux versions de Sésame à moins de 30 jours d'écart**, sauf retrait d'urgence. L'app garde aussi le `grappeId` de son dernier dépôt.
- **Nouvelle version, ou nouvelle grappe après un effacement total** : une grappe neuve et vide démarre, par l'amorçage commandé ; l'ancienne version est arrêtée par l'exploitant dès que la nouvelle est en service (§11.6), ce qui efface sa grappe. Aucune copie d'état d'une version à l'autre n'existe, par construction.
- **Renouvellement de l'image Confidential Space** et mises à jour : une VM à la fois, par ajout de sa remplaçante puis retrait ; la grappe garde toujours une majorité. Les maintenances de Google migrent les VM à chaud en option B, sans effet sur la grappe.

**Ce que l'exploitant peut encore faire** : arrêter deux VM, ce qui **efface** au lieu de révéler et se voit (journaux publics, trou dans les bornes) ; démarrer une nouvelle grappe, qui se voit dans les bornes et ne lui donne rien tant que le titulaire n'y a pas redéposé, puis au plus 10 essais contre le même PIN, au plus une fois tous les 30 jours par titulaire. Il ne peut plus multiplier les essais en silence (`THREAT_MODEL.md`, limite 19).

## 10. Hiérarchie et formats des clés

Tout est AES-256-GCM, IV 12 octets, tag 16 octets **concaténé à la fin**, encodage base64 standard. HKDF = HKDF-SHA256. Les chaînes de domaine sont en UTF-8 sans terminateur. JSON canonique = **RFC 8785 (JCS)** : clés triées par unités UTF-16, sans espace, nombres et échappements selon la RFC, UTF-8 ; les chaînes saisies par une personne (noms, prénoms, adresse) sont normalisées en **NFC** avant tout chiffrement ou comparaison. Les identifiants qui entrent dans une AAD ou un message signé (`uid`, `contactId`, `contentId`, `signalementId`) n'ont que des caractères `[A-Za-z0-9_-]`, vérifiés avant chiffrement : le séparateur `|` et l'accolade `{` ne peuvent pas y apparaître. Les jetons de Sésame (session, reçus) ne sont acceptés qu'en `ES256` (en-tête `alg` imposé, tout autre refusé).

### 10.1 Vue d'ensemble

```
PIN ─► KDP ─► KMU (clé maître du coffre, sur le téléphone)
              ├─ KMF (par fichier), KME (par mot de passe), KMO, KMV (mot pour un proche, dernières volontés)
              │     → clés chiffrées pour le titulaire, rangées par le client
              ├─ graine de cleVie (32 o tirés au hasard) → graineVieChiffree / graineVieIv, rangés par le client
              │     └─ cleVie (Ed25519) : preuve de vie, annulation, publication de génération, preuve d'enrôlement,
              │        dépôts du PIN, fiche d'alerte (vers Sésame)
              └─ KTS (clé de transmission) → ktsChiffree / ktsIv, rangés par le client
                    ├─ HKDF info "fatoom-sesame-enveloppe" ─► KTS_env
                    │     └─ enveloppe = AES-GCM(KTS_env, KMF | KME | KMO | KMV, aad contenu+proche) → enveloppes.{contactId}
                    ├─ HKDF info "fatoom-sesame-charte"    ─► KTS_auth
                    │     └─ charte du proche → charte / charteIv, rangés par le client
                    └─ scellée pour Sésame : sealedKts = KMS(kts-seal).encrypt(KTS, aad uid|génération)
                          → rangé par le client
Sésame (enclave attestée) : KMS.decrypt(sealedKts) ─► KTS ─► KTS_env, KTS_auth
       (en mémoire, le temps d'une session de proche ou d'une lettre ; jamais pour une preuve de vie, vérifiée avec cleViePub)
       └─ ouvre les enveloppes du proche authentifié ─► clés re-chiffrées pour le navigateur (canal §5)
```

### 10.2 KTS, dérivations et clé de vie

- `KTS` = 32 octets aléatoires (générateur du système). Rangée chez le client sous `AES-GCM(KMU, KTS)` (`ktsChiffree`, `ktsIv`). Jamais stockée en clair localement ; en RAM avec la KMU, effacée au verrouillage.
- `KTS_env = HKDF(ikm = KTS, salt = UTF-8(uid), info = "fatoom-sesame-enveloppe", L = 32)`.
- `KTS_auth = HKDF(ikm = KTS, salt = UTF-8(uid), info = "fatoom-sesame-charte", L = 32)`.
- `cleVie` = paire Ed25519 dont la graine, `graineVie`, est **tirée au hasard** sur le téléphone (32 octets) à la création du coffre, avec la KMU et KTS, et rangée chez le client sous `AES-256-GCM(KMU, graineVie, aad "fatoom-cle-vie|1|" ‖ uid)` (`graineVieChiffree`, `graineVieIv`). Elle n'est **jamais dérivée de KTS** : Sésame tient KTS en clair à l'enrôlement, à chaque session de proche et à chaque lettre, et une faille de sa mémoire ou une image autorisée en douce ne doit pas donner le pouvoir de signer au nom du titulaire. Ni Sésame ni KMS ne peuvent signer à sa place. `cleViePub` est envoyée à Sésame à l'enrôlement ; elle ne change qu'à une rotation de KTS, qui tire aussi une nouvelle graine, prouvée par l'ancienne clé (§11.5).
- `ktsGeneration` : entier ≥ 1, incrémenté à chaque rotation ; porté par le client, par l'AAD du scellé et par le registre.
- Il n'existe pas de coffre non enrôlé : un coffre neuf s'enrôle **pendant** sa création (§11.8). Seule une perte du registre de Sésame rend un coffre « à ré-enrôler » (§6.4).

### 10.3 Enveloppe d'un contenu pour un proche

```
aad_env = "fatoom-sesame-env|1|" ‖ uid ‖ "|" ‖ type ‖ "|" ‖ contentId ‖ "|" ‖ contactId ‖ "|" ‖ ktsGeneration
          type ∈ { "fichier", "motdepasse", "mot", "volontes" }
enveloppe = AES-256-GCM(KTS_env, iv aléatoire 12 o, clé du contenu (32 o), aad_env)   → 48 octets
```

Le client range, pour chaque contenu, `enveloppes.{contactId} = {enveloppe, iv}` et `destinataires` = liste des `contactId`, avec l'invariant **`c ∈ destinataires ⇔ enveloppes.c` existe** et une borne de 100 destinataires par contenu. Le client ne peut pas vérifier l'AAD : c'est Sésame qui, à l'ouverture, refuse une enveloppe dont l'AAD ne correspond pas à l'uid, au type, à l'identifiant du contenu, au proche authentifié et à la génération courante de KTS. Une enveloppe copiée d'un contenu à un autre, ou d'un proche à un autre, ne s'ouvre pas. Un contenu dont `destinataires` devient vide perd toutes ses enveloppes : il redevient « gardé pour vous ».

**Types `mot` et `volontes`** (pour Fatoom : mot pour un proche, `contentId = contactId` ; dernières volontés, une enveloppe par proche, `contentId = "courant"`) : aucune construction nouvelle, mêmes enveloppes. Le type `volontes` s'ouvre comme les autres types : pour le **contact de confiance** (charte `role: gardien`), de son vivant compris, et pour les autres proches dans toute transmission arrivée à terme, après décès ou par inactivité.

### 10.4 Charte d'un proche

```
charteJson (canonique) = {
  "v": 1,
  "uid": "<uid>",
  "contactId": "<id>",
  "generation": <int ≥ 1>,
  "role": "proche" | "gardien",
  "codeVerifier": "<base64(SHA-256(\"fatoom-code|\" ‖ normalize(code)))>",
  "telephone": "<E.164>" | null,          # second facteur SMS (obligatoire si role = gardien)
  "email": "<email normalisé>" | null,    # second facteur email
  "prenom": "<prénom tel que saisi>",     # affichage (« Bonjour Léa ») et comparaison d'identité (§11.9)
  "nom": "<nom tel que saisi>" | null,    # exigé pour un proche "courrier"
  "dateNaissance": "<AAAA-MM-JJ>" | null, # exigée pour un proche "courrier" ; sert à la comparaison d'identité
  "adresse": "<adresse en un champ, lignes séparées par \n>" | null,   # scellée : le client ne peut pas rediriger la lettre
  "titulaireAffiche": "<nom du titulaire>",
  "courrier": <bool>,                     # vrai si telephone et email sont null : lettre au décès ; adresse, nom et dateNaissance obligatoires
                                          # faux avec une adresse : lettre de secours seulement (§11.9)
  "creeLe": "<ISO-8601 UTC>"
}
aad_charte = "fatoom-sesame-charte|1|" ‖ uid ‖ "|" ‖ contactId ‖ "|" ‖ generation ‖ "|" ‖ ktsGeneration
charte     = AES-256-GCM(KTS_auth, iv, UTF-8(charteJson), aad_charte)
```

La charte ne porte aucun lien de parenté. `normalize(code)` retire tirets et espaces et met en majuscules. Le code a 12 caractères dans un alphabet sans `0`, `1`, `I`, `O` (60 bits d'entropie), tirés sans biais modulo.

Le client range `charte`, `charteIv`, `charteGeneration` et l'**index du code** :

```
indexAcces = hex(Argon2id(UTF-8(normalize(code)), sel = UTF-8("fatoom-lookup-v1"), m ≥ 64 Mio, t = 3, p = 1))
```

(paramètres exacts figés à l'étude du lot 1 du client), qui sert à retrouver le proche à partir de son code. Il est calculé par l'app à la création et à la régénération du code, et par le navigateur à la saisie (`POST /session/ouvrir {code, lookup}`) ; Sésame ne le calcule jamais sur le chemin chaud et vérifie le code par le `codeVerifier` de la charte. L'étirement rend coûteuse la recherche d'un code sur une copie de la base ; un code trouvé n'ouvre rien sans le second facteur.

Le registre tient `chartes.{contactId}` = génération courante : une charte d'une génération inférieure est refusée, même valide (révocation effective, §11.5). Un `contactId` absent du registre n'accepte que la génération 1 : créer un proche ne sollicite donc pas Sésame ; toute génération ultérieure (révocation, changement de rôle ou de coordonnées, suppression = `-1`) est publiée par l'app avant d'être écrite chez le client.

### 10.5 Messages signés par la clé de vie

Messages app → Sésame (canal §5), **signés Ed25519 par `cleVie`** et vérifiés avec `cleViePub` du registre, **sans rien desceller**. La preuve de vie, l'annulation et la preuve d'enrôlement répondent à un **défi** tiré par Sésame :

```
défi       = POST /defi → {defi: <32 o base64>, expireLe, grappeId}   # tiré par Sésame, usage unique, 5 min, lié au canal
vie        = Ed25519(cleVie, "fatoom-sesame-vie|1|" ‖ uid ‖ JSON canonique {"v":1,"uid","defi"})
annulation = Ed25519(cleVie, "fatoom-sesame-annulation|1|" ‖ uid ‖ "|" ‖ signalementId
                             ‖ JSON canonique {"v":1,"uid","signalementId","defi"})
charteGen  = Ed25519(cleVie, "fatoom-sesame-gen|1|" ‖ uid ‖ "|" ‖ contactId ‖ JSON canonique {"v":1,"uid","contactId","generation":<int>})
enroler    = Ed25519(cleVie, "fatoom-sesame-enroler|1|" ‖ uid ‖ JSON canonique {"v":1, uid, defi, generation, cleViePub})
depot PIN  = Ed25519(cleVie, "fatoom-pin-depot|1|" ‖ uid ‖ "|" ‖ grappeId ‖ JSON canonique, §9.2)
fiche      = Ed25519(cleVie, "fatoom-sesame-alerte|1|" ‖ uid ‖ "|" ‖ generation ‖ JSON canonique de la fiche : `{"email":…,"generation":n,"telephone":…|null,"uid":…,"v":1}`, clés triées, UTF-8, sans espace)
# message signé = chaîne de domaine ‖ JSON canonique (RFC 8785), UTF-8 ; l'alphabet des identifiants exclut « { » :
# la frontière est sans ambiguïté ; encodage exact figé par les vecteurs de test
```

- Sésame n'accepte une preuve de vie, une annulation ou une preuve d'enrôlement que si son `defi` est un défi qu'il a lui-même tiré pour ce canal, non expiré et non encore utilisé ; il l'efface aussitôt. Un défi refusé donne `defi_invalide`. Chaque message n'est valable qu'une fois et seulement juste après sa demande ; aucune preuve de vie ne peut être calculée d'avance.
- `charteGen` n'a pas de défi : la génération strictement croissante du registre suffit à refuser un rejeu. `generation = -1` signifie « proche supprimé » ou « rôle refusé » ; `chartesDerniere.{contactId}` garde alors la dernière génération positive, et seule une génération supérieure est acceptée après un refus du rôle.
- L'**annulation** est la construction de la preuve de vie avec sa propre chaîne de domaine et l'identifiant du signalement : une preuve de vie ordinaire ne peut jamais être prise pour une annulation, ni une annulation rejouée sur un autre signalement. Elle n'est produite que sur le geste « Je suis en vie, annuler la transmission », coffre déverrouillé, sur n'importe quel appareil où le titulaire a ouvert son coffre ; Sésame l'accepte seulement pour le signalement courant non annulé du titulaire. Aucune voie support n'existe.
- La KTS ne sort de KMS que pour une ouverture par un proche (§11.2) et la lettre (§11.9). La charte reste chiffrée et authentifiée sous `KTS_auth`, illisible pour le client.
- La vérification quotidienne `/etat` (coffre verrouillé) n'est pas signée : elle est authentifiée par l'`idToken` seul, n'a aucun effet de bord et ne permet ni preuve de vie ni annulation (§11.10). Aucune clé d'appareil n'est inscrite chez Sésame.

### 10.6 Ce que le client range pour le titulaire

| Champ | Écrit par | Contenu |
|---|---|---|
| `ktsChiffree`, `ktsIv` | app | KTS sous KMU |
| `graineVieChiffree`, `graineVieIv` | app | graine de `cleVie` sous KMU |
| `cleViePub` | app, à la création du coffre | clé publique de `cleVie` ; immuable sauf par une rotation prouvée par l'ancienne clé |
| `sealedKts` | app (valeur renvoyée par `/enroler`), à la création du coffre | KTS scellée par KMS, AAD `uid ‖ "\|" ‖ ktsGeneration` |
| `ktsGeneration` | app | entier ≥ 1, strictement croissant |
| `enroleLe`, `enroleAvec` | app, à la création du coffre | `enroleLe` et `imageDigest` renvoyés par `/enroler` (`API.md`) |
| `preuveDeVieLe` | app | `vivantLe` renvoyé par Sésame : confort d'affichage et repère du client pour planifier les relances du signe de vie ; jamais transmis à Sésame ; la vérité est au registre |

Le client fait respecter : le bloc complet dès la création du coffre ; `ktsGeneration` strictement croissant ; `ktsChiffree`, `ktsIv`, `sealedKts`, graine et `cleViePub` ne changent qu'avec `ktsGeneration`, sous une preuve signée par l'ancienne `cleVie` (rotation).

### 10.7 Ce qui reste en clair chez le client, et pourquoi

| Donnée | Clair ? | Raison |
|---|---|---|
| Charte, enveloppes, KTS, graine de vie | non | §10 |
| Contenus et leurs noms (pour Fatoom : noms de fichiers, services des mots de passe, photos des proches, messages, mots, dernières volontés) | non | chiffrés sur l'appareil |
| Adresse postale, nom, date de naissance des proches | non | la lettre et la comparaison d'identité sont faites par Sésame depuis la charte |
| `destinataires`, dates, tailles, catégories | oui | listes, quotas, limites |
| Coordonnées d'envoi des proches (téléphone, email, prénom) | oui | envois par le client ; la **copie qui fait foi** pour le second facteur est celle de la charte |

La liste complète des métadonnées vues par le client est tenue par lui (`THREAT_MODEL.md`, limite 5).

## 11. Protocoles pas à pas

### 11.1 Destiner un contenu depuis le téléphone (chemin chaud, local)

1. L'utilisateur choisit les proches. Coffre déverrouillé exigé.
2. Pour chaque contenu : l'app déchiffre la clé du contenu puis, pour chaque proche **ajouté**, calcule l'enveloppe (§10.3). Pour un proche retiré, rien à calculer. Clés effacées aussitôt.
3. Un seul appel au client avec, pour chaque contenu, `destinataires` et `enveloppes` (≤ 500 contenus par appel). Interface optimiste avec retour arrière.
4. Le client vérifie : propriétaire, contenu chiffré de bout en bout, chaque `contactId` existe, `|destinataires| ≤ 100`, invariant du §10.3, forme (48 octets, IV 12 octets) ; écrit en transaction ; aucune cryptographie.

Aucun appel à Sésame, aucun réseau autre que le client : destiner 100 fichiers à 100 proches coûte 10 000 AES-GCM et un appel. Les règles commerciales (offre, compte limité) sont appliquées par le client seul.

### 11.2 Ouverture par un proche sur le portail

```
Navigateur (portail)                  Sésame (enclave)                         Client (passerelle)
GET /attestation ───────────────────▶ jwt, sessionPub, kemPub, instanceId
vérifie jwt (liste des versions compilée) ; canal (K_req, K_res)
POST /session/ouvrir {code, lookup} ─▶ lookup calculé par le navigateur (Argon2id, §10.4)
                                      GET bundle?lookup ──────────────────────▶ vérifie l'attestation de Sésame
                                      ◀── {uid, contactId, charte, charteIv, charteGeneration,
                                           sealedKts, ktsGeneration, isUnlocked, titulaireAffiche}
                                      KMS.decrypt(sealedKts) → KTS → KTS_auth ; ouvre la charte
                                      vérifie codeVerifier, generation == registre.chartes[contactId],
                                      ktsGeneration == registre, isUnlocked (code remis)
                                      rôle : gardien ⇒ ok ; proche ⇒ exige un signalement (décès ou inactivité)
                                        transmissibleLe ≤ now, non annulé ; inactivité : sans preuve de vie
                                        reçue après le terme ; décès : définitif
                                      Twilio Verify : envoie le dernier code par SMS si charte.telephone, sinon à charte.email
◀─── {sessionId, canal: "sms", masque: "+33 6 •• •• •• 47"}
saisit le dernier code
POST /session/verifier {sessionId, otp} ─▶ Twilio Verify check ; 5 essais / 15 min
                                      registre.ouvertures += 1 ; notifie le client
◀─── {jeton de session (ES256, 1 h, scope: destine, base: vivant | deces | inactivite), role, expireLe, prenom, titulaireAffiche}
GET liste des contenus (jeton) ──────────────────────────────────────────────▶ vérifie la signature session-sign
◀─── contenus chiffrés (aucun nom en clair) et la seule enveloppe enveloppes[contactId]
POST /session/ouvrir-cles {sessionId, enveloppes: [{type, id, contactId, enveloppe, iv}]} ─▶
                                      pour chaque : vérifie aad_env (uid, type, id, contactId authentifié, génération ;
                                      type volontes comme les autres types)
                                      ouvre sous KTS_env ; efface KTS après réponse
◀─── {cles: [{type, id, cle}], refusees, jeton renouvelé, expireLe} (sous K_res)
GET URL signée (jeton, id) ─────────────────────────────────────────────────▶ URL signée de l'objet chiffré (10 min)
télécharge et déchiffre par blocs dans le navigateur
POST /session/fermer, ou 1 h sans appel ─▶ notifie le client (contenus ouverts pendant la session)
```

- **Un seul scope** : chacun, contact de confiance compris, ne reçoit que les enveloppes à son `contactId`. Il n'existe pas d'accès élargi. La `base` du jeton (`vivant` : contact de confiance avant toute transmission ; `deces` ; `inactivite`, qui le reste jusqu'à la fin des accès quand un décès est vérifié ensuite, §11.3) décide seulement de l'affichage neutre ; les dernières volontés s'ouvrent comme les autres contenus (§10.3). Le contact de confiance peut ouvrir et télécharger de son vivant ce qui lui est destiné ; chaque ouverture est notifiée au client et alertée au titulaire par Sésame (§11.10).
- **Code neutre** : `POST /session/ouvrir` renvoie le **même** refus (`code_refuse`) quel que soit le motif (code inconnu, `codeVerifier` faux, charte révoquée, code non remis, proche avant l'échéance, signalement annulé, preuve de vie reçue après une transmission par inactivité) : un porteur de code n'apprend rien sur l'état du titulaire. Le motif est journalisé.
- **Canaux croisés** : pour un proche dont la charte a un email **et** un téléphone, le client envoie le code d'accès **par email** seulement et Sésame envoie le dernier code **par SMS** : un opérateur d'envoi compromis ne tient qu'un facteur. Proche à email seul : code et dernier code par email ; proche à téléphone seul (dont un contact de confiance sans email) : les deux par SMS ; risque accepté (`THREAT_MODEL.md`, limite 13). Le contact de confiance a un téléphone obligatoire.
- **Code reçu par courrier** (`charte.courrier == true`) : le proche saisit son code d'invitation ; le second facteur est la vérification d'identité menée par Sésame (§11.9). Le client n'intervient pas.
- **Coordonnées perdues** : après `POST /session/ouvrir`, le portail propose « Je n'ai plus ce numéro » (ou « … cet email ») ; Sésame bascule alors la session sur la vérification d'identité du §11.9 (`/session/identite/bascule`), mêmes règles et même blocage, au lieu du dernier code. Une charte sans `nom` ni `dateNaissance` ne permet pas cette bascule (réponse neutre). Seul le titulaire, de son vivant, fixe les coordonnées dans la charte.
- **Session liée à son canal** : Sésame lie chaque session au canal qui l'a ouverte (`instanceId`, `cliPub`) et refuse toute route `/session/*` qui la désigne depuis un autre canal (`canal_invalide`) ; le jeton ne porte pas l'identifiant de session. Les fonctions du client ne peuvent donc pas demander à Sésame, par leur propre canal, les clés d'une session de proche.
- **Durée** : `exp = iat + 1 h`, et chaque `POST /session/ouvrir-cles` réussi renvoie un jeton renouvelé ; sans appel pendant 1 h, la session Sésame expire. Les clés ouvertes vivent dans la mémoire de l'onglet et disparaissent avec lui. `POST /session/fermer` détruit la session côté Sésame.
- **Blocage par le client** : la passerelle peut refuser un `bundle` (pour Fatoom : contact de confiance d'un compte limité au-delà de la période de transmission) ; Sésame répond alors `code_refuse`. Le client peut empêcher une ouverture, jamais en provoquer une (`THREAT_MODEL.md`, limite 14).
- **Refus du rôle** : depuis une session, `POST /session/refuser-role` ⇒ registre `chartes.{contactId} = -1`, client prévenu pour l'email et le retrait du statut. Un refus par un lien envoyé par le client ne passe pas par Sésame : l'app publie ensuite la nouvelle génération de la charte (`role: proche`, code désactivé, §11.5) ; jusque-là, l'ancien code reste ouvrable chez Sésame par ce seul proche.
- Le contact de confiance ne signale pas de décès depuis le portail.

### 11.3 Transmission après décès

1. **Signalement.** Deux sources de décès, `insee` (registre officiel des décès, pour Fatoom : le fichier de l'INSEE) et `acte` (acte de décès vérifié par un administrateur du client avant toute inscription), plus `inactivite` (§11.4). Toutes sont inscrites par le client par `POST /deces/signaler {uid, source, dateDeces, preuve}` (`preuve` = `SHA-256` du fichier de l'acte, obligatoire pour `acte`, gardée au registre), authentifié par le jeton OIDC de son compte de service (§11.3.1). Sésame crée le signalement avec **sa** date (`signaleLe = now`) et fixe lui-même l'échéance `transmissibleLe` (§6.1). Il répond avec un reçu `{signalementId, signaleLe, transmissibleLe}` signé `session-sign`, unique : l'échéance ne change plus. Un second signalement alors qu'un signalement non annulé **de même nature** existe renvoie l'existant. Sésame ne voit pas l'acte et ne le vérifie pas : l'empreinte au registre fige la pièce sur laquelle le client s'est appuyé (`THREAT_MODEL.md`, limite 18). Le droit à transmission (offre, identité) est appliqué par le client seul, qui n'inscrit rien sans droit ; Sésame ne connaît ni l'offre ni l'identité.
2. **Vérification de 7 jours.** Le client prévient le titulaire ; l'app apprend aussi le signalement directement de Sésame (réponses de `/vie` et de `/etat`, signées), et Sésame alerte lui-même le titulaire à J0, J3 et J6 par SMS et email (§11.10). **Le déverrouillage n'annule pas** : il envoie une preuve de vie ordinaire, qui ne touche pas un signalement de décès. Le titulaire annule en touchant « Je suis en vie, annuler la transmission », coffre déverrouillé, sur son téléphone ou sur n'importe quel autre appareil où il ouvre son coffre : l'app envoie l'**annulation** (§10.5) directement à Sésame (`POST /deces/annuler`), qui marque le signalement `annuleLe`, `annulePar: annulation`, répond l'état courant et le notifie au client (`signalementAnnule`). Le client ne peut ni retarder ni supprimer une annulation : elle n'a pas transité par lui. Après l'échéance, `/deces/annuler` répond `deja_transmis`.
3. **Échéance.** Le client envoie les codes d'accès aux proches qui ont un email ou un téléphone ; **Sésame** poste la lettre aux proches « courrier », et la lettre de secours quand le client lui signale l'échec technique de l'email et du téléphone d'un proche (§11.9). Sésame n'ouvre pour un proche que si `transmissibleLe ≤ now` selon **son** horloge et si aucune annulation n'est arrivée pour ce signalement. Les conditions sont indépendantes : un client qui envoie les codes trop tôt n'ouvre rien.
4. **Ouverture par un proche** : §11.2, rôle `proche`, base `deces` ; même parcours pour le contact de confiance ; les dernières volontés s'ouvrent pour tous.
5. **La transmission sur décès est définitive.** Après l'échéance, ni un déverrouillage, ni une preuve de vie, ni une annulation ne coupent les ouvertures (`THREAT_MODEL.md`, limite 7). La suppression des contenus gardés et du coffre est pilotée par le client.
6. **Suppression du titulaire** : le client demande à Sésame `POST /titulaire/supprimer {uid}` (jeton OIDC), qui passe la ligne du titulaire à `supprime` (pierre tombale, §6.2) : plus rien ne s'ouvre, ne s'enrôle ni ne s'évalue pour cet `uid`.
7. **Décès vérifié après une transmission par inactivité** : Sésame accepte un signalement `deces` même si un signalement `inactivite` a déjà été transmis ; il ne renvoie l'existant que pour un signalement de même nature non annulé. À son terme, rien n'est renvoyé aux proches, qui ont déjà tout reçu ; la base du jeton de session reste `inactivite` jusqu'à la fin des accès (ton neutre sur le portail) ; la suppression des contenus gardés et la durée d'accès des proches sont pilotées par le client.

**Comment Sésame sait que le décès est vérifié sans faire confiance au client seul.** Il ne vérifie ni le registre officiel ni l'acte : il **mesure** les 7 jours avec sa propre horloge et exige **l'absence d'annulation**, qui ne transite jamais par le client. Un client compromis peut au pire déclencher une fausse alerte ; il ne peut pas transmettre si le titulaire touche « Je suis en vie » dans les 7 jours. Un titulaire qui ouvre son app sans toucher ce bouton est transmis.

#### 11.3.1 Jeton OIDC du client

Les routes appelées par le serveur du client (`/deces/signaler`, `/titulaire/supprimer`, `/lettre/secours`) sont authentifiées par un jeton OIDC de son compte de service, qui voyage **dans le corps chiffré**, jamais en en-tête (l'en-tête traverse le répartiteur en clair). Sésame ne l'accepte que si `iss` vaut `https://accounts.google.com`, `aud` vaut l'adresse publique de Sésame sans chemin (pour Fatoom : `https://sesame.fatoom.fr`), `email` est l'adresse du compte de service attestée dans `env_override` (§2.3) avec `email_verified` vrai, la signature est vérifiée avec les clés publiques de Google et `exp` n'est pas passé ; sinon `jeton_invalide`.

### 11.4 Signe de vie et transmission par inactivité

Option du client (pour Fatoom : offre Sérénité). Elle réutilise la preuve de vie (§10.5), le registre (§6) et le parcours du portail (§11.2) : aucun format nouveau. **Le client** tient le réglage (délai de 3, 6 ou 12 mois), relance et inscrit le signalement ; **Sésame** mesure les 7 derniers jours et refuse d'ouvrir après une preuve de vie qu'il a reçue.

1. **Un seul signe de vie : le déverrouillage.** Seul un déverrouillage réussi de l'app, qui envoie la preuve de vie signée par `cleVie`, remet le délai à zéro. Il n'existe ni bouton, ni lien, ni relais par le client : le client ne relaie rien vers Sésame pour le signe de vie et ne peut pas en fabriquer.
2. **Relances** par le client à partir de l'échéance du délai (pour Fatoom : J0, J7, J14, J30) ; elles demandent seulement d'ouvrir et de déverrouiller l'app.
3. **Signalement d'inactivité** (J30) : le client appelle `POST /deces/signaler {uid, source: "inactivite"}` ; Sésame crée un signalement `deces: false` et fixe l'échéance (J37, 23 h 59 heure de Paris). Sésame **refuse** l'inscription si sa dernière preuve de vie (`vivantLe`, **initialisé à `enroleLe`**) date de moins de **3 mois + 30 jours**, plus courte combinaison possible du réglage : le client ne peut pas transmettre un titulaire actif, ni un compte récent jamais déverrouillé. Sésame ne connaît pas le délai choisi (`THREAT_MODEL.md`, limite 15).
4. **Preuve de vie à n'importe quelle étape** : reçue par Sésame après `signaleLe`, elle annule le signalement d'inactivité (`annulePar: preuve_de_vie`) et Sésame le notifie au client (`signalementAnnule`), qui n'envoie aucun code.
5. **Transmission** (J37) : le client envoie les codes avec un message neutre ; Sésame poste la lettre des proches « courrier » avec un texte neutre (gabarit attesté comme celui de la lettre de décès, §2.3). Sur le portail, la base du jeton est `inactivite` et les dernières volontés s'ouvrent comme les autres contenus.
6. **Retour du titulaire** : la première preuve de vie après l'échéance coupe les ouvertures suivantes (notification `lifeAfterTransmission`, propre à la transmission par inactivité) ; la réponse de `/vie` porte `transmis`, vrai dès l'échéance passée d'un signalement non annulé, et les ouvertures, pour montrer qui a ouvert quoi.
7. **Suspension** : le client n'envoie aucune relance et n'inscrit aucun signalement d'inactivité quand l'option n'est plus active, ou tant que l'app signale un appareil qui ne peut pas émettre de preuve de vie (système ou app sous la version minimale, §11.6) : un blocage, jamais une ouverture.
8. **Décès vérifié ensuite** : §11.3, étape 7.

### 11.5 Révocation, régénération, remplacement du contact de confiance, rotation de KTS

| Action | Sur le téléphone | Vers Sésame (canal, bloquant, rare) | Vers le client |
|---|---|---|---|
| **Régénérer le code** | nouveau code, nouvelle charte `generation + 1` | `charteGen(generation + 1)` ; échec ⇒ rien n'est écrit | remplace la charte et l'index (ancien supprimé), code marqué non remis ; envoi du code si le titulaire le renvoie |
| **Révoquer l'accès** | idem, code non redistribué de son vivant | idem | idem, sans envoi |
| **Désigner un contact de confiance** (à la création du proche) | charte `role: gardien` (génération 1) avec `telephone` obligatoire | aucun (génération 1, §10.4) | proche, statut, envoi du code |
| **Remplacer le contact de confiance** | l'ancien reçoit une charte `role: proche` avec un nouveau code non remis de son vivant ; le nouveau, sa charte `gardien` | `charteGen` pour l'ancien | charte de l'ancien, création du nouveau, statut |
| **Changer le téléphone ou l'email d'un proche** | nouvelle charte (mêmes code et rôle) : la copie qui fait foi pour le second facteur est la charte | `charteGen` | charte et coordonnées |
| **Retirer un destinataire d'un contenu** | enveloppe retirée | rien | destinataires du contenu |
| **Supprimer un proche** | — | `charteGen(-1)` | retire `enveloppes.{contactId}` et `destinataires` de tous les contenus, supprime l'index et le proche |
| **Rotation de KTS** (sur demande du titulaire, après le PIN ou la biométrie) | nouvelle KTS, `ktsGeneration + 1`, nouvelle graine de `cleVie` ; **toutes** les enveloppes et chartes re-chiffrées | `/enroler` avec la nouvelle génération et une preuve signée par la `cleVie` courante (§11.8) ; le registre passe `ktsGeneration` et `cleViePub` | écriture en lots, sous une preuve signée par l'ancienne `cleVie` ; les anciennes enveloppes deviennent inutilisables (AAD) |

Toute action sur une charte passe **d'abord** par Sésame : tant que le registre n'a pas la nouvelle génération, rien n'est écrit chez le client. Si Sésame est injoignable, l'action échoue proprement et l'ancien code reste valide : on ne laisse jamais un état où le client croit l'accès révoqué alors que Sésame l'ouvrirait encore. Une récupération du coffre par le code de récupération rend la même KMU et la même KTS : aucune rotation n'est imposée.

### 11.6 Versions de Sésame : liste compilée, mise à jour forcée, refus de l'inconnu

```
versions.json = {                          # ce dépôt, branche main protégée
  "v": 1,
  "versions": [
    {"version": "1.2.0", "imageDigest": "sha256:…", "referenceImage": "<dépôt d'images>/sesame@sha256:…",
     "envDigest": "sha256:…", "hwmodels": ["GCP_AMD_SEV", "GCP_INTEL_TDX"], "tag": "v1.2.0", "commit": "<sha git>", "publieLe": "<ISO>",
     "statut": "en_service" | "retiree", "retireLe": "<ISO>" | null, "notes": "<fr, une phrase>"}
  ],
  "clesSession": {"session-sign/<version KMS>": {"cle": "<base64, ECDSA P-256 publique>", "depuis": "<ISO>"}}
}
```

- **Liste compilée** : `versions.json` ne change que par un commit sur `main` (historique public). La CI de l'app, du portail et de la passerelle du client le recopie au build : ils ne téléchargent rien et ne vérifient aucune signature ; la clé publique `session-sign` est lue par `kid` dans `clesSession` de la liste des versions compilée ; une version n'est acceptée que par une version de l'app, du portail ou de la passerelle qui la contient. Un `imageDigest` absent ou `retiree`, ou un `hwmodel` hors de `hwmodels` ⇒ attestation refusée. La passerelle applique la même règle aux appels de Sésame. Il n'y a pas de délai d'annonce : la revue des boutiques et l'historique public de ce dépôt en tiennent lieu (`THREAT_MODEL.md`, limite 12).
- **Ancre de confiance** : les comptes qui publient l'app (boutiques), le portail et la passerelle, et ce dépôt (GitHub), connectés par passkey, jamais de second facteur par SMS. Un vol des accès Google Cloud seul ne change aucune de ces listes : une image autorisée en douce dans KMS reste refusée par les clients et laisse sa trace dans les journaux exportés (§8).
- **Mise en service par mise à jour forcée** (`RELEASE.md`) : (1) tag signé par la clé SSH GitHub du porteur du produit et build reproductible, ligne `en_service` ajoutée à `versions.json` ; (2) versions de l'app, du portail et de la passerelle qui la contiennent, **publiées et disponibles dans les deux boutiques avant** toute mise en service ; (3) l'IAM KMS autorise l'empreinte (la liste d'abord, l'IAM ensuite) et la nouvelle version démarre ; (4) la **version minimale de l'app** est relevée à la première version qui la contient : une app plus ancienne impose la mise à jour à sa prochaine ouverture ; (5) l'ancienne version est arrêtée (effacement de sa grappe, redépôt dans la nouvelle, §9.3). Aucune période de recouvrement n'est prévue. Jamais deux versions à moins de 30 jours d'écart, sauf retrait d'urgence (§9.3).
- **Téléphone trop ancien** : sur un appareil dont le système est sous la version minimale exigée par la dernière app, l'app laisse ouvrir et consulter le coffre localement, sans aucun appel à Sésame (preuve de vie, annulation, changement de PIN, droits des proches, clé de transmission indisponibles sur cet appareil ; ils se font depuis un appareil à jour). **Le signe de vie est alors suspendu** (§11.4, étape 7) : un appareil qui ne peut pas émettre de preuve de vie ne déclenche pas de transmission par inactivité.
- **Retrait d'urgence** : ligne passée `retiree`, empreinte retirée de KMS **immédiatement** (la version ne descelle plus rien), version minimale de l'app relevée, VM de la version retirée arrêtées dès que la nouvelle est en service.
- **Matériel admis** : `hwmodels` liste les valeurs de `hwmodel` acceptées par les clients, la passerelle et la condition KMS.
- **Cohérence KMS ⇔ liste** : le contrôle CI de ce dépôt lit la condition IAM et échoue si l'ensemble des empreintes autorisées, la clause `dbgstat` ou le dictionnaire `env_override` ne correspond pas aux versions `en_service` et à leur `envDigest`.
- **Historique** : la page de transparence du client affiche `versions.json` (version en service, date, empreinte, statut) et son historique Git ; ce dépôt publie, pour chaque version, le journal de construction reproductible.

### 11.7 Échec d'attestation

| Où | Quand | Comportement |
|---|---|---|
| App, création du coffre ou nouveau PIN après une récupération | jeton invalide, empreinte hors de la liste des versions, Sésame injoignable ou en pause, `/enroler` ou dépôt du PIN sans réponse, `grappe_recente` | message d'échec avec « Réessayer » ; **rien n'est écrit** chez le client ; à la création, le compte reste sans coffre ; après une récupération, le code de récupération qui vient de servir reste valide. |
| App, ouverture sur un nouvel appareil ou changement de PIN | idem, ou `pin_indisponible_temporaire`, `grappe_recente`, `reessayer_plus_tard` (contrôle anti-retour au changement de grappe) | message d'échec avec « Réessayer » et le **code de récupération** toujours proposé ; aucun essai décompté ; changement de PIN : l'ancien reste valide. `pin_indisponible` : code de récupération, ou ouverture sur un appareil connu qui redépose ; ligne du registre rejetée : code de récupération seul jusqu'à la reprise publiée. |
| App, enrôlement ou rotation | jeton invalide, empreinte hors de la liste des versions, Sésame injoignable | « Empreinte attendue » (celle de la version en service dans la liste compilée) / « Empreinte reçue » (celle du jeton, ou « Non vérifiable ») ; « Réessayer » ; « Rien n'a été exposé » : rien n'a été envoyé. Le coffre reste utilisable, sans destination nouvelle. |
| App, action sur une charte (§11.5) | idem | Même écran avec le titre de l'action ; rien n'est écrit, l'état précédent reste cohérent. |
| App, preuve de vie (arrière-plan) | idem | Silencieux ; nouvel essai au prochain déverrouillage ; après 3 jours sans succès, bandeau discret (« Sésame injoignable depuis le … »). Jamais bloquant ; pendant une relance du signe de vie, nouvel essai à chaque ouverture. |
| App, annulation d'un décès signalé | idem | L'annulation n'aboutit que si Sésame l'accepte : sinon l'échec s'affiche avec « Réessayer », et rien n'est dit annulé. |
| App, destiner un contenu | jamais : opération locale | — |
| Portail | idem | « Connexion non vérifiée » : aucune saisie de code n'est proposée tant que l'attestation n'est pas vérifiée ; lien « Pourquoi cette vérification ? » vers la page de transparence. |
| Passerelle | appel de Sésame avec un jeton invalide | `401`, journal avec `imageDigest`. |

### 11.8 Enrôlement d'un coffre

Un coffre neuf s'enrôle **pendant** sa création : KTS, `cleVie` et le code de récupération sont tirés avec la KMU ; `/enroler` puis le dépôt `nouveau_pin` (§9.2) ont lieu **avant** l'écriture unique qui crée le coffre chez le client. Sans Sésame, pas de coffre : il n'existe pas de coffre non enrôlé, et le bloc du §10.6 est complet dès cette écriture.

| Étape | Ce qui se passe |
|---|---|
| 1 · Vérification de Sésame | `GET /attestation` ; empreinte comparée à la liste des versions compilée ; jeton vérifié (§3) ; canal ouvert. Échec ⇒ §11.7. |
| 2 · Clés | KTS tirée ; pour un ré-enrôlement après perte du registre ou une rotation : enveloppes recalculées sous `KTS_env` et, pour chaque proche, nouvelle charte. Tout sur l'appareil. |
| 3 · Envoi sécurisé | `POST /enroler {uid, idToken, ktsGeneration, kts, cleViePub, preuve?: {defi, signature}}` (`kts` = les 32 octets de KTS, en clair à l'intérieur du corps chiffré par le canal) ⇒ Sésame vérifie l'`idToken` (clés publiques de l'émetteur configuré ; pour Fatoom : Firebase Auth), crée le marqueur d'inscription (premier enrôlement, §6.2), scelle KTS (`sealedKts`), crée ou met à jour `titulaires/{uid}` (`ktsGeneration`, `cleViePub`, `vivantLe = enroleLe` ; les chartes ne sont pas touchées) et renvoie `{sealedKts, ktsGeneration, imageDigest, cleViePub, enroleLe}`. |

**Enrôlement prouvé** : le premier enrôlement (aucune ligne au registre) n'exige que l'`idToken`. Tout ré-enrôlement ou rotation (une ligne existe) exige en plus une **preuve signée par la `cleVie` courante** sur un défi (§10.5), avec la génération : supérieure pour une rotation ; **égale** pour la reprise d'une création interrompue (rescellement seul : la preuve doit se vérifier avec la `cleViePub` déjà inscrite, la `cleViePub` reçue doit lui être égale, ni `chartes` ni `grappePin` ne changent) ; inférieure refusée (`generation_non_croissante`). Sans preuve valide : `preuve_requise`. Le client, qui peut fabriquer un `idToken`, ne peut donc pas faire sceller une KTS de son choix pour un titulaire déjà enrôlé.

**Coffre enrôlé par un autre** : `/enroler` renvoie la `cleViePub` reçue ; une réponse `preuve_requise` à un premier enrôlement, ou une `cleViePub` renvoyée différente de celle de l'app, signifie que quelqu'un d'autre a enrôlé ce coffre : l'app le signale et ne propose aucune destination (`THREAT_MODEL.md`, limite 24).

**Coffre perdu** (ni PIN ni code de récupération) : aucune voie de ré-enrôlement ; une requête sans preuve pour un titulaire déjà enrôlé est toujours refusée. Ce qui est déjà destiné reste transmissible (`THREAT_MODEL.md`, limite 20).

La fiche d'alerte (§11.10) est envoyée après la création du coffre, sans bloquer, et retentée à chaque déverrouillage tant qu'elle n'a pas réussi.

### 11.9 Proche sans téléphone ni email : lettre et vérification d'identité, par Sésame

Tout se passe dans Sésame, pour que le client ne connaisse ni le code de la lettre ni le résultat de l'identité. Le code reste petit : trois appels REST pour Pingen, deux pour Stripe, et une lettre PDF texte.

**Lettre.** Une tâche interne de Sésame (toutes les heures) parcourt les signalements `transmissibleLe ≤ now`, non annulés, sans `lettresEnvoyeesLe` :

1. `GET bundle?uid=…&courrier=1` à la passerelle : les chartes dont `courrier == true` (le client ne peut que les retenir, jamais les modifier : déni de service visible, `THREAT_MODEL.md`, limite 8).
2. Pour chaque charte : ouvre la charte (KTS via KMS), tire le **code d'invitation** (12 caractères, même alphabet que le code d'accès, `4-4-4`), écrit `invitations/{lookup}` (`lookup` = même Argon2id que l'index du code d'accès, calculé une fois par lettre par Sésame, avec le `codeVerifier` du code) et `lettres/{contactId}` au registre.
3. Produit la lettre : **PDF texte d'une page**, A4, police standard Helvetica (encodage WinAnsi, accents français, aucune police embarquée), adresse du destinataire dans la zone fenêtre gauche définie par Pingen, texte fixe en français pris dans un fichier de l'image (qui écrit, pourquoi, le code en grand, l'adresse du portail, la vérification d'identité à prévoir, rappel que le client ne demande jamais rien d'autre ; texte neutre pour une transmission par inactivité), nom du titulaire et prénom du proche depuis la charte. Générateur maison d'environ 150 lignes, déterministe (test par comparaison d'octets), sans navigateur sans tête ni bibliothèque PDF.
4. Pingen (**compte** Pingen de Sésame ; API `https://api.pingen.com`, jeton OAuth2 *client credentials* sur `https://identity.pingen.com`) : `GET /file-upload` (renvoie `url` et `url_signature`), `PUT` du PDF brut vers `url`, puis `POST /organisations/{organisationId}/deliveries/letters` (`file_original_name`, `file_url`, `file_url_signature`, `auto_send: true`, `address_position: left`, `delivery_product: cheap`, `print_mode: simplex`, `print_spectrum: grayscale`). L'adresse n'est pas un paramètre : Pingen la lit dans la zone fenêtre de la première page. Enregistre `pingenDocumentId`, `statut: commandee`, `envoyeeLe`.
5. Notifie le client (`letterSent {uid, contactId, pingenDocumentId}`) pour l'historique des envois, **sans** le code.

La même tâche relit chaque heure les lettres non `distribuee` chez Pingen (`GET /organisations/{organisationId}/deliveries/letters/{letterId}` ; pas de webhook, pour n'ouvrir aucune route entrante à un tiers sur l'enclave) et notifie le client des changements (`letterStatus`) ; Pingen ne publie pas de liste fermée de ses états : leur correspondance avec les cinq statuts du registre se fixe à l'étape des fournisseurs (`README.md`, « État du projet »). Une `erreur` déclenche un nouvel envoi (nouvelle lettre, même code) le lendemain, 3 fois au plus, puis `letterFailed` (support). Renvois : si `ouverteLe` est vide à J+15 puis J+45, une nouvelle lettre portant le **même** code est postée (`renvoyeeLe[]`). Le code expire 12 mois après la transmission.

**Lettre de secours.** Un proche qui a un email ou un téléphone peut aussi avoir une adresse dans sa charte (`courrier: false`) : elle ne sert qu'en secours. Quand le client constate l'**échec technique** de l'email **et** du téléphone de ce proche (rebond de l'email, SMS non distribuable, après ses propres nouveaux essais ; l'absence d'ouverture ne compte pas), il le signale à Sésame (`POST /lettre/secours {uid, contactId}`, jeton OIDC, §11.3.1) ; Sésame vérifie que la transmission est ouverte (signalement à terme, non annulé) et que la charte a une adresse, puis suit les étapes 2 à 5. Le client ne peut que déclencher une lettre vers l'adresse scellée par le titulaire, jamais la rediriger. Le format de l'adresse en lignes se confirme avec Pingen.

**Ouverture.** `POST /session/ouvrir {code, lookup}` : Sésame cherche d'abord `invitations/{lookup}` et vérifie le code par son `codeVerifier` ; trouvé ⇒ parcours « courrier » :

1. `POST /session/identite/demarrer {sessionId}` : Sésame crée une session Stripe Identity (`type: document`, `options.document.require_matching_selfie: true`, aucune donnée du proche transmise à Stripe) avec sa clé restreinte et renvoie le `client_secret` ; le navigateur ouvre le module Stripe (consentement explicite au selfie affiché avant).
2. `POST /session/identite/verifier {sessionId}` : Sésame relit la session chez Stripe (`expand[]=verified_outputs` et `expand[]=verified_outputs.dob` : la date de naissance n'est rendue que sur demande explicite, et par une clé restreinte seulement dans les 48 h qui suivent la vérification). `status == verified` ⇒ comparaison avec la charte sur les **trois** champs : nom **égal** au `last_name` lu sur la pièce ; au moins un prénom de la pièce égal à `prenom` ; `dateNaissance` égale. Comparaison **tolérante** des noms et prénoms (sans accents, casse, espaces, tirets et apostrophes normalisés), jamais de la date. Succès ⇒ `identiteVerifieeLe`, jeton de session (§11.2), suite identique au parcours SMS. Échec de correspondance ou `requires_input` ⇒ message neutre (« Nous n'avons pas pu confirmer votre identité »), `essaisIdentite + 1`, blocage 24 h au troisième échec, `letterIdentityFailed` notifié au client pour le support.
3. Les ouvertures suivantes dans les 12 mois ne refont pas la vérification d'identité : `identiteVerifieeLe` présent ⇒ jeton de session directement après le code (la lettre reste le seul facteur possédé ; `THREAT_MODEL.md`, limite 4).

La même comparaison sert à la bascule « je n'ai plus ce numéro / cet email » (§11.2). Ce que Stripe voit : la pièce et le selfie du proche. Ce que le client voit : qu'une lettre est partie et son suivi ; jamais le code, jamais le résultat de l'identité. Ce que Pingen voit : la lettre, donc le code.

### 11.10 Alertes envoyées par Sésame

Sans elles, toutes les alertes au titulaire passeraient par le client : un client compromis, ou une erreur honnête (homonyme dans le registre officiel, envoi cassé), pourrait laisser passer un faux décès chez un titulaire qui n'ouvre pas l'app pendant 7 jours, et cacher les ouvertures du contact de confiance.

- **Fiche d'alerte** : à la création du coffre (après l'écriture qui le crée, envoi non bloquant retenté à chaque déverrouillage) et à chaque changement de son téléphone ou de son email, l'app envoie à Sésame, sous le canal, `POST /alerte/fiche {uid, generation, telephone, email, signature}`, signé par `cleVie` sur `"fatoom-sesame-alerte|1|" ‖ uid ‖ "|" ‖ generation` et le JSON canonique de la fiche (`{"email","generation","telephone","uid","v":1}`, clés triées, sans espace) ; la génération est strictement croissante. **Sésame scelle lui-même** la fiche sous `alerte-seal` (AAD `"fatoom-sesame-alerte|1|" ‖ uid ‖ "|" ‖ generation`) et la garde **dans son registre** (`ficheScellee`). Le client n'en garde rien : il ne peut ni la lire, ni la rediriger, ni la retenir.
- **Ce que Sésame envoie lui-même**, par ses comptes Twilio et SendGrid distincts de ceux du client (SMS par Programmable Messaging, `POST /2010-04-01/Accounts/{sid}/Messages.json` ; emails par l'API Mail Send de SendGrid, `POST /v3/mail/send` ; Twilio Verify ne peut pas porter ces textes), **en plus** des envois du client, avec des textes attestés pris dans l'image :
  - un signalement de décès ou d'inactivité inscrit : à l'inscription, puis à J3 et J6 ;
  - une ouverture par le contact de confiance de son vivant : une fois par session ;
  - **chaque essai de PIN** sur un nouvel appareil, réussi ou non (§9.2), au plus un message par heure : « Le … à …, votre code PIN a été saisi pour ouvrir votre coffre-fort Fatoom sur un appareil où il n'était pas ouvert (nouveau téléphone, ou le vôtre après une déconnexion). Si c'est vous, rien à faire. Sinon, changez votre code PIN depuis votre téléphone. » (texte de Fatoom) ;
  - une ligne de ce titulaire rejetée par le contrôle de fraîcheur (`ligne_rejetee`, §6.3).
  Ces envois ouvrent la fiche par un seul déchiffrement KMS d'`alerte-seal`, sans desceller la KTS ni rien demander au client. Un titulaire sans fiche (téléphone et email absents) ne reçoit rien de Sésame.
- **Registre plus ancien que la dernière borne** (§6.3) : événement global, l'alerte part à l'**exploitant** et est publiée sur la page de transparence du client ; elle ne part pas aux titulaires.
- **Alertes réessayées, échéance inchangée** : si Twilio ou SendGrid refuse un envoi ou ne répond pas, Sésame le réessaie pendant **24 h** avec un délai croissant, puis journalise l'échec et le publie dans sa borne (`alerte_refusee`, ressource `uidMasque ‖ "/" ‖ signalementId`, jamais l'uid, §8.1). `transmissibleLe` ne bouge pas : le mécanisme échoue **ouvert**, un signalement dont aucune alerte de Sésame n'est partie se transmet quand même à son échéance. Un exploitant qui coupe la sortie de Sésame vers Twilio et SendGrid, ou suspend ses comptes chez le fournisseur, supprime les alertes de Sésame, pas celles du client ni les réponses de `/vie` et `/etat` (`THREAT_MODEL.md`, limite 8).
- **Vérification en arrière-plan** : une fois par jour et à chaque ouverture de l'app, l'app appelle `POST /etat`, authentifié par l'`idToken` seul, coffre verrouillé (au plus 100 appels par jour et par uid). La requête porte le repère de la dernière ouverture que l'app a déjà vue (un repère tenu par Sésame avancerait à chaque requête rejouée par le client et masquerait des ouvertures). Sésame répond, signé par `session-sign`, le signalement courant éventuel, les ouvertures postérieures à ce repère, `pinPresent`, `pinBloque` et `evaluationsDepuis` ; l'app affiche une notification locale sans dépendre du client. La réponse à `/vie` porte aussi les ouvertures.
- Aucune alerte n'est envoyée par Sésame pour une suppression du compte demandée au client sans PIN.
- Coût : quelques SMS et emails par titulaire et par an.
