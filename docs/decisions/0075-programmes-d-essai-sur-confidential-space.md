<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), essais restants de Confidential Space -->

# 0075 · Programmes d'essai publics et jetables pour une séance sur Confidential Space

**Date** : 2026-11-11 · **Statut** : actée (séance acceptée par l'opérateur ; écriture des programmes déléguée)

## Contexte et menace

Trois faits que la spécification tient pour acquis ne se vérifient que sur une vraie VM Confidential Space :

- **l'horloge** : les 7 jours d'un signalement et le ± 2 min du contrôle de fraîcheur (`ARCHITECTURE.md` §2.2, « Heure », et §6.3) supposent une horloge de VM tenue par le serveur de métadonnées, y compris pendant une migration à chaud (`THREAT_MODEL.md`, limite 10) ;
- **le jeton d'attestation** : les clients vérifient un jeton PKI (`ARCHITECTURE.md` §3) dont les vecteurs ne portent qu'une chaîne fabriquée sous une racine de test ; un écart de forme entre un vrai jeton et la vérification ferait refuser Sésame par chaque client le jour de la mise en service ;
- **le débit de `chaine/tete`** : chaque écriture du registre relit et réécrit ce document unique (§6.2) ; Firestore limite le débit soutenu d'écriture d'un document, et `sesame-fake` tient son registre en mémoire : il ne peut pas le mesurer.

Mesurer cela avec le code de l'image demanderait les adaptateurs réels (étapes 3 et 4) et l'infrastructure (étape 5). L'opérateur a accepté une séance unique, sur un projet Google Cloud d'essai, avec un budget d'alerte, avant ces étapes.

## Décision

- **Un programme d'essai public**, `essais/cmd/essai` et `essais/internal/essai`, dans le module de ce dépôt, bibliothèque standard seule ; une image `essais/Dockerfile` sur les mêmes images de base épinglées que le `Dockerfile` de `cmd/sesame`, sans shell, et une marche à suivre pour l'opérateur, `essais/README.md`, une commande par bloc pour Cloud Shell, du projet créé au projet supprimé. Le programme sert en HTTP trois routes qui rendent du JSON :
  - `/horloge` : écart et délai SNTP (RFC 4330) sur le serveur de métadonnées (`metadata.google.internal` ou `169.254.169.254`, UDP 123), mesurés par le programme lui-même, sans `chronyc` ni shell ; une mesure par minute dans son journal pour suivre une migration à chaud (`gcloud compute instances simulate-maintenance-event`) ;
  - `/attestation` : jeton PKI demandé au lanceur par `POST http://localhost/v1/token` sur `/run/container_launcher/teeserver.sock`, corps `{"audience", "token_type": "PKI", "nonces"}`, audience d'au plus 512 octets, au plus 6 nonces de 10 à 74 octets, réponse lue telle quelle (interface lue sur la documentation publique de Google : <https://docs.cloud.google.com/confidential-computing/confidential-space/docs/connect-external-resources>) ; le jeton brut est rendu pour que le client de référence le vérifie ;
  - `/firestore` : transactions qui relisent et réécrivent `chaine/tete` et créent une ligne en création seule, à débit visé pendant une durée donnée, depuis trois VM ensemble ; débit tenu, transactions rejouées, échecs, collisions, latences. Firestore est appelé par son API REST, `net/http` et le jeton d'accès du serveur de métadonnées : le SDK Firestore n'est pas nommé par chemin de module dans `ARCHITECTURE.md` §2.4, et un programme jetable n'y fait entrer aucun module.
- **Image de production** de Confidential Space (`confidential-space`) pour les trois essais : le jeton doit porter `dbgstat` `disabled-since-boot`. Les journaux du conteneur vont à Cloud Logging par la politique `tee.launch_policy.log_redirect=always` de l'image d'essai ; ils ne portent que des compteurs et des mesures, jamais un jeton.
- **Ce que ce n'est pas** : ni le lanceur réel de Sésame, ni un adaptateur de l'image, ni du code des décisions. `cmd/sesame` et `cmd/sesame-fake` ne le lient pas, et le paquet est sous `essais/internal/` : aucun autre paquet du dépôt ne peut l'importer. Il n'entre donc ni dans le budget (`outils/decompte-lignes`, décision 0074), ni dans le contrôle des dépendances de `cmd/sesame` ; il est testé par `go test ./...` comme le reste (SNTP contre un serveur local, socket du lanceur et Firestore simulés), et la CI contrôle l'épinglage de son `Dockerfile`. Aucun job de la CI ne parle à Google.
- **Rien du déploiement** : la marche à suivre ne lit que des variables (`PROJET`, `FACTURATION`, `REGION`, `COMMIT`…) que l'opérateur pose dans Cloud Shell ; ni nom ni numéro réel n'est écrit ici (`outils/controle-noms`). Les relevés reviennent à l'opérateur ; ce qu'ils mesurent s'écrit dans la spécification, puis dans les documents de ce dépôt.
- **Restent à l'étape de l'infrastructure** : la grappe du PIN sur trois vraies VM (coupure par le pare-feu, migration à chaud du leader, recréation d'une VM par son groupe zonal, passage à TDX) et les journaux écrits par Sésame lui-même (Cloud Logging, STS, politique réseau de la VM) : ils demandent le lanceur réel de l'image et ses adaptateurs.

## Alternatives écartées

- **Attendre l'image et ses adaptateurs** : un écart de forme du jeton ou un débit insuffisant de `chaine/tete` changerait le format ou le registre après l'écriture des adaptateurs, au lieu d'avant.
- **Programmes privés, hors de ce dépôt** : ce qui fonde une mesure publiée doit pouvoir être relu et rejoué par chacun ; un programme jetable public ne publie rien du déploiement.
- **SDK Firestore pour Go** : un module nouveau, et ses dépendances transitives, pour un programme jetable ; l'API REST suffit à mesurer le débit d'un document.
- **Image de débogage** (`confidential-space-debug`) : son jeton porte un autre `dbgstat`, que les clients refusent ; elle ne servirait qu'à comprendre un conteneur qui ne démarre pas.
- **`chronyc` ou un outil NTP dans l'image** : l'image n'a pas de shell, et l'image de Sésame non plus ; la mesure se fait comme Sésame la ferait, depuis le conteneur.

## Conséquences et limites

- Le programme n'a pas tourné sur Google Cloud avant la séance ; `essais/README.md` dit, commande par commande, d'où vient sa forme et où un ajustement peut être nécessaire.
- Une VM Confidential Space SEV ne protège pas l'intégrité de la mémoire (`THREAT_MODEL.md`, limite 1) ; l'horloge de l'invité vient de l'hyperviseur : la mesure dit la dérive d'une VM honnête, pas ce qu'un hyperviseur hostile pourrait faire.
- Le débit mesuré est celui d'une base d'essai vide, à un instant donné ; il fonde une marge, pas une garantie.

## Documents concernés

`essais/README.md` ; `ARCHITECTURE.md` §2.2 (« Heure »), §2.4 ; `CONTRIBUTING.md` (« Outils et commandes ») ; `README.md` (« État du projet ») ; `.github/workflows/ci.yml`.
