<!-- Source : fatoom-app docs/TRD.md §16.3.3, §16.3.5, §16.4.5, §16.5, §16.7.1 -->

# API de Sésame

> **État du document (2026-10-26) : en retard sur la spécification.** Ce texte décrit la conception du 2026-10-12. La spécification qui fait foi est `docs/TRD.md` §16 du dépôt `Fatoom-Heritage-Numerique/Fatoom` ; ce document est à réécrire avant toute publication du dépôt (TRD §16.10). Ont changé depuis, entre autres : plus de manifeste signé, de clé de release ni de délai de 7 jours avant une mise en service (liste `versions.json` compilée dans les clients, mise à jour forcée, TRD §16.5.5) ; matériel AMD SEV, pas SEV-SNP (Intel TDX en option, §16.3.1) ; un seul administrateur (§16.9 #2) ; canaux d'envoi croisés (§16.9 #13) ; plus d'accès élargi ; annulation explicite d'un signalement et transmission sur décès définitive après l'échéance (§16.5.3) ; signe de vie, transmission par inactivité (§16.5.10) ; protocole de PIN sur une grappe de trois VM à majorité (§16.3.9) ; canal hybride X25519 + ML-KEM-768 (§16.3.5) ; registre en ajout seul signé par `registre-sign`, borne publique (§16.3.6, §16.3.8) ; alertes envoyées par Sésame (§16.5.11) ; index du code d'accès étiré par Argon2id et calculé par le client (§16.4.4).

Ce document est le contrat des routes que Sésame expose et des routes que le client doit exposer à Sésame (la passerelle). L'OpenAPI, les vecteurs de test et le double local `sesame-fake` (étape 2 du plan de construction) en dérivent ; en cas d'écart, ce document se corrige d'abord.

Les limites de débit du §6 sont des valeurs initiales, à ajuster avec les mesures de l'étape 2 ; tout le reste est fixé par la spécification de référence.

## 1. Conventions

### 1.1 Adresse et transport

- Sésame est joint à l'adresse publique de l'instance (pour Fatoom : `https://sesame.fatoom.fr`). TLS est terminé par le répartiteur de charge, hors de l'enclave : il ne protège rien à lui seul.
- Toute route, **sauf `GET /attestation`**, passe par le canal chiffré d'`ARCHITECTURE.md` §5 : le client vérifie d'abord l'attestation, puis chiffre chaque requête pour la clé de session de l'instance. Les corps et réponses décrits ci-dessous sont les JSON **en clair, avant chiffrement**. Sur le fil, chaque requête est `{instanceId, cliPub, n, body}` et chaque réponse `{n, body}`.
- Horodatages en ISO 8601 UTC (`2026-10-12T14:03:00Z`), octets en base64 standard, identifiants opaques en chaînes.
- Les requêtes sont en `POST` avec `Content-Type: application/json`, sauf `GET /attestation`. Les réponses `204` n'ont pas de corps (sous le canal : `body` chiffre un objet vide `{}`).

### 1.2 Erreurs

Toute erreur a la forme :

```json
{"erreur": "<code>", "message": "<phrase en français, non destinée à l'affichage>", "details": {…}}
```

`erreur` est un code stable en `snake_case` français (liste §5) ; `message` peut changer ; `details` est présent quand indiqué (`essaisRestants`, `reessayerLe`, …). Les clients décident de l'affichage d'après `erreur`, jamais d'après `message`. Une erreur du canal lui-même (`410`, `400 canal_invalide`) est renvoyée **en clair** : l'instance ne peut pas chiffrer pour un canal qu'elle ne reconnaît pas.

### 1.3 Authentification

| Appelant | Comment Sésame l'authentifie |
|---|---|
| App du titulaire (enrôlement) | jeton d'identité du titulaire émis par le fournisseur d'identité du client (pour Fatoom : `idToken` Firebase Auth), vérifié avec les clés publiques de l'émetteur configuré en variable attestée |
| App du titulaire (preuve de vie, génération) | message authentifié sous `KTS_auth` (`ARCHITECTURE.md` §8.5) : seul le coffre déverrouillé peut le produire |
| Portail d'un proche | code d'accès ou d'invitation, puis second facteur (SMS, email ou identité), puis `sessionId` sous le canal |
| Serveur du client | jeton OIDC d'un compte de service dont l'adresse est en variable attestée, `aud` = adresse publique de Sésame, placé dans le corps chiffré (jamais en en-tête : l'en-tête traverse le répartiteur en clair) |

## 2. Routes de Sésame

### 2.1 `GET /attestation`

Hors canal. Renvoie l'attestation courante de l'instance et sa clé de session.

**Réponse `200`** :

```json
{"jwt": "<JWT signé par Google>", "sessionPub": "<base64, 32 octets X25519>", "instanceId": "<id opaque>"}
```

Le client vérifie chaque claim d'`ARCHITECTURE.md` §3 et que `eat_nonce` contient `base64url(SHA-256(sessionPub))` ; sinon il n'envoie rien. Le jeton est renouvelé toutes les 10 min (validité 1 h). **Erreurs** : `503 attestation_indisponible` (lanceur injoignable : l'instance ne sert rien d'autre).

### 2.2 `POST /enroler`

Enrôle un coffre (première KTS) ou remplace sa KTS (rotation, `ktsGeneration + 1`). Appelé par l'app, coffre déverrouillé.

**Corps** :

```json
{"uid": "<uid>", "idToken": "<jeton d'identité du titulaire>", "ktsGeneration": 1, "kts": "<base64, 32 octets>"}
```

`kts` est la KTS en clair **à l'intérieur du corps chiffré** par le canal ; elle n'existe nulle part ailleurs en clair que dans la mémoire du téléphone et, le temps du scellement, dans celle de l'enclave.

**Traitement** : vérifie `idToken` (signature, `exp`, `sub == uid`) ; exige `ktsGeneration == 1` si le titulaire est absent du registre, sinon `ktsGeneration == registre.ktsGeneration + 1` ; scelle `sealedKts = KMS(kts-seal).encrypt(kts, aad = uid ‖ "|" ‖ ktsGeneration)` ; crée ou met à jour `titulaires/{uid}` (`ktsGeneration`, `enroledAt`, chartes conservées) ; efface `kts`.

**Réponse `200`** :

```json
{"sealedKts": "<base64>", "ktsGeneration": 1, "imageDigest": "sha256:…"}
```

L'app écrit `sealedKts`, `ktsGeneration`, `enroledAt`, `enroledWith = imageDigest` chez le client (`ARCHITECTURE.md` §8.6), puis les enveloppes et chartes rechiffrées en lots.

**Erreurs** : `401 jeton_invalide`, `409 generation_non_croissante` (`details.generationCourante`), `400 requete_invalide`.

### 2.3 `POST /vie`

Preuve de vie. Appelée par l'app à chaque déverrouillage, au plus une fois par jour, ou immédiatement si un signalement de décès est en cours.

**Corps** :

```json
{"uid": "<uid>", "iv": "<base64, 12 octets>", "vie": "<base64, AES-GCM(KTS_auth, iv, {\"v\":1,\"uid\",\"t\",\"nonce\"}, aad \"fatoom-sesame-vie|1|\" ‖ uid)>"}
```

**Traitement** : descelle KTS (KMS), dérive `KTS_auth`, ouvre `vie` ; refuse `|t − now| > 10 min` et un `nonce` déjà vu dans la journée ; écrit `vivantLe = now` ; si un signalement non annulé existe : `annuleLe = now`, `annulePar = preuve_de_vie` ; si une ouverture a déjà eu lieu après `transmissibleLe` (transmission réelle) : les ouvertures suivantes sont bloquées et le client est prévenu (`lifeAfterTransmission`, §3.3) ; efface KTS.

**Réponse `200`** (l'état courant, qui fait foi pour l'écran d'alerte de l'app) :

```json
{"vivantLe": "<ISO>", "ktsGeneration": 1,
 "signalement": null | {"signalementId": "<id>", "source": "insee", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "annuleLe": "<ISO>" | null},
 "transmis": false}
```

`transmis` vaut `true` quand au moins une ouverture a eu lieu après `transmissibleLe` : l'app affiche alors le cas « téléphone retrouvé », traité par le support.

**Erreurs** : `404 titulaire_inconnu`, `400 message_refuse` (tag GCM invalide : mauvaise KTS ou mauvais uid), `400 horodatage_invalide`, `409 nonce_rejoue`.

### 2.4 `POST /charte/generation`

Publie la génération courante de la charte d'un proche, **avant** que l'app n'écrive la nouvelle charte chez le client. `-1` = proche supprimé.

**Corps** :

```json
{"uid": "<uid>", "contactId": "<id>", "iv": "<base64>", "charteGen": "<base64, AES-GCM(KTS_auth, iv, {\"v\":1,\"uid\",\"contactId\",\"generation\"}, aad \"fatoom-sesame-gen|1|\" ‖ uid ‖ \"|\" ‖ contactId)>"}
```

**Traitement** : ouvre le message ; exige `generation > registre.chartes[contactId]` (absent ⇒ `0` ; un `contactId` absent n'a donc jamais besoin d'être publié pour la génération 1) ou `generation == -1` ; écrit `chartes.{contactId} = generation` ; toute session de portail ouverte pour ce proche est fermée.

**Réponse `200`** : `{"contactId": "<id>", "generation": 2}`.

**Erreurs** : `404 titulaire_inconnu`, `400 message_refuse`, `409 generation_non_croissante` (`details.generationCourante`).

### 2.5 `POST /session/ouvrir`

Première étape du portail : le proche saisit son code d'accès (ou son code d'invitation reçu par lettre).

**Corps** : `{"code": "<code saisi, tirets et espaces ignorés>"}`

**Traitement** : `lookup = hex(SHA-256("fatoom-lookup|" ‖ normalize(code)))` ; cherche d'abord `invitations/{lookup}` au registre (parcours « courrier », §2.7) ; sinon `GET bundle?lookup=` à la passerelle (§3.2) ; descelle KTS ; ouvre la charte ; vérifie `codeVerifier`, `charte.generation == registre.chartes[contactId]`, `ktsGeneration == registre.ktsGeneration`, `isUnlocked` (code remis) ; rôle `gardien` ⇒ ok ; rôle `proche` ⇒ exige un signalement `transmissibleLe ≤ now` non annulé ; envoie le dernier code par Twilio Verify à `charte.phone` (SMS) sinon `charte.email` ; crée la session (durée de vie 15 min jusqu'à vérification) ; efface KTS.

Tout refus de ce traitement renvoie le **même** code `code_refuse`, quel qu'en soit le motif (code inconnu, `codeVerifier` faux, charte révoquée, code non remis, proche avant J+7, preuve de vie reçue) : le portail n'a pas à distinguer, et un porteur de code n'apprend rien sur l'état du titulaire. Le motif est journalisé par Sésame (`refus par motif`).

**Réponse `200`** :

```json
{"sessionId": "<id>", "canal": "sms" | "email" | "identite", "masque": "+33 6 •• •• •• 47" | "l••@exemple.fr" | null}
```

`canal: "identite"` ⇒ pas de dernier code : le portail enchaîne sur §2.7.

**Erreurs** : `403 code_refuse` (`details.essaisRestants`), `429 trop_d_essais` (`details.reessayerLe`), `502 passerelle_indisponible`, `502 fournisseur_indisponible`.

### 2.6 `POST /session/verifier`

Le proche saisit le dernier code reçu par SMS ou email.

**Corps** : `{"sessionId": "<id>", "otp": "<code reçu>"}`

**Traitement** : Twilio Verify check ; 5 essais puis blocage 15 min ; succès ⇒ `ouvertures/{id}` au registre (`contactId`, `role`, `elargi`, `ouvertLe`), notification `openingNotified` au client (§3.3), émission du jeton de session (§4.1).

**Réponse `200`** :

```json
{"jeton": "<JWT ES256>", "scope": "destine" | "elargi", "role": "proche" | "gardien", "expireLe": "<ISO>", "prenom": "<prénom de la charte>", "titulaireAffiche": "<nom du titulaire>"}
```

`prenom` et `titulaireAffiche` servent à l'affichage du portail (« Bonjour Léa ») ; ils viennent de la charte, pas du client.

**Erreurs** : `404 session_inconnue`, `401 session_expiree`, `401 otp_refuse` (`details.essaisRestants`), `429 trop_d_essais` (`details.reessayerLe`), `409 parcours_invalide` (session en parcours « courrier »), `502 fournisseur_indisponible`.

### 2.7 `POST /session/identite/demarrer` et `POST /session/identite/verifier`

Parcours « courrier » : le second facteur est la vérification d'identité, menée par Sésame (`ARCHITECTURE.md` §9.8). Si la lettre a déjà donné lieu à une identité vérifiée (`identiteVerifieeLe` présent), `/session/ouvrir` renvoie directement `canal: "identite"` et `/session/identite/verifier` renvoie le jeton sans nouvelle vérification.

**`demarrer`, corps** : `{"sessionId": "<id>"}`. **Réponse `200`** : `{"clientSecret": "<client_secret Stripe Identity>"}`. Sésame crée la session Stripe (`type: document`, selfie exigé, aucune donnée du proche transmise).

**`verifier`, corps** : `{"sessionId": "<id>"}`. Sésame relit la session Stripe ; `verified` et nom, prénom, date de naissance conformes à la charte ⇒ `identiteVerifieeLe`, puis **même réponse que §2.6**. Sinon `essaisIdentite + 1`, blocage 24 h au troisième échec, `letterIdentityFailed` au client.

**Erreurs** : `404 session_inconnue`, `401 session_expiree`, `409 parcours_invalide` (session SMS/email, ou `verifier` sans `demarrer`), `403 identite_non_confirmee` (message neutre, `details.essaisRestants`), `429 identite_bloquee` (`details.reessayerLe`), `409 identite_en_attente` (Stripe n'a pas fini : réessayer), `502 fournisseur_indisponible`.

### 2.8 `POST /session/ouvrir-cles`

Ouvre les enveloppes du proche authentifié. Les enveloppes viennent du client (liste des contenus obtenue avec le jeton de session) ; Sésame ne les a jamais vues.

**Corps** :

```json
{"sessionId": "<id>",
 "enveloppes": [{"type": "fichier" | "motdepasse", "id": "<contentId>", "contactId": "<id>", "enveloppe": "<base64, 48 octets>", "iv": "<base64, 12 octets>"}]}
```

**Traitement** : session valide ; descelle KTS ; pour chaque enveloppe, reconstruit `aad_env` avec `uid`, `type`, `id`, `contactId`, `ktsGeneration` du registre et exige `contactId == session.contactId` (scope `destine`) ou `contactId` quelconque (scope `elargi`) ; ouvre sous `KTS_env` ; efface KTS après la réponse. Au plus **500 enveloppes par appel**, comme la borne du client pour destiner.

**Réponse `200`** (sous le canal : les clés ne sont lisibles que par le navigateur) :

```json
{"cles": [{"id": "<contentId>", "cle": "<base64, 32 octets>"}],
 "refusees": [{"id": "<contentId>", "erreur": "enveloppe_refusee"}],
 "jeton": "<JWT ES256 renouvelé>", "expireLe": "<ISO>"}
```

Une enveloppe refusée (AAD étranger, tag invalide) n'empêche pas les autres. Le jeton renouvelé matérialise « 1 h d'inactivité » : chaque appel réussi prolonge la session ; sans appel pendant 1 h, elle expire.

**Erreurs** : `404 session_inconnue`, `401 session_expiree`, `400 requete_invalide` (plus de 500 enveloppes, forme), `429 trop_d_appels`.

### 2.9 `POST /session/fermer`

**Corps** : `{"sessionId": "<id>"}`. Détruit la session côté Sésame ; le jeton n'est plus renouvelé (il reste formellement valide jusqu'à `exp`, au plus 1 h : le client ne liste alors que des objets chiffrés, et plus aucune clé ne s'ouvre). **Réponse** : `204`. Idempotent.

### 2.10 `POST /session/signaler-deces`

Réservé au contact de confiance authentifié (`role == gardien`).

**Corps** : `{"sessionId": "<id>"}`. **Traitement** : crée `signalements/{id}` (`source: gardien`, `signaleLe = now`, `transmissibleLe = now + 7 j`) ; s'il existe déjà un signalement non annulé, le renvoie sans en créer un second ; notifie `deathSignaled` au client (§3.3).

**Réponse `200`** : `{"signalementId": "<id>", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "recu": "<JWS, §4.2>"}`.

**Erreurs** : `404 session_inconnue`, `401 session_expiree`, `403 role_insuffisant`.

### 2.11 `POST /session/refuser-role`

Réservé au contact de confiance authentifié. **Corps** : `{"sessionId": "<id>"}`. **Traitement** : `chartes.{contactId} = -1`, session fermée, `roleRefused` au client. **Réponse** : `204`. **Erreurs** : `404 session_inconnue`, `401 session_expiree`, `403 role_insuffisant`.

### 2.12 `POST /deces/signaler` (serveur du client)

**Corps** :

```json
{"jeton": "<OIDC du compte de service du client>", "uid": "<uid>", "source": "insee" | "admin", "preuve": "<référence opaque conservée au registre, ≤ 1 Kio>"}
```

**Traitement** : vérifie le jeton OIDC (signature Google, `aud`, `exp`, `email` = compte de service configuré) ; crée le signalement avec **la date de Sésame** ; s'il existe déjà un signalement non annulé, le renvoie sans en créer un second.

**Réponse `200`** : `{"signalementId": "<id>", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "recu": "<JWS, §4.2>"}`.

**Erreurs** : `401 jeton_invalide`, `403 appelant_non_autorise`, `404 titulaire_inconnu`, `400 requete_invalide`.

### 2.13 `POST /titulaire/supprimer` (serveur du client)

**Corps** : `{"jeton": "<OIDC>", "uid": "<uid>"}`. Efface `titulaires/{uid}` et ses sous-collections, et les `invitations` du titulaire. **Réponse** : `204`. Idempotent. **Erreurs** : `401 jeton_invalide`, `403 appelant_non_autorise`.

## 3. Passerelle : routes que le client expose à Sésame

Le client expose une passerelle HTTPS que seule une enclave attestée peut appeler. Son adresse est une variable attestée de Sésame. Fatoom l'implémente par une Cloud Function ; les noms de routes ci-dessous sont ceux de son contrat.

### 3.1 Authentification des appels de Sésame

Chaque appel porte quatre en-têtes :

| En-tête | Contenu |
|---|---|
| `X-Sesame-Attestation` | le JWT d'attestation courant de l'instance |
| `X-Sesame-Session-Pub` | `sessionPub` de l'instance (base64), pour vérifier `eat_nonce` |
| `X-Sesame-Date` | horodatage ISO 8601 UTC de la requête |
| `X-Sesame-Request-Id` | UUID v4 unique par requête |
| `X-Sesame-Signature` | base64 d'une signature ECDSA P-256 / SHA-256 par la clé KMS `session-sign` sur `UTF-8(méthode ‖ "|" ‖ chemin-et-query ‖ "|" ‖ X-Sesame-Date ‖ "|" ‖ X-Sesame-Request-Id ‖ "|" ‖ hex(SHA-256(corps)))` (corps vide ⇒ SHA-256 de zéro octet) |

La passerelle : vérifie le jeton d'attestation avec **les mêmes règles qu'un client** (`ARCHITECTURE.md` §3, manifeste compris) et `eat_nonce` contre `X-Sesame-Session-Pub` ; vérifie la signature avec la clé publique `session-sign` épinglée ; refuse `|X-Sesame-Date − now| > 5 min` et un `X-Sesame-Request-Id` déjà vu dans les 10 min ; applique un rate limit par `instanceId` (claim du jeton). Échec ⇒ `401`, journal avec `imageDigest`.

Le trafic Sésame → passerelle n'a pas de canal applicatif : ce qu'il transporte (chartes et scellés chiffrés, identifiants opaques, statuts) n'est pas exploitable sans l'enclave, et il n'y a pas d'enclave de l'autre côté. TLS protège la confidentialité des métadonnées.

### 3.2 `GET bundle`

**`?lookup=<hex>`** : retrouve le proche à partir de l'index du code d'accès. **Réponse `200`** :

```json
{"uid": "<uid>", "contactId": "<id>", "charte": "<base64>", "charteIv": "<base64>", "charteGeneration": 1,
 "sealedKts": "<base64>", "ktsGeneration": 1, "isUnlocked": true, "titulaireAffiche": "<nom>"}
```

`404` si l'index est inconnu (Sésame le traduit en `code_refuse`).

**`?uid=<uid>&courrier=1`** : les chartes des proches à prévenir par lettre (`courrier == true`). **Réponse `200`** :

```json
{"uid": "<uid>", "sealedKts": "<base64>", "ktsGeneration": 1,
 "chartes": [{"contactId": "<id>", "charte": "<base64>", "charteIv": "<base64>", "charteGeneration": 1}]}
```

Le client ne peut que retenir une charte, jamais la modifier (elle est authentifiée sous KTS) : un déni de service est visible, une substitution est impossible.

### 3.3 Notifications (`POST`, corps JSON, réponse `204`)

| Route | Corps | Usage côté client |
|---|---|---|
| `openingNotified` | `{uid, contactId, role, elargi, ouvertLe, lieu, appareil}` (`lieu` approximatif et `appareil` déduits par Sésame de l'IP et de l'agent du navigateur, jamais conservés au registre) | email au titulaire à chaque ouverture par le contact de confiance |
| `deathSignaled` | `{uid, signalementId, source: "gardien", signaleLe, transmissibleLe, recu}` | alerte au titulaire, vérification de 7 jours |
| `roleRefused` | `{uid, contactId}` | email, retrait du statut de contact de confiance |
| `lifeAfterTransmission` | `{uid, signalementId, vivantLe}` | support : téléphone retrouvé après une transmission |
| `letterSent` | `{uid, contactId, pingenDocumentId, envoyeeLe, renvoi: false \| true}` | historique des envois (sans le code) |
| `letterStatus` | `{uid, contactId, pingenDocumentId, statut: "commandee" \| "imprimee" \| "en_distribution" \| "distribuee" \| "erreur", le}` | suivi de la lettre en 4 étapes |
| `letterFailed` | `{uid, contactId, essais}` | support : trois erreurs d'impression ou de distribution |
| `letterIdentityFailed` | `{uid, contactId, essaisIdentite, bloqueJusqua}` | support : vérification humaine |

Toute notification est idempotente côté client sur `X-Sesame-Request-Id` ; Sésame réessaie une notification en échec (`5xx`, réseau) avec un délai croissant pendant 24 h, puis la journalise comme perdue. Les notifications ne portent **jamais** de code, de clé, ni de résultat d'identité.

## 4. Jetons signés par Sésame

Les deux jetons sont signés par la clé KMS `session-sign` (ECDSA P-256, SHA-256) ; la clé publique est épinglée par le client.

### 4.1 Jeton de session (JWT, `alg: ES256`)

```json
{"iss": "sesame", "sub": "<contactId>", "uid": "<uid>", "sid": "<sessionId>",
 "scope": "destine" | "elargi", "role": "proche" | "gardien", "iat": 1760270580, "exp": 1760274180}
```

En-tête : `{"alg": "ES256", "typ": "JWT", "kid": "session-sign/<version KMS>"}`. `exp = iat + 3600`. Le client vérifie signature, `iss`, `exp`, et lit `uid`, `sub`, `scope` pour lister et filtrer ; il ne lit **rien d'autre** sur l'identité du proche. Scope `destine` : seules les enveloppes `sharedKeys[sub]` ; scope `elargi` : tout contenu dont `sharedWith` n'est pas vide, avec toutes ses enveloppes.

### 4.2 Reçu de signalement (JWS compact, `alg: ES256`)

```json
{"iss": "sesame", "typ": "signalement", "uid": "<uid>", "signalementId": "<id>",
 "source": "insee" | "gardien" | "admin", "signaleLe": "<ISO>", "transmissibleLe": "<ISO>", "iat": 1760270580}
```

Le client le conserve pour prévenir le titulaire ; il ne lui donne aucun droit : Sésame ne lit jamais ce reçu, il relit son registre.

## 5. Codes d'erreur

| Code | HTTP | Quand |
|---|---|---|
| `requete_invalide` | 400 | JSON mal formé, champ manquant, taille ou forme hors contrat |
| `canal_invalide` | 400 | corps indéchiffrable, `n` non strictement croissant, `cliPub` inconnue (en clair) |
| `instance_inconnue` | 410 | `instanceId` inconnu (redémarrage) : le client reprend à `GET /attestation` (en clair) |
| `attestation_indisponible` | 503 | lanceur injoignable |
| `jeton_invalide` | 401 | jeton d'identité ou OIDC refusé |
| `appelant_non_autorise` | 403 | compte de service OIDC non configuré |
| `titulaire_inconnu` | 404 | `uid` absent du registre |
| `generation_non_croissante` | 409 | `ktsGeneration` ou génération de charte ≤ courante (`details.generationCourante`) |
| `message_refuse` | 400 | preuve de vie ou génération dont le tag GCM est invalide |
| `horodatage_invalide` | 400 | preuve de vie avec `\|t − now\| > 10 min` |
| `nonce_rejoue` | 409 | nonce de preuve de vie déjà vu dans la journée |
| `code_refuse` | 403 | code inconnu, faux, révoqué, non remis, ou proche avant la transmission (`details.essaisRestants`) |
| `otp_refuse` | 401 | dernier code faux (`details.essaisRestants`) |
| `trop_d_essais` | 429 | 5 essais de code ou de dernier code en 15 min (`details.reessayerLe`) |
| `trop_d_appels` | 429 | rate limit d'une route (`details.reessayerLe`) |
| `session_inconnue` | 404 | `sessionId` inconnu ou fermé |
| `session_expiree` | 401 | session sans activité depuis 1 h, ou non vérifiée dans les 15 min |
| `parcours_invalide` | 409 | route du mauvais parcours (SMS/email vs courrier) ou dans le mauvais ordre |
| `role_insuffisant` | 403 | signalement ou refus de rôle par un proche qui n'est pas le contact de confiance |
| `identite_non_confirmee` | 403 | identité non conforme à la charte (`details.essaisRestants`) |
| `identite_bloquee` | 429 | troisième échec : 24 h (`details.reessayerLe`) |
| `identite_en_attente` | 409 | Stripe n'a pas encore rendu son verdict |
| `enveloppe_refusee` | — | par enveloppe, dans `refusees` : AAD étranger ou tag invalide |
| `passerelle_indisponible` | 502 | le client ne répond pas ou répond hors contrat |
| `fournisseur_indisponible` | 502 | Twilio, Stripe ou Pingen en échec |
| `service_indisponible` | 503 | KMS, Secret Manager ou registre injoignables |

## 6. Limites et rate limits

Spécifiées : **5 essais puis blocage 15 min** pour le code d'accès et le dernier code ; **3 échecs puis blocage 24 h** pour l'identité ; preuve de vie refusée hors `± 10 min` et nonce rejoué dans la journée ; session de portail expirée après **1 h** d'inactivité.

Valeurs initiales pour le reste, à ajuster avec les mesures de l'étape 2 :

| Route | Limite | Clé |
|---|---|---|
| `GET /attestation` | 60 / min | IP |
| `POST /enroler` | 5 / h | `uid` ; 20 / h par IP |
| `POST /vie` | 10 / jour | `uid` (le client n'en envoie qu'une par jour hors signalement) |
| `POST /charte/generation` | 30 / min | `uid` |
| `POST /session/ouvrir` | 5 / 15 min par `lookup` (spécifié) ; 20 / 15 min | IP |
| `POST /session/verifier` | 5 / 15 min (spécifié) | `sessionId` |
| `POST /session/identite/*` | 5 / h | `sessionId` |
| `POST /session/ouvrir-cles` | ≤ 500 enveloppes par appel ; 60 appels / h | `sessionId` |
| `POST /deces/signaler`, `POST /titulaire/supprimer` | 100 / min | compte de service |
| Passerelle (appels de Sésame) | 300 / min | `instanceId` |
| Taille d'un corps déchiffré | 64 Kio ; `ouvrir-cles` 256 Kio | — |
| Sessions de portail simultanées | 3 par `contactId` | — |

Un dépassement renvoie `429 trop_d_appels` avec `details.reessayerLe`.
