<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), « Sésame : contrat HTTP » -->

# API de Sésame

Ce document est le contrat des routes que Sésame expose et des routes que le client doit exposer à Sésame (la passerelle). Il publie le contrat HTTP de la spécification de l'opérateur (« Sésame : contrat HTTP », dépôt Fatoom), qui fait foi ; l'OpenAPI, les vecteurs de test et le double local `sesame-fake` en dérivent. En cas d'écart, la source se corrige d'abord.

Les limites de débit du §7 sont des valeurs initiales, révisables avec les tests de charge du lot 1 bis, sauf celles que le §7 dit spécifiées ou décidées. Tout le reste est fixé par la spécification de référence.

## 1. Conventions

### 1.1 Adresse et transport

- Sésame est joint à l'adresse publique de l'instance (pour Fatoom : `https://sesame.fatoom.fr`). TLS est terminé par le répartiteur de charge, hors de l'enclave : il ne protège rien à lui seul.
- Sésame tourne sur trois VM, une par zone. `GET /attestation` est servie sans préfixe, par n'importe quelle VM, hors canal. Elle renvoie l'`instanceId` de la VM, dont le client déduit le préfixe de la VM (`a`, `b` ou `c`, §1.2) ; il n'y a pas de champ `zone`.
- Toute autre route passe par le **canal chiffré** d'`ARCHITECTURE.md`, à **un seul chemin** : `POST /n/{a|b|c}/canal`, adressé à la VM dont le client a vérifié l'attestation. Le nom de la route est dans le corps chiffré : le répartiteur, que l'administrateur règle, ne distingue pas une annulation d'une ouverture par un proche.
- Le canal est hybride X25519 + ML-KEM-768, avec une clé par sens (`K_req` pour les requêtes, `K_res` pour les réponses), dérivées comme le décrit `ARCHITECTURE.md`. Les messages du canal sont décrits au §1.2.
- Requêtes et réponses sont en `Content-Type: application/json; charset=utf-8` ; toute réponse porte `Cache-Control: no-store`.
- Le portail appelle Sésame depuis une autre origine. Toute réponse porte `Access-Control-Allow-Origin: *` ; une requête `OPTIONS` sur `/attestation` ou `/n/{a|b|c}/canal` reçoit `204` avec `Access-Control-Allow-Methods: GET, POST`, `Access-Control-Allow-Headers: Content-Type` et `Access-Control-Max-Age: 86400`. Sésame ne lit ni cookie ni en-tête d'authentification : l'origine ne protège rien, le canal protège tout.
- L'adresse IP d'un appelant, clé des limites par IP (§7), est l'avant-dernière valeur de `X-Forwarded-For`, celle qu'ajoute le répartiteur. Elle sert contre l'abus, jamais contre l'administrateur, qui règle le répartiteur.

### 1.2 Messages du canal

**Requête**, corps de `POST /n/{a|b|c}/canal` :

```json
{"instanceId": "<id>", "cliPub": "<base64, X25519, 32 octets>", "kemCt": "<base64, ML-KEM-768, 1088 octets>",
 "ivPrefixe": "<base64, 4 octets>", "n": 1, "body": "<base64>"}
```

- Un **canal** est désigné par (`instanceId`, `cliPub`). Le client le crée : il tire une paire X25519 (`cliPriv`, `cliPub`), encapsule vers `kemPub` (`kemCt`) et tire `ivPrefixe`, 4 octets de la primitive d'aléa. `kemCt` et `ivPrefixe` sont les mêmes sur toutes les requêtes du canal ; une requête qui présente un `cliPub` connu avec un autre `kemCt` ou un autre `ivPrefixe` est refusée (`canal_invalide`). Le client ne réutilise jamais une paire X25519 d'un canal à l'autre.
- `n` est le compteur des requêtes du client sur ce canal : entier de 1 à 2³² − 1, strictement supérieur au dernier `n` accepté sur ce canal. Les trous sont admis : une requête perdue ne bloque pas le canal. Le client n'a qu'une requête en cours par canal ; pour des requêtes parallèles, il ouvre plusieurs canaux.
- `IV_n` = `ivPrefixe` ‖ `n` en entier non signé sur 8 octets, gros-boutiste : 12 octets.
- `body` = AES-256-GCM(`K_req`, `IV_n`, UTF-8(JSON canonique de `{"route", "corps"}`), aad = UTF-8(`"fatoom-sesame-req|"` ‖ `n` en décimal)), chiffré ‖ tag de 16 octets, en base64 standard. Le corps déchiffré nomme la route et porte ses données :

```json
{"route": "deces/annuler", "corps": {…}}
```

- Sésame n'avance le dernier `n` d'un canal qu'une fois la requête déchiffrée : une requête indéchiffrable ne consomme rien.

**Réponse**, en `HTTP 200` :

```json
{"n": 1, "body": "<base64>"}
```

- `n` est celui de la requête ; `body` = AES-256-GCM(`K_res`, `IV_n`, UTF-8(JSON canonique de la réponse, §1.3), aad = UTF-8(`"fatoom-sesame-res|"` ‖ `n` en décimal)).
- Il n'y a pas de compteur des réponses : la réponse à la requête `n` reprend `IV_n` sous l'autre clé. Chaque couple (clé, IV) ne sert qu'une fois : `K_req` avec `IV_n` pour la seule requête `n`, `K_res` avec `IV_n` pour sa seule réponse. Sésame ne chiffre jamais deux réponses pour un même `n` d'un même canal : une requête rejouée, ou renvoyée avec le même `n`, reçoit `400 canal_invalide` en clair.
- Le client refuse une réponse dont `n` n'est pas celui de sa requête, ou qui ne s'ouvre pas sous `K_res` ; il abandonne alors le canal. Il ne sait pas si la requête a été traitée : il ne la renvoie qu'avec un nouveau `n` ou sur un nouveau canal, et chaque route dit ce que donne une répétition.

**Vie d'un canal** :

- Le client ouvre un nouveau canal (nouvelle paire X25519, nouvelle encapsulation, nouveau `ivPrefixe`) avant que `n` dépasse 2³² − 1, après `410 instance_inconnue`, après `400 canal_invalide`, après une réponse refusée et après 1 h sans requête sur le canal. Vers la même instance, il reprend `sessionPub` et `kemPub` déjà vérifiées tant que le jeton vérifié n'a pas expiré (`exp`) ; sinon il refait `GET /attestation`.
- Sésame ferme un canal resté 1 h sans requête (valeur initiale, révisable avec les tests de charge du lot 1 bis) : ses clés, ses défis et ses sessions de portail sont effacés. L'instance garde jusqu'à son arrêt la trace de chaque canal qu'elle a fermé, dans un filtre de Bloom de taille fixe (16 Mio, valeur initiale, révisable avec les tests de charge du lot 1 bis), et refuse toute requête d'un canal qu'il contient (`400 canal_invalide`) : elle ne chiffre plus jamais sous ses clés. Un faux positif refuse un canal neuf : le client en ouvre un autre.
- Un redémarrage de la VM ferme tous ses canaux : `410 instance_inconnue`.

**`instanceId`** :

- Forme : `<p>-<22 caractères>`, expression `^[abc]-[A-Za-z0-9_-]{22}$`. `<p>` est le préfixe de routage de la VM : le répartiteur envoie `/n/<p>/…` à cette VM, et `<p>` est le dernier caractère du nom de la VM, qui se termine par `-a`, `-b` ou `-c` ; une VM dont le nom ne se termine pas ainsi refuse de démarrer. Les 22 caractères sont 16 octets de la primitive d'aléa, en base64url sans remplissage.
- Tiré par l'instance à son démarrage, en mémoire seulement, jamais réutilisé : une VM redémarrée en tire un nouveau. Il est distinct de l'identifiant de nœud de la grappe (§4.1).
- Émis : inscrit à `instances/{instanceId}` au démarrage (`ARCHITECTURE.md`), renvoyé par `GET /attestation`, porté par les bornes du registre et les journaux.
- Vérifié par l'instance sur chaque requête du canal : un `instanceId` autre que le sien reçoit `410 instance_inconnue`. Le client en lit `<p>` pour choisir son chemin.
- Il n'est pas attesté : un `instanceId` faux ne peut que faire échouer les requêtes, jamais exposer un message, car les clés du canal sont liées à `sessionPub` et `kemPub`, attestées.

### 1.3 Statut HTTP et erreurs

Toute réponse que l'enclave chiffre part en **`HTTP 200`**. Le vrai statut est dans le corps chiffré :

```json
{"statut": 200, "corps": {…}}
{"statut": 403, "erreur": "<code>", "message": "<phrase en français, non destinée à l'affichage>", "details": {…}}
```

- `statut` est un entier : `200`, ou le statut du code au §6.
- `corps` n'existe que pour `200` ; `erreur` et `message` n'existent que pour une erreur ; `details` n'existe que quand le code l'indique : `essaisRestants` (entier ≥ 0), `reessayerLe` (horodatage). Aucun autre champ.

Un `403` ou un `409` en clair dirait au répartiteur quelle action a eu lieu. `erreur` est un code stable en `snake_case` français (liste §6) ; `message` peut changer. Les clients décident de l'affichage d'après `erreur`, jamais d'après `message`.

Seules les erreurs hors canal sont renvoyées **en clair**, avec leur statut HTTP et le corps `{"erreur": "<code>", "message": "<phrase>"}` (plus `details.reessayerLe` pour `429`) : l'instance ne peut pas chiffrer pour un canal qu'elle ne reconnaît pas.

| Statut en clair | Code | Quand |
|---|---|---|
| `410` | `instance_inconnue` | `instanceId` inconnu (VM redémarrée) : le client recommence à `GET /attestation` |
| `400` | `canal_invalide` | enveloppe de requête mal formée ou de plus de 400 Kio, corps indéchiffrable, `n` hors de 1 à 2³² − 1 ou non strictement croissant, `kemCt` ou `ivPrefixe` différent pour un même `cliPub`, canal fermé, secret X25519 nul, ou session présentée depuis un autre canal que celui qui l'a ouverte |
| `503` | `attestation_indisponible` | `GET /attestation` avant que l'instance ait obtenu son jeton |
| `429` | `trop_d_appels` | `GET /attestation` au-delà de sa limite (`details.reessayerLe`) |

Toute autre réponse (statut ou forme hors contrat, erreur du répartiteur, coupure) est pour le client une panne réseau : il ne sait pas si la requête a été traitée (§1.2).

### 1.4 Authentification

| Appelant | Comment Sésame l'authentifie |
|---|---|
| App du titulaire, coffre ouvert | signature Ed25519 par la clé de vie `cleVie`, vérifiée avec la `cleViePub` du registre, sur un message « chaîne de domaine ‖ JSON canonique » ; la plupart des messages portent un **défi** (§1.5) |
| App du titulaire, coffre verrouillé (`etat`) et essai de PIN (`pin/evaluer`) | jeton d'identité du titulaire émis par le fournisseur d'identité du client (pour Fatoom : `idToken` Firebase) |
| App du titulaire, `enroler` | `idToken` ; en plus, une preuve `cleVie` dès qu'une ligne existe pour cet `uid` |
| App du titulaire, `pin/deposer` | `idToken` **et** signature `cleVie` ; l'`idToken` n'est jamais suffisant seul |
| Portail d'un proche | code d'accès ou d'invitation, puis dernier code (SMS ou email) ou vérification d'identité, puis `sessionId`. Une session est **liée au canal** qui l'a ouverte (`instanceId`, `cliPub`) : présentée depuis un autre canal, elle est refusée (`canal_invalide`) |
| Serveur du client | jeton OIDC Google d'un compte de service (pour Fatoom : `<compte-deces>`), dans le champ `jetonFatoom` du corps chiffré, jamais en en-tête (l'en-tête traverse le répartiteur en clair). Accepté seulement si la signature Google est valide, `exp` non passé, `iss` = `https://accounts.google.com`, `aud` = `https://sesame.fatoom.fr`, `email` = compte de service en variable attestée et `email_verified` vrai ; sinon `jeton_invalide` |
| Exploitant de Sésame (`grappe/amorcer`) | jeton OIDC Google du compte d'exploitation de Sésame (`<compte-exploitation>`), dans le champ `jetonExploitation` du corps chiffré, vérifié comme `jetonFatoom` : signature Google valide, `exp` non passé, `iss` = `https://accounts.google.com`, `aud` = `https://sesame.fatoom.fr`, `email` = compte d'exploitation en variable attestée et `email_verified` vrai ; sinon `jeton_invalide` |
| VM de Sésame | canal mutuellement attesté entre VM : même `image_digest`, `hwmodel` admis (§4) |

L'`idToken` est accepté seulement si sa signature est valide pour les clés publiques de l'émetteur, `exp` non passé, `iss` et `aud` ceux de l'émetteur en variable attestée (pour Fatoom : `iss` = `https://securetoken.google.com/<identifiant du projet Firebase>`, `aud` = cet identifiant) et `sub` égal à l'`uid` du corps ; sinon `jeton_invalide`.

### 1.5 Défis

Un défi est tiré par Sésame (`defi`, §2.2) : 32 octets, à usage unique, valable 5 minutes, lié au canal qui l'a demandé et à l'`uid` du corps, gardé en mémoire de l'instance. Il sert à la preuve de vie, à l'annulation d'un signalement, à la preuve d'enrôlement et au dépôt du PIN. Sésame l'efface dès qu'il l'a vérifié, que la signature soit juste ou non. Un défi inconnu, expiré, déjà utilisé, demandé pour un autre `uid` ou sur un autre canal est refusé (`defi_invalide`). Une VM redémarrée a perdu ses défis : le client reçoit `410` et recommence. Un canal fermé perd aussi les siens.

### 1.6 Pièces signées par `session-sign`

Toutes sont signées par la clé KMS `session-sign` (ECDSA P-256, SHA-256), en `ES256` seul : `alg` est imposé, tout autre est refusé. La clé publique `session-sign` est lue par `kid` dans `clesSession` de la liste des versions compilée (`ARCHITECTURE.md`) ; le Sésame factice du lot 1 bis signe avec une clé fixe épinglée dans sa configuration. Le client conserve un reçu ; il ne lui donne aucun droit : Sésame ne lit jamais un reçu, il relit son registre.

- Chaque pièce est un JWS compact (RFC 7515) d'en-tête `{"alg": "ES256", "kid": "session-sign/<version KMS>"}`, plus `"typ": "JWT"` pour le jeton de session.
- La signature est au format JWS, `R ‖ S` sur 64 octets (RFC 7518) ; KMS rend une signature DER, que Sésame convertit.
- `iat` et `exp` sont en secondes depuis l'époque Unix ; les autres dates sont des horodatages (§1.7).

| Pièce | Forme |
|---|---|
| Jeton de session | JWT ; claims `{iss: "sesame", sub: <contactId>, uid, scope: "destine", base: "vivant" \| "deces" \| "inactivite", role: "proche" \| "gardien", iat, exp}`, `exp = iat + 3600` ; pas de `sid` |
| Reçu de signalement | `{iss: "sesame", typ: "signalement", uid, signalementId, source, signaleLe, transmissibleLe, iat}` |
| Reçu d'annulation | `{iss: "sesame", typ: "annulation", uid, signalementId, annuleLe, iat}` |
| Reçu de génération | `{iss: "sesame", typ: "generation", uid, contactId, generation, iat}` |
| Réponse de `etat` | `{iss: "sesame", typ: "etat", …}` (§2.5) |

Le `typ` sépare les domaines : une pièce d'un type n'est jamais acceptée pour un autre. Le jeton de session n'a pas de claim `typ` : il se reconnaît à son `scope`.

### 1.7 Formats des champs

- Les corps décrits ci-dessous sont les JSON **en clair, avant chiffrement**, écrits en JSON canonique (RFC 8785), UTF-8. Sésame refuse un corps à clé dupliquée, à champ inconnu, à champ obligatoire absent, à `null` hors contrat ou à type faux, et une route inconnue (`message_refuse`).
- Taille d'un corps déchiffré : 64 Kio au plus, 256 Kio pour `session/ouvrir-cles` ; au-delà, `message_refuse`.
- Octets : base64 standard avec remplissage (RFC 4648, §4), sauf mention de base64url ou d'hexadécimal (en minuscules). Une longueur fausse est refusée (`message_refuse`). Longueurs : `defi` 32 octets ; `signature` Ed25519 64 ; `cleViePub` 32 ; `kts` 32 ; `elementAveugle` et `evaluation` 32 (élément ristretto255 sérialisé, mode OPRF de la RFC 9497, suite `ristretto255-SHA512`) ; `enveloppe` 48 ; `iv` 12 ; `cle` 32 ; `sessionPub` et `cliPub` 32 ; `kemPub` 1184 ; `kemCt` 1088.
- Identifiants : `uid`, `contactId` et `id` d'un contenu ont de 1 à 128 caractères dans `[A-Za-z0-9_-]` (`ARCHITECTURE.md`). Ceux que Sésame tire viennent de la primitive d'aléa, en base64url sans remplissage : `signalementId` (16 octets, 22 caractères), `sessionId` (32 octets, 43 caractères), `instanceId` (§1.2). `grappeId` est un hexadécimal de 64 caractères (`ARCHITECTURE.md`).
- Horodatages : ISO 8601 UTC à la milliseconde, suffixe `Z` (`2026-10-12T14:03:00.000Z`). Dates civiles : `AAAA-MM-JJ`.
- Nombres : entiers JSON sans fraction ni exposant, de −(2⁵³ − 1) à 2⁵³ − 1.
- Dans une chaîne de domaine ou une aad, un entier s'écrit en décimal ASCII sans zéro en tête, `-1` avec son signe.
- Téléphone : E.164, `^\+[1-9][0-9]{1,14}$`.
- Chaînes saisies par une personne : en NFC (`ARCHITECTURE.md`) ; Sésame refuse une chaîne qui n'est pas en NFC (`message_refuse`) au lieu de la normaliser. Seul le code d'accès est envoyé tel que saisi : Sésame le normalise lui-même (`normalize`, `ARCHITECTURE.md`).

## 2. Routes de Sésame

Sauf `GET /attestation`, chaque route s'appelle par `POST /n/{a|b|c}/canal` avec `{"route": "<nom>", "corps": {…}}` ; le nom est donné en titre. Les statuts sont ceux du corps chiffré (§1.3). En plus des erreurs de chaque route, toute route du canal peut renvoyer :

- `message_refuse` (400) : route inconnue, corps hors contrat (§1.7) ;
- `trop_d_appels` (429, `details.reessayerLe`) : au-delà de sa limite (§7) ;
- `registre_indisponible` (503) : registre illisible, pour une route qui lit ou écrit le registre, hors routes du PIN (`pin_indisponible_temporaire`).

### 2.1 `GET /attestation`

| | |
|---|---|
| Appelant | tout client : app, portail, passerelle du client |
| Authentification | aucune ; hors canal |
| Entrée | aucune |
| Limite | 60 / min par IP |

**Réponse `200`**, en clair :

```json
{"jwt": "<JWT signé par Google>", "sessionPub": "<base64, X25519, 32 octets>", "kemPub": "<base64, ML-KEM-768, 1184 octets>", "instanceId": "<id>"}
```

Le client vérifie chaque claim du jeton (§3) et que `eat_nonce` contient deux valeurs, `base64url(SHA-256(sessionPub))` et `base64url(SHA-256(kemPub))` ; sinon il n'envoie rien. `instanceId` a la forme du §1.2.

**Erreurs**, en clair : `attestation_indisponible` (503 ; jeton pas encore obtenu du lanceur : l'instance ne sert rien d'autre) ; `trop_d_appels` (429).

### 2.2 `defi`

| | |
|---|---|
| Appelant | app, coffre ouvert, avant une preuve de vie, une annulation, un enrôlement prouvé ou un dépôt du PIN |
| Authentification | aucune au-delà du canal : un défi ne donne rien seul |
| Limite | 30 / h par canal, 120 / h par IP (valeur initiale, révisable avec les tests de charge du lot 1 bis) ; aucune limite par `uid` : qui connaît un `uid` ne peut pas épuiser les défis du titulaire |

**Corps** : `{"uid": "<uid>"}`

**Traitement** : Sésame tire le défi, le lie au canal et à l'`uid` (§1.5) ; il ne lit pas le registre : un défi s'obtient pour tout `uid`.

**Réponse `200`** : `{"defi": "<base64, 32 octets>", "expireLe": "<ISO>", "grappeId": "<hex, 64 caractères>"}`

`grappeId` identifie la grappe du protocole de PIN en service : l'app en a besoin pour signer un dépôt (§2.12), y compris à la création du coffre, où aucune évaluation ne précède.

**Erreurs** : seulement les erreurs communes.

### 2.3 `enroler`

| | |
|---|---|
| Appelant | app : création du coffre (avant toute écriture chez le client), reprise d'une création interrompue, rotation de la KTS, ré-enrôlement après la perte du registre |
| Authentification | `idToken` ; preuve `cleVie` exigée dès qu'une ligne `titulaires/{uid}` existe |
| Limite | 5 / h par `uid`, 20 / h par IP |

**Corps** :

```json
{"uid": "<uid>", "idToken": "<jeton d'identité>", "ktsGeneration": 1, "kts": "<base64, 32 octets>",
 "cleViePub": "<base64, Ed25519, 32 octets>", "preuve": {"defi": "<base64>", "signature": "<base64>"},
 "cleViePubAttendue": "<base64, Ed25519, 32 octets>"}
```

`ktsGeneration` est un entier ≥ 1. `kts` est la KTS en clair **à l'intérieur du corps chiffré** par le canal. `cleViePubAttendue` n'existe que pendant une reprise sans sauvegarde (ci-dessous). `preuve` est absente au premier enrôlement. Sinon, c'est la signature par la `cleVie` courante de :

```
"fatoom-sesame-enroler|1|" ‖ uid ‖ JSON canonique {"v":1, "uid", "defi", "generation", "cleViePub"}
```

`generation` est la `ktsGeneration` demandée, `cleViePub` la clé de vie envoyée : une rotation ne peut pas substituer une autre clé de vie.

**Traitement** :

- **premier enrôlement** (aucune ligne) : `idToken` seul ; le marqueur d'inscription est créé avant la ligne ; `vivantLe = enroleLe` ;
- **génération supérieure** : rotation, preuve exigée ;
- **génération égale** : rescellement seul (reprise d'une création interrompue) ; la preuve doit se vérifier avec la `cleViePub` inscrite, la `cleViePub` reçue doit lui être égale, ni les chartes ni l'enregistrement du PIN ne sont touchés ;
- **génération inférieure** : refusée.

Sésame scelle la KTS sous la clé KMS `kts-seal`, crée ou met à jour la ligne, puis efface `kts`. Une répétition à la même génération, avec une preuve sur un nouveau défi, est un rescellement : même effet.

**Reprise sans sauvegarde** : pendant une reprise publiée marquée `sansSauvegarde` (`ARCHITECTURE.md`), une ligne absente dont le marqueur d'inscription existe admet un enrôlement avec `preuve`, vérifiée contre `cleViePubAttendue`, champ ajouté au corps, que l'app lit dans `users/{uid}.sesame.cleViePub`. Hors reprise, ce champ est refusé. Fatoom pourrait y substituer une autre clé : limite assumée (LIM-54). Le titulaire est alerté par Fatoom du ré-enrôlement.

**Réponse `200`** : `{"sealedKts": "<base64>", "ktsGeneration": 1, "imageDigest": "sha256:<hex, 64 caractères>", "cleViePub": "<base64>", "enroleLe": "<ISO>"}`

`sealedKts` est le chiffré rendu par KMS, opaque pour le client. `enroleLe` est la date du premier enrôlement de la ligne, inchangée par une rotation ou un rescellement.

**Erreurs** : `jeton_invalide` (401) ; `preuve_requise` (403 ; ligne existante, preuve absente ou invalide) ; `defi_invalide` (409) ; `generation_non_croissante` (409) ; `message_refuse` (400 ; corps mal formé, `cleViePub` absente, `cleViePub` différente de l'inscrite à génération égale, `cleViePubAttendue` hors reprise) ; `titulaire_indisponible` (409 ; ligne `supprime` ou rejetée, ou ligne absente alors qu'un marqueur d'inscription existe) ; `registre_indisponible` (503 ; registre ou bucket des marqueurs illisible : aucun premier enrôlement).

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

`signalement` est le signalement courant : le dernier inscrit, annulé ou non. `ouvertures` : sessions de proches depuis la dernière preuve de vie. `transmis` vaut `true` dès que `transmissibleLe` d'un signalement non annulé est passé, qu'une ouverture ait eu lieu ou non : l'app affiche alors `TransmissionDefinitive` (R-DECES-22) ; `signalement` et `ouvertures` disent de quoi il s'agit et qui a ouvert quoi. `pinPresent: false` : l'enregistrement du PIN manque dans la grappe en service, l'app redépose. `pinBloque: true` : le plafond d'essais est atteint, seul un nouveau PIN en sort.

**Erreurs** : `defi_invalide` (409) ; `message_refuse` (400 ; signature invalide) ; `titulaire_indisponible` (409 ; ligne absente ou `supprime`).

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

Le repère est tenu par l'app : `ouvertures` est l'`ouvertLe` de la dernière ouverture qu'elle a vue ; `evaluations` est la dernière valeur qu'elle a reçue du compteur des évaluations du PIN décomptées, tenu par la grappe (entier ≥ 0). L'app avance son repère après sa propre évaluation. Le compteur repart de zéro dans une nouvelle grappe : un `repere.evaluations` supérieur au compteur courant est traité comme `0`.

**Réponse `200`** : `{"etat": "<JWS compact ES256>"}`, de charge utile :

```json
{"iss": "sesame", "typ": "etat", "uid": "<uid>", "iat": 1760270580,
 "signalement": null | {…}, "ouvertures": […], "pinPresent": true, "pinBloque": false,
 "evaluationsDepuis": 0, "repere": {"ouvertures": "<ISO>" | null, "evaluations": 0}}
```

`signalement` et les éléments d'`ouvertures` ont la forme de `vie` ; `ouvertures` ne contient que celles dont `ouvertLe` est strictement postérieur au repère. `evaluationsDepuis` = compteur courant − `repere.evaluations` ; non nul, l'app affiche une alerte locale. `repere` est le repère à jour, que l'app garde.

**Erreurs** : `jeton_invalide` (401) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée).

### 2.6 `charte/generation`

| | |
|---|---|
| Appelant | app, **avant** toute écriture d'une charte chez le client : régénération du code, révocation, changement de rôle ou de coordonnées, suppression du proche, republication après un refus du rôle |
| Authentification | signature `cleVie`, sans défi : la génération strictement croissante refuse le rejeu |
| Limite | 30 / min par `uid` |

**Corps** : `{"uid": "<uid>", "contactId": "<id>", "generation": 2, "signature": "<base64>"}`, `generation` entier ≥ 1 ou `-1`, signature sur :

```
"fatoom-sesame-gen|1|" ‖ uid ‖ "|" ‖ contactId ‖ JSON canonique {"v":1, "uid", "contactId", "generation"}
```

**Traitement** : exige une `generation` supérieure à la génération inscrite (absente ⇒ `0`), ou `-1` ; écrit `chartes.{contactId}`. `-1` signifie que le code de ce proche ne s'ouvre plus :

- posé par une **suppression** du proche, `-1` est **définitif** : aucune génération n'est plus acceptée pour ce `contactId` ;
- posé par un **refus du rôle** (`session/refuser-role`, §2.21), `-1` accepte ensuite une génération supérieure à la dernière génération positive, que le registre garde : le titulaire peut republier la charte de ce proche et le réinviter.

La même requête renvoyée après son succès reçoit `generation_non_croissante` : le reçu déjà obtenu fait foi pour l'app.

**Réponse `200`** : `{"contactId": "<id>", "generation": 2, "recu": "<JWS, reçu de génération>"}`

**Erreurs** : `generation_non_croissante` (409 ; y compris toute génération après une suppression) ; `message_refuse` (400 ; signature invalide) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée).

### 2.7 `alerte/fiche`

| | |
|---|---|
| Appelant | app : fin de la création du coffre (non bloquant, retenté à chaque déverrouillage jusqu'au succès), changement du téléphone ou de l'email du titulaire |
| Authentification | signature `cleVie` |
| Limite | 10 / jour par `uid` (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"uid": "<uid>", "generation": 1, "telephone": "<E.164>" | null, "email": "<adresse>", "signature": "<base64>"}`. `generation` est un entier ≥ 1 ; `email`, l'adresse du compte, en NFC (§1.7). La signature porte sur `"fatoom-sesame-alerte|1|" ‖ uid ‖ "|" ‖ generation ‖ JSON canonique de la fiche`, la fiche étant `{"email":"<adresse>","generation":1,"telephone":"<E.164>"|null,"uid":"<uid>","v":1}` : clés triées, UTF-8, sans espace ni retour à la ligne. La génération empêche de rejouer une ancienne fiche. Les vecteurs sont générés par script (A-27).

**Traitement** : exige une génération strictement supérieure à `ficheGeneration` (absente ⇒ `0`) ; scelle la fiche sous la clé KMS `alerte-seal` (aad `"fatoom-sesame-alerte|1|" ‖ uid ‖ "|" ‖ generation`) et la garde au registre (`ficheGeneration`, `ficheScellee`). Le client n'en garde aucune copie. Sésame l'ouvre seul pour envoyer ses alertes au titulaire.

**Réponse `200`** : `{"ficheGeneration": 1}`

**Erreurs** : `generation_non_croissante` (409) ; `message_refuse` (400 ; signature invalide, `email` ou `telephone` hors format, chaîne qui n'est pas en NFC) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée).

### 2.8 `deces/signaler`

| | |
|---|---|
| Appelant | serveur du client (pour Fatoom : décès trouvé dans le fichier de l'INSEE, acte de décès validé, inactivité) |
| Authentification | `jetonFatoom` (§1.4) |
| Limite | 100 / min pour le compte de service appelant |

**Corps** :

```json
{"jetonFatoom": "<OIDC>", "uid": "<uid>", "source": "insee" | "acte" | "inactivite", "dateDeces": "<AAAA-MM-JJ>", "preuve": "<hex, SHA-256 du fichier de l'acte>"}
```

`dateDeces` est obligatoire pour `insee` et `acte`. `preuve` (64 caractères hexadécimaux) est obligatoire pour `acte` et absente pour `insee`. Pour `inactivite`, le corps est `{jetonFatoom, uid, source: "inactivite"}`, sans `dateDeces`.

**Traitement** : crée le signalement à **l'heure de Sésame** ; `transmissibleLe` = 23 h 59 min 59 s, heure de Paris, du 7ᵉ jour calendaire. Un signalement non annulé **de même nature** déjà présent est renvoyé sans en créer un second : même `signalementId`, `signaleLe` et `transmissibleLe`. Un signalement `deces` est accepté après une transmission par inactivité. Un signalement d'inactivité est refusé tant que `vivantLe` a moins de 3 mois et 30 jours. Sésame envoie lui-même ses alertes au titulaire (à l'inscription, puis à J3 et J6).

**Réponse `200`** : `{"signalementId": "<id>", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "recu": "<JWS, reçu de signalement>"}`

**Erreurs** : `jeton_invalide` (401) ; `message_refuse` (400 ; source inconnue, `acte` sans `preuve`, `dateDeces` absente pour `insee` ou `acte`) ; `inactivite_trop_recente` (409) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée).

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

**Erreurs** : `deja_transmis` (409 ; `transmissibleLe` passé) ; `defi_invalide` (409) ; `message_refuse` (400 ; signature invalide, ou signalement autre que le signalement courant) ; `titulaire_indisponible` (409 ; ligne absente ou `supprime`).

### 2.10 `lettre/secours`

| | |
|---|---|
| Appelant | serveur du client, quand l'email **et** le téléphone d'un proche ont échoué techniquement |
| Authentification | `jetonFatoom` |
| Limite | 100 / min pour le compte de service appelant |

**Corps** : `{"jetonFatoom": "<OIDC>", "uid": "<uid>", "contactId": "<id>"}`

**Traitement** : Sésame lit la charte du proche à la passerelle (§5.4) et vérifie que la transmission est ouverte (signalement à terme, non annulé) et que la charte a une adresse ; il tire alors un code d'invitation, inscrit `invitations/{lookup}` et commande la lettre ; le proche vérifiera son identité sur le portail. Le client ne peut que déclencher une lettre vers l'adresse scellée par le titulaire, jamais la rediriger.

**Réponse `200`** : `{"commandee": true | false}`. `false` quand la transmission n'est pas ouverte, que la charte n'a pas d'adresse ou qu'une lettre est déjà commandée ; le motif est journalisé par Sésame, jamais renvoyé : le client ne doit rien apprendre de la charte. Une répétition après une lettre commandée reçoit donc `false`.

**Erreurs** : `jeton_invalide` (401) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée) ; `reessayer_plus_tard` (503 ; contrôle anti-retour du registre en cours) ; `passerelle_indisponible` (503) ; `fournisseur_indisponible` (503).

### 2.11 `titulaire/supprimer`

| | |
|---|---|
| Appelant | serveur du client : suppression du compte, suppression planifiée, suppression du coffre après une transmission |
| Authentification | `jetonFatoom` |
| Limite | 100 / min pour le compte de service appelant |

**Corps** : `{"jetonFatoom": "<OIDC>", "uid": "<uid>"}`

**Traitement** : passe la ligne à l'état `supprime` (pierre tombale) : plus d'enrôlement, d'ouverture ni de PIN pour cet `uid`, définitivement. Sans ligne, rien n'est écrit.

**Réponse `200`** : `{"etat": "supprime"}`, ou `{"etat": "absent"}` pour un `uid` jamais enrôlé. Idempotente : un second appel répond pareil.

**Erreurs** : `jeton_invalide` (401) ; `titulaire_indisponible` (409 ; ligne rejetée : le client réessaie après la reprise).

### 2.12 `pin/deposer`

| | |
|---|---|
| Appelant | app, coffre ouvert : création du coffre (après `enroler`, avant toute écriture chez le client), changement de PIN, nouveau PIN après une récupération, ouverture réussie sur un nouvel appareil, redépôt (enregistrement effacé ou nouvelle grappe) |
| Authentification | `idToken` **et** signature `cleVie` sur un défi |
| Limite | 10 / h par `uid` (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** :

```json
{"uid": "<uid>", "idToken": "<jeton d'identité>", "grappeId": "<hex, 64 caractères>", "elementAveugle": "<base64, 32 octets>",
 "defi": "<base64>", "motif": "nouveau_pin" | "ouverture" | "redepot", "signature": "<base64>"}
```

Signature sur :

```
"fatoom-pin-depot|1|" ‖ uid ‖ "|" ‖ grappeId ‖ JSON canonique {"v":1, "uid", "grappeId", "elementAveugle": hex(SHA-256(élément)), "defi", "motif"}
```

`élément` est l'élément aveuglé sérialisé (32 octets) ; son empreinte s'écrit en hexadécimal de 64 caractères.

**Traitement** : exige une ligne `actif` ; chaque changement est validé par la majorité de la grappe avant la réponse.

- `nouveau_pin` (création du coffre, changement de PIN, nouveau PIN après une récupération) : nouvelle `cleOprf`, 10 essais ; seul motif admis au plafond ;
- `ouverture` : seulement juste après une évaluation sur **le même canal**, pour le même `elementAveugle` ; garde `cleOprf`, remet 10 essais, sans évaluation ;
- `redepot` (même PIN) : nouvelle `cleOprf`, garde le compteur d'essais.

Un dépôt dans une autre grappe que celle de la ligne n'est admis que 30 jours après l'entrée dans la grappe inscrite. Mémoire de la grappe à 90 % : les nouveaux dépôts sont refusés. Une répétition demande un nouveau défi : un `nouveau_pin` ou un `redepot` répété tire encore une nouvelle `cleOprf`, et seule la dernière évaluation reçue vaut.

**Réponse `200`** : `{"evaluation": "<base64, 32 octets>" | null, "grappeId": "<hex, 64 caractères>"}` (`null` pour `ouverture`)

**Erreurs** : `jeton_invalide` (401) ; `message_refuse` (400 ; signature absente ou d'une autre clé, `ouverture` sans évaluation notée sur ce canal ou pour un autre élément) ; `defi_invalide` (409) ; `pin_bloque` (409 ; plafond atteint, motif autre que `nouveau_pin`) ; `grappe_recente` (409) ; `pin_indisponible` (409 ; ligne absente, rejetée ou `supprime`) ; `pin_indisponible_temporaire` (503 ; pas de majorité, registre illisible, transaction refusée, mémoire pleine) ; `reessayer_plus_tard` (503 ; contrôle anti-retour en échec avant un dépôt dans une autre grappe, sans détail).

### 2.13 `pin/evaluer`

| | |
|---|---|
| Appelant | app : ouverture du coffre sur un nouvel appareil, ou sur un appareil connu après une déconnexion |
| Authentification | `idToken` seul |
| Limite | délai croissant ci-dessous ; au-delà, 20 appels / h par `uid` et 60 / h par IP (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"uid": "<uid>", "idToken": "<jeton d'identité>", "elementAveugle": "<base64, 32 octets>"}`

**Traitement** : le leader de la grappe lit la ligne du registre, puis **décompte un essai**, validé par la majorité, **avant** d'évaluer. Délai croissant entre essais : aucun pour les 3 premiers, puis 1 min, 5 min, 15 min, 1 h, 4 h, 12 h, 24 h. Le 10ᵉ essai est évalué et répondu comme les autres (l'app vérifie le PIN sur place) ; `essaisRestants` tombe alors à 0 et `cleOprf` est effacée : un dépôt `ouverture` ultérieur est refusé (`pin_bloque`). Un 10ᵉ PIN juste n'ouvre donc pas : seul le code de récupération reste possible (R-PIN-02). Sésame alerte le titulaire à chaque essai décompté (au plus un message par heure). Sésame ne sait jamais si le PIN est juste : il n'existe pas de `pin_refuse`. Sésame note sur le canal la dernière évaluation, pour un dépôt `ouverture` (§2.12).

**Réponse `200`** : `{"evaluation": "<base64, 32 octets>"}`, sans nombre d'essais restants.

**Erreurs** : `jeton_invalide` (401) ; `pin_indisponible` (409 ; aucun enregistrement, compte inconnu, ligne absente, rejetée ou `supprime` ; sans décompte, même réponse dans tous ces cas) ; `pin_indisponible_temporaire` (503 ; registre illisible, pas de majorité) ; `pin_bloque` (409) ; `trop_d_essais` (429 ; délai en cours, `details.reessayerLe`).

### 2.14 `session/ouvrir`

| | |
|---|---|
| Appelant | portail, saisie du code d'accès (ou du code d'invitation reçu par lettre) |
| Authentification | le code |
| Limite | 20 / 15 min par IP ; 5 essais / 15 min par IP ; 3 sessions simultanées par `contactId` |

**Corps** : `{"code": "<code saisi>", "lookup": "<hex>"}`. `code` est envoyé tel que saisi (§1.7). `lookup` est l'index du code (`indexAcces`, hexadécimal en minuscules), étiré par Argon2id et calculé par le navigateur ; Sésame ne calcule un Argon2id que pour l'index d'un code d'invitation, une fois par lettre (§2.10).

**Traitement** :

1. Sésame cherche `invitations/{lookup}` au registre et vérifie le code par son `codeVerifier` ; trouvé : parcours « courrier » (identité, §2.16).
2. Sinon : `GET /bundle?lookup=` à la passerelle (§5.2), descellement de la KTS, ouverture de la charte ; vérifie `codeVerifier`, `charte.generation == registre.chartes[contactId]` (absente du registre ⇒ seule la génération 1 est acceptée, `ARCHITECTURE.md`), `ktsGeneration`, `isUnlocked`. Rôle `gardien` : accepté. Rôle `proche` : exige un signalement à terme, non annulé (inactivité : sans preuve de vie reçue après le terme ; décès : définitif).
3. Envoi du dernier code par Twilio Verify au téléphone de la charte, sinon à son email ; création de la session, liée au canal ; KTS effacée.

**Réponse `200`** :

```json
{"sessionId": "<id>", "canal": "sms" | "email" | "identite" | "aucun", "masque": "+33 6 •• •• •• 47" | "l•••••@exemple.fr" | null}
```

- `sms`, `email` : le portail demande le dernier code (§2.15). Le masque d'un email garde la première lettre et le domaine entier.
- `identite` : parcours « courrier » ; le portail enchaîne sur §2.16 (`masque: null`).
- `aucun` : invitation dont l'identité est déjà vérifiée (`identiteVerifieeLe`) : le proche entre avec son code seul, sans nouvelle vérification ni dernier code (`masque: null`). La réponse porte en plus les champs de §2.15 (`jeton`, `scope`, `base`, `role`, `expireLe`, `prenom`, `titulaireAffiche`) ; l'ouverture est inscrite et notifiée comme au §2.15.

**Erreurs** : `code_refuse` (403), **identique quel que soit le motif** (code inconnu, `codeVerifier` faux, charte révoquée, code non remis, proche avant l'échéance, signalement annulé, preuve de vie après une transmission par inactivité, `bundle` refusé par la passerelle, ligne rejetée), avec `details.essaisRestants` ; le motif est journalisé par Sésame. `trop_d_essais` (429 ; 5 codes faux en 15 min, `details.reessayerLe`) ; `reessayer_plus_tard` (503 ; contrôle anti-retour du registre en cours, sans détail ni raison, sans décompter les essais) ; `trop_d_appels` (429 ; y compris une 4ᵉ session simultanée pour le même proche) ; `passerelle_indisponible` (503) ; `fournisseur_indisponible` (503).

### 2.15 `session/verifier`

| | |
|---|---|
| Appelant | portail, saisie du dernier code |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 5 essais / 15 min par proche ; 20 appels / 15 min par session (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"sessionId": "<id>", "otp": "<code reçu>"}`, `otp` en chiffres seulement, tel que Twilio Verify l'a envoyé.

**Traitement** : vérification par Twilio Verify ; succès : ouverture inscrite au registre, jeton de session émis, alerte de connexion au client (`openingNotified`, §5.5). Pour le contact de confiance du vivant du titulaire, Sésame alerte aussi lui-même le titulaire, une fois par session.

**Réponse `200`** :

```json
{"jeton": "<JWT ES256>", "scope": "destine", "base": "vivant" | "deces" | "inactivite", "role": "proche" | "gardien", "expireLe": "<ISO>", "prenom": "<prénom de la charte>", "titulaireAffiche": "<nom du titulaire>"}
```

`scope`, `base` et `role` sont ceux du jeton (§1.6) ; `expireLe` est son `exp`. `base` vaut `vivant` pour le contact de confiance avant toute transmission, `deces` pour une transmission après décès, `inactivite` pour une transmission par inactivité ; elle reste `inactivite` jusqu'à la fin des accès quand un décès est vérifié ensuite (`ARCHITECTURE.md`, étape 7). `prenom` et `titulaireAffiche` sont lus dans la charte, pour l'affichage du portail (« Bonjour Léa »).

**Erreurs** : `otp_refuse` (403, `details.essaisRestants`) ; `trop_d_essais` (429, `details.reessayerLe`) ; `reessayer_plus_tard` (503 ; contrôle anti-retour, sans décompte) ; `session_expiree` (410) ; `canal_invalide` (400, en clair) ; `fournisseur_indisponible` (503).

### 2.16 `session/identite/demarrer`

| | |
|---|---|
| Appelant | portail : parcours « courrier », ou session basculée (§2.18) |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 5 / h par session |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : Sésame crée une session Stripe Identity (pièce d'identité et selfie exigés, aucune donnée du proche transmise à Stripe) avec sa propre clé restreinte.

**Réponse `200`** : `{"clientSecret": "<client_secret Stripe Identity>"}`

**Erreurs** : `identite_bloquee` (423 ; blocage de 24 h après 3 échecs, `details.reessayerLe`) ; `message_refuse` (400 ; session ni « courrier » ni basculée) ; `session_expiree` (410) ; `canal_invalide` (400, en clair) ; `fournisseur_indisponible` (503).

### 2.17 `session/identite/verifier`

| | |
|---|---|
| Appelant | portail, au retour du module Stripe |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 3 échecs / 24 h ; 5 / h par session |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : Sésame relit la session Stripe. `verified` et prénom, nom et date de naissance conformes à la charte (comparaison tolérante) : `identiteVerifieeLe` inscrit, puis comme au §2.15 (ouverture inscrite, jeton, `openingNotified`). Sinon : `essaisIdentite + 1`, `letterIdentityFailed` au client, blocage de 24 h au 3ᵉ échec.

**Réponse `200`** : comme au §2.15.

**Erreurs** : `identite_non_confirmee` (403 ; identité non conforme ou `requires_input` ; message neutre) ; `identite_bloquee` (423 ; 3ᵉ échec, `details.reessayerLe`) ; `identite_en_attente` (409 ; Stripe n'a pas fini : le portail réessaie) ; `reessayer_plus_tard` (503 ; contrôle anti-retour, sans décompte) ; `session_expiree` (410) ; `canal_invalide` (400, en clair) ; `fournisseur_indisponible` (503).

### 2.18 `session/identite/bascule`

| | |
|---|---|
| Appelant | portail, « Je n'ai plus ce numéro » ou « Je n'ai plus cet email », après `session/ouvrir` |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 5 / h par session (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : remplace le dernier code par la vérification d'identité (§2.16, §2.17 : mêmes règles, même blocage). Exige que la charte porte **le nom et la date de naissance** du proche ; sinon la bascule est refusée par une réponse neutre. Personne d'autre que le titulaire, de son vivant, ne fixe les coordonnées d'un proche.

**Réponse `200`** : `{"canal": "identite"}` ; le portail enchaîne sur `session/identite/demarrer`.

**Erreurs** : `bascule_impossible` (403 ; charte sans nom ou sans date de naissance ; affiché « Nous n'avons pas pu confirmer votre identité ») ; `session_expiree` (410) ; `canal_invalide` (400, en clair).

### 2.19 `session/ouvrir-cles`

| | |
|---|---|
| Appelant | portail, avec un jeton de session |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 500 enveloppes par appel, 60 appels / h par session, corps de 256 Kio |

**Corps** :

```json
{"sessionId": "<id>",
 "enveloppes": [{"type": "fichier" | "motdepasse" | "mot" | "volontes", "id": "<contentId>", "contactId": "<id>", "enveloppe": "<base64, 48 octets>", "iv": "<base64, 12 octets>"}]}
```

Les enveloppes viennent du client (contenus listés avec le jeton de session) ; Sésame ne les a jamais vues.

**Traitement** : pour chaque enveloppe, Sésame reconstruit l'aad (`uid`, `type`, `id`, proche authentifié, génération du registre) et l'ouvre sous la KTS ; une enveloppe destinée à un autre proche est refusée ; le type `volontes` s'ouvre comme les autres types. La KTS est effacée après la réponse.

**Réponse `200`** (sous le canal : les clés ne sont lisibles que par le navigateur) :

```json
{"cles": [{"type": "<type>", "id": "<contentId>", "cle": "<base64, 32 octets>"}],
 "refusees": [{"type": "<type>", "id": "<contentId>", "erreur": "enveloppe_refusee"}],
 "jeton": "<JWT ES256 renouvelé>", "expireLe": "<ISO>"}
```

`type` accompagne `id` : deux contenus de types différents peuvent avoir le même identifiant. Une enveloppe refusée n'empêche pas les autres. Le jeton renouvelé matérialise l'inactivité : chaque appel réussi prolonge la session ; sans appel pendant 1 h, elle expire.

**Erreurs** : `enveloppe_refusee` (par élément, dans `refusees`) ; `message_refuse` (400 ; plus de 500 enveloppes, forme invalide) ; `reessayer_plus_tard` (503 ; contrôle anti-retour) ; `session_expiree` (410) ; `canal_invalide` (400, en clair).

### 2.20 `session/fermer`

| | |
|---|---|
| Appelant | portail |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 10 / h par canal (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : détruit la session côté Sésame et notifie `sessionTerminee` (§5.5), comme pour une session expirée après 1 h sans appel. Le jeton n'est plus renouvelé.

**Réponse `200`** : `{"fermee": true}`. Idempotente : une session déjà fermée ou expirée reçoit la même réponse.

**Erreurs** : `canal_invalide` (400, en clair).

### 2.21 `session/refuser-role`

| | |
|---|---|
| Appelant | portail, contact de confiance dans une session |
| Authentification | `sessionId`, sur le canal qui l'a ouverte, de rôle `gardien` |
| Limite | 5 / h par session (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : écrit `chartes.{contactId} = -1` au registre, notifie `roleRefused`, puis ferme la session (`sessionTerminee`). Le titulaire peut ensuite republier une charte pour ce proche (§2.6).

**Réponse `200`** : `{"refuse": true}`

**Erreurs** : `role_insuffisant` (403 ; session d'un proche qui n'est pas contact de confiance) ; `session_expiree` (410) ; `canal_invalide` (400, en clair).

## 3. Jetons d'attestation

Les contrôles d'un jeton d'attestation (émetteur, `image_digest` dans la liste compilée, version `en_service`, `hwmodel` admis, `eat_nonce`) sont décrits dans `ARCHITECTURE.md`. Ils s'appliquent à l'identique au client qui appelle Sésame, à la passerelle qui reçoit un appel de Sésame (§5.1) et à une VM qui rejoint la grappe (§4).

## 4. Routes entre VM

Servies seulement sur le canal mutuellement attesté entre VM de Sésame, jamais aux clients, sauf `grappe/amorcer` (§4.3), route du canal du §1.2. Chaque membre est joint à son adresse DNS interne zonale, sans passer par le répartiteur : sur ce canal, le statut HTTP est le vrai statut et le corps d'erreur est `{"erreur": "<code>", "message": "<phrase>"}`.

### 4.1 `grappe/rejoindre`

| | |
|---|---|
| Appelant | une VM de Sésame qui démarre sans état |
| Authentification | attestation mutuelle : même `image_digest`, `hwmodel` admis |
| Limite | 2 / min par `nodeId` (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"jwt": "<jeton d'attestation de la VM>", "nodeId": "<id>"}`. `nodeId` est l'identifiant de nœud neuf : 16 octets de la primitive d'aléa, en base64url sans remplissage. Le jeton porte `image_digest`, `hwmodel` et la zone ; il lie l'identifiant de nœud neuf par `eat_nonce`.

**Traitement** : la VM est admise sans intervention si elle a le même `image_digest` et un `hwmodel` admis, si elle prend la zone d'un votant injoignable depuis au moins 2 minutes et si aucune autre modification de membres n'est en cours. Elle est ajoutée comme apprenante, promue, puis l'ancien membre est retiré. Sinon elle est refusée et la demande journalisée (`grappe_admission_refusee`, alerte à l'exploitant, `ARCHITECTURE.md`). Un ancien identifiant de nœud est refusé. Une VM sans grappe répond `refusee`.

**Réponse `200`** : `{"admise": true, "role": "apprenant"}`

**Erreurs** : `refusee` (409 ; hors de ces conditions, ou VM jointe sans grappe) ; `pin_indisponible_temporaire` (503 ; pas de majorité) ; `message_refuse` (400) ; `trop_d_appels` (429).

### 4.2 `grappe/membres`

Route d'exploitation sous le même canal, validée par la majorité : passage à une autre option matérielle, déplacement de la place d'une VM pendant l'indisponibilité longue d'une zone (`ARCHITECTURE.md`) et toute autre modification de membres. Même contrôle d'attestation qu'au §4.1. Sa forme est conçue au lot 1 bis avec le prototype Raft ; elle entre dans ce contrat avant d'être servie.

### 4.3 Amorçage d'une grappe neuve

Commandé par l'exploitant après le remplacement des trois VM (nouvelle version, effacement total) ; procédure et raisons : `ARCHITECTURE.md`. Conçu au lot 1 bis avec le prototype Raft, écrit ici avant d'être servi.

**Configuration initiale**, identique pour les trois VM, en JSON canonique, `membres` triés par `zone`, exactement trois éléments :

```json
{"v": 1, "imageDigest": "sha256:<hex, 64 caractères>",
 "membres": [{"nodeId": "<id>", "zone": "<zone>", "cle": "<base64, clé du canal entre VM>"}, …]}
```

`nodeId` est l'identifiant de nœud neuf du membre (§4.1), `zone` sa zone (`submods.gce.zone` de son jeton), `cle` la clé publique de son canal entre VM, liée à son attestation par `eat_nonce`. `grappeId` = hex(SHA-256(ce JSON canonique)).

#### `grappe/amorcer`

| | |
|---|---|
| Appelant | l'exploitant, après avoir remplacé les trois VM, une par zone |
| Authentification | `jetonExploitation` (§1.4), en plus du canal (§1.2) ; la route n'agit que sur trois VM sans grappe et ne donne ni état, ni essai, ni clé |
| Limite | 2 / min par IP (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

Route du canal du §1.2 : `POST /n/{a|b|c}/canal`, `{"route": "grappe/amorcer", "corps": {…}}`, vers l'une des trois VM ; statuts dans le corps chiffré (§1.3).

**Corps** : `{"jetonExploitation": "<OIDC>"}`

**Traitement** : servie seulement par une VM qui n'est dans aucune grappe et n'a réservé aucune configuration. Elle joint les deux autres VM à leur adresse DNS interne zonale sur le canal entre VM et vérifie leur attestation (§3 : même `image_digest`, `hwmodel` admis) ; elle exige exactement trois VM, une par zone, aucune dans une grappe. Elle compose la configuration initiale, la fait réserver par les deux autres (`grappe/preparer`), puis ordonne leur démarrage (`grappe/demarrer`) et démarre la dernière. Les trois démarrent Raft en état `new`, les trois membres votants, avec la même configuration initiale. La grappe neuve est publiée dans la borne de chaque VM (`grappe`, `ARCHITECTURE.md`) ; l'amorçage est journalisé (`grappe_amorcee`, avec l'`email` du jeton).

**Réponse `200`**, une fois une première entrée validée par la majorité de la grappe neuve : `{"grappeId": "<hex, 64 caractères>", "votants": 3}`

**Erreurs** : `jeton_invalide` (401) ; `amorcage_refuse` (409 ; VM déjà dans une grappe ou configuration réservée, autre VM injoignable, déjà dans une grappe ou d'une autre empreinte, attestation refusée, deux VM dans une même zone, réservation refusée) ; `pin_indisponible_temporaire` (503 ; grappe démarrée sans majorité dans le délai de réservation : elle reste en pause) ; `trop_d_appels` (429).

#### `grappe/preparer`

| | |
|---|---|
| Appelant | la VM qui a reçu `grappe/amorcer`, sur le canal entre VM |
| Authentification | attestation mutuelle : même `image_digest`, `hwmodel` admis |
| Limite | 2 / min par `nodeId` (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"configuration": {…}}` (configuration initiale ci-dessus)

**Traitement** : la VM, qui n'est dans aucune grappe et n'a rien réservé, vérifie sur ses propres canaux vers chaque membre que la configuration décrit les VM qu'elle voit (`nodeId`, `zone`, `cle`, `imageDigest` égal au sien), puis la réserve pendant 2 minutes (valeur initiale, révisable avec les tests de charge du lot 1 bis) : elle refuse alors toute autre configuration. Sans `grappe/demarrer` dans ce délai, elle l'oublie et redevient sans grappe.

**Réponse `200`** : `{"reservee": true, "grappeId": "<hex, 64 caractères>"}`

**Erreurs** : `amorcage_refuse` (409 ; VM dans une grappe, réservation en cours, configuration qui ne décrit pas les VM vues) ; `message_refuse` (400) ; `trop_d_appels` (429).

#### `grappe/demarrer`

| | |
|---|---|
| Appelant | la VM qui a fait réserver la configuration, sur le canal entre VM |
| Authentification | attestation mutuelle : même `image_digest`, `hwmodel` admis |
| Limite | 2 / min par `nodeId` (valeur initiale, révisable avec les tests de charge du lot 1 bis) |

**Corps** : `{"grappeId": "<hex, 64 caractères>"}`

**Traitement** : accepté seulement de la VM qui a fait réserver, pour la configuration réservée, dans le délai ; la VM démarre Raft en état `new` avec les trois membres votants de la configuration.

**Réponse `200`** : `{"demarree": true}`

**Erreurs** : `amorcage_refuse` (409 ; aucune réservation, autre `grappeId` ou autre VM, délai passé) ; `message_refuse` (400) ; `trop_d_appels` (429).

## 5. Passerelle : routes que le client expose à Sésame

Le client expose une passerelle HTTPS que seule une enclave attestée peut appeler. Son adresse est une variable attestée de Sésame. Fatoom l'implémente par une Cloud Function (`sesameGateway`).

### 5.1 Règles communes

Chaque appel porte cinq en-têtes :

| En-tête | Contenu |
|---|---|
| `X-Sesame-Attestation` | le JWT d'attestation courant de l'instance |
| `X-Sesame-Session-Pub` | `sessionPub` de l'instance (base64), pour vérifier `eat_nonce` |
| `X-Sesame-Date` | horodatage ISO 8601 UTC de la requête (§1.7) |
| `X-Sesame-Request-Id` | UUID v4 unique par requête, en minuscules |
| `X-Sesame-Signature` | base64 de la signature ECDSA P-256 / SHA-256, au format DER que rend KMS, par la clé KMS `session-sign` sur `UTF-8(méthode ‖ "|" ‖ chemin-et-query ‖ "|" ‖ X-Sesame-Date ‖ "|" ‖ X-Sesame-Request-Id ‖ "|" ‖ hex(SHA-256(corps)))` (corps vide ⇒ SHA-256 de zéro octet) |

La passerelle :

- vérifie le jeton d'attestation avec **les mêmes règles qu'un client** (§3) et `eat_nonce` contre `X-Sesame-Session-Pub` ;
- vérifie la signature ; la clé publique `session-sign` est lue par `kid` dans `clesSession` de la liste des versions compilée (§1.6) ;
- refuse un horodatage à plus de 5 min ;
- répond `200 {}`, sans nouveau traitement, à une notification (§5.5) dont le `X-Sesame-Request-Id` est déjà traité ; ce contrôle passe avant le refus de rejeu ;
- refuse un `X-Sesame-Request-Id` de `GET /bundle` déjà vu dans les 10 min ;
- limite les appels à 300 / min par instance.

Échec de l'attestation, de la signature, de l'horodatage ou de l'identifiant : `401`, journal `SESAME_ATTESTATION_REJECTED` avec `imageDigest`. Au-delà de la limite : `429`. Requête hors contrat : `400`. Sésame traite toute réponse autre que celles de ce §5 comme une panne : `passerelle_indisponible` pour la route du canal qui en dépend.

Le trafic Sésame → passerelle n'a pas de canal applicatif : JSON en clair sous TLS, `Content-Type: application/json`. Ce qu'il transporte (chartes et scellés chiffrés, identifiants opaques, statuts) n'est pas exploitable sans l'enclave.

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

`chartes` est vide quand aucun proche n'est « courrier » ; `404` sans motif pour un `uid` inconnu.

### 5.4 `GET /bundle?uid=<uid>&contactId=<id>`

Un proche par son identifiant : la lettre de secours vise un proche qui n'est pas « courrier », et le parcours « courrier » retrouve le proche par `invitations/{lookup}` (`uid`, `contactId`). Sésame ne garde aucune charte. **Réponse** : comme au §5.2 ; `404` sans motif sinon.

Le client ne peut que retenir une charte, jamais la modifier (elle est authentifiée sous la KTS) : un déni de service est visible, une substitution est impossible.

### 5.5 Notifications

`POST /<nom>`, corps JSON. La passerelle répond `200 {}` ; tout autre statut, ou une erreur réseau, fait réessayer Sésame avec un délai croissant pendant 24 h, puis la notification est journalisée comme perdue. Toute notification est idempotente côté client sur `X-Sesame-Request-Id`, que Sésame garde d'un essai à l'autre : une notification déjà traitée reçoit `200 {}` (§5.1). Les notifications ne portent **jamais** de code, de clé ni de résultat d'identité.

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
| `canal_invalide` | 400, en clair | enveloppe mal formée ou trop grande, corps indéchiffrable, `n` hors bornes ou non croissant, `kemCt` ou `ivPrefixe` changé, canal fermé, secret nul, session présentée depuis un autre canal (§1.3) |
| `instance_inconnue` | 410, en clair | `instanceId` inconnu (redémarrage) : le client reprend à `GET /attestation` |
| `attestation_indisponible` | 503, en clair | jeton d'attestation pas encore obtenu du lanceur |
| `message_refuse` | 400 | route inconnue, corps mal formé, champ manquant ou inconnu, valeur hors contrat, longueur fausse, chaîne hors NFC, corps trop grand, signature `cleVie` invalide |
| `jeton_invalide` | 401 | `idToken` ou jeton OIDC refusé |
| `code_refuse` | 403 | code d'accès refusé, quel que soit le motif (`details.essaisRestants`) |
| `otp_refuse` | 403 | dernier code faux (`details.essaisRestants`) |
| `role_insuffisant` | 403 | action réservée au contact de confiance |
| `preuve_requise` | 403 | ré-enrôlement sans preuve `cleVie` valide |
| `identite_non_confirmee` | 403 | identité non conforme à la charte ; message neutre |
| `bascule_impossible` | 403 | charte sans nom ou sans date de naissance ; message neutre |
| `deja_transmis` | 409 | annulation après `transmissibleLe` |
| `generation_non_croissante` | 409 | génération de KTS, de charte ou de fiche non croissante, ou génération après une suppression de proche |
| `defi_invalide` | 409 | défi inconnu, expiré, déjà utilisé, ou d'un autre canal ou d'un autre `uid` |
| `inactivite_trop_recente` | 409 | signalement d'inactivité avant le plancher |
| `grappe_recente` | 409 | dépôt dans une autre grappe moins de 30 jours après l'entrée dans la grappe inscrite |
| `pin_bloque` | 409 | plafond de 10 essais atteint |
| `pin_indisponible` | 409 | aucun enregistrement du PIN, compte inconnu, ligne absente, rejetée ou `supprime` |
| `identite_en_attente` | 409 | Stripe n'a pas encore rendu son verdict |
| `titulaire_indisponible` | 409 | ligne absente, `supprime` ou rejetée (hors routes du PIN) |
| `refusee` | 409 | VM hors des conditions d'entrée dans la grappe, ou VM jointe sans grappe |
| `amorcage_refuse` | 409 | amorçage d'une grappe neuve hors de ses conditions (§4.3) |
| `session_expiree` | 410 | session sans activité depuis 1 h, fermée ou inconnue |
| `identite_bloquee` | 423 | 3 échecs d'identité : 24 h (`details.reessayerLe`) |
| `trop_d_essais` | 429 | 5 essais de code ou de dernier code en 15 min, ou délai d'essai du PIN en cours (`details.reessayerLe`) |
| `trop_d_appels` | 429 | limite d'une route dépassée (`details.reessayerLe`) ; en clair pour `GET /attestation` |
| `pin_indisponible_temporaire` | 503 | pas de majorité, registre illisible, transaction refusée, mémoire pleine |
| `reessayer_plus_tard` | 503 | le contrôle anti-retour global refuse d'ouvrir, de transmettre ou de changer de grappe (`ARCHITECTURE.md`) ; aucun détail, aucun essai décompté |
| `registre_indisponible` | 503 | registre ou bucket des marqueurs illisible |
| `passerelle_indisponible` | 503 | la passerelle ne répond pas ou répond hors contrat |
| `fournisseur_indisponible` | 503 | Twilio, Stripe ou le prestataire d'impression en échec |
| `enveloppe_refusee` | — | par enveloppe, dans `refusees` : aad étranger, tag invalide, contenu non ouvrable |

Sauf les trois premiers et `trop_d_appels` sur `GET /attestation`, le statut est celui du corps chiffré (§1.3) ; la réponse HTTP est `200`.

## 7. Limites

Spécifiées : 5 essais / 15 min pour le code d'accès (par IP) et le dernier code (par proche) ; 3 échecs / 24 h pour l'identité ; défi à usage unique valable 5 min ; 10 essais par PIN, avec le délai croissant du §2.13 ; 1 h d'inactivité pour une session de portail ; `n` de 1 à 2³² − 1 par canal.

Décidée : `etat`, 100 / jour par `uid` (`ARCHITECTURE.md`).

Toutes les autres valeurs sont des valeurs initiales.

Décompte :

- une limite par `uid` ne compte que les appels authentifiés (signature, jeton ou preuve valides) ; un appel refusé à l'authentification ne compte que dans les limites par IP et par canal : qui connaît un `uid` ne peut pas épuiser les appels du titulaire ;
- les limites par `uid` sont décomptées par le leader de la grappe, en mémoire, sans réplication : la VM qui reçoit l'appel lui demande le décompte sur le canal entre VM, et un changement de leader les remet à zéro ; sans leader joignable (pause de la grappe), la VM décompte elle-même ;
- les limites par IP, par canal et par session sont décomptées par la VM qui reçoit l'appel.

| Route | Limite | Statut |
|---|---|---|
| `GET /attestation` | 60 / min par IP | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `defi` | 30 / h par canal, 120 / h par IP | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `enroler` | 5 / h par `uid`, 20 / h par IP | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `vie` | 10 / jour par `uid` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `etat` | 100 / jour par `uid` | décidée |
| `etat` | 60 / min par IP | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `charte/generation` | 30 / min par `uid` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `alerte/fiche` | 10 / jour par `uid` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `deces/signaler`, `titulaire/supprimer`, `lettre/secours` | 100 / min chacune, pour le compte de service appelant | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `deces/annuler` | 10 / jour par `uid` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `pin/deposer` | 10 / h par `uid` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `pin/evaluer`, au-delà du délai croissant | 20 appels / h par `uid`, 60 / h par IP | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/ouvrir` | 20 / 15 min par IP ; 3 sessions simultanées par `contactId` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/verifier`, au-delà des essais | 20 appels / 15 min par session | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/identite/demarrer`, `session/identite/verifier`, `session/identite/bascule` | 5 / h par session, chacune | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/ouvrir-cles` | 500 enveloppes par appel, 60 appels / h par session | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/fermer` | 10 / h par canal | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/refuser-role` | 5 / h par session | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `grappe/rejoindre` | 2 / min par `nodeId` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `grappe/amorcer` | 2 / min par IP | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `grappe/preparer`, `grappe/demarrer` | 2 / min par `nodeId`, chacune | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Réservation d'une configuration initiale (§4.3) | 2 min | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Corps déchiffré | 64 Kio ; 256 Kio pour `session/ouvrir-cles` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Enveloppe HTTP d'une requête du canal | 400 Kio | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Canal sans requête | fermé après 1 h | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Canaux fermés (§1.2) | filtre de Bloom de 16 Mio par instance | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Passerelle (appels de Sésame) | 300 / min par instance | valeur initiale, révisable avec les tests de charge du lot 1 bis |

Un dépassement renvoie `429 trop_d_appels` avec `details.reessayerLe`.
