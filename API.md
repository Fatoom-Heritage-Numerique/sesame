<!-- Source : Fatoom-Heritage-Numerique/Fatoom docs/TRD.md §16.3.3, §16.3.5, §16.3.9, §16.4.5, §16.5, §16.7.1 (état du 2026-10-26) -->

# API de Sésame

Ce document est le contrat des routes que Sésame expose et des routes que le client doit exposer à Sésame (la passerelle). L'OpenAPI, les vecteurs de test et le double local `sesame-fake` en dérivent ; en cas d'écart, ce document se corrige d'abord.

Les limites de débit du §7 sont des valeurs initiales, à ajuster avec les mesures du double local ; celles notées « à fixer au lot 1 bis » le seront avec les tests de charge. Tout le reste est fixé par la spécification de référence.

## 1. Conventions

### 1.1 Adresse et transport

- Sésame est joint à l'adresse publique de l'instance (pour Fatoom : `https://sesame.fatoom.fr`). TLS est terminé par le répartiteur de charge, hors de l'enclave : il ne protège rien à lui seul.
- Sésame tourne sur trois VM, une par zone. `GET /attestation` est servie sans préfixe, par n'importe quelle VM, hors canal. Elle renvoie l'`instanceId` de la VM, dont le client déduit la zone (`a`, `b` ou `c`) ; il n'y a pas de champ `zone`.
- Toute autre route passe par le **canal chiffré** d'`ARCHITECTURE.md`, à **un seul chemin** : `POST /n/{a|b|c}/canal`, adressé à la VM dont le client a vérifié l'attestation. Le nom de la route est dans le corps chiffré : le répartiteur, que l'administrateur règle, ne distingue pas une annulation d'une ouverture par un proche.
- Le canal est hybride X25519 + ML-KEM-768, avec une clé par sens (`K_req` pour les requêtes, `K_res` pour les réponses). Sur le fil, chaque requête est :

```json
{"instanceId": "<id>", "cliPub": "<base64, X25519>", "kemCt": "<base64, ML-KEM-768>", "n": 1, "body": "<base64>"}
```

- `body` = AES-256-GCM(`K_req`, `IV_n`, UTF-8(JSON), aad `"fatoom-sesame-req|" ‖ n`). `n` est un compteur strictement croissant par client ; `IV_n` = 4 octets tirés à la création du canal ‖ `n` sur 8 octets gros-boutiste. La réponse à la requête `n` est chiffrée sous `K_res`, avec le même `IV_n` et l'aad `"fatoom-sesame-res|" ‖ n`.
- Le corps déchiffré d'une requête nomme la route et porte ses données :

```json
{"route": "deces/annuler", "corps": {…}}
```

- Les corps décrits ci-dessous sont les JSON **en clair, avant chiffrement**. JSON canonique (RFC 8785), UTF-8 ; octets en base64 standard ; horodatages en ISO 8601 UTC (`2026-10-12T14:03:00Z`) ; identifiants dans `[A-Za-z0-9_-]`.
- Taille d'un corps déchiffré : 64 Kio au plus, 256 Kio pour `session/ouvrir-cles`.

### 1.2 Statut HTTP et erreurs

Toute réponse que l'enclave chiffre part en **`HTTP 200`**. Le vrai statut est dans le corps chiffré :

```json
{"statut": 200, "corps": {…}}
{"statut": 403, "erreur": "<code>", "message": "<phrase en français, non destinée à l'affichage>", "details": {…}}
```

Un `403` ou un `409` en clair dirait au répartiteur quelle action a eu lieu. `erreur` est un code stable en `snake_case` français (liste §6) ; `message` peut changer ; `details` est présent quand indiqué (`essaisRestants`, `reessayerLe`). Les clients décident de l'affichage d'après `erreur`, jamais d'après `message`.

Seules les erreurs hors canal sont renvoyées **en clair**, avec leur statut HTTP : l'instance ne peut pas chiffrer pour un canal qu'elle ne reconnaît pas.

| Statut en clair | Code | Quand |
|---|---|---|
| `410` | `instance_inconnue` | `instanceId` inconnu (VM redémarrée) : le client recommence à `GET /attestation` |
| `400` | `canal_invalide` | corps indéchiffrable, `n` non strictement croissant, secret X25519 nul, ou session présentée depuis un autre canal que celui qui l'a ouverte |
| `503` | `attestation_indisponible` | `GET /attestation` avant que l'instance ait obtenu son jeton |

### 1.3 Authentification

| Appelant | Comment Sésame l'authentifie |
|---|---|
| App du titulaire, coffre ouvert | signature Ed25519 par la clé de vie `cleVie`, vérifiée avec la `cleViePub` du registre, sur un message « chaîne de domaine ‖ JSON canonique » ; la plupart des messages portent un **défi** (§1.4) |
| App du titulaire, coffre verrouillé (`etat`) et essai de PIN (`pin/evaluer`) | jeton d'identité du titulaire émis par le fournisseur d'identité du client (pour Fatoom : `idToken` Firebase) |
| App du titulaire, `enroler` | `idToken` ; en plus, une preuve `cleVie` dès qu'une ligne existe pour cet `uid` |
| App du titulaire, `pin/deposer` | `idToken` **et** signature `cleVie` ; l'`idToken` n'est jamais suffisant seul |
| Portail d'un proche | code d'accès ou d'invitation, puis dernier code (SMS ou email) ou vérification d'identité, puis `sessionId`. Une session est **liée au canal** qui l'a ouverte (`instanceId`, `cliPub`) : présentée depuis un autre canal, elle est refusée (`canal_invalide`) |
| Serveur du client | jeton OIDC Google d'un compte de service (pour Fatoom : `fatoom-death@…`), dans le champ `jetonFatoom` du corps chiffré, jamais en en-tête (l'en-tête traverse le répartiteur en clair). Accepté seulement si la signature Google est valide, `exp` non passé, `iss` = `https://accounts.google.com`, `aud` = `https://sesame.fatoom.fr`, `email` = compte de service en variable attestée et `email_verified` vrai ; sinon `jeton_invalide` |
| VM de Sésame | canal mutuellement attesté entre VM : même `image_digest`, `hwmodel` admis (§4) |

### 1.4 Défis

Un défi est tiré par Sésame (`defi`, §2.2) : 32 octets, à usage unique, valable 5 minutes, lié au canal qui l'a demandé, gardé en mémoire de l'instance. Il sert à la preuve de vie, à l'annulation d'un signalement, à la preuve d'enrôlement et au dépôt du PIN. Sésame l'efface dès qu'il l'a vérifié. Un défi inconnu, expiré ou déjà utilisé est refusé (`defi_invalide`). Une VM redémarrée a perdu ses défis : le client reçoit `410` et recommence.

### 1.5 Pièces signées par `session-sign`

Toutes sont signées par la clé KMS `session-sign` (ECDSA P-256, SHA-256), en `ES256` seul : `alg` est imposé, tout autre est refusé. La clé publique est épinglée par le client. Le client conserve un reçu ; il ne lui donne aucun droit : Sésame ne lit jamais un reçu, il relit son registre.

| Pièce | Forme |
|---|---|
| Jeton de session | JWT, en-tête `{"alg": "ES256", "typ": "JWT", "kid": "session-sign/<version KMS>"}` ; claims `{iss: "sesame", sub: <contactId>, uid, scope: "destine", base: "vivant" \| "deces" \| "inactivite", role: "proche" \| "gardien", iat, exp}`, `exp = iat + 3600` ; pas de `sid` |
| Reçu de signalement | JWS compact, `{iss: "sesame", typ: "signalement", uid, signalementId, source, signaleLe, transmissibleLe, iat}` |
| Reçu d'annulation | JWS compact, `{iss: "sesame", typ: "annulation", uid, signalementId, annuleLe, iat}` |
| Reçu de génération | JWS compact, `{iss: "sesame", typ: "generation", uid, contactId, generation, iat}` |
| Réponse de `etat` | JWS compact, `{iss: "sesame", typ: "etat", …}` (§2.5) |

Le `typ` sépare les domaines : une pièce d'un type n'est jamais acceptée pour un autre.

## 2. Routes de Sésame

Sauf `GET /attestation`, chaque route s'appelle par `POST /n/{a|b|c}/canal` avec `{"route": "<nom>", "corps": {…}}` ; le nom est donné en titre. Toute route peut renvoyer `429 trop_d_appels` (`details.reessayerLe`) au-delà de sa limite (§7).

### 2.1 `GET /attestation`

| | |
|---|---|
| Appelant | tout client : app, portail, passerelle du client |
| Authentification | aucune ; hors canal |
| Entrée | aucune |
| Limite | 60 / min par IP |

**Réponse `200`** :

```json
{"jwt": "<JWT signé par Google>", "sessionPub": "<base64, X25519>", "kemPub": "<base64, ML-KEM-768>", "instanceId": "<id>"}
```

Le client vérifie chaque claim du jeton (§3) et que `eat_nonce` contient deux valeurs, `base64url(SHA-256(sessionPub))` et `base64url(SHA-256(kemPub))` ; sinon il n'envoie rien.

**Erreurs** : `503 attestation_indisponible`, en clair (jeton pas encore obtenu du lanceur : l'instance ne sert rien d'autre).

### 2.2 `defi`

| | |
|---|---|
| Appelant | app, coffre ouvert, avant une preuve de vie, une annulation, un enrôlement prouvé ou un dépôt du PIN |
| Authentification | aucune au-delà du canal : un défi ne donne rien seul |
| Limite | 30 / h par `uid` |

**Corps** : `{"uid": "<uid>"}`

**Réponse `200`** : `{"defi": "<base64, 32 octets>", "expireLe": "<ISO>", "grappeId": "<id>"}`

`grappeId` identifie la grappe du protocole de PIN en service : l'app en a besoin pour signer un dépôt (§2.12), y compris à la création du coffre, où aucune évaluation ne précède.

**Erreurs** : `trop_d_appels`.

### 2.3 `enroler`

| | |
|---|---|
| Appelant | app : création du coffre (avant toute écriture chez le client), reprise d'une création interrompue, rotation de la KTS, ré-enrôlement après la perte du registre |
| Authentification | `idToken` ; preuve `cleVie` exigée dès qu'une ligne `titulaires/{uid}` existe |
| Limite | 5 / h par `uid`, 20 / h par IP |

**Corps** :

```json
{"uid": "<uid>", "idToken": "<jeton d'identité>", "ktsGeneration": 1, "kts": "<base64, 32 octets>",
 "cleViePub": "<base64, Ed25519>", "preuve": {"defi": "<base64>", "signature": "<base64>"}}
```

`kts` est la KTS en clair **à l'intérieur du corps chiffré** par le canal. `preuve` est absente au premier enrôlement. Sinon, c'est la signature par la `cleVie` courante de :

```
"fatoom-sesame-enroler|1|" ‖ uid ‖ JSON canonique {"v":1, "uid", "defi", "generation", "cleViePub"}
```

`generation` est la `ktsGeneration` demandée, `cleViePub` la clé de vie envoyée : une rotation ne peut pas substituer une autre clé de vie.

**Traitement** :

- **premier enrôlement** (aucune ligne) : `idToken` seul ; le marqueur d'inscription est créé avant la ligne ; `vivantLe = enroledAt` ;
- **génération supérieure** : rotation, preuve exigée ;
- **génération égale** : rescellement seul (reprise d'une création interrompue) ; la preuve doit se vérifier avec la `cleViePub` inscrite, la `cleViePub` reçue doit lui être égale, ni les chartes ni l'enregistrement du PIN ne sont touchés ;
- **génération inférieure** : refusée.

Sésame scelle la KTS sous la clé KMS `kts-seal`, crée ou met à jour la ligne, puis efface `kts`.

**Réponse `200`** : `{"sealedKts": "<base64>", "ktsGeneration": 1, "imageDigest": "sha256:…", "cleViePub": "<base64>"}`

**Erreurs** : `jeton_invalide` ; `preuve_requise` (ligne existante, preuve absente ou invalide) ; `defi_invalide` ; `generation_non_croissante` ; `message_refuse` (corps mal formé, `cleViePub` absente) ; `titulaire_indisponible` (ligne `supprime` ou rejetée, ou ligne absente alors qu'un marqueur d'inscription existe) ; `registre_indisponible` (bucket des marqueurs illisible : aucun premier enrôlement) ; `trop_d_appels`.

### 2.4 `vie`

| | |
|---|---|
| Appelant | app, à chaque déverrouillage (au plus une fois par 24 h, ou à chaque ouverture pendant une relance) |
| Authentification | signature `cleVie` sur un défi |
| Limite | 10 / jour par `uid` |

**Corps** : `{"uid": "<uid>", "defi": "<base64>", "signature": "<base64>"}`, signature sur :

```
"fatoom-sesame-vie|1|" ‖ uid ‖ JSON canonique {"v":1, "uid", "defi"}
```

**Traitement** : écrit `vivantLe` ; annule un signalement d'**inactivité** en cours (`annulePar: preuve_de_vie`, notification `signalementAnnule`) ; après une transmission par inactivité, bloque les ouvertures suivantes et notifie `lifeAfterTransmission`. Une preuve de vie n'a **aucun effet** sur un signalement de décès, ni avant ni après son échéance. Sur une ligne rejetée par le contrôle de fraîcheur, la preuve est répondue mais pas inscrite.

**Réponse `200`** :

```json
{"vivantLe": "<ISO>", "ktsGeneration": 1,
 "signalement": null | {"signalementId": "<id>", "source": "insee" | "acte" | "inactivite", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "annuleLe": "<ISO>" | null},
 "transmis": false, "pinPresent": true, "pinBloque": false,
 "ouvertures": [{"contactId": "<id>", "role": "proche" | "gardien", "ouvertLe": "<ISO>", "contenus": [{"type": "<type>", "id": "<id>"}]}]}
```

`ouvertures` : sessions de proches depuis la dernière preuve de vie. `transmis` vaut `true` quand une ouverture a eu lieu après `transmissibleLe` : l'app affiche le cas « transmission définitive ». `pinPresent: false` : l'enregistrement du PIN manque dans la grappe en service, l'app redépose. `pinBloque: true` : le plafond d'essais est atteint, seul un nouveau PIN en sort.

**Erreurs** : `defi_invalide` ; `message_refuse` (signature invalide) ; `titulaire_indisponible` (ligne absente ou `supprime`) ; `trop_d_appels`.

### 2.5 `etat`

| | |
|---|---|
| Appelant | app, coffre verrouillé, une fois par jour et à chaque ouverture de l'app |
| Authentification | `idToken` seul |
| Effets | aucun |
| Limite | 100 / jour par `uid`, 60 / min par IP |

**Corps** :

```json
{"uid": "<uid>", "idToken": "<jeton d'identité>", "repere": {"ouvertures": "<ISO>" | null, "evaluations": 0}}
```

Le repère est tenu par l'app : `ouvertures` est l'`ouvertLe` de la dernière ouverture qu'elle a vue ; `evaluations` est la dernière valeur qu'elle a reçue du compteur des évaluations du PIN décomptées, tenu par la grappe. L'app avance son repère après sa propre évaluation. Le compteur repart de zéro dans une nouvelle grappe.

**Réponse `200`** : `{"etat": "<JWS compact ES256>"}`, de charge utile :

```json
{"iss": "sesame", "typ": "etat", "uid": "<uid>", "iat": 1760270580,
 "signalement": null | {…}, "ouvertures": […], "pinPresent": true, "pinBloque": false,
 "evaluationsDepuis": 0, "repere": {"ouvertures": "<ISO>" | null, "evaluations": 0}}
```

`signalement` et les éléments d'`ouvertures` ont la forme de `vie` ; `ouvertures` ne contient que celles postérieures au repère. `evaluationsDepuis` = compteur courant − `repere.evaluations` ; non nul, l'app affiche une alerte locale. `repere` est le repère à jour, que l'app garde.

**Erreurs** : `jeton_invalide` ; `titulaire_indisponible` (ligne absente, `supprime` ou rejetée) ; `trop_d_appels`.

### 2.6 `charte/generation`

| | |
|---|---|
| Appelant | app, **avant** toute écriture d'une charte chez le client : régénération du code, révocation, changement de rôle ou de coordonnées, suppression du proche, republication après un refus du rôle |
| Authentification | signature `cleVie`, sans défi : la génération strictement croissante refuse le rejeu |
| Limite | 30 / min par `uid` |

**Corps** : `{"uid": "<uid>", "contactId": "<id>", "generation": 2, "signature": "<base64>"}`, signature sur :

```
"fatoom-sesame-gen|1|" ‖ uid ‖ "|" ‖ contactId ‖ JSON canonique {"v":1, "uid", "contactId", "generation"}
```

**Traitement** : exige une `generation` supérieure à la génération inscrite (absente ⇒ `0`), ou `-1` ; écrit `chartes.{contactId}`. `-1` signifie que le code de ce proche ne s'ouvre plus :

- posé par une **suppression** du proche, `-1` est **définitif** : aucune génération n'est plus acceptée pour ce `contactId` ;
- posé par un **refus du rôle** (`session/refuser-role`, §2.21), `-1` accepte ensuite une génération supérieure à la dernière génération positive, que le registre garde : le titulaire peut republier la charte de ce proche et le réinviter.

**Réponse `200`** : `{"contactId": "<id>", "generation": 2, "recu": "<JWS, reçu de génération>"}`

**Erreurs** : `generation_non_croissante` (y compris toute génération après une suppression) ; `message_refuse` (signature invalide) ; `titulaire_indisponible` (ligne absente, `supprime` ou rejetée) ; `trop_d_appels`.

### 2.7 `alerte/fiche`

| | |
|---|---|
| Appelant | app : fin de la création du coffre (non bloquant, retenté à chaque déverrouillage jusqu'au succès), changement du téléphone ou de l'email du titulaire |
| Authentification | signature `cleVie` |
| Limite | à fixer au lot 1 bis |

**Corps** : `{"uid": "<uid>", "generation": 1, "telephone": "<E.164>", "email": "<adresse>", "signature": "<base64>"}`, signature sur `"fatoom-sesame-alerte|1|" ‖ uid ‖ "|" ‖ generation` et le JSON canonique de la fiche.

**Traitement** : exige une génération strictement croissante ; scelle la fiche sous la clé KMS `alerte-seal` (aad `"fatoom-sesame-alerte|1|" ‖ uid ‖ "|" ‖ generation`) et la garde au registre (`ficheGeneration`, `ficheScellee`). Le client n'en garde aucune copie. Sésame l'ouvre seul pour envoyer ses alertes au titulaire.

**Réponse `200`** : `{"ficheGeneration": 1}`

**Erreurs** : `generation_non_croissante` ; `message_refuse` (signature invalide) ; `titulaire_indisponible` ; `trop_d_appels`.

### 2.8 `deces/signaler`

| | |
|---|---|
| Appelant | serveur du client (pour Fatoom : décès trouvé dans le fichier de l'INSEE, acte de décès validé, inactivité) |
| Authentification | `jetonFatoom` (§1.3) |
| Limite | 100 / min |

**Corps** :

```json
{"jetonFatoom": "<OIDC>", "uid": "<uid>", "source": "insee" | "acte" | "inactivite", "dateDeces": "<AAAA-MM-JJ>", "preuve": "<hex, SHA-256 du fichier de l'acte>"}
```

`preuve` est obligatoire pour `acte`. Pour `inactivite`, le corps est `{jetonFatoom, uid, source: "inactivite"}`, sans `dateDeces`.

**Traitement** : crée le signalement à **l'heure de Sésame** ; `transmissibleLe` = 23 h 59 min 59 s, heure de Paris, du 7ᵉ jour calendaire. Un signalement non annulé **de même nature** déjà présent est renvoyé sans en créer un second. Un signalement `deces` est accepté après une transmission par inactivité. Un signalement d'inactivité est refusé tant que `vivantLe` a moins de 3 mois et 30 jours. Sésame envoie lui-même ses alertes au titulaire (à l'inscription, puis à J3 et J6).

**Réponse `200`** : `{"signalementId": "<id>", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "recu": "<JWS, reçu de signalement>"}`

**Erreurs** : `jeton_invalide` ; `message_refuse` (source inconnue, `acte` sans `preuve`) ; `inactivite_trop_recente` ; `titulaire_indisponible` (ligne absente, `supprime` ou rejetée) ; `trop_d_appels`.

### 2.9 `deces/annuler`

| | |
|---|---|
| Appelant | app, geste « Je suis en vie, annuler la transmission », depuis n'importe quel appareil où le coffre est ouvert |
| Authentification | signature `cleVie` sur un défi |
| Limite | 10 / jour par `uid` |

**Corps** : `{"uid": "<uid>", "signalementId": "<id>", "defi": "<base64>", "signature": "<base64>"}`, signature sur :

```
"fatoom-sesame-annulation|1|" ‖ uid ‖ "|" ‖ signalementId ‖ JSON canonique {"v":1, "uid", "signalementId", "defi"}
```

**Traitement** : accepté seulement pour le signalement courant non annulé, avant `transmissibleLe` ; écrit `annuleLe`, `annulePar: annulation` ; notifie `signalementAnnule`. Sur une ligne rejetée par le contrôle de fraîcheur, l'annulation est acceptée (vérifiée avec la `cleViePub` de la ligne). Un signalement déjà annulé reçoit la même réponse que la première annulation. Après `transmissibleLe`, la transmission sur décès est définitive.

**Réponse `200`** :

```json
{"signalement": {"signalementId": "<id>", "source": "insee", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "annuleLe": "<ISO>"},
 "recu": "<JWS, reçu d'annulation>"}
```

**Erreurs** : `deja_transmis` (`transmissibleLe` passé) ; `defi_invalide` ; `message_refuse` (signature invalide, ou signalement autre que le signalement courant) ; `trop_d_appels`.

### 2.10 `lettre/secours`

| | |
|---|---|
| Appelant | serveur du client, quand l'email **et** le téléphone d'un proche ont échoué techniquement |
| Authentification | `jetonFatoom` |
| Limite | 100 / min |

**Corps** : `{"jetonFatoom": "<OIDC>", "uid": "<uid>", "contactId": "<id>"}`

**Traitement** : Sésame lit la charte du proche à la passerelle (§5.4) et vérifie que la transmission est ouverte (signalement à terme, non annulé) et que la charte a une adresse ; il tire alors un code d'invitation, inscrit `invitations/{lookup}` et commande la lettre ; le proche vérifiera son identité sur le portail. Le client ne peut que déclencher une lettre vers l'adresse scellée par le titulaire, jamais la rediriger.

**Réponse `200`** : `{"commandee": true | false}`. `false` quand la transmission n'est pas ouverte, que la charte n'a pas d'adresse ou qu'une lettre est déjà commandée ; le motif est journalisé par Sésame, jamais renvoyé : le client ne doit rien apprendre de la charte.

**Erreurs** : `jeton_invalide` ; `titulaire_indisponible` (ligne rejetée) ; `passerelle_indisponible` ; `fournisseur_indisponible` ; `trop_d_appels`.

### 2.11 `titulaire/supprimer`

| | |
|---|---|
| Appelant | serveur du client : suppression du compte, suppression planifiée, suppression du coffre après une transmission |
| Authentification | `jetonFatoom` |
| Limite | 100 / min |

**Corps** : `{"jetonFatoom": "<OIDC>", "uid": "<uid>"}`

**Traitement** : passe la ligne à l'état `supprime` (pierre tombale) : plus d'enrôlement, d'ouverture ni de PIN pour cet `uid`, définitivement. Sans ligne, rien n'est écrit.

**Réponse `200`** : `{"etat": "supprime"}`, ou `{"etat": "absent"}` pour un `uid` jamais enrôlé. Idempotente : un second appel répond pareil.

**Erreurs** : `jeton_invalide` ; `titulaire_indisponible` (ligne rejetée : le client réessaie après la reprise) ; `trop_d_appels`.

### 2.12 `pin/deposer`

| | |
|---|---|
| Appelant | app, coffre ouvert : création du coffre (après `enroler`, avant toute écriture chez le client), changement de PIN, nouveau PIN après une récupération, ouverture réussie sur un nouvel appareil, redépôt (enregistrement effacé ou nouvelle grappe) |
| Authentification | `idToken` **et** signature `cleVie` sur un défi |
| Limite | à fixer au lot 1 bis |

**Corps** :

```json
{"uid": "<uid>", "idToken": "<jeton d'identité>", "grappeId": "<id>", "elementAveugle": "<base64>",
 "defi": "<base64>", "motif": "nouveau_pin" | "ouverture" | "redepot", "signature": "<base64>"}
```

Signature sur :

```
"fatoom-pin-depot|1|" ‖ uid ‖ "|" ‖ grappeId ‖ JSON canonique {"v":1, "uid", "grappeId", "elementAveugle": hex(SHA-256(élément)), "defi", "motif"}
```

**Traitement** : exige une ligne `actif` ; chaque changement est validé par la majorité de la grappe avant la réponse.

- `nouveau_pin` (création du coffre, changement de PIN, nouveau PIN après une récupération) : nouvelle `cleOprf`, 10 essais ; seul motif admis au plafond ;
- `ouverture` : seulement juste après une évaluation sur **le même canal**, pour le même `elementAveugle` ; garde `cleOprf`, remet 10 essais, sans évaluation ;
- `redepot` (même PIN) : nouvelle `cleOprf`, garde le compteur d'essais.

Un dépôt dans une autre grappe que celle de la ligne n'est admis que 30 jours après l'entrée dans la grappe inscrite. Mémoire de la grappe à 90 % : les nouveaux dépôts sont refusés.

**Réponse `200`** : `{"evaluation": "<base64>" | null, "grappeId": "<id>"}` (`null` pour `ouverture`)

**Erreurs** : `jeton_invalide` ; `message_refuse` (signature absente ou d'une autre clé) ; `defi_invalide` ; `pin_bloque` (plafond atteint, motif autre que `nouveau_pin`) ; `grappe_recente` ; `pin_indisponible` (ligne absente, rejetée ou `supprime`) ; `pin_indisponible_temporaire` (pas de majorité, registre illisible, transaction refusée, mémoire pleine) ; `trop_d_appels`.

### 2.13 `pin/evaluer`

| | |
|---|---|
| Appelant | app : ouverture du coffre sur un nouvel appareil, ou sur un appareil connu après une déconnexion |
| Authentification | `idToken` seul |
| Limite | délai croissant ci-dessous ; au-delà, à fixer au lot 1 bis |

**Corps** : `{"uid": "<uid>", "idToken": "<jeton d'identité>", "elementAveugle": "<base64>"}`

**Traitement** : le leader de la grappe lit la ligne du registre, puis **décompte un essai**, validé par la majorité, **avant** d'évaluer. Délai croissant entre essais : aucun pour les 3 premiers, puis 1 min, 5 min, 15 min, 1 h, 4 h, 12 h, 24 h. Le dernier essai perdu efface `cleOprf`. Sésame alerte le titulaire à chaque essai décompté (au plus un message par heure). Sésame ne sait jamais si le PIN est juste : il n'existe pas de `pin_refuse`.

**Réponse `200`** : `{"evaluation": "<base64>"}`, sans nombre d'essais restants.

**Erreurs** : `jeton_invalide` ; `pin_indisponible` (aucun enregistrement, compte inconnu, ligne absente, rejetée ou `supprime` ; sans décompte, même réponse dans tous ces cas) ; `pin_indisponible_temporaire` (registre illisible, pas de majorité) ; `pin_bloque` ; `trop_d_essais` (délai en cours, `details.reessayerLe`) ; `trop_d_appels`.

### 2.14 `session/ouvrir`

| | |
|---|---|
| Appelant | portail, saisie du code d'accès (ou du code d'invitation reçu par lettre) |
| Authentification | le code |
| Limite | 20 / 15 min par IP ; 5 essais / 15 min par IP ; 3 sessions simultanées par `contactId` |

**Corps** : `{"code": "<code saisi>", "lookup": "<hex>"}`. `lookup` est l'index du code, étiré par Argon2id et calculé par le navigateur ; Sésame ne calcule aucun Argon2id.

**Traitement** :

1. Sésame cherche `invitations/{lookup}` au registre et vérifie le code par son `codeVerifier` ; trouvé : parcours « courrier » (identité, §2.16).
2. Sinon : `GET /bundle?lookup=` à la passerelle (§5.2), descellement de la KTS, ouverture de la charte ; vérifie `codeVerifier`, `charte.generation == registre.chartes[contactId]`, `ktsGeneration`, `isUnlocked`. Rôle `gardien` : accepté. Rôle `proche` : exige un signalement à terme, non annulé (inactivité : sans preuve de vie reçue après le terme ; décès : définitif).
3. Envoi du dernier code par Twilio Verify au téléphone de la charte, sinon à son email ; création de la session ; KTS effacée.

**Réponse `200`** :

```json
{"sessionId": "<id>", "canal": "sms" | "email" | "identite" | "aucun", "masque": "+33 6 •• •• •• 47" | "l•••••@exemple.fr" | null}
```

- `sms`, `email` : le portail demande le dernier code (§2.15). Le masque d'un email garde la première lettre et le domaine entier.
- `identite` : parcours « courrier » ; le portail enchaîne sur §2.16 (`masque: null`).
- `aucun` : invitation dont l'identité est déjà vérifiée (`identiteVerifieeLe`) : le proche entre avec son code seul, sans nouvelle vérification ni dernier code (`masque: null`). La réponse porte en plus les champs de §2.15 (`jeton`, `scope`, `role`, `expireLe`, `prenom`, `titulaireAffiche`) ; l'ouverture est inscrite et notifiée comme au §2.15.

**Erreurs** : `code_refuse`, **identique quel que soit le motif** (code inconnu, `codeVerifier` faux, charte révoquée, code non remis, proche avant l'échéance, signalement annulé, preuve de vie après une transmission par inactivité, `bundle` refusé par la passerelle, ligne rejetée), avec `details.essaisRestants` ; le motif est journalisé par Sésame. `trop_d_essais` (5 codes faux en 15 min, `details.reessayerLe`) ; `trop_d_appels` (y compris une 4ᵉ session simultanée pour le même proche) ; `passerelle_indisponible` ; `fournisseur_indisponible`.

### 2.15 `session/verifier`

| | |
|---|---|
| Appelant | portail, saisie du dernier code |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 5 essais / 15 min par proche |

**Corps** : `{"sessionId": "<id>", "otp": "<code reçu>"}`

**Traitement** : vérification par Twilio Verify ; succès : ouverture inscrite au registre, jeton de session émis, alerte de connexion au client (`openingNotified`, §5.5). Pour le contact de confiance du vivant du titulaire, Sésame alerte aussi lui-même le titulaire, une fois par session.

**Réponse `200`** :

```json
{"jeton": "<JWT ES256>", "scope": "destine", "role": "proche" | "gardien", "expireLe": "<ISO>", "prenom": "<prénom de la charte>", "titulaireAffiche": "<nom du titulaire>"}
```

`prenom` et `titulaireAffiche` sont lus dans la charte, pour l'affichage du portail (« Bonjour Léa »).

**Erreurs** : `otp_refuse` (`details.essaisRestants`) ; `trop_d_essais` (`details.reessayerLe`) ; `session_expiree` ; `canal_invalide` ; `fournisseur_indisponible` ; `trop_d_appels`.

### 2.16 `session/identite/demarrer`

| | |
|---|---|
| Appelant | portail : parcours « courrier », ou session basculée (§2.18) |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 5 / h par session |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : Sésame crée une session Stripe Identity (pièce d'identité et selfie exigés, aucune donnée du proche transmise à Stripe) avec sa propre clé restreinte.

**Réponse `200`** : `{"clientSecret": "<client_secret Stripe Identity>"}`

**Erreurs** : `identite_bloquee` (blocage de 24 h après 3 échecs, `details.reessayerLe`) ; `message_refuse` (session ni « courrier » ni basculée) ; `session_expiree` ; `canal_invalide` ; `fournisseur_indisponible` ; `trop_d_appels`.

### 2.17 `session/identite/verifier`

| | |
|---|---|
| Appelant | portail, au retour du module Stripe |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 3 échecs / 24 h ; 5 / h par session |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : Sésame relit la session Stripe. `verified` et prénom, nom et date de naissance conformes à la charte (comparaison tolérante) : `identiteVerifieeLe` inscrit, puis comme au §2.15 (ouverture inscrite, jeton, `openingNotified`). Sinon : `essaisIdentite + 1`, `letterIdentityFailed` au client, blocage de 24 h au 3ᵉ échec.

**Réponse `200`** : comme au §2.15.

**Erreurs** : `identite_non_confirmee` (identité non conforme ou `requires_input` ; message neutre) ; `identite_bloquee` (3ᵉ échec, `details.reessayerLe`) ; `identite_en_attente` (Stripe n'a pas fini : le portail réessaie) ; `session_expiree` ; `canal_invalide` ; `fournisseur_indisponible` ; `trop_d_appels`.

### 2.18 `session/identite/bascule`

| | |
|---|---|
| Appelant | portail, « Je n'ai plus ce numéro » ou « Je n'ai plus cet email », après `session/ouvrir` |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | à fixer au lot 1 bis |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : remplace le dernier code par la vérification d'identité (§2.16, §2.17 : mêmes règles, même blocage). Exige que la charte porte **le nom et la date de naissance** du proche ; sinon la bascule est refusée par une réponse neutre. Personne d'autre que le titulaire, de son vivant, ne fixe les coordonnées d'un proche.

**Réponse `200`** : `{"canal": "identite"}` ; le portail enchaîne sur `session/identite/demarrer`.

**Erreurs** : `bascule_impossible` (charte sans nom ou sans date de naissance ; affiché « Nous n'avons pas pu confirmer votre identité ») ; `session_expiree` ; `canal_invalide` ; `trop_d_appels`.

### 2.19 `session/ouvrir-cles`

| | |
|---|---|
| Appelant | portail, avec un jeton de session |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 500 enveloppes par appel, 60 appels / h par session, corps de 256 Kio |

**Corps** :

```json
{"sessionId": "<id>",
 "enveloppes": [{"type": "<type>", "id": "<contentId>", "contactId": "<id>", "enveloppe": "<base64>", "iv": "<base64>"}]}
```

Les enveloppes viennent du client (contenus listés avec le jeton de session) ; Sésame ne les a jamais vues.

**Traitement** : pour chaque enveloppe, Sésame reconstruit l'aad (`uid`, `type`, `id`, proche authentifié, génération du registre) et l'ouvre sous la KTS ; une enveloppe destinée à un autre proche est refusée ; le type `volontes` n'est ouvert que si `base = deces` ou `role = gardien`. La KTS est effacée après la réponse.

**Réponse `200`** (sous le canal : les clés ne sont lisibles que par le navigateur) :

```json
{"cles": [{"type": "<type>", "id": "<contentId>", "cle": "<base64, 32 octets>"}],
 "refusees": [{"type": "<type>", "id": "<contentId>", "erreur": "enveloppe_refusee"}],
 "jeton": "<JWT ES256 renouvelé>", "expireLe": "<ISO>"}
```

`type` accompagne `id` : deux contenus de types différents peuvent avoir le même identifiant. Une enveloppe refusée n'empêche pas les autres. Le jeton renouvelé matérialise l'inactivité : chaque appel réussi prolonge la session ; sans appel pendant 1 h, elle expire.

**Erreurs** : `enveloppe_refusee` (par élément, dans `refusees`) ; `message_refuse` (plus de 500 enveloppes, forme invalide) ; `session_expiree` ; `canal_invalide` ; `trop_d_appels`.

### 2.20 `session/fermer`

| | |
|---|---|
| Appelant | portail |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | à fixer au lot 1 bis |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : détruit la session côté Sésame et notifie `sessionTerminee` (§5.5), comme pour une session expirée après 1 h sans appel. Le jeton n'est plus renouvelé.

**Réponse `200`** : `{"fermee": true}`. Idempotente : une session déjà fermée ou expirée reçoit la même réponse.

**Erreurs** : `canal_invalide` ; `trop_d_appels`.

### 2.21 `session/refuser-role`

| | |
|---|---|
| Appelant | portail, contact de confiance dans une session |
| Authentification | `sessionId`, sur le canal qui l'a ouverte, de rôle `gardien` |
| Limite | à fixer au lot 1 bis |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : écrit `chartes.{contactId} = -1` au registre, notifie `roleRefused`, puis ferme la session (`sessionTerminee`). Le titulaire peut ensuite republier une charte pour ce proche (§2.6).

**Réponse `200`** : `{"refuse": true}`

**Erreurs** : `role_insuffisant` (session d'un proche qui n'est pas contact de confiance) ; `session_expiree` ; `canal_invalide` ; `trop_d_appels`.

## 3. Jetons d'attestation

Les contrôles d'un jeton d'attestation (émetteur, `image_digest` dans la liste compilée, version `en_service`, `hwmodel` admis, `eat_nonce`) sont décrits dans `ARCHITECTURE.md`. Ils s'appliquent à l'identique au client qui appelle Sésame, à la passerelle qui reçoit un appel de Sésame (§5.1) et à une VM qui rejoint la grappe (§4).

## 4. Routes entre VM

Servies seulement sur le canal mutuellement attesté entre VM de Sésame, jamais aux clients. Chaque membre est joint à son adresse DNS interne zonale.

### 4.1 `grappe/rejoindre`

| | |
|---|---|
| Appelant | une VM de Sésame qui démarre sans état |
| Authentification | attestation mutuelle : même `image_digest`, `hwmodel` admis |
| Limite | à fixer au lot 1 bis |

**Corps** : `{"jwt": "<jeton d'attestation de la VM>", "nodeId": "<id>"}`. Le jeton porte `image_digest`, `hwmodel` et la zone ; il lie l'identifiant de nœud neuf par `eat_nonce`.

**Traitement** : la VM est admise sans intervention si elle a le même `image_digest` et un `hwmodel` admis, si elle prend la zone d'un votant injoignable depuis au moins 2 minutes et si aucune autre modification de membres n'est en cours. Elle est ajoutée comme apprenante, promue, puis l'ancien membre est retiré. Sinon elle est refusée et la demande journalisée. Un ancien identifiant de nœud est refusé.

**Réponse** : `{"admise": true, "role": "apprenant"}`

**Erreurs** : `refusee` (hors de ces conditions) ; `pin_indisponible_temporaire` (pas de majorité).

### 4.2 `grappe/membres`

Route d'exploitation sous le même canal, validée par la majorité : passage à une autre option matérielle et toute autre modification de membres. Même contrôle d'attestation qu'au §4.1. Sa forme est conçue au lot 1 bis ; elle entre dans ce contrat avant d'être servie.

## 5. Passerelle : routes que le client expose à Sésame

Le client expose une passerelle HTTPS que seule une enclave attestée peut appeler. Son adresse est une variable attestée de Sésame. Fatoom l'implémente par une Cloud Function (`sesameGateway`).

### 5.1 Règles communes

Chaque appel porte cinq en-têtes :

| En-tête | Contenu |
|---|---|
| `X-Sesame-Attestation` | le JWT d'attestation courant de l'instance |
| `X-Sesame-Session-Pub` | `sessionPub` de l'instance (base64), pour vérifier `eat_nonce` |
| `X-Sesame-Date` | horodatage ISO 8601 UTC de la requête |
| `X-Sesame-Request-Id` | UUID v4 unique par requête |
| `X-Sesame-Signature` | base64 d'une signature ECDSA P-256 / SHA-256 par la clé KMS `session-sign` sur `UTF-8(méthode ‖ "|" ‖ chemin-et-query ‖ "|" ‖ X-Sesame-Date ‖ "|" ‖ X-Sesame-Request-Id ‖ "|" ‖ hex(SHA-256(corps)))` (corps vide ⇒ SHA-256 de zéro octet) |

La passerelle :

- vérifie le jeton d'attestation avec **les mêmes règles qu'un client** (§3) et `eat_nonce` contre `X-Sesame-Session-Pub` ;
- vérifie la signature avec la clé publique `session-sign` épinglée ;
- refuse un horodatage à plus de 5 min et un `X-Sesame-Request-Id` déjà vu dans les 10 min ;
- limite les appels à 300 / min par instance.

Échec : `401`, journal `SESAME_ATTESTATION_REJECTED` avec `imageDigest`.

Le trafic Sésame → passerelle n'a pas de canal applicatif : JSON en clair sous TLS. Ce qu'il transporte (chartes et scellés chiffrés, identifiants opaques, statuts) n'est pas exploitable sans l'enclave.

Les routes sont servies sous l'adresse de la passerelle : `GET /bundle` et `POST /<notification>` (corps JSON).

### 5.2 `GET /bundle?lookup=<hex>`

Retrouve un proche par l'index de son code d'accès.

**Réponse `200`** :

```json
{"uid": "<uid>", "contactId": "<id>", "charte": "<base64>", "charteIv": "<base64>", "charteGeneration": 1,
 "sealedKts": "<base64>", "ktsGeneration": 1, "isUnlocked": true, "titulaireAffiche": "<nom>"}
```

**Refus** : `404` sans motif, pour un index inconnu comme pour un `bundle` refusé (contact de confiance d'un compte limité au-delà de la date de fin de sa transmission). Sésame répond dans les deux cas `code_refuse` au portail.

### 5.3 `GET /bundle?uid=<uid>&courrier=1`

Les chartes des proches à prévenir par lettre (`courrier == true`).

**Réponse `200`** :

```json
{"uid": "<uid>", "sealedKts": "<base64>", "ktsGeneration": 1, "titulaireAffiche": "<nom>",
 "chartes": [{"contactId": "<id>", "charte": "<base64>", "charteIv": "<base64>", "charteGeneration": 1}]}
```

### 5.4 `GET /bundle?uid=<uid>&contactId=<id>`

Un proche par son identifiant : la lettre de secours vise un proche qui n'est pas « courrier », et le parcours « courrier » retrouve le proche par `invitations/{lookup}` (`uid`, `contactId`). Sésame ne garde aucune charte. **Réponse** : comme au §5.2 ; `404` sans motif sinon.

Le client ne peut que retenir une charte, jamais la modifier (elle est authentifiée sous la KTS) : un déni de service est visible, une substitution est impossible.

### 5.5 Notifications

`POST /<nom>`, corps JSON. La passerelle répond `200 {}` ; tout autre statut, ou une erreur réseau, fait réessayer Sésame avec un délai croissant pendant 24 h, puis la notification est journalisée comme perdue. Toute notification est idempotente côté client sur `X-Sesame-Request-Id`. Les notifications ne portent **jamais** de code, de clé ni de résultat d'identité.

| Route | Quand | Corps |
|---|---|---|
| `openingNotified` | **alerte de connexion**, une seule fois par session, à la délivrance du jeton (`session/verifier`, `session/identite/verifier`, ou `session/ouvrir` pour une identité déjà vérifiée), pour **tout** proche | `{uid, contactId, role, ouvertLe, lieu, navigateur}` : qui, quand, lieu approximatif et navigateur, déduits par Sésame de l'IP et de l'agent du navigateur et jamais conservés au registre. Elle ne porte pas les contenus ouverts |
| `sessionTerminee` | fermeture d'une session de proche, ou expiration après 1 h sans appel ; une seule fois par session | `{uid, contactId, ouvertLe, fermeeLe, contenus: [{type, id}]}` : les contenus dont la clé a été ouverte |
| `signalementAnnule` | annulation explicite, ou preuve de vie reçue pendant un signalement d'inactivité | `{uid, signalementId, annulePar: "annulation" \| "preuve_de_vie", annuleLe}` |
| `roleRefused` | refus du rôle par le contact de confiance | `{uid, contactId}` |
| `lifeAfterTransmission` | première preuve de vie après une transmission par **inactivité** | `{uid, signalementId, vivantLe}` |
| `letterSent` | lettre commandée | `{uid, contactId, pingenDocumentId}`, sans le code |
| `letterStatus` | changement d'état chez le prestataire d'impression | `{uid, contactId, pingenDocumentId, statut: "commandee" \| "imprimee" \| "en_distribution" \| "distribuee" \| "erreur"}` |
| `letterFailed` | trois erreurs d'impression ou de distribution | `{uid, contactId}` |
| `letterIdentityFailed` | échec de la vérification d'identité d'un proche « courrier » | `{uid, contactId, essaisIdentite}` |

Il n'existe pas de notification `echeanceModifiee` : l'échéance d'un signalement ne change jamais, et la passerelle refuse une notification de ce nom.

## 6. Codes d'erreur

| Code | Statut | Quand |
|---|---|---|
| `canal_invalide` | 400, en clair | corps indéchiffrable, `n` non croissant, secret nul, session présentée depuis un autre canal |
| `instance_inconnue` | 410, en clair | `instanceId` inconnu (redémarrage) : le client reprend à `GET /attestation` |
| `attestation_indisponible` | 503, en clair | jeton d'attestation pas encore obtenu du lanceur |
| `message_refuse` | 400 | corps mal formé, champ manquant, valeur hors contrat, signature `cleVie` invalide |
| `jeton_invalide` | 401 | `idToken` ou jeton OIDC refusé |
| `code_refuse` | 403 | code d'accès refusé, quel que soit le motif (`details.essaisRestants`) |
| `otp_refuse` | 403 | dernier code faux (`details.essaisRestants`) |
| `role_insuffisant` | 403 | action réservée au contact de confiance |
| `preuve_requise` | 403 | ré-enrôlement sans preuve `cleVie` valide |
| `identite_non_confirmee` | 403 | identité non conforme à la charte ; message neutre |
| `bascule_impossible` | 403 | charte sans nom ou sans date de naissance ; message neutre |
| `deja_transmis` | 409 | annulation après `transmissibleLe` |
| `generation_non_croissante` | 409 | génération de KTS, de charte ou de fiche non croissante, ou génération après une suppression de proche |
| `defi_invalide` | 409 | défi inconnu, expiré ou déjà utilisé |
| `inactivite_trop_recente` | 409 | signalement d'inactivité avant le plancher |
| `grappe_recente` | 409 | dépôt dans une autre grappe moins de 30 jours après l'entrée dans la grappe inscrite |
| `pin_bloque` | 409 | plafond de 10 essais atteint |
| `pin_indisponible` | 409 | aucun enregistrement du PIN, compte inconnu, ligne absente, rejetée ou `supprime` |
| `identite_en_attente` | 409 | Stripe n'a pas encore rendu son verdict |
| `titulaire_indisponible` | 409 | ligne absente, `supprime` ou rejetée (hors routes du PIN) |
| `refusee` | 409 | VM hors des conditions d'entrée dans la grappe |
| `session_expiree` | 410 | session sans activité depuis 1 h, fermée ou inconnue |
| `identite_bloquee` | 423 | 3 échecs d'identité : 24 h (`details.reessayerLe`) |
| `trop_d_essais` | 429 | 5 essais de code ou de dernier code en 15 min, ou délai d'essai du PIN en cours (`details.reessayerLe`) |
| `trop_d_appels` | 429 | limite d'une route dépassée (`details.reessayerLe`) |
| `pin_indisponible_temporaire` | 503 | pas de majorité, registre illisible, transaction refusée, mémoire pleine |
| `registre_indisponible` | 503 | registre ou bucket des marqueurs illisible |
| `passerelle_indisponible` | 503 | la passerelle ne répond pas ou répond hors contrat |
| `fournisseur_indisponible` | 503 | Twilio, Stripe ou le prestataire d'impression en échec |
| `enveloppe_refusee` | — | par enveloppe, dans `refusees` : aad étranger, tag invalide, contenu non ouvrable |

Sauf les trois premiers, le statut est celui du corps chiffré (§1.2) ; la réponse HTTP est `200`.

## 7. Limites

Spécifiées : 5 essais / 15 min pour le code d'accès (par IP) et le dernier code (par proche) ; 3 échecs / 24 h pour l'identité ; défi à usage unique valable 5 min ; 10 essais par PIN, avec le délai croissant du §2.13 ; 1 h d'inactivité pour une session de portail.

Valeurs initiales pour le reste :

| Route | Limite |
|---|---|
| `GET /attestation` | 60 / min par IP |
| `defi` | 30 / h par `uid` |
| `enroler` | 5 / h par `uid`, 20 / h par IP |
| `vie` | 10 / jour par `uid` |
| `etat` | 100 / jour par `uid`, 60 / min par IP |
| `charte/generation` | 30 / min par `uid` |
| `deces/annuler` | 10 / jour par `uid` |
| `deces/signaler`, `titulaire/supprimer`, `lettre/secours` | 100 / min |
| `session/ouvrir` | 20 / 15 min par IP ; 3 sessions simultanées par `contactId` |
| `session/identite/*` | 5 / h par session |
| `session/ouvrir-cles` | 500 enveloppes par appel, 60 appels / h par session |
| Corps déchiffré | 64 Kio ; 256 Kio pour `session/ouvrir-cles` |
| Passerelle (appels de Sésame) | 300 / min par instance |
| `pin/deposer`, `pin/evaluer` au-delà du délai croissant, `alerte/fiche`, `session/identite/bascule`, `session/fermer`, `session/refuser-role`, `grappe/rejoindre` | à fixer au lot 1 bis |

Un dépassement renvoie `429 trop_d_appels` avec `details.reessayerLe`.
