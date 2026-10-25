<!-- Source : fatoom-app docs/TRD.md §16.3.2, §16.5.5, §16.8.4, §16.10 (« Clé de release », « Publier une version ») -->

# Publier une version de Sésame

> **État du document (2026-10-26) : en retard sur la spécification.** Ce texte décrit la conception du 2026-10-12. La spécification qui fait foi est `docs/TRD.md` §16 du dépôt `Fatoom-Heritage-Numerique/Fatoom` ; ce document est à réécrire avant toute publication du dépôt (TRD §16.10). Ont changé depuis, entre autres : plus de manifeste signé, de clé de release ni de délai de 7 jours avant une mise en service (liste `versions.json` compilée dans les clients, mise à jour forcée, TRD §16.5.5) ; matériel AMD SEV, pas SEV-SNP (Intel TDX en option, §16.3.1) ; un seul administrateur (§16.9 #2) ; canaux d'envoi croisés (§16.9 #13) ; plus d'accès élargi ; annulation explicite d'un signalement et transmission sur décès définitive après l'échéance (§16.5.3) ; signe de vie, transmission par inactivité (§16.5.10) ; protocole de PIN sur une grappe de trois VM à majorité (§16.3.9) ; canal hybride X25519 + ML-KEM-768 (§16.3.5) ; registre en ajout seul signé par `registre-sign`, borne publique (§16.3.6, §16.3.8) ; alertes envoyées par Sésame (§16.5.11) ; index du code d'accès étiré par Argon2id et calculé par le client (§16.4.4).

Une version de Sésame n'existe que si elle figure dans le manifeste, signé sur une clé matérielle et publié **7 jours avant** sa mise en service. Tant qu'une empreinte n'est pas `en_service` dans le manifeste, aucun client ne lui parle et KMS ne lui libère aucune clé. Il n'y a pas de voie rapide : un correctif attend 7 jours, les transmissions patientent. Une version se retire, elle, immédiatement.

## 1. Build reproductible

Le but : quiconque reconstruit la version `vX.Y.Z` obtient **la même empreinte d'image**, caractère pour caractère, que celle publiée. Ce qui le garantit :

- un binaire Go statique, construit avec `-trimpath`, sans CGO, horodatages figés par `SOURCE_DATE_EPOCH` (la date du commit du tag) ;
- des dépendances verrouillées par somme de contrôle (`go.sum`, `GOFLAGS=-mod=readonly`) ;
- une image de base `gcr.io/distroless/static-debian12` épinglée **par digest**, jamais par tag ;
- un conteneur de build lui-même épinglé par digest (image Go officielle), de sorte que la version de Go et de la chaîne de compilation ne dépendent pas de la machine ;
- un `Dockerfile` multi-étapes sans étape réseau autre que le téléchargement des modules vérifiés par `go.sum`.

### 1.1 Reconstruire

```bash
git clone https://github.com/Fatoom-Heritage-Numerique/sesame
cd sesame
git checkout vX.Y.Z
git tag -v vX.Y.Z                      # tag signé par la clé de release (§3)
./build/reproduire.sh vX.Y.Z           # construit, puis affiche sha256:<empreinte>
```

Le script n'a besoin que de `docker` (ou `podman`) et de `git`. Il refuse de construire une arborescence modifiée ou un tag non signé. Son journal complet est écrit dans `build/sortie/vX.Y.Z.log` ; le dépôt publie, pour chaque version mise en service, le journal de la construction officielle dans `manifeste/journaux/vX.Y.Z.log`, à comparer au vôtre.

### 1.2 Double construction en CI

La CI construit chaque tag **deux fois, sur deux exécuteurs distincts**, et échoue si les deux digests diffèrent. Une version dont la construction n'est pas reproductible n'entre pas au manifeste.

### 1.3 Comparer

```bash
./build/verifier-manifeste.sh manifeste.json vX.Y.Z sha256:<empreinte obtenue>
```

Vérifie les signatures du manifeste (§3) avec les clés publiques du dépôt, puis que la ligne `vX.Y.Z` porte exactement cette empreinte. Un écart se signale avec le gabarit d'issue « Rapport de vérification ».

## 2. Le manifeste : `manifeste.json`

Format exact, à la racine du dépôt et publié par le client sur sa page de transparence (pour Fatoom : `https://fatoom.fr/sesame/manifeste.json`) :

```json
{
  "v": 1,
  "versions": [
    {
      "version": "1.2.0",
      "imageDigest": "sha256:…",
      "imageReference": "<region>-docker.pkg.dev/<projet>/sesame/sesame@sha256:…",
      "envDigest": "sha256:…",
      "tag": "v1.2.0",
      "commit": "<sha git>",
      "publishedAt": "<ISO 8601 UTC>",
      "activeFrom": "<ISO 8601 UTC, ≥ publishedAt + 7 jours>",
      "status": "annoncee" | "en_service" | "retiree",
      "retiredAt": "<ISO 8601 UTC>" | null,
      "notes": "<français, une phrase>"
    }
  ],
  "requiredSignatures": 1,
  "signatures": [
    {"keyId": "release-1", "sig": "<base64, Ed25519 sur UTF-8(JSON canonique sans « signatures »)>"}
  ]
}
```

| Champ | Règle |
|---|---|
| `imageDigest` | l'empreinte attestée (`submods.container.image_digest`) et autorisée dans KMS |
| `imageReference` | la référence complète de l'image dans le dépôt d'images de l'opérateur ; les clients exigent qu'elle commence par le préfixe gravé chez eux |
| `envDigest` | `SHA-256` de l'ensemble des variables d'environnement non secrètes de la version (`ARCHITECTURE.md` §2.2), que les clients comparent à `submods.container.env` |
| `publishedAt`, `activeFrom` | `activeFrom ≥ publishedAt + 7 jours`, sans exception |
| `status` | `annoncee` à la publication ; `en_service` à `activeFrom`, une fois la liaison IAM posée ; `retiree` avec `retiredAt` |
| `requiredSignatures` | informatif : le nombre exigé est **gravé dans les clients**, pas lu dans le fichier ; `1` au départ, `2` dès qu'une seconde clé existe |
| `signatures` | une par clé de release ; `sig` est calculée sur le JSON canonique (clés triées, sans espace, UTF-8) de l'objet **sans** le champ `signatures` |

Le fichier est en ajout seul : une version ne disparaît jamais, elle passe `retiree`. L'historique complet reste lisible.

## 3. Signature sur clé matérielle

- La clé de release est une clé **Ed25519** générée sur une clé matérielle (YubiKey ou équivalent), détenue par le porteur du produit, hors de tout serveur : le secret ne quitte jamais la clé, signer exige sa présence physique.
- Elle signe deux choses : le **tag Git** de chaque version et le **manifeste**.
- **Une** signature est exigée au départ. Dès qu'une seconde personne de confiance existe, une seconde clé est ajoutée et le nombre de signatures exigées passe à 2 par une version des clients (le nombre est gravé chez eux). Rotation ou ajout : une version des clients (clé ajoutée, puis ancienne retirée une version plus tard).
- Les clés publiques sont publiées dans `manifeste/cles/<keyId>.pub` et sur la page de transparence du client, et embarquées dans l'app, le portail et la passerelle.
- **Jamais** de clé de release dans un fichier, un dépôt, une CI ou un service en ligne. La CI vérifie des signatures ; elle n'en produit aucune.

Procédure (sur le poste du signataire, clé matérielle branchée) :

```bash
./build/canoniser-manifeste.sh manifeste.json > manifeste.canonique.json   # JSON canonique sans « signatures »
./build/signer-manifeste.sh release-1 manifeste.canonique.json             # Ed25519 sur la clé matérielle → sig base64
# coller la signature dans manifeste.json, puis :
./build/verifier-manifeste.sh manifeste.json                              # toutes les signatures exigées valides
```

L'interface entre `signer-manifeste.sh` et la clé matérielle (OpenPGP card ou FIDO2) est fixée à l'étape 2 du plan de construction, avec les vecteurs à une et deux signatures.

## 4. Publier une version

1. **Figer** : tag `vX.Y.Z` signé sur le commit ; CI verte (double construction, vecteurs identiques à ceux du client, dépendances dans la liste, image sans shell).
2. **Annoncer** : ligne au manifeste en `annoncee` avec `imageDigest`, `imageReference`, `envDigest`, `publishedAt = now`, `activeFrom ≥ now + 7 j` ; signature sur clé matérielle ; publication dans le dépôt et sur la page de transparence du client, avec le journal de construction. À partir de là, la version est visible de tous ; aucun client ne lui parle encore, KMS ne la connaît pas.
3. **Attendre 7 jours.** Un problème découvert pendant ce délai : la version passe `retiree` sans avoir jamais servi ; une nouvelle version repart à l'étape 1 avec ses 7 jours.
4. **Mettre en service** (à `activeFrom`) : ajouter l'empreinte à la condition IAM de KMS et de Secret Manager (scripts `gcloud` du dépôt), passer la ligne `en_service`, signer, publier ; basculer le groupe d'instances sur la nouvelle image. Les anciennes versions encore `en_service` le restent jusqu'à leur retrait.
5. **Retirer** (à tout moment, immédiat) : ligne `retiree` avec `retiredAt`, signer, publier ; retirer l'empreinte de la condition IAM ; remplacer les instances. Un retrait n'attend rien : il ferme.

## 5. Cohérence KMS ⇔ manifeste

L'ensemble des empreintes autorisées dans la condition IAM de `kts-seal`, `session-sign`, `registre-mac` et des secrets doit être **identique** à l'ensemble des lignes `en_service` du manifeste (plus les lignes `annoncee` dont `activeFrom` est passé et dont la mise en service est en cours). Un contrôle CI du dépôt, exécuté à chaque publication et chaque jour, lit la condition avec `gcloud` et échoue si les deux ensembles diffèrent. Une empreinte autorisée dans KMS mais absente du manifeste est l'alerte la plus grave que ce dépôt peut lever : elle signifie qu'une image non publiée pourrait desceller des clés (`THREAT_MODEL.md` §2, limite 2).

## 6. Contrôles CI du dépôt

| Contrôle | Échoue si |
|---|---|
| Double construction | les deux digests diffèrent |
| Vecteurs | les vecteurs de test diffèrent de ceux du client |
| Manifeste | signatures invalides ou en nombre insuffisant ; `activeFrom < publishedAt + 7 j` ; version `en_service` sans journal de construction |
| KMS ⇔ manifeste | les ensembles d'empreintes diffèrent |
| Dépendances | une dépendance réseau hors liste (`ARCHITECTURE.md` §2.1) |
| Image | un shell, un port de débogage, une variable de lanceur factice lue hors du paquet de test |
| Budget | plus de 5 000 lignes de Go hors tests |
