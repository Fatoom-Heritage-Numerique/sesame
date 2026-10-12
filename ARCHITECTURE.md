<!-- Source : fatoom-app docs/TRD.md §16.1, §16.3, §16.4, §16.5 -->

# Architecture de Sésame

Sésame est un service de garde de clés de transmission : il scelle, pour chaque titulaire d'un coffre-fort chiffré de bout en bout, une clé de transmission, et il ne la rouvre que dans deux cas vérifiés par lui seul : la transmission après un décès dont il tient lui-même le compte des 7 jours, ou l'ouverture par le contact de confiance qui a prouvé auprès de lui son code d'accès et son second facteur. Il tourne dans une enclave Google Cloud Confidential Space, au code public, et les clés qu'il rouvre repartent chiffrées pour le navigateur du proche.

Ce document décrit le service indépendamment de son client. Fatoom est le premier client ; quand une valeur lui est propre, elle est donnée en exemple et signalée comme telle. La spécification de référence est tenue par l'opérateur ; ce dépôt en est la reprise publique et fait foi pour le code.

Conventions : « app » = le téléphone du titulaire, coffre déverrouillé ; « portail » = le navigateur d'un proche ; « client » = le service qui range les objets chiffrés et les transporte (pour Fatoom : ses Cloud Functions et sa base) ; « passerelle » = les routes que le client expose à Sésame (`API.md` §3) ; « Sésame » = l'enclave, jointe après attestation vérifiée et par le canal chiffré du §5.

## 1. Principe et vocabulaire

- **Le téléphone décide de tout.** Le coffre déverrouillé produit lui-même les clés chiffrées pour les proches, les droits de chaque proche et la preuve que le titulaire est vivant. Le client range et transporte des objets qu'il ne sait ni lire ni fabriquer.
- **Sésame ouvre, et seulement lui.** Il n'ouvre une clé que dans deux cas : transmission après un décès dont il tient lui-même le compte des 7 jours, ou ouverture par le contact de confiance qui a prouvé auprès de Sésame son code d'accès et son second facteur. Les clés ouvertes repartent chiffrées pour le navigateur du proche ; le client ne les voit pas.
- **Ce qui n'est destiné à personne ne traverse jamais Sésame** : aucune enveloppe n'existe pour un contenu « gardé pour vous ».
- **Rien ne coûte à l'utilisateur.** Destiner un contenu, changer ses destinataires, créer un proche, créer, modifier ou afficher un mot de passe sont des opérations locales ou un simple appel au client, sans appel à Sésame. Sésame n'est joint qu'à l'enrôlement (une fois), en arrière-plan pour la preuve de vie (au plus une fois par jour), lors d'un changement de droits d'un proche (révocation, rôle, coordonnées : rare), sur le portail et pour la transmission.

| Terme | Définition |
|---|---|
| **Sésame** | Le programme (ce dépôt) et son enclave. Projet cloud distinct de celui du client, joignable directement par l'app et le portail (pour Fatoom : `sesame.fatoom.fr`). |
| **Empreinte** | `sha256` de l'image de conteneur de Sésame (`image_digest`). C'est elle qui est publiée, attestée et autorisée dans KMS. |
| **Attestation** | Jeton signé par Google pour une enclave Confidential Space, portant l'empreinte de l'image en cours d'exécution et un nonce choisi par Sésame (§3). |
| **Manifeste** | Liste signée des versions de Sésame (empreintes, dates, statut), publiée 7 jours avant toute mise en service (`RELEASE.md`). |
| **KTS** (clé de transmission du titulaire) | 32 octets tirés sur le téléphone, protégés par la clé maître du coffre (KMU), confiés une fois à Sésame qui les scelle sous KMS. Tout ce que le titulaire destine est chiffré sous une clé dérivée de KTS. |
| **Enveloppe** | La clé d'un contenu (clé de fichier ou clé de mot de passe) chiffrée sous KTS pour **un** proche, liée au contenu et au proche (§8.3). Rangée par le client à côté du contenu. |
| **Charte** | Les droits d'un proche (rôle, accès élargi, code d'accès, coordonnées du second facteur, génération), chiffrés et authentifiés sous KTS par le téléphone (§8.4). Seuls le téléphone et Sésame peuvent la lire ou l'écrire. |
| **Registre** | Base minimale tenue par Sésame dans son propre projet : signalements de décès, preuves de vie, génération courante des chartes et de KTS (§6). Aucune clé, aucun contenu. |
| **Preuve de vie** | Message authentifié sous KTS envoyé par l'app à Sésame à chaque déverrouillage (au plus une fois par jour, ou immédiatement si un signalement de décès est en cours) : « ce titulaire est vivant ». Annule un signalement en cours. |
| **Jeton de session** | Jeton signé par Sésame (clé KMS P-256) remis au navigateur d'un proche authentifié ; le client le vérifie pour lister les contenus et émettre les URL signées (§9.2). |
| **Code d'invitation** | Code remis par lettre, au décès, au proche sans téléphone ni email : tiré par Sésame, imprimé et posté par Sésame, connu de lui seul ; ouvre le portail après une vérification d'identité menée par Sésame (§9.8). |
| **Scellé KTS** | `sealedKts` : KTS chiffrée par la clé KMS de Sésame (AES-256, HSM), rangée par le client, inutilisable sans l'enclave attestée. |

## 2. L'enclave

### 2.1 Confidential Space

Sésame est un conteneur OCI exécuté par l'image **Confidential Space** de Google (famille `confidential-space`, jamais `confidential-space-debug`) sur une VM confidentielle (AMD SEV-SNP ou Intel TDX) du projet cloud de Sésame, distinct de celui du client. Le lanceur de Confidential Space mesure l'image, démarre le conteneur et expose le service d'attestation sur une socket locale (`/run/container_launcher/teeserver.sock`). Le conteneur n'a ni shell, ni accès SSH, ni port de débogage ; `dbgstat` vaut `disabled-since-boot`.

Le code de Sésame est **minimal** : un serveur HTTPS, un client KMS, un client Firestore (registre, projet Sésame), un client HTTP vers la passerelle du client, trois clients REST sans SDK (Twilio Verify, Stripe Identity, Pingen : quelques appels chacun, §9.8), un générateur de lettre PDF texte d'une page sans dépendance (§9.8), et les primitives cryptographiques de la plateforme. Pas de dépendance réseau non listée ; la politique réseau sortante de la VM n'autorise que KMS, Secret Manager, Firestore, Twilio, Stripe, Pingen, le service d'attestation, les clés publiques des émetteurs de jetons que Sésame vérifie et la passerelle du client. Budget visé : moins de 5 000 lignes hors tests, pour rester auditable.

### 2.2 Fournisseurs et réglages

Les fournisseurs (SMS et email, identité, courrier) sont appelés par leur API dans le code public : leurs adresses de base y sont écrites en dur, jamais lues dans une variable au démarrage, car une adresse modifiable permettrait à qui contrôle la VM de détourner l'envoi du dernier code vers son propre serveur.

Ce qui est propre au client va dans des variables d'environnement passées par les métadonnées `tee-env-*` (identifiant du service de vérification, identifiant d'organisation du courrier, nom et adresse de l'expéditeur, textes du SMS et gabarit de la lettre, adresse de la passerelle, émetteur des jetons d'identité du client, compte de service du client autorisé à signaler un décès) ou dans Secret Manager (clés d'API). Ces variables figurent dans le jeton d'attestation (`submods.container.env`) ; Sésame refuse de démarrer si l'une d'elles manque ou est vide, et le manifeste publie `SHA-256` de l'ensemble des variables non secrètes (`envDigest`) que les clients comparent au jeton : un texte de lettre ou un expéditeur ne se change donc pas en douce. Ce dépôt ne contient ni identifiant de compte, ni texte de marque.

### 2.3 Image reproductible et dépôt public

- Ce dépôt contient le code, le `Dockerfile`, le script de build reproductible, le manifeste des versions, les vecteurs de test recopiés depuis le client et la procédure de vérification en 4 étapes (`README.md`).
- Build reproductible : binaire statique (`-trimpath`, dates figées par `SOURCE_DATE_EPOCH`), image de base `distroless` épinglée par digest, dépendances verrouillées par somme de contrôle, construction dans un conteneur de build lui-même épinglé. La CI construit l'image deux fois sur deux exécuteurs et échoue si les digests diffèrent. Langage : **Go** (dernière version stable épinglée dans `go.mod` et dans la CI), module unique, bibliothèque standard pour la cryptographie (`crypto/ecdh`, `crypto/aes`, `crypto/ed25519`, `golang.org/x/crypto/hkdf`), SDK Google officiels (KMS, Firestore, Secret Manager), `net/http` brut pour les fournisseurs, image de base `gcr.io/distroless/static-debian12` épinglée par digest, build par Dockerfile multi-étapes.
- Chaque version porte un tag Git signé (`vX.Y.Z`) et une ligne du manifeste (`RELEASE.md`).

## 3. Attestation : jeton, claims et vérification

Sésame demande au lanceur un jeton d'attestation (JWT signé par Google, émetteur `https://confidentialcomputing.googleapis.com`, clés publiques `https://confidentialcomputing.googleapis.com/.well-known/jwks`) avec :

- `audience` = `Fatoom-Heritage-Numerique/sesame` ;
- `eat_nonce` = `base64url(SHA-256(sessionPub))`, où `sessionPub` est la clé publique X25519 de session de l'instance (§5). Le jeton lie donc la clé de session à l'enclave attestée. Il est renouvelé toutes les 10 min (validité du jeton 1 h) et servi à chaque client sur `GET /attestation`.

Un client (app, portail, passerelle) **n'accepte** le jeton que si toutes les conditions suivantes sont vraies :

| Claim | Valeur exigée |
|---|---|
| signature | valide pour une clé du JWKS de Google (cache 24 h, rafraîchi sur `kid` inconnu) |
| `iss` | `https://confidentialcomputing.googleapis.com` |
| `aud` | `Fatoom-Heritage-Numerique/sesame` |
| `exp`, `iat` | `iat ≤ now + 5 min`, `exp > now` |
| `eat_nonce` | contient `base64url(SHA-256(sessionPub))` pour la `sessionPub` reçue |
| `swname` | `CONFIDENTIAL_SPACE` |
| `submods.confidential_space.support_attributes` | contient `STABLE` |
| `hwmodel` | `GCP_AMD_SEV` ou `GCP_INTEL_TDX` |
| `dbgstat` | `disabled-since-boot` |
| `secboot` | `true` |
| `submods.container.image_digest` | figure dans le manifeste vérifié, statut `en_service`, `activeFrom ≤ now` |
| `submods.container.image_reference` | commence par le préfixe du dépôt d'images de l'opérateur, gravé dans les clients (`<region>-docker.pkg.dev/<projet>/sesame/`) ; la valeur complète de chaque version est publiée dans le manifeste (`imageReference`) |
| `submods.container.env` | `SHA-256` des variables non secrètes égal à l'`envDigest` de la version dans le manifeste (§2.2) |

Toute condition fausse ⇒ aucune donnée n'est envoyée ; le client affiche un échec d'attestation (§9.6).

## 4. Libération des clés : Cloud KMS conditionné à l'attestation

Deux options ont été comparées : une clé privée scellée et gardée en mémoire de l'enclave (plus rapide côté serveur, mais une faille mémoire expose toutes les enveloppes de tous les titulaires jusqu'à rotation), et une clé KMS HSM que l'enclave ne détient jamais. **Décision : KMS HSM, avec une clé de transmission par titulaire** (§8.2), ce qui ne change rien pour l'utilisateur et borne l'exposition d'une faille mémoire aux titulaires ouverts pendant la session (`docs/decisions/0002`).

Dans le projet de Sésame, trousseau `sesame`, même région que l'enclave :

| Clé | Type | Usage |
|---|---|---|
| `kts-seal` | symétrique AES-256-GCM, niveau de protection **HSM**, non exportable, rotation annuelle (les anciennes versions restent déchiffrables) | Scelle et descelle KTS (`sealedKts`, AAD = `uid ‖ "\|" ‖ ktsGeneration`) |
| `session-sign` | asymétrique EC P-256 (ECDSA SHA-256), HSM | Signe les jetons de session du portail, les reçus de signalement et les appels à la passerelle ; la clé publique est lue par le client et épinglée dans sa configuration |
| `registre-mac` | symétrique HMAC-SHA256, HSM | Authentifie chaque ligne du registre (§6) |

Accès : un **pool d'identités de charge de travail** (`sesame-pool`, fournisseur OIDC Confidential Space) ; la liaison IAM `roles/cloudkms.cryptoKeyEncrypterDecrypter` (et `cryptoKeySignerVerifier` pour `session-sign`) porte une **condition d'attribut** :

```
assertion.swname == "CONFIDENTIAL_SPACE"
&& "STABLE" in assertion.submods.confidential_space.support_attributes
&& assertion.submods.container.image_digest in ["sha256:<empreinte publiée>", …]
```

La liste des empreintes autorisées dans KMS est **identique** à celles du manifeste en statut `en_service` ; un contrôle CI de ce dépôt compare les deux à chaque publication (`RELEASE.md` §6). Aucun compte humain n'a `cryptoKeyEncrypterDecrypter` sur `kts-seal` : seule la charge attestée descelle. Les secrets Twilio, Stripe (clé **restreinte** aux seules fonctions Identity) et Pingen (organisation Pingen **distincte** de celle du client : le client ne doit pas pouvoir télécharger une lettre) sont dans Secret Manager du même projet, avec la même condition d'attribut.

**Identité de la charge de travail** : le compte de service de la charge (`sesame-workload@<projet>`) n'est utilisé qu'à travers le pool (usurpation conditionnée à l'attestation) ; il a `datastore.user` sur le registre, `secretmanager.secretAccessor` sur les secrets des fournisseurs, et rien d'autre.

## 5. Clé de session et canal chiffré

Au démarrage, chaque instance tire une paire X25519 **de session** (`sessionPriv`, `sessionPub`), en mémoire seulement, et l'inscrit dans le nonce de son attestation. Un client :

1. `GET /attestation` → `{jwt, sessionPub, instanceId}` ; vérifie le jeton (§3) et que `eat_nonce` = `SHA-256(sessionPub)`.
2. Tire sa propre paire X25519 (`cliPriv`, `cliPub`), calcule `shared = X25519(cliPriv, sessionPub)` (refusé s'il est entièrement nul) et `K = HKDF-SHA256(ikm = shared, salt = sessionPub ‖ cliPub, info = "fatoom-sesame-canal", L = 32)`.
3. Chaque requête est `{instanceId, cliPub, n, body: AES-256-GCM(K, IV_n, UTF-8(JSON), aad = "fatoom-sesame-req|" ‖ n)}` avec `n` un compteur strictement croissant par client (IV = 4 octets aléatoires tirés à la création du canal ‖ `n` sur 8 octets gros-boutiste) ; chaque réponse est chiffrée de même avec `aad = "fatoom-sesame-res|" ‖ n`. Une requête hors instance (`instanceId` inconnu après redémarrage) reçoit `410` et le client recommence à l'étape 1.

Ce canal est **obligatoire** par-dessus TLS : TLS est terminé par le répartiteur de charge Google, hors de l'enclave. Primitives : X25519, HKDF-SHA256, AES-256-GCM, disponibles sur mobile, dans WebCrypto et dans Node. Les chaînes de domaine (`fatoom-sesame-…`) portent le nom du premier client ; elles font partie du format et des vecteurs de test et ne changent pas d'un client à l'autre.

## 6. Registre : l'état minimal de Sésame

Sésame est sans état pour tout ce qui est clé ou contenu. Il tient un **registre** (Firestore natif dans le projet de Sésame, accessible uniquement à la charge attestée) parce que trois garanties l'exigent : la mesure des 7 jours, l'annulation par preuve de vie, et le refus d'une charte révoquée. Chaque ligne porte un `mac = HMAC(registre-mac, UTF-8(JSON canonique sans mac))` : une ligne modifiée hors de l'enclave est ignorée.

| Collection | Document | Champs |
|---|---|---|
| `titulaires/{uid}` | un par titulaire enrôlé | `ktsGeneration` (int), `enroledAt`, `vivantLe` (dernière preuve de vie, horloge Sésame), `chartes.{contactId}` = génération courante (int ; `-1` = proche supprimé), `mac` |
| `titulaires/{uid}/signalements/{id}` | un par signalement de décès | `signaleLe` (horloge Sésame), `source` ∈ `insee` \| `gardien` \| `admin`, `transmissibleLe` = `signaleLe + 7 j`, `annuleLe?`, `annulePar` ∈ `preuve_de_vie`, `lettresEnvoyeesLe?`, `mac` |
| `titulaires/{uid}/ouvertures/{id}` | une par session de proche réussie | `contactId`, `role`, `elargi`, `ouvertLe`, `nbCles`, `mac` (sert à la notification du titulaire et à l'historique) |
| `titulaires/{uid}/lettres/{contactId}` | une par proche prévenu par lettre (§9.8) | `invitationLookup`, `pingenDocumentId`, `statut` ∈ `commandee` \| `imprimee` \| `en_distribution` \| `distribuee` \| `erreur`, `envoyeeLe`, `renvoyeeLe[]`, `ouverteLe?`, `identiteVerifieeLe?`, `essaisIdentite`, `mac` |
| `invitations/{lookup}` | une par code d'invitation | `uid`, `contactId`, `expireLe` (12 mois), `mac` |
| `instances/{instanceId}` | une par instance en vie | `imageDigest`, `demarreLe`, `vuLe` (toutes les 60 s) |

Taille : quelques Ko par titulaire. Aucune coordonnée, aucun nom, aucune clé. Le registre est exporté chaque nuit vers un bucket du projet Sésame (sauvegarde) ; sa perte ne détruit aucune clé mais remet à zéro les signalements et générations (`THREAT_MODEL.md` §2, limite 11).

## 7. Journalisation sans fuite

Sésame journalise en JSON structuré : `event`, `uidMasque` (8 premiers hexadécimaux de `SHA-256(uid)`), `contactMasque` (idem), `instanceId`, `imageDigest`, `resultat`, `codeErreur`. Jamais de clé, de code d'accès, de code SMS, de numéro, d'email, de nom, de libellé ni de corps de requête. Les journaux vont dans Cloud Logging du projet Sésame avec une rétention de 90 jours. Un compteur agrégé par jour (`enrôlements`, `preuves de vie`, `ouvertures`, `refus par motif`) est publié par le client sur sa page de transparence (pour Fatoom : `fatoom.fr/sesame`).

## 8. Hiérarchie et formats des clés

Tout est AES-256-GCM, IV 12 octets, tag 16 octets **concaténé à la fin**, encodage base64 standard. HKDF = HKDF-SHA256. Les chaînes de domaine sont en UTF-8 sans terminateur. JSON canonique = clés triées, sans espace, UTF-8.

### 8.1 Vue d'ensemble

```
PIN ─► KDP ─► KMU (clé maître du coffre, sur le téléphone)
              ├─ KMF (par fichier)      → clé de fichier chiffrée pour le titulaire
              ├─ KME (par mot de passe) → clé de mot de passe chiffrée pour le titulaire
              └─ KTS (clé de transmission) → encryptedKts / ktsIv, rangés par le client
                    ├─ HKDF info "fatoom-sesame-enveloppe" ─► KTS_env
                    │     └─ enveloppe = AES-GCM(KTS_env, KMF | KME, aad contenu+proche) → sharedKeys.{contactId}
                    ├─ HKDF info "fatoom-sesame-charte"    ─► KTS_auth
                    │     ├─ charte du proche      → charte / charteIv, rangés par le client
                    │     └─ preuve de vie, publication de génération (vers Sésame)
                    └─ scellée pour Sésame : sealedKts = KMS(kts-seal).encrypt(KTS, aad uid|génération)
                          → rangé par le client, écrit par l'app après l'enrôlement
Sésame (enclave attestée) : KMS.decrypt(sealedKts) ─► KTS ─► KTS_env, KTS_auth (en mémoire, le temps d'une session)
       └─ ouvre les enveloppes du proche authentifié ─► clés re-chiffrées pour le navigateur (canal §5)
```

### 8.2 KTS et dérivations

- `KTS` = 32 octets aléatoires (générateur du système). Rangée chez le client sous `AES-GCM(KMU, KTS)` (`encryptedKts`, `ktsIv`). Jamais stockée en clair localement ; en RAM avec la KMU, effacée au verrouillage.
- `KTS_env = HKDF(ikm = KTS, salt = UTF-8(uid), info = "fatoom-sesame-enveloppe", L = 32)`.
- `KTS_auth = HKDF(ikm = KTS, salt = UTF-8(uid), info = "fatoom-sesame-charte", L = 32)`.
- `ktsGeneration` : entier ≥ 1, incrémenté à chaque rotation (§9.4) ; porté par le client, par l'AAD du scellé et par le registre.
- Un coffre sans KTS n'est pas enrôlé : il ne peut rien destiner (l'app propose l'enrôlement, §9.7). Un coffre neuf s'enrôle dans la foulée de la création du PIN, silencieusement si Sésame répond, sinon à la première destination.

### 8.3 Enveloppe d'un contenu pour un proche

```
aad_env = "fatoom-sesame-env|1|" ‖ uid ‖ "|" ‖ type ‖ "|" ‖ contentId ‖ "|" ‖ contactId ‖ "|" ‖ ktsGeneration
          type ∈ { "fichier", "motdepasse" }
enveloppe = AES-256-GCM(KTS_env, iv aléatoire 12 o, clé du contenu (32 o), aad_env)   → 48 octets
```

Le client range, pour chaque contenu, `sharedKeys.{contactId} = {enveloppe, iv}` et `sharedWith` = liste des `contactId`, avec l'invariant **`c ∈ sharedWith ⇔ sharedKeys.c` existe** et une borne de 100 destinataires par contenu. Le client ne peut pas vérifier l'AAD : c'est Sésame qui, à l'ouverture, refuse une enveloppe dont l'AAD ne correspond pas à l'uid, au type, à l'identifiant du contenu, au proche authentifié (ou à un proche quelconque du titulaire pour l'accès élargi) et à la génération courante de KTS. Une enveloppe copiée d'un contenu à un autre, ou d'un proche à un autre, ne s'ouvre pas.

Un contenu dont `sharedWith` devient vide perd toutes ses enveloppes : il redevient « gardé pour vous ».

### 8.4 Charte d'un proche

```
charteJson (canonique) = {
  "v": 1,
  "uid": "<uid>",
  "contactId": "<id>",
  "generation": <int ≥ 1>,
  "role": "proche" | "gardien",
  "elargi": <bool>,                       # vrai seulement si role = gardien
  "codeVerifier": "<base64(SHA-256(\"fatoom-code|\" ‖ normalize(code)))>",
  "phone": "<E.164>" | null,              # second facteur SMS (obligatoire si role = gardien)
  "email": "<email normalisé>" | null,    # second facteur email
  "prenom": "<prénom tel que saisi>",     # affichage (« Bonjour Léa ») et comparaison d'identité (§9.8)
  "nom": "<nom tel que saisi>",
  "dateNaissance": "<AAAA-MM-JJ>" | null,  # conseillée, sert à la comparaison d'identité
  "adresse": {"ligne1", "ligne2" | null, "codePostal", "ville", "pays"} | null,   # scellée : le client ne peut pas rediriger la lettre
  "titulaireAffiche": "<nom du titulaire>",
  "courrier": <bool>,                     # vrai si phone et email sont null : lettre au décès, adresse obligatoire
  "createdAt": "<ISO-8601 UTC>"
}
aad_charte = "fatoom-sesame-charte|1|" ‖ uid ‖ "|" ‖ contactId ‖ "|" ‖ generation ‖ "|" ‖ ktsGeneration
charte     = AES-256-GCM(KTS_auth, iv, UTF-8(charteJson), aad_charte)
```

Le client range `charte`, `charteIv`, `charteGeneration` et `accessLookup = hex(SHA-256("fatoom-lookup|" ‖ normalize(code)))`, qui indexe le proche pour le retrouver à partir de son code sur le portail. Le code a 12 caractères dans un alphabet sans `0`, `1`, `I`, `O` (60 bits d'entropie) ; l'index ne sert qu'à retrouver un document. `normalize(code)` retire tirets et espaces et met en majuscules.

Le registre tient `chartes.{contactId}` = génération courante : une charte d'une génération inférieure est refusée, même valide (révocation effective, §9.4). Un `contactId` absent du registre n'accepte que la génération 1 : créer un proche ne sollicite donc pas Sésame ; toute génération ultérieure (révocation, changement de rôle ou de coordonnées, suppression = `-1`) est publiée par l'app avant d'être écrite chez le client.

### 8.5 Preuve de vie et publication de génération

Messages app → Sésame (canal §5), authentifiés sous `KTS_auth` :

```
vie       = AES-GCM(KTS_auth, iv, UTF-8({"v":1,"uid","t":<epoch s>,"nonce":<16 o base64>}), aad = "fatoom-sesame-vie|1|" ‖ uid)
charteGen = AES-GCM(KTS_auth, iv, UTF-8({"v":1,"uid","contactId","generation":<int>}), aad = "fatoom-sesame-gen|1|" ‖ uid ‖ "|" ‖ contactId)
```

Sésame refuse `|t − now| > 10 min` et un `nonce` déjà vu dans la journée. `generation = -1` signifie « proche supprimé ».

### 8.6 Ce que le client range pour le titulaire

| Champ | Écrit par | Contenu |
|---|---|---|
| `encryptedKts`, `ktsIv` | app | KTS sous KMU |
| `sealedKts` | app (valeur renvoyée par Sésame) | KTS scellée par KMS, AAD `uid ‖ "\|" ‖ ktsGeneration` |
| `ktsGeneration` | app | entier ≥ 1, ne peut qu'augmenter |
| `enroledAt`, `enroledWith` | app | date, `imageDigest` de l'instance qui a scellé |
| `lastProofOfLifeAt` | app | confort d'affichage ; la vérité est au registre |

Le client fait respecter : `ktsGeneration` strictement croissant ; `encryptedKts`, `ktsIv`, `sealedKts` ne changent qu'avec `ktsGeneration` (rotation).

### 8.7 Ce qui reste en clair chez le client, et pourquoi

| Donnée | Clair ? | Raison |
|---|---|---|
| Nom du fichier, service d'un mot de passe | oui | tri, liste et recherche sans déchiffrement |
| Identifiant, URL, notes, tags, mot de passe | non | déchiffrés localement à l'ouverture du coffre ou sur le portail |
| `sharedWith`, dates, tailles, types | oui | listes, quotas, limites |
| Coordonnées des proches | oui | envois par le client ; la **copie qui fait foi** pour le second facteur est celle de la charte |
| Charte, enveloppes, KTS | non | §8 |

## 9. Protocoles pas à pas

### 9.1 Destiner un contenu depuis le téléphone (chemin chaud, local)

1. L'utilisateur choisit les proches. Coffre déverrouillé exigé ; coffre non enrôlé ⇒ enrôlement d'abord (§9.7).
2. Pour chaque contenu : l'app déchiffre la clé du contenu puis, pour chaque proche **ajouté**, calcule l'enveloppe (§8.3). Pour un proche retiré, rien à calculer. Clés effacées aussitôt.
3. Un seul appel au client avec, pour chaque contenu, `sharedWith` et `sharedKeys` (≤ 500 contenus par appel). Interface optimiste avec rollback.
4. Le client vérifie : propriétaire, contenu chiffré de bout en bout, chaque `contactId` existe, `|sharedWith| ≤ 100`, invariant du §8.3, forme (48 octets, IV 12 octets) ; écrit en transaction ; aucune cryptographie.

Aucun appel à Sésame, aucun réseau autre que le client : destiner 100 fichiers à 100 proches coûte 10 000 AES-GCM (< 100 ms dans un isolate) et un appel.

### 9.2 Ouverture par le contact de confiance sur le portail

```
Navigateur (portail)                  Sésame (enclave)                         Client (passerelle)
GET /attestation ───────────────────▶ jwt + sessionPub
vérifie jwt, manifeste ; canal K
POST /session/ouvrir {code} ────────▶ lookup = SHA-256("fatoom-lookup|"‖code)
                                      GET bundle?lookup ──────────────────────▶ vérifie l'attestation de Sésame
                                      ◀── {uid, contactId, charte, charteIv, charteGeneration,
                                           sealedKts, ktsGeneration, isUnlocked, titulaireAffiche}
                                      KMS.decrypt(sealedKts) → KTS → KTS_auth ; ouvre la charte
                                      vérifie codeVerifier, generation == registre.chartes[contactId],
                                      ktsGeneration == registre, isUnlocked (code remis)
                                      rôle : gardien ⇒ ok ; proche ⇒ exige un signalement
                                        transmissibleLe ≤ now et non annulé
                                      Twilio Verify : envoie le code à charte.phone (SMS) ou charte.email
◀─── {sessionId, canal: "sms", masque: "+33 6 •• •• •• 47"}
saisit le dernier code
POST /session/verifier {sessionId, otp} ─▶ Twilio Verify check ; 5 essais / 15 min
                                      registre.ouvertures += 1 ; notifie le client (openingNotified)
◀─── {jeton de session (ES256, 1 h), scope: destine | elargi}
GET listContactContents (jeton) ─────────────────────────────────────────────▶ vérifie la signature session-sign
◀─── fichiers et mots de passe (noms, tailles, types, champs chiffrés, sharedKeys[scope])
POST /session/ouvrir-cles {sessionId, enveloppes: [{type, id, contactId, enveloppe, iv}]} ─▶
                                      pour chaque : vérifie aad_env (uid, type, id, contactId ∈ scope, génération)
                                      ouvre sous KTS_env ; efface KTS après réponse
◀─── {cles: [{id, cle}]} (sous le canal K)
GET getContactFileUrl (jeton, fileId) ──────────────────────────────────────▶ URL signée de l'objet chiffré (10 min)
télécharge et déchiffre par blocs dans le navigateur ; mots de passe : KME → champs chiffrés
```

- **Accès élargi** : `charte.elargi == true` ⇒ scope `elargi` : le client liste tout contenu dont `sharedWith` n'est pas vide, avec **toutes** ses enveloppes ; Sésame accepte dans l'AAD tout `contactId` (l'AAD est authentifié sous KTS : seul le titulaire a pu destiner ce contenu à quelqu'un). Le portail sépare « ce qu'il vous destine » et « destiné à ses autres proches » d'après `sharedWith`.
- **Code reçu par courrier** (`charte.courrier == true`) : le proche saisit son **code d'invitation** ; il n'y a ni SMS ni email possible, le second facteur est la **vérification d'identité menée par Sésame** (Stripe Identity, comparaison avec `prenom`, `nom`, `dateNaissance` de la charte), §9.8. Le client n'intervient pas.
- **Session** : le jeton de session expire après 1 h d'inactivité ; les clés ouvertes vivent dans la mémoire de l'onglet et disparaissent avec lui. Fermer la session détruit le jeton côté Sésame (`POST /session/fermer`).
- **Signalement du décès par le contact de confiance** : `POST /session/signaler-deces {sessionId}` ⇒ Sésame crée le signalement au registre (`source: gardien`) **lui-même**, puis prévient le client (`deathSignaled`) pour l'alerte au titulaire.
- **Refus du rôle** : `POST /session/refuser-role` ⇒ registre `chartes.{contactId} = -1`, client prévenu (`roleRefused`) pour l'email et le retrait du statut ; le titulaire est invité à choisir un autre contact de confiance.

### 9.3 Transmission après décès

1. **Signalement.** Source registre officiel des décès (pour Fatoom : l'INSEE) : le client appelle `POST /deces/signaler {uid, source: "insee", preuve}` authentifié par un jeton OIDC de son compte de service, que Sésame vérifie ; Sésame crée le signalement avec **sa** date (`signaleLe = now`, `transmissibleLe = now + 7 j`). Source contact de confiance : §9.2. Source administration : idem, `source: "admin"`. Sésame répond avec le reçu `{signalementId, signaleLe, transmissibleLe}` signé `session-sign`, que le client conserve pour prévenir le titulaire.
2. **Vérification de 7 jours.** Le client prévient le titulaire (email, SMS, écran). Le titulaire annule en déverrouillant son coffre : l'app envoie sa **preuve de vie** directement à Sésame (§8.5), qui marque le signalement `annuleLe` et répond à l'app l'état courant (`{signalement: null}`) ; l'app prévient le client pour les emails et le nettoyage. Le client ne peut ni retarder ni supprimer cette annulation : elle n'a pas transité par lui.
3. **J+7.** Le client envoie les codes d'accès par email et SMS aux proches qui ont ces coordonnées ; **Sésame** poste la lettre aux proches qui n'en ont pas (§9.8). Sésame n'ouvrira pour un proche que si `transmissibleLe ≤ now` selon **son** horloge et si aucune preuve de vie n'est arrivée depuis `signaleLe`. Les conditions sont indépendantes : un client qui envoie les codes trop tôt n'ouvre rien.
4. **Ouverture par un proche** : §9.2, rôle `proche` ; même parcours pour le contact de confiance, qui n'est plus soumis à la limite de scope avant décès (l'accès élargi reste tel que choisi par le titulaire).
5. **Après la transmission** : les 12 mois d'accès et la suppression sont pilotés par le client ; la suppression du compte demande à Sésame `POST /titulaire/supprimer {uid}` (jeton OIDC du client) qui efface le registre du titulaire. Une preuve de vie reçue **après** une transmission réelle (téléphone retrouvé) ne referme pas ce qui a été ouvert, mais bloque les ouvertures suivantes et alerte le client (`lifeAfterTransmission`) : cas traité par le support.

**Comment Sésame sait que le décès est vérifié sans faire confiance au client seul.** Il ne vérifie pas le registre officiel lui-même (le signalement par le contact de confiance ne vient pas de l'INSEE, et le client doit pouvoir alerter le titulaire) : il se contente de **mesurer** les 7 jours avec sa propre horloge et d'exiger **l'absence de preuve de vie**, qui ne transite jamais par le client. Un client compromis peut donc au pire déclencher une fausse alerte ; il ne peut pas transmettre tant que le titulaire ouvre son app dans les 7 jours.

### 9.4 Révocation, régénération, retrait d'un destinataire, remplacement du contact de confiance, rotation de KTS

| Action | Sur le téléphone | Vers Sésame (canal, bloquant, rare) | Vers le client |
|---|---|---|---|
| **Régénérer le code** | nouveau code (12 caractères, sans biais modulo), nouvelle charte `generation + 1` | `POST /charte/generation` (`generation + 1`) ; échec ⇒ rien n'est écrit | remplace la charte, l'index (ancien supprimé), `isUnlocked: false` ; envoi du code si le titulaire le renvoie |
| **Révoquer l'accès** | idem, code non redistribué | idem | idem, sans envoi |
| **Retirer le statut de gardien** | nouvelle charte `role: proche`, `elargi: false`, nouveau code | `POST /charte/generation` | charte + statut + email de fin de rôle |
| **Désigner un contact de confiance** | charte `role: gardien` avec `phone` obligatoire ; l'ancien contact de confiance, s'il existe, reçoit une charte `proche` (ligne précédente) | `POST /charte/generation` pour les deux | statut + email de désignation avec le code (le client reçoit le code pour l'envoyer, `THREAT_MODEL.md` §2 limite 3) |
| **Accès élargi** | charte `elargi: true/false`, même code (le `codeVerifier` ne change pas) | `POST /charte/generation` | charte |
| **Changer le téléphone ou l'email d'un proche** | nouvelle charte (mêmes code et rôle) : la copie qui fait foi pour le second facteur est la charte | `POST /charte/generation` | charte + coordonnées |
| **Retirer un destinataire d'un contenu** | enveloppe retirée | rien | destinataires du contenu |
| **Supprimer un proche** | — | `POST /charte/generation` (`-1`) | retire `sharedKeys.{contactId}` et `sharedWith` de tous les contenus, supprime l'index et le proche |
| **Rotation de KTS** (sur demande du titulaire, et après récupération du coffre) | nouvelle KTS, `ktsGeneration + 1` ; **toutes** les enveloppes et chartes re-chiffrées (N AES-GCM, isolate) | enrôlement (§9.7) avec la nouvelle génération ; le registre passe `ktsGeneration` | écriture en lots ; les anciennes enveloppes deviennent inutilisables (AAD) |

Toute action sur une charte passe **d'abord** par Sésame : tant que le registre n'a pas la nouvelle génération, rien n'est écrit chez le client. Si Sésame est injoignable, l'action échoue proprement et l'ancien code reste valide : on ne laisse jamais un état où le client croit l'accès révoqué alors que Sésame l'ouvrirait encore.

### 9.5 Versions de Sésame : manifeste, transparence, refus de l'inconnu

Le manifeste, sa signature, la publication 7 jours avant, le retrait immédiat et la cohérence avec KMS sont décrits dans `RELEASE.md`. Côté clients :

- L'app et le portail téléchargent le manifeste (pour Fatoom : `https://fatoom.fr/sesame/manifeste.json`, hébergé hors de l'infrastructure du client qui sert les données), vérifient le nombre de signatures exigé (clés gravées dans les clients, nombre gravé lui aussi), gardent la copie en cache 24 h, la rafraîchissent en arrière-plan ; une copie de plus de 30 jours est périmée. Un `imageDigest` absent du manifeste, `annoncee` ou `retiree`, ou `activeFrom > now` ⇒ attestation refusée. La passerelle applique la même règle aux appels de Sésame.
- La page de transparence du client affiche le manifeste (version en service, date, empreinte, statut) et son historique complet ; ce dépôt publie, pour chaque version, le journal de construction reproductible.

### 9.6 Échec d'attestation

| Où | Quand | Comportement |
|---|---|---|
| App, enrôlement ou rotation | jeton invalide, empreinte hors manifeste, manifeste invérifiable ou périmé, Sésame injoignable | Écran d'erreur : « Empreinte attendue » (celle du manifeste en service) / « Empreinte reçue » (celle du jeton, ou « Non vérifiable ») ; « Réessayer » ; « Rien n'a été exposé » : rien n'a été envoyé. Le coffre reste utilisable, sans destination nouvelle. |
| App, action sur une charte (§9.4) | idem | Même écran avec le titre de l'action ; rien n'est écrit, l'état précédent reste cohérent. |
| App, preuve de vie (arrière-plan) | idem | Silencieux ; nouvel essai au prochain déverrouillage ; après 3 jours sans succès, bandeau discret (« Sésame injoignable depuis le … »). Jamais bloquant, **sauf** pendant un signalement de décès : l'annulation exige alors une preuve de vie acceptée, et l'écran d'alerte affiche l'échec avec « Réessayer ». |
| App, destiner un contenu | jamais : opération locale | — |
| Portail | idem | « Connexion non vérifiée » : aucune saisie de code n'est proposée tant que l'attestation n'est pas vérifiée ; lien « Pourquoi cette vérification ? » vers la page de transparence. |
| Passerelle | appel de Sésame avec un jeton invalide | `401`, journal avec `imageDigest`. |

### 9.7 Enrôlement d'un coffre

L'enrôlement crée la KTS d'un coffre qui n'en a pas encore, ou la remplace (rotation). Il se déclenche à la création du PIN (silencieux), au premier déverrouillage suivant une mise à jour de l'app qui l'exige, ou à la première destination si elle a été reportée. L'écran est bloquant mais rapide (« moins d'une minute »).

| Étape | Ce qui se passe |
|---|---|
| 1 · Vérification de Sésame | manifeste téléchargé et vérifié ; `GET /attestation` ; jeton vérifié (§3) ; canal ouvert. Échec ⇒ écran d'erreur (§9.6). |
| 2 · Rechiffrement des clés | KTS tirée ; pour chaque contenu déjà destiné : enveloppes recalculées sous `KTS_env` ; pour chaque proche : **nouveau** code et charte (génération 1 ; aucun code antérieur n'est repris) ; le contact de confiance reçoit son nouveau code par email à la fin. Tout en isolate, sur l'appareil. |
| 3 · Envoi sécurisé | `POST /enroler {uid, idToken, ktsGeneration, kts}` ⇒ Sésame vérifie le jeton d'identité du titulaire (clés publiques de l'émetteur configuré ; pour Fatoom : Firebase Auth), scelle KTS (`sealedKts`), crée ou met à jour `titulaires/{uid}` au registre (`ktsGeneration`, chartes à leur génération) et renvoie `sealedKts` ; l'app écrit le bloc §8.6 chez le client, puis les contenus et chartes en lots. |
| Terminé | « Ce qui change pour votre contact de confiance » : il garde l'accès élargi s'il l'avait. |

### 9.8 Proche sans téléphone ni email : lettre et vérification d'identité, par Sésame

Tout se passe dans Sésame, pour que le client ne connaisse ni le code de la lettre ni le résultat de l'identité. Le code reste petit : trois appels REST pour Pingen, deux pour Stripe, et une lettre PDF texte.

**Lettre.** Une tâche interne de Sésame (toutes les heures) parcourt les signalements `transmissibleLe ≤ now`, non annulés, sans `lettresEnvoyeesLe` :

1. `GET bundle?uid=…&courrier=1` à la passerelle : les chartes dont `courrier == true` (le client ne peut que les retenir, jamais les modifier : déni de service visible, `THREAT_MODEL.md` §2 limite 8).
2. Pour chaque charte : ouvre la charte (KTS via KMS), tire le **code d'invitation** (12 caractères, même alphabet que le code d'accès, `4-4-4`), écrit `invitations/{SHA-256("fatoom-lookup|" ‖ code)}` et `lettres/{contactId}` au registre.
3. Produit la lettre : **PDF texte d'une page**, A4, police standard Helvetica (encodage WinAnsi, accents français, aucune police embarquée), adresse du destinataire dans la zone fenêtre gauche définie par Pingen, texte fixe en français (gabarit en variable attestée : qui écrit, pourquoi, le code en grand, l'adresse du portail, la vérification d'identité à prévoir, rappel que le client ne demande jamais rien d'autre), nom du titulaire et prénom du proche depuis la charte. Générateur maison d'environ 150 lignes, déterministe (test par comparaison d'octets), sans navigateur sans tête ni bibliothèque PDF.
4. Pingen (organisation de Sésame) : `GET file-upload-url`, `PUT` du PDF, `POST documents` (`delivery_product: cheap`, `print_mode: simplex`, `print_spectrum: grayscale`, adresse de la charte). Enregistre `pingenDocumentId`, `statut: commandee`, `envoyeeLe`.
5. Notifie le client (`letterSent {uid, contactId, pingenDocumentId}`) pour l'historique des envois ; **sans** le code.

La même tâche relit chaque heure les lettres non `distribuee` chez Pingen (`GET documents/{id}`) et notifie le client des changements (`letterStatus` : commandée, imprimée, en distribution, distribuée) ; une `erreur` déclenche un nouvel envoi (nouvelle lettre, même code) le lendemain, 3 fois au plus, puis `letterFailed` (support). Renvois : si `ouverteLe` est vide à J+15 puis J+45, une nouvelle lettre portant le **même** code est postée (`renvoyeeLe[]`). Le code expire 12 mois après la transmission.

**Ouverture.** `POST /session/ouvrir {code}` : Sésame cherche d'abord `invitations/{lookup}` ; trouvé ⇒ parcours « courrier » :

1. `POST /session/identite/demarrer {sessionId}` : Sésame crée une session Stripe Identity (`type: document`, `options.document.require_matching_selfie: true`, aucune donnée du proche transmise à Stripe) avec sa clé restreinte et renvoie le `client_secret` ; le navigateur ouvre le module Stripe (consentement explicite au selfie affiché avant).
2. `POST /session/identite/verifier {sessionId}` : Sésame relit la session chez Stripe (`expand[]=verified_outputs`). `status == verified` ⇒ comparaison avec la charte : nom (sans accents, majuscules, espaces et tirets normalisés) **égal** ; prénom : au moins un prénom de la pièce égal à `prenom` ; `dateNaissance` égale si la charte en a une. Succès ⇒ `identiteVerifieeLe`, jeton de session (§9.2), suite identique au parcours SMS. Échec de correspondance ou `requires_input` ⇒ message neutre (« Nous n'avons pas pu confirmer votre identité »), `essaisIdentite + 1`, blocage 24 h au troisième échec, `letterIdentityFailed` notifié au client pour le support (vérification humaine).
3. Les ouvertures suivantes dans les 12 mois ne refont pas la vérification d'identité : `identiteVerifieeLe` présent ⇒ jeton de session directement après le code (la lettre reste le seul facteur possédé ; `THREAT_MODEL.md` §2 limite 4).

Ce que Stripe voit : la pièce et le selfie du proche. Ce que le client voit : qu'une lettre est partie et son suivi ; jamais le code, jamais le résultat de l'identité. Ce que Pingen voit : la lettre, donc le code.
