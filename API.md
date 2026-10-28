<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), « Sésame : contrat HTTP » -->

# API de Sésame

Ce document est le contrat des routes que Sésame expose et des routes que le client doit exposer à Sésame (la passerelle). Il publie le contrat HTTP de la spécification de l'opérateur (« Sésame : contrat HTTP », dépôt Fatoom), qui fait foi ; l'OpenAPI, les vecteurs de test et le double local `sesame-fake` en dérivent. En cas d'écart, la source se corrige d'abord.

Le corps (§1 à 7) reprend mot pour mot la spécification de référence, avec quelques termes du premier client, Fatoom : « lot 1 bis » réunit les étapes 1 et 2 de l'ordre de construction de ce dépôt (documentation, puis contrat exécutable : OpenAPI, vecteurs, `sesame-fake` ; `README.md`, « État du projet ») ; un identifiant `R-<DOMAINE>-<NN>` est une règle métier du client, `A-<NN>` une de ses règles d'architecture, `LIM-<n>` une limite assumée (`THREAT_MODEL.md` §2, limite n) ; un nom en `CamelCase` sans espace (`TransmissionDefinitive`, `DmsRetour`) est un écran de l'app du client.

Les limites de débit du §7 sont des valeurs initiales, révisables avec les tests de charge du lot 1 bis, sauf celles que le §7 dit spécifiées ou décidées. Tout le reste est fixé par la spécification de référence.

## 1. Conventions

### 1.1 Adresse et transport

- Sésame est joint à l'adresse publique de l'instance (pour Fatoom : `https://sesame.fatoom.fr`). TLS est terminé par le répartiteur de charge, hors de l'enclave : il ne protège rien à lui seul.
- Sésame tourne sur trois VM, une par zone. `GET /attestation` est servie sans préfixe, par n'importe quelle VM de la version servie, hors canal ; pendant le passage d'une version à la suivante, le répartiteur sert l'ancienne jusqu'au basculement des trois préfixes (`ARCHITECTURE.md`). Elle renvoie l'`instanceId` de la VM, dont le client déduit le préfixe de la VM (`a`, `b` ou `c`, §1.2) ; il n'y a pas de champ `zone`.
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
- Sésame ferme un canal resté 1 h sans requête (valeur initiale, révisable avec les tests de charge du lot 1 bis) : ses clés, ses défis et ses sessions de portail sont effacés, et chaque session effacée notifie `sessionTerminee` (§5.5). L'instance garde jusqu'à son arrêt la trace de chaque canal qu'elle a fermé, dans un filtre de Bloom de taille fixe (16 Mio, valeur initiale, révisable avec les tests de charge du lot 1 bis), et refuse toute requête d'un canal qu'il contient (`400 canal_invalide`) : elle ne chiffre plus jamais sous ses clés. Un faux positif refuse un canal neuf : le client en ouvre un autre.
- Un redémarrage de la VM ferme tous ses canaux : `410 instance_inconnue`. Ses sessions de portail sont perdues avec sa mémoire : leur `sessionTerminee` part d'après l'ouverture inscrite au registre (§5.5).

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
- `corps` n'existe que pour `200` ; `erreur` et `message` n'existent que pour une erreur ; `details` n'existe que quand le code l'indique : `essaisRestants` (entier ≥ 0), `reessayerLe` (horodatage), `reprise` (`true`, seulement avec `titulaire_indisponible` sur `vie` et `etat`, §2.4). Aucun autre champ.

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
| App du titulaire, `enroler` | `idToken` ; en plus, une preuve `cleVie` dès qu'une ligne ou un marqueur d'inscription existe pour cet `uid` (§2.3) |
| App du titulaire, `pin/deposer` | `idToken` **et** signature `cleVie` ; l'`idToken` n'est jamais suffisant seul |
| Portail d'un proche | code d'accès ou d'invitation, puis dernier code (SMS ou email) ou vérification d'identité, puis `sessionId`. Une session est **liée au canal** qui l'a ouverte (`instanceId`, `cliPub`) : présentée depuis un autre canal, elle est refusée (`canal_invalide`) |
| Serveur du client | jeton OIDC Google d'un compte de service (pour Fatoom : `<compte-deces>`), dans le champ `jetonFatoom` du corps chiffré, jamais en en-tête (l'en-tête traverse le répartiteur en clair). Accepté seulement si la signature Google est valide, `exp` non passé, `iss` = `https://accounts.google.com`, `aud` = adresse publique de Sésame en variable attestée (`SESAME_AUDIENCE` ; pour Fatoom : `https://sesame.fatoom.fr`), `email` = compte de service en variable attestée et `email_verified` vrai ; sinon `jeton_invalide` |
| Exploitant de Sésame (`grappe/membres`, `grappe/amorcer`) | jeton OIDC Google du compte d'exploitation de Sésame (`<compte-exploitation>`), dans le champ `jetonExploitation` du corps chiffré, vérifié comme `jetonFatoom` : signature Google valide, `exp` non passé, `iss` = `https://accounts.google.com`, `aud` = `SESAME_AUDIENCE`, `email` = compte d'exploitation en variable attestée et `email_verified` vrai ; sinon `jeton_invalide` |
| VM de Sésame | canal mutuellement attesté entre VM : jeton de paire vérifié par les contrôles entre VM (§3) |

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

La ligne « Limite » de chaque route donne sa valeur ; son statut (spécifiée, décidée, ou valeur initiale, révisable avec les tests de charge du lot 1 bis) est au §7.

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
| Limite | 30 / h par canal, 120 / h par IP ; aucune limite par `uid` : qui connaît un `uid` ne peut pas épuiser les défis du titulaire |

**Corps** : `{"uid": "<uid>"}`

**Traitement** : Sésame tire le défi, le lie au canal et à l'`uid` (§1.5) ; il ne lit pas le registre : un défi s'obtient pour tout `uid`.

**Réponse `200`** : `{"defi": "<base64, 32 octets>", "expireLe": "<ISO>", "grappeId": "<hex, 64 caractères>" | null}`

`grappeId` identifie la grappe du protocole de PIN en service : l'app en a besoin pour signer un dépôt (§2.12), y compris à la création du coffre, où aucune évaluation ne précède. Une VM sans grappe (`ARCHITECTURE.md`) répond `grappeId: null` : aucun dépôt n'est possible par elle.

**Erreurs** : seulement les erreurs communes.

### 2.3 `enroler`

| | |
|---|---|
| Appelant | app : création du coffre (avant toute écriture chez le client), reprise d'une création interrompue, rotation de la KTS, ré-enrôlement après la perte du registre |
| Authentification | `idToken` ; preuve `cleVie` exigée dès qu'une ligne `titulaires/{uid}` existe, et pour le rejeu d'un premier enrôlement |
| Limite | 5 / h par `uid`, 20 / h par IP |

**Corps** :

```json
{"uid": "<uid>", "idToken": "<jeton d'identité>", "ktsGeneration": 1, "kts": "<base64, 32 octets>",
 "cleViePub": "<base64, Ed25519, 32 octets>", "preuve": {"defi": "<base64>", "signature": "<base64>"},
 "cleViePubAttendue": "<base64, Ed25519, 32 octets>"}
```

`ktsGeneration` est un entier ≥ 1. `kts` est la KTS en clair **à l'intérieur du corps chiffré** par le canal. `cleViePubAttendue` n'existe que pour une ligne absente pendant une reprise publiée (ci-dessous). `preuve` est absente au premier enrôlement, et présente à son rejeu (ci-dessous). Sinon, c'est la signature par la `cleVie` courante de :

```
"fatoom-sesame-enroler|1|" ‖ uid ‖ JSON canonique {"v":1, "uid", "defi", "generation", "cleViePub"}
```

`generation` est la `ktsGeneration` demandée, `cleViePub` la clé de vie envoyée : une rotation ne peut pas substituer une autre clé de vie.

**Traitement** :

- **premier enrôlement** (ni ligne ni marqueur) : `idToken` seul ; le marqueur d'inscription, qui porte la `cleViePub` reçue, est créé avant la ligne ; `vivantLe = enroleLe` ;
- **génération supérieure** : rotation, preuve exigée ; elle ferme les sessions ouvertes des proches de ce titulaire, comme une nouvelle génération de charte (§2.6) : la VM qui reçoit ferme celles qu'elle tient, `session/ouvrir-cles` refuse sur toute VM une session dont la `ktsGeneration` a changé (§2.19), et chaque session fermée notifie `sessionTerminee` (§5.5) ;
- **génération égale** : rescellement seul (reprise d'une création interrompue) ; la preuve doit se vérifier avec la `cleViePub` inscrite, la `cleViePub` reçue doit lui être égale, ni les chartes ni l'enregistrement du PIN ne sont touchés ;
- **génération inférieure** : refusée.

Sésame scelle la KTS sous la clé KMS `kts-seal`, crée ou met à jour la ligne, puis efface `kts`. Une répétition à la même génération, avec une preuve sur un nouveau défi, est un rescellement : même effet.

**Rejeu d'un premier enrôlement** : l'app qui reprend une création interrompue (`ARCHITECTURE.md`) ne sait pas si son premier `/enroler` a abouti. Elle le rejoue à l'identique (même `uid`, `ktsGeneration`, `kts` et `cleViePub`), avec une `preuve` de la `cleVie` de son brouillon sur un nouveau défi. Le rejeu est idempotent sur l'`uid` et la `cleViePub` :

- ni ligne ni marqueur : premier enrôlement, la preuve se vérifiant avec la `cleViePub` reçue ;
- marqueur sans ligne (premier enrôlement interrompu entre le marqueur et la ligne) : la preuve se vérifie avec la `cleViePub` que porte le marqueur, qui doit être la `cleViePub` reçue ; la ligne est alors créée comme à un premier enrôlement ;
- ligne de même `cleViePub` : rescellement à génération égale (ci-dessus) ;
- `cleViePub` différente, ou preuve qui ne se vérifie pas : `preuve_requise` sur une ligne, `titulaire_indisponible` sur un marqueur sans ligne.

Sans preuve de la `cleVie` que porte le marqueur, une ligne absente n'est jamais recréée : le marqueur garde son rôle d'échec fermé (`ARCHITECTURE.md`).

**Ligne absente pendant une reprise** : pendant une reprise publiée, avec ou sans sauvegarde, c'est-à-dire tant que la version servie porte dans son code la constante `reprise` (`{epoque, empreinte}`, ou `{sansSauvegarde: true}` ; `ARCHITECTURE.md`), une ligne absente dont le marqueur d'inscription existe (aucune sauvegarde, ou coffre enrôlé après le dernier export) admet un enrôlement avec `preuve`, vérifiée contre `cleViePubAttendue`, champ ajouté au corps, que l'app lit dans `users/{uid}.sesame.cleViePub`. Hors d'une reprise publiée, ce champ est refusé par `reprise_absente` ; pendant une reprise, pour un `uid` qui a une ligne ou qui n'a pas de marqueur d'inscription, par `message_refuse`. La ligne recréée reçoit, comme à un premier enrôlement, `enroleLe` et `vivantLe` à la date de cet enrôlement ; une rotation ou un rescellement ne touche ni l'un ni l'autre. Fatoom pourrait y substituer une autre clé : limite assumée (LIM-54). Le titulaire est alerté par Fatoom du ré-enrôlement.

**Réponse `200`** : `{"sealedKts": "<base64>", "ktsGeneration": 1, "imageDigest": "sha256:<hex, 64 caractères>", "cleViePub": "<base64>", "enroleLe": "<ISO>"}`

`sealedKts` est le chiffré rendu par KMS, opaque pour le client. `enroleLe` est la date du premier enrôlement de la ligne, inchangée par une rotation ou un rescellement.

**Ordre des contrôles** : chaque situation a un seul code, celui du premier contrôle qui échoue.

1. Forme du corps : `message_refuse`.
2. `idToken` : `jeton_invalide`.
3. Lecture de la ligne et du marqueur d'inscription : registre ou bucket des marqueurs illisible ⇒ `registre_indisponible` ; ligne `supprime` ou rejetée ⇒ `titulaire_indisponible`.
4. `cleViePubAttendue`, s'il est présent : hors d'une reprise publiée ⇒ `reprise_absente` ; ligne présente, ou ni ligne ni marqueur ⇒ `message_refuse`.
5. Défi de la `preuve`, si elle est présente : `defi_invalide`.
6. Preuve :
   - ligne présente : preuve absente, ou qui ne se vérifie pas avec la `cleViePub` inscrite ⇒ `preuve_requise`, rejeu d'un premier enrôlement compris ;
   - ligne absente avec `cleViePubAttendue` pendant une reprise : preuve absente, ou qui ne se vérifie pas contre `cleViePubAttendue` ⇒ `preuve_requise` ;
   - marqueur sans ligne, hors de ce cas : preuve absente, qui ne se vérifie pas avec la `cleViePub` du marqueur, ou `cleViePub` reçue différente de celle du marqueur ⇒ `titulaire_indisponible` ;
   - ni ligne ni marqueur : une preuve présente doit se vérifier avec la `cleViePub` reçue, sinon `message_refuse`.
7. Génération, sur une ligne présente : inférieure ⇒ `generation_non_croissante` ; égale avec une `cleViePub` reçue différente de l'inscrite ⇒ `message_refuse`.

**Erreurs** : `jeton_invalide` (401) ; `preuve_requise` (403 ; contrôle 6, ligne présente ou reprise) ; `defi_invalide` (409) ; `generation_non_croissante` (409) ; `reprise_absente` (409 ; `cleViePubAttendue` hors d'une reprise publiée) ; `message_refuse` (400 ; contrôles 1, 4, 6 et 7) ; `titulaire_indisponible` (409 ; contrôles 3 et 6) ; `registre_indisponible` (503 ; registre ou bucket des marqueurs illisible : aucun premier enrôlement).

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

**Traitement** : écrit `vivantLe` ; annule un signalement d'**inactivité** en cours (`annulePar: preuve_de_vie`, notification `signalementAnnule`) ; après une transmission par inactivité, la première preuve de vie clôt le signalement (`closLe`, `closPar: retour`, `ARCHITECTURE.md`), bloque les ouvertures suivantes, ferme les sessions ouvertes des proches de cet `uid` (§2.19, notification `sessionTerminee`) et notifie `vieApresTransmission` ; une inactivité qui suit ouvre un nouveau signalement (§2.8). Une preuve de vie n'a **aucun effet** sur un signalement de décès, ni avant ni après son échéance. Sur une ligne rejetée par le contrôle de fraîcheur, la preuve est répondue mais pas inscrite : réponse `200` de la forme ci-dessous, lue dans la ligne rejetée telle qu'elle est (`vivantLe` inchangé), sans aucun effet.

**Réponse `200`** :

```json
{"vivantLe": "<ISO>", "ktsGeneration": 1,
 "signalement": null | {"signalementId": "<id>", "source": "insee" | "acte" | "inactivite", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "annuleLe": "<ISO>" | null, "closLe": "<ISO>" | null},
 "transmis": false, "pinPresent": true, "pinBloque": false,
 "ouvertures": [{"contactId": "<id>", "role": "proche" | "gardien", "ouvertLe": "<ISO>"}]}
```

`signalement` est le signalement courant : le dernier signalement `deces` non annulé s'il en existe un, sinon le dernier inscrit, annulé, clos ou non (§2.8). `ouvertures` : sessions de proches depuis la dernière preuve de vie. `transmis` vaut `true` dès que `transmissibleLe` d'un signalement non annulé est passé, qu'une ouverture ait eu lieu ou non ; pour un signalement d'inactivité, la réponse qui le clôt porte encore `transmis: true`, les suivantes `transmis: false`. L'app affiche alors `TransmissionDefinitive` (R-DECES-22) si la `source` du signalement n'est pas `inactivite`, et `DmsRetour` (R-VIE-05) sinon ; `signalement` et `ouvertures` disent de quoi il s'agit et qui a ouvert ce que le titulaire lui destinait, avec la date. Une ouverture ne porte jamais le détail des éléments ouverts : Sésame ne le garde pas (§2.19). `pinPresent: false` : l'enregistrement du PIN manque dans la grappe en service, l'app redépose. `pinBloque: true` : le plafond d'essais est atteint, seul un nouveau PIN en sort. Pendant une pause de la grappe (pas de majorité) ou sur une VM sans grappe, `pinPresent` et `pinBloque` sont le dernier état connu, lu dans la ligne du registre (`pinDeposeLe`, `pinBloqueLe`, `ARCHITECTURE.md`)  ; copies écrites par une autre grappe que celle de la VM qui répond, ou VM sans grappe : `pinPresent: true` et `pinBloque: false` (`ARCHITECTURE.md` §9) ; une pause ne donne jamais `pinPresent: false` à un titulaire qui a déposé.

**Erreurs** : `defi_invalide` (409) ; `message_refuse` (400 ; signature invalide) ; `titulaire_indisponible` (409 ; ligne absente, avec ou sans marqueur d'inscription, ou `supprime` : rien n'est écrit ; `details.reprise: true` quand la ligne manque, que le marqueur existe et qu'une reprise publiée est en cours : c'est le signal qui fait lancer `Migration` à l'app, `ARCHITECTURE.md`).

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

Le repère est tenu par l'app : `ouvertures` est l'`ouvertLe` de la dernière ouverture qu'elle a vue ; `evaluations` est la dernière valeur qu'elle a reçue du compteur des évaluations du PIN décomptées, tenu par la grappe (entier ≥ 0). L'app avance son repère après sa propre évaluation. Le compteur repart de zéro dans une nouvelle grappe : un `repere.evaluations` supérieur au compteur courant est traité comme `0`. L'app garde avec son repère le dernier `grappeId` reçu (`defi`, §2.2, ou `pin/deposer`, §2.12) ; quand une réponse lui en rend un autre, elle remet `repere.evaluations` à `0` avant son `etat` suivant : les évaluations d'une nouvelle grappe ne sont jamais retranchées d'un repère de l'ancienne (au pire, une alerte locale montre de nouveau des évaluations déjà vues).

**Réponse `200`** : `{"etat": "<JWS compact ES256>"}`, de charge utile :

```json
{"iss": "sesame", "typ": "etat", "uid": "<uid>", "iat": 1760270580,
 "signalement": null | {…}, "ouvertures": […], "pinPresent": true, "pinBloque": false,
 "evaluationsDepuis": 0, "repere": {"ouvertures": "<ISO>" | null, "evaluations": 0}}
```

`signalement` et les éléments d'`ouvertures` ont la forme de `vie` ; `ouvertures` ne contient que celles dont `ouvertLe` est strictement postérieur au repère. `evaluationsDepuis` = compteur courant − `repere.evaluations` ; non nul, l'app affiche une alerte locale. Pendant une pause de la grappe ou sur une VM sans grappe, `pinPresent`, `pinBloque` et le compteur sont le dernier état connu, lu dans la ligne du registre (`pinDeposeLe`, `pinBloqueLe`, `pinEvaluations`), sauf copies d'une autre grappe ou VM sans grappe : `pinPresent: true`, `pinBloque: false`, `evaluationsDepuis: 0` ; aucune évaluation n'a lieu pendant une pause. `repere` est le repère à jour, que l'app garde.

**Erreurs** : `jeton_invalide` (401) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée ; `details.reprise: true` comme pour `vie`).

### 2.6 `charte/generation`

| | |
|---|---|
| Appelant | app, **avant** toute écriture d'une charte chez le client : régénération du code, révocation, changement de rôle ou de coordonnées, suppression du proche, republication après un refus du rôle, nouveau code de chaque proche pendant une `Migration` (`ARCHITECTURE.md`) |
| Authentification | signature `cleVie`, sans défi : la génération strictement croissante refuse le rejeu |
| Limite | 30 / min par `uid` |

**Corps** : `{"uid": "<uid>", "contactId": "<id>", "generation": 2, "signature": "<base64>"}`, `generation` entier ≥ 1 ou `-1`, signature sur :

```
"fatoom-sesame-gen|1|" ‖ uid ‖ "|" ‖ contactId ‖ JSON canonique {"v":1, "uid", "contactId", "generation"}
```

**Traitement** : exige une `generation` supérieure à la génération inscrite (absente ⇒ `0`), ou `-1` ; écrit `chartes.{contactId}`. Toute nouvelle valeur ferme les sessions ouvertes de ce `contactId` : la VM qui reçoit ferme celles qu'elle tient, et `session/ouvrir-cles` refuse sur toute VM une session dont la génération a changé (§2.19) ; chaque session fermée notifie `sessionTerminee` (§5.5). `-1` est la **suppression** du proche, **définitive** : son code ne s'ouvre plus, et le registre n'accepte plus aucune génération pour ce `contactId` (`ARCHITECTURE.md`). Un refus du rôle ne touche pas à `chartes` (§2.21) : une nouvelle génération publiée ensuite (republication en rôle `proche`, ou réinvitation en rôle `gardien`) remplace la charte refusée.

La même requête renvoyée après son succès reçoit `generation_non_croissante` : le reçu déjà obtenu fait foi pour l'app.

**Réponse `200`** : `{"contactId": "<id>", "generation": 2, "recu": "<JWS, reçu de génération>"}`

**Erreurs** : `generation_non_croissante` (409 ; y compris toute génération après une suppression) ; `message_refuse` (400 ; signature invalide) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée).

### 2.7 `alerte/fiche`

| | |
|---|---|
| Appelant | app : fin de la création du coffre et fin d'une `Migration`, la fiche ayant disparu avec la ligne (non bloquant, retenté à chaque déverrouillage jusqu'au succès) ; changement du téléphone ou de l'email du titulaire |
| Authentification | signature `cleVie` |
| Limite | 10 / jour par `uid` |

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
{"jetonFatoom": "<OIDC>", "uid": "<uid>", "source": "insee" | "acte" | "inactivite", "dateDeces": "<AAAA-MM-JJ>",
 "preuve": "<hex, SHA-256 de la pièce>", "preuveNom": "<nom du fichier mensuel des décès>"}
```

`dateDeces` et `preuve` (64 caractères hexadécimaux) sont obligatoires pour `insee` et `acte`. Pour `acte`, `preuve` est le SHA-256 du fichier de l'acte ; pour `insee`, le SHA-256 de la ligne du fichier mensuel des décès qui désigne le titulaire (octets de la ligne, sans sa fin de ligne), et `preuveNom` est le nom de ce fichier tel que publié (pour Fatoom : le fichier de l'INSEE ; 1 à 128 caractères de `[A-Za-z0-9._-]`), obligatoire pour `insee` et absent sinon. Pour `inactivite`, le corps est `{jetonFatoom, uid, source: "inactivite"}`, sans `dateDeces` ni `preuve`.

**Traitement** : crée le signalement à **l'heure de Sésame** ; `transmissibleLe` = 23 h 59 min 59 s, heure de Paris, du 7ᵉ jour calendaire. Sésame inscrit `preuve` et `preuveNom` au registre (`empreintePreuve`, `preuveNom`, `ARCHITECTURE.md`) et les cite dans ses alertes au titulaire : une inscription fausse laisse une trace vérifiable, la pièce publiée ou gardée se comparant à l'empreinte. Un signalement non annulé et non clos **de même nature** déjà présent est renvoyé sans en créer un second : même `signalementId`, `signaleLe` et `transmissibleLe`. Un signalement d'inactivité clos par le retour du titulaire (§2.4) n'est jamais renvoyé : une nouvelle inactivité ouvre un nouveau signalement. Un signalement `deces` est accepté après une transmission par inactivité. Un signalement d'inactivité est refusé tant que `vivantLe` a moins de 3 mois et 30 jours, comptés en mois calendaires à l'heure de Paris (`ARCHITECTURE.md`) (`inactivite_trop_recente`), et tant qu'un signalement `deces` non annulé existe, avant ou après son échéance (`message_refuse`, motif journalisé) : le signalement courant reste le décès (§2.4). Sésame envoie lui-même ses alertes au titulaire, à l'inscription puis 3 et 6 jours après, par email et, si la fiche d'alerte porte un téléphone, par SMS (`ARCHITECTURE.md`).

**Réponse `200`** : `{"signalementId": "<id>", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "recu": "<JWS, reçu de signalement>"}`

**Erreurs** : `jeton_invalide` (401) ; `message_refuse` (400 ; source inconnue, `insee` ou `acte` sans `preuve`, `insee` sans `preuveNom`, `preuveNom` hors `insee`, `dateDeces` absente pour `insee` ou `acte`, `inactivite` pendant un signalement `deces` non annulé) ; `inactivite_trop_recente` (409) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée).

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

**Erreurs** : `deja_transmis` (409 ; `transmissibleLe` passé) ; `defi_invalide` (409) ; `message_refuse` (400 ; signature invalide, ou signalement autre que le signalement courant) ; `titulaire_indisponible` (409 ; ligne absente, avec ou sans marqueur d'inscription, ou `supprime` : rien n'est écrit).

### 2.10 `lettre/secours`

| | |
|---|---|
| Appelant | serveur du client, quand chacun des canaux d'un proche a échoué techniquement : son email **et** son téléphone, ou le seul des deux qu'il a |
| Authentification | `jetonFatoom` |
| Limite | 100 / min pour le compte de service appelant |

**Corps** : `{"jetonFatoom": "<OIDC>", "uid": "<uid>", "contactId": "<id>"}`

**Traitement** : Sésame lit la charte du proche à la passerelle (§5.4) et vérifie que la transmission est ouverte (signalement à terme, non annulé, non clos), que la charte est de la génération du registre avec la `ktsGeneration` de la ligne (mêmes contrôles qu'au §2.14, étape 2) et qu'elle a une adresse ; il tire alors un **nouveau** code d'invitation, le scelle sous sa clé KMS des codes d'invitation (`codeInvitationScelle`, `ARCHITECTURE.md`), inscrit `invitations/{lookup}` et `lettres/{contactId}`, puis commande la lettre ; le proche vérifiera son identité sur le portail. Le code n'est descellé que pour vérifier un code saisi sur le portail (§2.14) et pour réimprimer cette lettre (nouvel envoi après une erreur, renvois de J+15 et J+45, reprise d'un envoi interrompu), jamais rendu au client. Le client ne peut que déclencher une lettre vers l'adresse scellée par le titulaire, jamais la rediriger.

**Réponse `200`** : `{"commandee": true | false}`. `false` quand la transmission n'est pas ouverte, que la charte n'est pas de la génération du registre ou n'a pas d'adresse, ou qu'une lettre est déjà commandée pour cette transmission (une lettre d'une transmission précédente, close par le retour du titulaire, ne compte pas) ; le motif est journalisé par Sésame, jamais renvoyé : le client ne doit rien apprendre de la charte. Une répétition après une lettre commandée reçoit donc `false`.

**Erreurs** : `jeton_invalide` (401) ; `titulaire_indisponible` (409 ; ligne absente, `supprime` ou rejetée) ; `reessayer_plus_tard` (503 ; contrôle anti-retour du registre en échec) ; `passerelle_indisponible` (503) ; `fournisseur_indisponible` (503).

### 2.11 `titulaire/supprimer`

| | |
|---|---|
| Appelant | serveur du client : suppression du compte, suppression planifiée, suppression du coffre après une transmission |
| Authentification | `jetonFatoom` |
| Limite | 100 / min pour le compte de service appelant |

**Corps** : `{"jetonFatoom": "<OIDC>", "uid": "<uid>"}`

**Traitement** : passe la ligne à l'état `supprime` (pierre tombale) : plus d'enrôlement, d'ouverture ni de PIN pour cet `uid`, définitivement. Sans ligne, rien n'est écrit : sans marqueur d'inscription, l'`uid` n'a jamais été enrôlé ; avec un marqueur, la ligne manque et la route refuse (`ARCHITECTURE.md`).

**Réponse `200`** : `{"etat": "supprime"}`, ou `{"etat": "absent"}` pour un `uid` jamais enrôlé (ni ligne ni marqueur). Idempotente : un second appel répond pareil.

**Erreurs** : `jeton_invalide` (401) ; `titulaire_indisponible` (409 ; ligne rejetée, ou ligne absente alors qu'un marqueur d'inscription existe : le client réessaie après la reprise).

### 2.12 `pin/deposer`

| | |
|---|---|
| Appelant | app, coffre ouvert : création du coffre (après `enroler`, avant toute écriture chez le client), changement de PIN, nouveau PIN après une récupération, ouverture réussie sur un nouvel appareil, redépôt (enregistrement effacé ou nouvelle grappe) |
| Authentification | `idToken` **et** signature `cleVie` sur un défi |
| Limite | 10 / h par `uid` |

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

**Erreurs** : `jeton_invalide` (401) ; `message_refuse` (400 ; signature absente ou d'une autre clé, `ouverture` sans évaluation notée sur ce canal ou pour un autre élément) ; `defi_invalide` (409 ; y compris un `grappeId` signé qui n'est plus celui de la grappe en service, par exemple tiré avant un amorçage : l'app redemande un défi, qui porte le `grappeId` courant, et signe de nouveau) ; `pin_bloque` (409 ; plafond atteint, motif autre que `nouveau_pin`) ; `grappe_recente` (409) ; `pin_indisponible` (409 ; ligne absente, rejetée ou `supprime`) ; `pin_indisponible_temporaire` (503 ; pas de majorité, registre illisible, transaction refusée, mémoire pleine) ; `reessayer_plus_tard` (503 ; contrôle anti-retour en échec avant un dépôt dans une autre grappe, sans détail).

### 2.13 `pin/evaluer`

| | |
|---|---|
| Appelant | app : ouverture du coffre sur un nouvel appareil, ou sur un appareil connu après une déconnexion |
| Authentification | `idToken` seul |
| Limite | délai croissant ci-dessous ; au-delà, 20 appels / h par `uid` et 60 / h par IP |

**Corps** : `{"uid": "<uid>", "idToken": "<jeton d'identité>", "elementAveugle": "<base64, 32 octets>"}`

**Traitement** : le leader de la grappe lit la ligne du registre, puis **décompte un essai**, validé par la majorité, **avant** d'évaluer. Délai croissant entre essais : aucun pour les 3 premiers, puis 1 min, 5 min, 15 min, 1 h, 4 h, 12 h, 24 h. Le délai dépend du seul rang de l'essai depuis la dernière remise à 10 (premier dépôt, `nouveau_pin`, `ouverture`) et se compte depuis l'essai décompté précédent ; un `redepot` ne le change pas. Le 10ᵉ essai est décompté comme les autres, mais il n'est pas évalué : `essaisRestants` tombe à 0 et `cleOprf` est effacée dans la même entrée validée par la majorité, **avant** toute évaluation, et la réponse est `pin_bloque`, sans évaluation. L'app ne reçoit donc rien qui ouvre la copie serveur de la KMU : un 10ᵉ PIN, même juste, n'ouvre pas, et seul le code de récupération reste possible (R-PIN-02). Sésame alerte le titulaire à chaque essai décompté (au plus un message par heure). Sésame ne sait jamais si le PIN est juste : il n'existe pas de `pin_refuse`. Sésame note sur le canal la dernière évaluation, pour un dépôt `ouverture` (§2.12).

**Réponse `200`**, du 1ᵉʳ au 9ᵉ essai : `{"evaluation": "<base64, 32 octets>"}`, sans nombre d'essais restants.

**Erreurs** : `jeton_invalide` (401) ; `pin_indisponible` (409 ; aucun enregistrement, compte inconnu, ligne absente, rejetée ou `supprime` ; sans décompte, même réponse dans tous ces cas) ; `pin_indisponible_temporaire` (503 ; registre illisible, pas de majorité) ; `pin_bloque` (409 ; plafond déjà atteint, ou 10ᵉ essai, décompté sans évaluation) ; `trop_d_essais` (429 ; délai en cours, `details.reessayerLe`).

### 2.14 `session/ouvrir`

| | |
|---|---|
| Appelant | portail, saisie du code d'accès (ou du code d'invitation reçu par lettre) |
| Authentification | le code |
| Limite | 20 / 15 min par IP ; 5 codes faux dans une fenêtre de 15 min, par IP, et par proche quand le code désigne un proche (au registre), puis blocage de 15 min à compter du 5ᵉ ; 3 sessions simultanées par `contactId` ; 3 renvois du dernier code par session (§2.15) |

**Corps** : `{"code": "<code saisi>", "lookup": "<hex>"}`, ou `{"sessionId": "<id>"}` seul pour « Renvoyer le code » (§2.15). `code` est envoyé tel que saisi (§1.7). `lookup` est l'index du code (`indexAcces`, hexadécimal en minuscules), étiré par Argon2id et calculé par le navigateur ; Sésame ne calcule un Argon2id que pour l'index d'un code d'invitation, une fois par lettre (§2.10).

**Traitement** :

1. Sésame cherche `invitations/{lookup}` au registre ; trouvée : parcours « courrier ». Il exige que cette invitation soit celle de la dernière lettre du proche (`lettres/{contactId}.invitationLookup` = `lookup` : le code d'une transmission précédente, close par le retour du titulaire, n'ouvre plus, `ARCHITECTURE.md`), puis vérifie le code en le comparant, à temps constant, au code descellé de `lettres/{contactId}.codeInvitationScelle` (`ARCHITECTURE.md`) : le registre ne porte aucune empreinte du code d'invitation. Il lit la charte de ce proche à la passerelle (`GET /bundle?uid=&contactId=`, §5.4), descelle la KTS et ouvre la charte, vérifie comme à l'étape 2 `charte.generation == registre.chartes[contactId]` (absente du registre ⇒ seule la génération 1 est acceptée ; un proche supprimé, `-1`, ne s'ouvre plus) et `ktsGeneration`, puis exige, comme pour le rôle `proche` ci-dessous, un signalement de cet `uid` à terme, non annulé et non clos ; la vérification d'identité suit (§2.16). À la création de la session, Sésame inscrit `invitationUtiliseeLe` dans `lettres/{contactId}` s'il est vide (première session ouverte par ce code) : les renvois de J+15 et J+45 s'arrêtent (`ARCHITECTURE.md`).
2. Sinon : `GET /bundle?lookup=` à la passerelle (§5.2), descellement de la KTS, ouverture de la charte ; vérifie `codeVerifier`, `charte.generation == registre.chartes[contactId]` (absente du registre ⇒ seule la génération 1 est acceptée, `ARCHITECTURE.md`), `ktsGeneration`, `isUnlocked`. Une charte `courrier: true` (ni téléphone ni email) ne s'ouvre jamais par ce parcours, qui n'a aucun canal pour le dernier code : `code_refuse` (code non remis), son seul accès étant le code d'invitation de sa lettre (étape 1). Rôle `gardien` : accepté, sauf si le rôle a été refusé pour cette génération (`rolesRefuses`, §2.21) : la charte vaut alors rôle `proche`. Rôle `proche` : exige un signalement à terme, non annulé (inactivité : non clos par une preuve de vie reçue après le terme ; décès : définitif).
3. Pour le parcours de l'étape 2, envoi du dernier code : par SMS au téléphone de la charte, par Twilio Verify ; sans téléphone, par email à l'adresse de la charte, par Sésame lui-même, qui tire un code de 6 chiffres (primitive d'aléa), valable 10 minutes, et l'envoie par l'API Mail Send de SendGrid, avec un texte attesté et l'adresse d'expédition attestée de ses alertes (`ARCHITECTURE.md`).
4. Dans les deux parcours, création de la session, liée au canal. Son **état** reste en mémoire de la VM jusqu'à la fin de la session : la KTS déchiffrée et sa `ktsGeneration`, les champs d'identité de la charte (`prenom`, `nom`, `dateNaissance`), son rôle, sa génération vérifiée (§2.19) et, pour un dernier code par email, ce code. `session/identite/verifier`, `session/identite/bascule` et `session/ouvrir-cles` s'en servent, sans relire la charte. Une session dont l'état est perdu (redémarrage de sa VM) n'existe plus : `session_expiree`.

**Codes faux** : dans les deux parcours, tout code qui désigne un proche (invitation trouvée, ou charte servie par la passerelle) et que Sésame refuse (`code_refuse`) est un code faux de ce proche, quel que soit le motif : code différent du code scellé de sa lettre ou refusé par le `codeVerifier` de sa charte, ou code juste refusé pour l'état de l'accès (charte révoquée, proche avant l'échéance, signalement annulé ou clos, invitation expirée…) : Sésame lit d'abord son document d'essais (`titulaires/{uid}/essais/{contactId}`, `ARCHITECTURE.md`), refuse sans vérifier tant que dure le blocage posé par le 5ᵉ code faux d'une fenêtre de 15 min (`codeBloqueJusquau`, 15 min après ce 5ᵉ code faux, même si la fenêtre a pris fin avant ; `trop_d_essais`, `details.reessayerLe` = `codeBloqueJusquau`), et inscrit chaque code faux, signé, avant de répondre `code_refuse` ; le 5ᵉ, inscrit avec le blocage qu'il pose, reçoit encore `code_refuse`, avec `essaisRestants: 0`. Pour une IP, tout `code_refuse` est un code faux, que le code désigne un proche ou non, avec la même règle, compté en mémoire par la VM qui reçoit (§7) : une IP qui s'adresse aux trois VM a donc jusqu'à trois fois ces essais, la limite par proche restant au registre. `essaisRestants` baisse ainsi de la même façon pour un code juste refusé que pour un code faux : le refus ne dit pas si le code est valable.

**Réponse `200`** :

```json
{"sessionId": "<id>", "canal": "sms" | "email" | "identite" | "aucun", "masque": "+33 6 •• •• •• 47" | "l•••••@exemple.fr" | null}
```

- `sms`, `email` : le portail demande le dernier code (§2.15). Le masque d'un email garde la première lettre et le domaine entier.
- `identite` : parcours « courrier » ; le portail enchaîne sur §2.16 (`masque: null`).
- `aucun` : invitation dont l'identité est déjà vérifiée (`identiteVerifieeLe` de `lettres/{contactId}`) : le proche entre avec son code seul, sans nouvelle vérification ni dernier code (`masque: null`). La réponse porte en plus les champs de §2.15 (`jeton`, `scope`, `base`, `role`, `expireLe`, `prenom`, `titulaireAffiche`) ; l'ouverture est inscrite et notifiée comme au §2.15.

**Erreurs** : `code_refuse` (403), **identique quel que soit le motif** (code inconnu, code d'invitation ou `codeVerifier` faux, invitation expirée ou qui n'est plus celle de la dernière lettre du proche, charte révoquée, code non remis, proche avant l'échéance, signalement annulé, preuve de vie après une transmission par inactivité, `bundle` refusé par la passerelle, ligne ou document d'essais rejeté), avec `details.essaisRestants` (le plus petit des essais restants de l'IP et, quand le code désigne un proche, de ce proche) ; le motif est journalisé par Sésame. `trop_d_essais` (429 ; blocage de 15 min posé par le 5ᵉ code faux d'une fenêtre de 15 min, d'une IP ou pour un même proche, `details.reessayerLe` : fin du blocage) ; `registre_indisponible` (503 ; code faux d'un proche que le registre n'a pas inscrit : aucune réponse de code faux sans inscription) ; `reessayer_plus_tard` (503 ; contrôle anti-retour du registre en échec, sans détail ni raison, sans décompter les essais) ; `trop_d_appels` (429 ; y compris une 4ᵉ session simultanée pour le même proche) ; `passerelle_indisponible` (503) ; `fournisseur_indisponible` (503). Pour un renvoi du dernier code : erreurs de « Renvoyer le code » (§2.15).

### 2.15 `session/verifier`

| | |
|---|---|
| Appelant | portail, saisie du dernier code |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 5 derniers codes faux par proche dans une fenêtre de 15 min, puis blocage de 15 min à compter du 5ᵉ, comptés au registre ; 20 appels / 15 min par session |

**Corps** : `{"sessionId": "<id>", "otp": "<code reçu>"}`, `otp` en chiffres seulement, tel qu'il a été envoyé.

**Traitement** : Sésame lit d'abord le document d'essais du proche (`titulaires/{uid}/essais/{contactId}`, `ARCHITECTURE.md`) : pendant le blocage posé par le 5ᵉ dernier code faux d'une fenêtre de 15 min (`dernierCodeBloqueJusquau`, 15 min après ce 5ᵉ code faux), `trop_d_essais` (`details.reessayerLe` = `dernierCodeBloqueJusquau`), sans vérifier ; le 5ᵉ reçoit encore `otp_refuse`, avec `essaisRestants: 0`. Puis vérification par Twilio Verify pour un code envoyé par SMS ; pour un code envoyé par email, comparaison en temps constant avec le code de l'état de la session, refusé après ses 10 minutes (`otp_refuse`) ; chaque code faux est inscrit au registre, signé, avant la réponse `otp_refuse` ; un document d'essais rejeté fait refuser l'essai sans vérifier le code (`otp_refuse`) ; succès : code effacé de l'état, ouverture inscrite au registre, jeton de session émis, alerte de connexion au client (`ouvertureNotifiee`, §5.5). Pour le contact de confiance du vivant du titulaire, Sésame alerte aussi lui-même le titulaire, une fois par session.

**Réponse `200`** :

```json
{"jeton": "<JWT ES256>", "scope": "destine", "base": "vivant" | "deces" | "inactivite", "role": "proche" | "gardien", "expireLe": "<ISO>", "prenom": "<prénom de la charte>", "titulaireAffiche": "<nom du titulaire>"}
```

`scope`, `base` et `role` sont ceux du jeton (§1.6) ; `expireLe` est son `exp`. `base` vaut `vivant` pour le contact de confiance avant toute transmission, `deces` pour une transmission après décès, `inactivite` pour une transmission par inactivité ; elle reste `inactivite` jusqu'à la fin des accès quand un décès est vérifié ensuite (étape 7 de « Transmission après décès », `ARCHITECTURE.md`). `prenom` et `titulaireAffiche` sont lus dans la charte, pour l'affichage du portail (« Bonjour Léa »).

**Erreurs** : `otp_refuse` (403, `details.essaisRestants`) ; `trop_d_essais` (429, `details.reessayerLe` : fin du blocage) ; `registre_indisponible` (503 ; code faux que le registre n'a pas inscrit) ; `reessayer_plus_tard` (503 ; contrôle anti-retour, sans décompte) ; `session_expiree` (410) ; `canal_invalide` (400, en clair) ; `fournisseur_indisponible` (503).

**Renvoyer le code** (R-PORTAIL-01) : le portail rappelle `session/ouvrir` (§2.14) avec `{"sessionId": "<id>"}` seul, sur le canal qui a ouvert la session. Sésame renvoie le dernier code de cette même session, par le même canal (`sms` par Twilio Verify, `email` par SendGrid : le même code tant qu'il est valable, un nouveau ensuite), sans nouvelle session et sans décompter les sessions simultanées ni les essais du code. Réponse `200` : `{"sessionId", "canal", "masque"}`, comme au §2.14. 3 renvois par session ; au 4ᵉ, `trop_d_essais` (429, `details.reessayerLe` : expiration de la session), et le proche saisit le dernier code reçu ou rouvre une session par son code d'accès. Erreurs : `trop_d_essais` (429) ; `message_refuse` (400 ; session `identite` ou `aucun`, ou dont le dernier code est déjà vérifié) ; `session_expiree` (410) ; `canal_invalide` (400, en clair) ; `fournisseur_indisponible` (503).

### 2.16 `session/identite/demarrer`

| | |
|---|---|
| Appelant | portail : parcours « courrier », ou session basculée (§2.18) |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 5 / h par session |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : Sésame crée une session Stripe Identity (pièce d'identité et selfie exigés, aucune donnée du proche transmise à Stripe) avec sa propre clé restreinte.

**Réponse `200`** : `{"clientSecret": "<client_secret Stripe Identity>"}`

**Erreurs** : `identite_bloquee` (423 ; blocage de 24 h après 3 échecs cumulés depuis le dernier succès ou la fin du blocage, tenu par proche au registre d'une session à l'autre, `details.reessayerLe` = `bloqueJusquau`, `ARCHITECTURE.md`) ; `message_refuse` (400 ; session ni « courrier » ni basculée) ; `session_expiree` (410) ; `canal_invalide` (400, en clair) ; `fournisseur_indisponible` (503).

### 2.17 `session/identite/verifier`

| | |
|---|---|
| Appelant | portail, au retour du module Stripe |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 3 échecs par proche, cumulés depuis le dernier succès ou la fin du blocage, puis blocage de 24 h, pour la lettre comme pour la bascule ; 5 / h par session |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : Sésame relit la session Stripe. `verified` et prénom, nom et date de naissance conformes aux champs d'identité de la charte gardés dans l'état de la session (§2.14 ; comparaison tolérante) : `essaisIdentite` remis à zéro, `identiteVerifieeLe` inscrit dans `lettres/{contactId}` (parcours « courrier »), puis comme au §2.15 (ouverture inscrite, jeton, `ouvertureNotifiee`). Sinon : `essaisIdentite + 1` dans le document d'identité du proche (`titulaires/{uid}/identites/{contactId}`, `ARCHITECTURE.md`), `identiteEchouee` au client, blocage de 24 h au 3ᵉ échec (`bloqueJusquau`). Dès le verdict (`verified` ou `requires_input`), Sésame demande à Stripe la suppression de la pièce et du selfie (R-COMPTE-20 ; nouvel essai et échec : `ARCHITECTURE.md`).

**Réponse `200`** : comme au §2.15.

**Erreurs** : `identite_non_confirmee` (403 ; identité non conforme ou `requires_input` ; message neutre) ; `identite_bloquee` (423 ; 3ᵉ échec, `details.reessayerLe`) ; `identite_en_attente` (409 ; Stripe n'a pas fini : le portail réessaie) ; `reessayer_plus_tard` (503 ; contrôle anti-retour, sans décompte) ; `session_expiree` (410) ; `canal_invalide` (400, en clair) ; `fournisseur_indisponible` (503).

### 2.18 `session/identite/bascule`

| | |
|---|---|
| Appelant | portail, « Je n'ai plus ce numéro » ou « Je n'ai plus cet email », après `session/ouvrir`, une fois la transmission arrivée à terme |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 5 / h par session |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : remplace le dernier code par la vérification d'identité (§2.16, §2.17 : mêmes règles, même blocage, tenu dans le même document d'identité du proche, d'une session à l'autre). Exige que la charte, dans l'état de la session (§2.14), porte **le nom et la date de naissance** du proche, et que la session ait une transmission arrivée à terme, celle qui donne au jeton la base `deces` ou `inactivite` (§2.15) ; sinon la bascule est refusée par une réponse neutre : une session de base `vivant` (contact de confiance avant toute transmission) garde son dernier code. Personne d'autre que le titulaire, de son vivant, ne fixe les coordonnées d'un proche.

**Réponse `200`** : `{"canal": "identite"}` ; le portail enchaîne sur `session/identite/demarrer`.

**Erreurs** : `bascule_impossible` (403 ; charte sans nom ou sans date de naissance, ou session sans transmission arrivée à terme ; affiché « Nous n'avons pas pu confirmer votre identité ») ; `session_expiree` (410) ; `canal_invalide` (400, en clair).

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

**Traitement** : Sésame relit la ligne du titulaire. Si `chartes.{contactId}` n'est plus la génération vérifiée à l'ouverture de la session (§2.6), si la `ktsGeneration` de la ligne n'est plus celle de la KTS de la session (rotation, §2.3), si le rôle `gardien` d'une session de base `vivant` a été refusé depuis son ouverture (§2.21), ou si un signalement d'inactivité de cet `uid` a été clos depuis l'ouverture (retour du titulaire, §2.4), la session est fermée (`sessionTerminee`) et la réponse est `session_expiree`. Sinon, pour chaque enveloppe, Sésame reconstruit l'aad (`uid`, `type`, `id`, proche authentifié, génération du registre) et l'ouvre sous la KTS de l'état de la session (§2.14) ; une enveloppe destinée à un autre proche est refusée ; le type `volontes` s'ouvre comme les autres types. La KTS reste dans l'état de la session jusqu'à sa fin, et s'efface avec lui (§2.20, §5.5). Sésame ne garde pas la liste des contenus ouverts : l'ouverture inscrite au registre ne dit que qui et quand (`ARCHITECTURE.md`).

**Réponse `200`** (sous le canal : les clés ne sont lisibles que par le navigateur) :

```json
{"cles": [{"type": "<type>", "id": "<contentId>", "cle": "<base64, 32 octets>"}],
 "refusees": [{"type": "<type>", "id": "<contentId>", "erreur": "enveloppe_refusee"}],
 "jeton": "<JWT ES256 renouvelé>", "expireLe": "<ISO>"}
```

`type` accompagne `id` : deux contenus de types différents peuvent avoir le même identifiant. Une enveloppe refusée n'empêche pas les autres. Le jeton renouvelé matérialise l'inactivité : chaque appel réussi prolonge la session ; sans appel pendant 1 h, elle expire. Un appel avec `enveloppes: []` est admis : il ne vérifie que la session et renouvelle le jeton. Le portail l'envoie avant un appel au client qui suit une action du proche (liste des contenus, URL d'un fichier) quand son jeton a plus de 50 minutes, et une fois quand le client vient de refuser son jeton, avant de refaire l'appel ; jamais sans action du proche : la session expire toujours après 1 h sans action. Un téléchargement commencé avec une URL signée valide va à son terme sans autre appel, l'URL n'étant vérifiée qu'au début de la requête.

**Erreurs** : `enveloppe_refusee` (par élément, dans `refusees`) ; `message_refuse` (400 ; plus de 500 enveloppes, forme invalide) ; `reessayer_plus_tard` (503 ; contrôle anti-retour) ; `session_expiree` (410 ; y compris génération de la charte changée, rotation de la KTS, rôle refusé ou retour du titulaire depuis l'ouverture, et état perdu au redémarrage de la VM) ; `canal_invalide` (400, en clair).

### 2.20 `session/fermer`

| | |
|---|---|
| Appelant | portail |
| Authentification | `sessionId`, sur le canal qui l'a ouverte |
| Limite | 10 / h par canal |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : détruit la session et son état (KTS comprise) côté Sésame et notifie `sessionTerminee` (§5.5), comme pour une session expirée après 1 h sans appel. Le jeton n'est plus renouvelé.

**Réponse `200`** : `{"fermee": true}`. Idempotente : une session déjà fermée ou expirée reçoit la même réponse.

**Erreurs** : `canal_invalide` (400, en clair).

### 2.21 `session/refuser-role`

| | |
|---|---|
| Appelant | portail, contact de confiance dans une session |
| Authentification | `sessionId`, sur le canal qui l'a ouverte, de rôle `gardien` |
| Limite | 5 / h par session |

**Corps** : `{"sessionId": "<id>"}`

**Traitement** : retire le rôle `gardien` sans invalider la charte. Sésame inscrit au registre le refus pour la génération courante de la charte (`rolesRefuses.{contactId}`, `ARCHITECTURE.md`) : cette charte vaut ensuite rôle `proche`, sans ouverture du vivant du titulaire, et son code ouvre à la transmission comme celui de tout proche (§2.14). Il notifie `roleRefuse`, puis ferme la session (`sessionTerminee`) ; la VM qui reçoit ferme aussi les autres sessions de ce proche qu'elle tient, et `session/ouvrir-cles` refuse sur toute VM une session de base `vivant` de ce proche ouverte avant le refus (§2.19). L'app met ensuite la fiche du proche à jour ; le titulaire peut le réinviter ou choisir un autre contact de confiance, par une nouvelle génération de la charte (§2.6), qui remplace la charte refusée.

**Réponse `200`** : `{"refuse": true}`

**Erreurs** : `role_insuffisant` (403 ; session d'un proche qui n'est pas contact de confiance) ; `session_expiree` (410) ; `canal_invalide` (400, en clair).

## 3. Jetons d'attestation

Les contrôles d'un jeton d'attestation par un client (émetteur, `image_digest` dans la liste compilée, version `en_service`, `hwmodel` admis, `eat_nonce`) sont décrits dans `ARCHITECTURE.md`. Ils s'appliquent à l'identique au client qui appelle Sésame et à la passerelle qui reçoit un appel de Sésame (§5.1).

**Entre VM de la grappe**, une VM vérifie le jeton d'un pair sans la liste des versions, qui n'intervient pas entre VM : une image ne peut pas contenir la liste qui porte sa propre empreinte. Le pair est accepté seulement si toutes les conditions suivantes sont vraies :

| Claim | Valeur exigée |
|---|---|
| signature, `iss`, `aud`, `exp` et `iat`, `swname`, `submods.confidential_space.support_attributes`, `dbgstat`, `secboot` | comme pour un client (`ARCHITECTURE.md`) |
| `submods.container.image_digest` | égal à celui du jeton de la VM qui vérifie : image identique |
| `submods.container.env_override` | égal, dictionnaire entier, à celui du jeton de la VM qui vérifie : mêmes variables attestées, donc même `SESAME_TEXTES` |
| `submods.gce.project_number`, `submods.container.image_reference` | égaux à ceux du jeton de la VM qui vérifie : leurs valeurs, propres à l'exploitant, ne sont pas dans l'image |
| `hwmodel` | une des valeurs de la constante `hwmodels` de l'image, égale aux `hwmodels` de la ligne de cette version dans `versions.json` et changée seulement par une nouvelle version |
| `eat_nonce` | exactement les trois valeurs du jeton de paire, ci-dessous |

**Jeton de paire.** À l'ouverture de chaque connexion entre deux VM (canal mutuellement attesté, §4), chaque VM tire un `noncePaire` de 32 octets (primitive d'aléa) et l'envoie à l'autre ; chacune obtient alors de son lanceur un jeton propre à cette paire et à cette connexion, dont `eat_nonce` vaut :

```
eat_nonce = [ base64url(noncePaire),         # tiré par la VM qui demande l'attestation de son pair, et renvoyé dans le jeton
              base64url(SHA-256(nodeId)),   # identifiant de nœud de la VM qui présente le jeton, 16 octets (§4.1)
              base64url(SHA-256(cle)) ]     # clé publique de son certificat TLS entre VM, SubjectPublicKeyInfo DER
```

- base64url sans remplissage ; l'ordre des trois valeurs est libre.
- `SHA-256(nodeId)` porte sur les 16 octets de l'identifiant, décodés de sa forme base64url, jamais sur la chaîne elle-même.
- La VM qui vérifie compare la première valeur au `noncePaire` qu'elle a tiré pour cette connexion, la deuxième à l'identifiant de nœud annoncé par le pair, la troisième à la clé du certificat TLS que le pair présente sur cette connexion. Un jeton recopié d'une autre paire ou d'une connexion précédente est refusé.
- L'échange des jetons (`POST /interne/attestation`) et les certificats TLS entre VM, dont la clé ne vaut que par ce jeton, sont décrits dans `ARCHITECTURE.md`.

## 4. Routes entre VM

Servies seulement sur le canal mutuellement attesté entre VM de Sésame, jamais aux clients, sauf `grappe/membres` (§4.2) et `grappe/amorcer` (§4.3), routes du canal du §1.2 commandées par l'exploitant. Chaque membre est joint à son adresse DNS interne zonale, sans passer par le répartiteur : sur ce canal, le statut HTTP est le vrai statut et le corps d'erreur est `{"erreur": "<code>", "message": "<phrase>"}`. Statut de chaque limite : §7. Les autres messages entre VM (transport Raft du consensus, relais vers le leader des routes du PIN et de `grappe/membres`, demande de décompte d'une limite par `uid`, par proche ou par compte de service) sont décrits, avec leur entrée, leur sortie et leurs erreurs, dans `ARCHITECTURE.md`.

### 4.1 `grappe/rejoindre`

| | |
|---|---|
| Appelant | une VM de Sésame qui démarre sans état |
| Authentification | attestation mutuelle : contrôles entre VM (§3) |
| Limite | 2 / min par `nodeId` |

**Corps** : `{"jwt": "<jeton d'attestation de la VM>", "nodeId": "<id>"}`. `nodeId` est l'identifiant de nœud neuf : 16 octets de la primitive d'aléa, en base64url sans remplissage. `jwt` est le jeton de paire que la VM a présenté à l'ouverture de cette connexion (§3), vérifié par la VM qui reçoit la demande : il porte `image_digest`, `hwmodel` et la zone, et lie l'identifiant de nœud neuf par `eat_nonce`.

**Traitement** : la VM est admise sans intervention si elle a le même `image_digest` et un `hwmodel` admis, si elle prend la zone d'un votant injoignable depuis au moins 2 minutes, ou la place inscrite pour son nom (`submods.gce.instance_name` de son jeton) par `grappe/membres` (§4.2), et si aucune autre modification de membres n'est en cours. Elle est ajoutée comme apprenante ; son journal rattrapé, l'ancien membre est retiré, puis elle est promue : jamais plus de trois votants. Sinon elle est refusée et la demande journalisée (`grappe_admission_refusee`, alerte à l'exploitant, `ARCHITECTURE.md`). Un ancien identifiant de nœud est refusé. Une VM sans grappe répond `refusee` et journalise de même `grappe_admission_refusee`.

**Réponse `200`** : `{"admise": true, "role": "apprenant"}`

**Erreurs** : `refusee` (409 ; hors de ces conditions, ou VM jointe sans grappe) ; `pin_indisponible_temporaire` (503 ; pas de majorité) ; `message_refuse` (400) ; `trop_d_appels` (429).

### 4.2 `grappe/membres`

| | |
|---|---|
| Appelant | l'exploitant : déplacement de la place d'une VM pendant l'indisponibilité longue d'une zone, passage à une autre option matérielle (`ARCHITECTURE.md`) |
| Authentification | `jetonExploitation` (§1.4), en plus du canal (§1.2) ; la route ne donne ni état, ni essai, ni clé |
| Limite | 2 / h pour la grappe, décomptée par le leader |

Route du canal du §1.2 : `POST /n/{a|b|c}/canal`, `{"route": "grappe/membres", "corps": {…}}`, vers une VM de la grappe ; statuts dans le corps chiffré (§1.3).

**Corps** :

```json
{"jetonExploitation": "<OIDC>", "grappeId": "<hex, 64 caractères>",
 "membres": [{"vm": "<nom de la VM>", "zone": "<zone>"}, …]}
```

`membres` est la nouvelle liste : exactement trois éléments, un par préfixe de routage (`vm` se termine par `-a`, `-b` ou `-c`, §1.2), triés par `vm` ; `zone` est une zone de la région de Sésame. Elle ne diffère de la liste en service que par la zone d'un seul membre.

**Traitement** : la VM qui reçoit transmet la commande au leader sur le canal entre VM. Le leader vérifie que `grappeId` est celui de la grappe en service, que la liste ne change la zone que d'un membre, que ce membre est injoignable depuis au moins 2 minutes et qu'aucune autre modification de membres n'est en cours ; il inscrit la nouvelle liste dans l'état répliqué, validée par la majorité. La VM de cette place est donc arrêtée, ou injoignable, avant la commande, et l'exploitant ne crée sa remplaçante, une place à la fois, qu'une fois la liste inscrite. Une VM de ce nom, dans la zone inscrite, qui demande ensuite à entrer (`grappe/rejoindre`, §4.1 : même `image_digest`, `hwmodel` admis) est ajoutée comme apprenante ; son journal rattrapé, l'ancien membre est retiré, puis elle est promue, chaque étape validée par la majorité, une modification à la fois. La commande est journalisée (`grappe_membres_commandee`, avec l'`email` du jeton) ; le nombre de votants se lit dans la borne (`grappe`, `ARCHITECTURE.md`). La place revient à sa zone d'origine par une nouvelle commande, une fois la VM déplacée arrêtée.

**Réponse `200`**, une fois la liste validée par la majorité : `{"grappeId": "<hex, 64 caractères>", "inscrite": true}`

**Erreurs** : `jeton_invalide` (401) ; `membres_refuses` (409 ; autre `grappeId`, liste qui ne change la zone d'aucun membre ou de plus d'un, membre remplacé encore joignable, autre modification de membres en cours, VM sans grappe) ; `pin_indisponible_temporaire` (503 ; pas de majorité) ; `message_refuse` (400) ; `trop_d_appels` (429).

### 4.3 Amorçage d'une grappe neuve

Commandé par l'exploitant après le remplacement des trois VM (nouvelle version, effacement total) ; procédure et raisons : `ARCHITECTURE.md`. Conçu au lot 1 bis avec le prototype Raft, écrit ici avant d'être servi.

**Configuration initiale**, identique pour les trois VM, en JSON canonique, `membres` triés par `zone`, exactement trois éléments :

```json
{"v": 1, "imageDigest": "sha256:<hex, 64 caractères>",
 "membres": [{"nodeId": "<id>", "zone": "<zone>", "cle": "<base64, SubjectPublicKeyInfo DER de la clé du certificat TLS entre VM>"}, …]}
```

`nodeId` est l'identifiant de nœud neuf du membre (§4.1), `zone` sa zone (`submods.gce.zone` de son jeton), `cle` la clé publique de son certificat TLS entre VM, liée à son jeton de paire par `eat_nonce` (§3). `grappeId` = hex(SHA-256(ce JSON canonique)).

#### `grappe/amorcer`

| | |
|---|---|
| Appelant | l'exploitant, après avoir remplacé les trois VM, une par zone |
| Authentification | `jetonExploitation` (§1.4), en plus du canal (§1.2) ; la route n'agit que sur trois VM sans grappe et ne donne ni état, ni essai, ni clé |
| Limite | 2 / min par IP |

Route du canal du §1.2 : `POST /n/{a|b|c}/canal`, `{"route": "grappe/amorcer", "corps": {…}}`, vers l'une des trois VM ; statuts dans le corps chiffré (§1.3).

**Corps** : `{"jetonExploitation": "<OIDC>"}`

**Traitement** : servie seulement par une VM qui n'est dans aucune grappe et n'a réservé aucune configuration. Elle joint les deux autres VM à leur adresse DNS interne zonale sur le canal entre VM et vérifie leur jeton de paire (contrôles entre VM du §3 : même `image_digest`, `hwmodel` admis) ; elle exige exactement trois VM, une par zone, aucune dans une grappe. Elle compose la configuration initiale, la fait réserver par les deux autres (`grappe/preparer`), puis ordonne leur démarrage (`grappe/demarrer`) et démarre la dernière. Les trois démarrent Raft en état `new`, les trois membres votants, avec la même configuration initiale. La grappe neuve est publiée dans la borne de chaque VM (`grappe`, `ARCHITECTURE.md`) ; l'amorçage est journalisé (`grappe_amorcee`, avec l'`email` du jeton).

**Réponse `200`**, une fois une première entrée validée par la majorité de la grappe neuve : `{"grappeId": "<hex, 64 caractères>", "votants": 3}`

**Erreurs** : `jeton_invalide` (401) ; `amorcage_refuse` (409 ; VM déjà dans une grappe ou configuration réservée, autre VM injoignable, déjà dans une grappe ou d'une autre empreinte, attestation refusée, deux VM dans une même zone, réservation refusée) ; `pin_indisponible_temporaire` (503 ; grappe démarrée sans majorité dans le délai de réservation : elle reste en pause) ; `trop_d_appels` (429).

#### `grappe/preparer`

| | |
|---|---|
| Appelant | la VM qui a reçu `grappe/amorcer`, sur le canal entre VM |
| Authentification | attestation mutuelle : contrôles entre VM (§3) |
| Limite | 2 / min par `nodeId` |

**Corps** : `{"configuration": {…}}` (configuration initiale ci-dessus)

**Traitement** : la VM, qui n'est dans aucune grappe et n'a rien réservé, vérifie sur ses propres canaux vers chaque membre que la configuration décrit les VM qu'elle voit (`nodeId`, `zone`, `cle`, `imageDigest` égal au sien), puis la réserve pendant 2 minutes (valeur initiale, révisable avec les tests de charge du lot 1 bis) : elle refuse alors toute autre configuration. Sans `grappe/demarrer` dans ce délai, elle l'oublie et redevient sans grappe.

**Réponse `200`** : `{"reservee": true, "grappeId": "<hex, 64 caractères>"}`

**Erreurs** : `amorcage_refuse` (409 ; VM dans une grappe, réservation en cours, configuration qui ne décrit pas les VM vues) ; `message_refuse` (400) ; `trop_d_appels` (429).

#### `grappe/demarrer`

| | |
|---|---|
| Appelant | la VM qui a fait réserver la configuration, sur le canal entre VM |
| Authentification | attestation mutuelle : contrôles entre VM (§3) |
| Limite | 2 / min par `nodeId` |

**Corps** : `{"grappeId": "<hex, 64 caractères>"}`

**Traitement** : accepté seulement de la VM qui a fait réserver, pour la configuration réservée, dans le délai ; la VM démarre Raft en état `new` avec les trois membres votants de la configuration.

**Réponse `200`** : `{"demarree": true}`

**Erreurs** : `amorcage_refuse` (409 ; aucune réservation, autre `grappeId` ou autre VM, délai passé) ; `message_refuse` (400) ; `trop_d_appels` (429).

## 5. Passerelle : routes que le client expose à Sésame

Le client expose une passerelle HTTPS que seule une enclave attestée peut appeler. Son adresse est une variable attestée de Sésame. Fatoom l'implémente par une Cloud Function (`sesameGateway`).

### 5.1 Règles communes

Chaque appel porte six en-têtes :

| En-tête | Contenu |
|---|---|
| `X-Sesame-Attestation` | le JWT d'attestation courant de l'instance |
| `X-Sesame-Session-Pub` | `sessionPub` de l'instance (base64), pour vérifier `eat_nonce` |
| `X-Sesame-Kem-Pub` | `kemPub` de l'instance (base64, 1184 octets), pour vérifier `eat_nonce` : le jeton en porte les deux empreintes (§3) |
| `X-Sesame-Date` | horodatage ISO 8601 UTC de la requête (§1.7) |
| `X-Sesame-Request-Id` | UUID v4 unique par requête, en minuscules |
| `X-Sesame-Signature` | base64 de la signature ECDSA P-256 / SHA-256, au format DER que rend KMS, par la clé KMS `session-sign` sur `UTF-8(méthode ‖ "|" ‖ chemin-et-query ‖ "|" ‖ X-Sesame-Date ‖ "|" ‖ X-Sesame-Request-Id ‖ "|" ‖ hex(SHA-256(corps)))` (corps vide ⇒ SHA-256 de zéro octet) |

La passerelle :

- vérifie le jeton d'attestation avec **les mêmes règles qu'un client** (§3) et `eat_nonce` contre `X-Sesame-Session-Pub` et `X-Sesame-Kem-Pub` ;
- vérifie la signature contre chaque clé publique `session-sign` de `clesSession` de la liste des versions compilée (§1.6) : la requête ne porte pas de `kid`, et la signature est acceptée si l'une de ces clés la vérifie ; aucune ne la vérifie ⇒ `401` ;
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

Les chartes des proches à prévenir par lettre (`courrier == true`). La passerelle ne lit pas les chartes : elle renvoie celles des proches du titulaire dont l'email et le téléphone en clair sont vides et qui ont une adresse (`aAdresse`, `ARCHITECTURE.md`). Sésame ouvre chaque charte et ne poste une lettre que si elle porte `courrier: true` ; une autre charte est ignorée, le motif journalisé (`ARCHITECTURE.md`).

**Réponse `200`** :

```json
{"uid": "<uid>", "sealedKts": "<base64>", "ktsGeneration": 1, "titulaireAffiche": "<nom>",
 "chartes": [{"contactId": "<id>", "charte": "<base64>", "charteIv": "<base64>", "charteGeneration": 1}]}
```

`chartes` est vide quand aucun proche n'est « courrier » ; `404` sans motif pour un `uid` inconnu.

### 5.4 `GET /bundle?uid=<uid>&contactId=<id>`

Un proche par son identifiant : la lettre de secours vise un proche qui n'est pas « courrier » (un email, un téléphone ou les deux), et le parcours « courrier » retrouve le proche par `invitations/{lookup}` (`uid`, `contactId`). Sésame ne garde aucune charte durablement : une charte ouverte ne reste en mémoire que dans l'état d'une session (§2.14) ou le temps d'une lettre. **Réponse** : comme au §5.2 ; `404` sans motif sinon.

Le client ne peut que retenir une charte, jamais la modifier (elle est authentifiée sous la KTS) : un déni de service est visible, une substitution est impossible.

### 5.5 Notifications

`POST /<nom>`, corps JSON. La passerelle répond `200 {}` ; tout autre statut, ou une erreur réseau, fait réessayer Sésame avec un délai croissant pendant 24 h, puis la notification est journalisée comme perdue. Toute notification est idempotente côté client sur `X-Sesame-Request-Id`, que Sésame garde d'un essai à l'autre : une notification déjà traitée reçoit `200 {}` (§5.1). Les notifications ne portent **jamais** de code, de clé ni de donnée d'identité (pièce, selfie, nom, date de naissance) : `identiteEchouee` et `ouvertureNotifiee` disent seulement qu'une vérification a échoué ou qu'une session a été délivrée.

| Route | Quand | Corps |
|---|---|---|
| `ouvertureNotifiee` | **alerte de connexion**, une seule fois par session, à la délivrance du jeton (`session/verifier`, `session/identite/verifier`, ou `session/ouvrir` pour une identité déjà vérifiée), pour **tout** proche | `{uid, contactId, role, ouvertLe, navigateur}` : qui, quand et le navigateur, déduit par Sésame de l'agent du navigateur et jamais conservé au registre. Elle ne porte ni l'IP, ni les contenus ouverts |
| `sessionTerminee` | fin d'une session de proche, une seule fois par session, que son dernier code ou son identité ait été vérifié ou non : `session/fermer`, `session/refuser-role`, nouvelle génération de la charte (§2.6), rotation de la KTS (§2.3), retour du titulaire (§2.4), expiration après 1 h sans appel, fermeture du canal (§1.2) ; une session perdue au redémarrage de sa VM est notifiée par la tâche horaire de Sésame, d'après l'ouverture inscrite au registre (`ARCHITECTURE.md`), ce qui exclut une session jamais vérifiée, dont rien n'est inscrit | `{uid, contactId, ouvertLe?, fermeeLe}` ; `ouvertLe` absent pour une session jamais vérifiée. Elle ne porte pas les contenus ouverts, que Sésame ne garde pas (§2.19) ; l'app lit qui a ouvert et quand dans les `ouvertures` de `vie` et `etat` (§2.4, §2.5) |
| `signalementAnnule` | annulation explicite, ou preuve de vie reçue pendant un signalement d'inactivité | `{uid, signalementId, annulePar: "annulation" \| "preuve_de_vie", annuleLe}` |
| `roleRefuse` | refus du rôle par le contact de confiance | `{uid, contactId}` |
| `vieApresTransmission` | première preuve de vie après une transmission par **inactivité** | `{uid, signalementId, vivantLe}` |
| `lettreEnvoyee` | lettre commandée | `{uid, contactId, pingenDocumentId}`, sans le code |
| `statutLettre` | changement d'état chez le prestataire d'impression | `{uid, contactId, pingenDocumentId, statut: "commandee" \| "imprimee" \| "en_distribution" \| "distribuee" \| "erreur"}` |
| `lettreEchouee` | trois erreurs d'impression ou de distribution | `{uid, contactId}` |
| `identiteEchouee` | échec de la vérification d'identité d'un proche, prévenu par lettre ou après une bascule (§2.17) | `{uid, contactId, essaisIdentite}` |

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
| `pair_refuse` | 403 | entre VM : jeton de paire refusé par les contrôles entre VM (§3), requête avant la fin de l'échange des jetons, ou identifiant de nœud que l'attestation du pair ne lie pas ou qui a été retiré (`ARCHITECTURE.md`) |
| `preuve_requise` | 403 | ré-enrôlement sans preuve `cleVie` valide |
| `identite_non_confirmee` | 403 | identité non conforme à la charte ; message neutre |
| `bascule_impossible` | 403 | charte sans nom ou sans date de naissance, ou session de base `vivant` (aucune transmission arrivée à terme) ; message neutre |
| `deja_transmis` | 409 | annulation après `transmissibleLe` |
| `generation_non_croissante` | 409 | génération de KTS, de charte ou de fiche non croissante, ou génération après une suppression de proche |
| `reprise_absente` | 409 | `cleViePubAttendue` hors d'une reprise publiée (§2.3) |
| `defi_invalide` | 409 | défi inconnu, expiré, déjà utilisé, ou d'un autre canal ou d'un autre `uid` ; pour `pin/deposer`, `grappeId` qui n'est plus celui de la grappe en service |
| `inactivite_trop_recente` | 409 | signalement d'inactivité avant le plancher |
| `grappe_recente` | 409 | dépôt dans une autre grappe moins de 30 jours après l'entrée dans la grappe inscrite |
| `pin_bloque` | 409 | plafond de 10 essais atteint, y compris par le 10ᵉ essai, décompté sans évaluation |
| `pin_indisponible` | 409 | aucun enregistrement du PIN, compte inconnu, ligne absente, rejetée ou `supprime` |
| `identite_en_attente` | 409 | Stripe n'a pas encore rendu son verdict |
| `titulaire_indisponible` | 409 | ligne absente ou `supprime`, ou ligne rejetée selon la route (hors routes du PIN ; `vie` et `deces/annuler` acceptent une ligne rejetée) ; sur `vie` et `etat`, `details.reprise: true` quand la ligne manque pendant une reprise publiée (§2.4) |
| `refusee` | 409 | VM hors des conditions d'entrée dans la grappe, ou VM jointe sans grappe |
| `amorcage_refuse` | 409 | amorçage d'une grappe neuve hors de ses conditions (§4.3) |
| `membres_refuses` | 409 | modification de membres commandée hors de ses conditions (§4.2) |
| `pas_leader` | 409 | entre VM : la VM jointe n'est plus leader, le relais ou le décompte se refait au nouveau leader (`ARCHITECTURE.md`) |
| `session_expiree` | 410 | session sans activité depuis 1 h, fermée (dont génération de la charte changée, rotation de la KTS, rôle refusé ou retour du titulaire) ou inconnue (dont état perdu au redémarrage de sa VM) |
| `identite_bloquee` | 423 | 3 échecs d'identité cumulés depuis le dernier succès ou la fin du blocage : blocage de 24 h (`details.reessayerLe`) |
| `trop_d_essais` | 429 | blocage de 15 min posé par le 5ᵉ code faux d'une fenêtre de 15 min, code d'accès (par IP ou par proche) ou dernier code (par proche), 4ᵉ renvoi du dernier code dans une session, ou délai d'essai du PIN en cours (`details.reessayerLe`) |
| `trop_d_appels` | 429 | limite d'une route dépassée (`details.reessayerLe`) ; en clair pour `GET /attestation` |
| `pin_indisponible_temporaire` | 503 | pas de majorité, registre illisible, transaction refusée, mémoire pleine |
| `reessayer_plus_tard` | 503 | le contrôle anti-retour global refuse d'ouvrir, de transmettre ou de changer de grappe (`ARCHITECTURE.md`) ; aucun détail, aucun essai décompté |
| `registre_indisponible` | 503 | registre ou bucket des marqueurs illisible ; code faux d'un proche que le registre n'a pas inscrit |
| `passerelle_indisponible` | 503 | la passerelle ne répond pas ou répond hors contrat |
| `fournisseur_indisponible` | 503 | Twilio, SendGrid, Stripe ou le prestataire d'impression en échec |
| `enveloppe_refusee` | — | par enveloppe, dans `refusees` : aad étranger, tag invalide, contenu non ouvrable |

Sauf les trois premiers et `trop_d_appels` sur `GET /attestation`, le statut est celui du corps chiffré (§1.3) ; la réponse HTTP est `200`.

## 7. Limites

Spécifiées : 5 codes faux dans une fenêtre de 15 min, puis blocage de 15 min à compter du 5ᵉ, pour le code d'accès (par IP : contre qui change d'adresse IP, le frein est l'Argon2id du `lookup`, à calculer pour chaque code essayé, `ARCHITECTURE.md` ; et par proche quand le code désigne un proche) et le dernier code (par proche) ; 3 échecs d'identité cumulés depuis le dernier succès ou la fin du blocage, puis blocage de 24 h (par proche) ; défi à usage unique valable 5 min ; 10 essais par PIN, avec le délai croissant du §2.13 ; 1 h d'inactivité pour une session de portail ; `n` de 1 à 2³² − 1 par canal.

Décidée par le porteur du produit : `etat`, 100 / jour par `uid` (§2.5).

Toutes les autres valeurs sont des valeurs initiales.

Décompte :

- une limite par `uid` ne compte que les appels authentifiés (signature, jeton ou preuve valides) ; un appel refusé à l'authentification ne compte que dans les limites par IP et par canal : qui connaît un `uid` ne peut pas épuiser les appels du titulaire ;
- les essais faux des codes d'un proche (code d'accès ou d'invitation qui le désigne, dernier code, vérification d'identité) sont comptés **au registre**, par proche (`essais/{contactId}` et `identites/{contactId}`, `ARCHITECTURE.md`), par une écriture signée à chaque essai faux, avant la réponse, qui inscrit aussi la fin du blocage posé par le 5ᵉ code faux (`codeBloqueJusquau`, `dernierCodeBloqueJusquau`) : ni un redémarrage des VM ni un changement de leader ne les remettent à zéro ;
- les autres limites par `uid`, par proche (`contactId` : sessions simultanées de `session/ouvrir`) et pour le compte de service appelant (`deces/signaler`, `titulaire/supprimer`, `lettre/secours`) sont décomptées par le leader de la grappe pour les trois VM ensemble, en mémoire, sans réplication : la VM qui reçoit l'appel lui demande le décompte sur le canal entre VM, et lui signale la fin de chaque session pour le décompte des sessions simultanées ; le leader retient l'`instanceId` de la VM qui a demandé chaque session comptée, et sa tâche horaire retire du compte, vérifiées ou non, les sessions d'une instance dont le `vuLe` a plus de 2 minutes, comme pour `sessionTerminee` (`ARCHITECTURE.md`) : une VM perdue ne bloque pas un proche plus d'une heure ; un changement de leader les remet à zéro ; sans leader joignable (pause de la grappe, VM sans grappe), chaque VM décompte elle-même ; qui provoque des élections (l'administrateur, qui tient le réseau) peut donc dépasser ces limites, celle d'`etat` comprise ;
- les limites par IP, par canal et par session sont décomptées par la VM qui reçoit l'appel ;
- la limite de `grappe/membres` est décomptée par le leader.

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
| `session/ouvrir`, renvoi du dernier code (§2.15) | 3 renvois par session | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/verifier`, au-delà des essais | 20 appels / 15 min par session | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/identite/demarrer`, `session/identite/verifier`, `session/identite/bascule` | 5 / h par session, chacune | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/ouvrir-cles` | 500 enveloppes par appel, 60 appels / h par session | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/fermer` | 10 / h par canal | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `session/refuser-role` | 5 / h par session | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `grappe/rejoindre` | 2 / min par `nodeId` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `grappe/membres` | 2 / h pour la grappe | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `grappe/amorcer` | 2 / min par IP | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| `grappe/preparer`, `grappe/demarrer` | 2 / min par `nodeId`, chacune | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Réservation d'une configuration initiale (§4.3) | 2 min | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Corps déchiffré | 64 Kio ; 256 Kio pour `session/ouvrir-cles` | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Enveloppe HTTP d'une requête du canal | 400 Kio | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Canal sans requête | fermé après 1 h | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Canaux fermés (§1.2) | filtre de Bloom de 16 Mio par instance | valeur initiale, révisable avec les tests de charge du lot 1 bis |
| Passerelle (appels de Sésame) | 300 / min par instance | valeur initiale, révisable avec les tests de charge du lot 1 bis |

Un dépassement renvoie `429 trop_d_appels` avec `details.reessayerLe`, sauf les blocages de codes faux et le 4ᵉ renvoi du dernier code, qui renvoient `429 trop_d_essais` (§2.14, §2.15).
