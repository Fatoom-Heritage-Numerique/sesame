<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), « Sésame : l'enclave » (image reproductible et dépôt public) et « Versions et alertes de Sésame » (versions de Sésame) -->

# Publier une version de Sésame

Une version de Sésame n'est acceptée que si elle figure dans `versions.json`, à la racine de ce dépôt, avec le statut `en_service`, **et** dans une version du client qui l'a compilée. Les clients ne téléchargent aucune liste et ne vérifient aucune signature : la liste fait partie de leur propre code. Il n'y a pas de délai d'annonce. La revue des boutiques et l'historique public de ce dépôt en tiennent lieu (`THREAT_MODEL.md` §2, limite 12). Une version se retire immédiatement.

## 1. Build reproductible

Le but : quiconque reconstruit la version `vX.Y.Z` obtient **la même empreinte d'image**, caractère pour caractère, que celle publiée. Ce qui le garantit :

- un binaire Go statique, construit avec `-trimpath`, horodatages figés par `SOURCE_DATE_EPOCH` ;
- des dépendances verrouillées par somme de contrôle (`go.sum`) ;
- une image de base `gcr.io/distroless/static-debian12` épinglée **par digest**, jamais par tag ;
- un conteneur de build lui-même épinglé par digest, de sorte que la version de Go et de la chaîne de compilation ne dépendent pas de la machine ;
- un `Dockerfile` multi-étapes.

### 1.1 Reconstruire

```bash
git clone https://github.com/Fatoom-Heritage-Numerique/sesame
cd sesame
git checkout vX.Y.Z
git tag -v vX.Y.Z                      # tag signé par une clé SSH (§3)
./build/reproduire.sh vX.Y.Z           # construit, puis affiche sha256:<empreinte>
```

Le script n'a besoin que de `docker` (ou `podman`) et de `git`. Le dépôt publie, pour chaque version, le journal de la construction reproductible officielle, à comparer au vôtre. `build/reproduire.sh` entre dans le dépôt à l'étape du contrat exécutable ; le journal officiel, avec la première version (`README.md`, « État du projet », étapes 2 et 6).

### 1.2 Double construction en CI

La CI construit chaque tag **deux fois, sur deux exécuteurs distincts**, et échoue si les deux digests diffèrent. Une version dont la construction n'est pas reproductible n'entre pas dans `versions.json`.

### 1.3 Comparer

```bash
jq -r '.versions[] | select(.tag == "vX.Y.Z") | .imageDigest' versions.json
```

L'empreinte affichée doit être celle que vous avez obtenue. Un écart se signale avec le gabarit d'issue « Rapport de vérification ».

## 2. La liste des versions : `versions.json`

Format exact, à la racine du dépôt, sur la branche `main` protégée :

```json
{
  "v": 1,
  "versions": [
    {
      "version": "1.2.0",
      "imageDigest": "sha256:…",
      "envDigest": "sha256:…",
      "hwmodels": ["GCP_AMD_SEV", "GCP_INTEL_TDX"],
      "tag": "v1.2.0",
      "commit": "<sha git>",
      "publieLe": "<ISO 8601 UTC>",
      "statut": "en_service" | "retiree",
      "retireLe": "<ISO 8601 UTC>" | null,
      "notes": "<français, une phrase>"
    }
  ],
  "clesSession": {
    "session-sign/<version KMS>": {"cle": "<base64, ECDSA P-256 publique>", "depuis": "<ISO 8601 UTC>"}
  }
}
```

| Champ | Règle |
|---|---|
| `imageDigest` | l'empreinte attestée (`submods.container.image_digest`) et autorisée dans KMS ; seule désignation de l'image : le chemin du dépôt d'images, propre à l'opérateur, n'est pas dans ce fichier, c'est une constante compilée dans les clients et un argument des scripts publics (`<registre-images>`, `ARCHITECTURE.md` §3) |
| `envDigest` | `SHA-256` du JSON canonique de `submods.container.env_override`, c'est-à-dire des variables non secrètes propres au client (`ARCHITECTURE.md`, « Fournisseurs et réglages ») ; les clients le comparent au jeton |
| `clesSession` | clés publiques de `session-sign`, par `kid` : `cle` et `depuis` ; la clé publique `session-sign` est lue par `kid` dans `clesSession` de la liste des versions compilée, jamais téléchargée ni épinglée à part ; une rotation ajoute une entrée |
| `hwmodels` | les valeurs de `hwmodel` acceptées pour cette version, par les clients comme par la condition KMS (§6) |
| `statut` | `en_service` à l'ajout ; `retiree` avec `retireLe` au retrait |

`versions.json` ne change que par un commit sur `main` : son historique est public. Le client le recopie à la construction de chacune de ses versions (pour Fatoom : l'app, le portail et la passerelle). Une empreinte absente de la liste compilée ou `retiree`, ou un `hwmodel` hors de `hwmodels`, fait refuser l'attestation. La passerelle applique la même règle aux appels de Sésame.

Le client affiche aussi la liste et son historique sur sa page de transparence (pour Fatoom : <https://fatoom.fr/sesame>).

## 3. Tag signé et comptes de publication

- Chaque version porte un tag Git `vX.Y.Z`, signé par la clé SSH du compte GitHub de l'administrateur de l'opérateur. C'est la seule signature d'une version.
- L'ancre de confiance est l'ensemble des comptes qui publient : ce dépôt (GitHub) et, chez le client, ceux qui publient l'app (Apple, Google), le site et les fonctions. Tous se connectent par **passkey**, jamais avec un second facteur par SMS ; leurs codes de récupération sont rangés hors ligne.
- Un vol des seuls accès Google Cloud de Sésame ne change aucune liste : une image autorisée en douce dans KMS reste refusée par les clients et laisse sa trace dans les journaux d'administration publics (§7). Qui tient **à la fois** Google Cloud et l'un de ces comptes peut faire accepter une image malveillante par les clients de cette publication (`THREAT_MODEL.md` §2, limite 12).

## 4. Mettre en service une version : mise à jour forcée

1. **Figer** : tag `vX.Y.Z` signé ; CI verte (§8) ; ligne `en_service` ajoutée à `versions.json`.
2. **Publier les clients d'abord** : les versions de l'app, du site et des fonctions qui contiennent la nouvelle ligne sont publiées, et l'app est **disponible dans les deux boutiques**, avant toute mise en service. Jamais l'inverse.
3. **Autoriser** : l'empreinte est ajoutée à la condition du fournisseur du pool d'identités (`ARCHITECTURE.md`, « Libération des clés »). La liste d'abord, l'IAM ensuite.
4. **Démarrer et basculer** : la nouvelle version démarre sur trois VM neuves, une par zone, à côté des anciennes ; puis les trois préfixes de la table d'URL du répartiteur basculent vers elles, en une seule opération journalisée (`ARCHITECTURE.md` §5).
5. **Amorcer la grappe** : juste après le basculement, l'exploitant amorce la grappe neuve du protocole de PIN (`grappe/amorcer`, `ARCHITECTURE.md` §9.3, `API.md` §4.3).
6. **Forcer la mise à jour** : la version minimale de l'app est relevée à la première version qui contient la ligne. Une app plus ancienne demande sa mise à jour à sa prochaine ouverture. Aucune période de recouvrement n'est prévue pour les apps non mises à jour : la boutique propose toujours la mise à jour.
7. **Arrêter l'ancienne version** dès que les trois préfixes ont basculé : sa grappe du protocole de PIN est effacée, et chaque titulaire redépose dans la nouvelle (`ARCHITECTURE.md`, « Protocole de PIN »). Sa ligne passe ensuite `retiree`, avec `retireLe`, puis son empreinte sort de la condition KMS, qui reste égale aux lignes `en_service` (§7).

Règles :

- **Jamais deux versions de Sésame à moins de 30 jours d'écart**, sauf retrait pour une faille (tout autre retrait d'urgence attend l'échéance pour la version suivante) : un titulaire entré dans une grappe depuis moins de 30 jours ne peut pas en changer (`THREAT_MODEL.md` §2, limite 19).
- **Téléphone trop ancien** : sur un appareil sous la version minimale du système exigée par la dernière app, l'app laisse ouvrir et consulter le coffre localement, sans aucun appel à Sésame, et le signe de vie y est suspendu. Les fonctions de Sésame se font depuis un appareil à jour.
- Le client publie sa version minimale de l'app dans un fichier statique versionné (pour Fatoom : `fatoom.fr/.well-known/fatoom-version-minimale.json`, `ARCHITECTURE.md` §8.3), que `build/verifier-journaux.sh` lit avec son historique (§7).

## 5. Retrait d'urgence

À tout moment, sans délai :

1. ligne passée `retiree`, avec `retireLe` ;
2. empreinte retirée de la condition KMS **immédiatement** : la version ne descelle plus rien ;
3. version minimale de l'app relevée ;
4. VM de la version retirée arrêtées dès que la nouvelle est en service (effacement de sa grappe, redépôt) ; pour un retrait pour faille, arrêtées aussitôt, sans attendre la nouvelle, car retirer l'empreinte de KMS n'arrête ni le protocole de PIN, tenu en mémoire, ni le canal : Sésame ne sert alors plus rien jusqu'à la version corrective. Celle-ci exige des clients publiés (§4, étape 2), donc plus de 2 heures sans borne : elle est en pratique toujours une reprise publiée, sans attendre l'échéance des 30 jours, et sa grappe neuve s'amorce comme au §4, étape 5 ; un signalement de décès arrivé à échéance pendant l'arrêt est transmis à la reprise sans avoir pu être annulé (`THREAT_MODEL.md` §2, limite 8 ; décision 0051). Hors faille, la nouvelle version attend l'échéance des 30 jours, et Sésame, sans KMS, ne signe ni ne descelle plus rien jusque-là : la version suivante est une reprise publiée (`ARCHITECTURE.md` §11.6, décision 0049).

Google ne garantit aucun délai de propagation : une révocation n'est tenue pour effective qu'après au moins 7 minutes et une vérification.

## 6. Matériel admis : `hwmodels`

`hwmodels` liste, pour chaque version, les valeurs de `hwmodel` acceptées par les clients, la passerelle et la condition KMS. `GCP_AMD_SEV` (en service) et `GCP_INTEL_TDX` (option de montée en gamme, qui protège aussi l'intégrité de la mémoire) sont admis **dès la première version**. Le passage d'AMD SEV à Intel TDX se fait sans nouvelle version : les VM sont remplacées une à la fois. `GCP_AMD_SEV` est retiré ensuite par une version suivante.

## 7. Cohérence KMS ⇔ `versions.json`

La condition du fournisseur du pool d'identités est le seul point d'autorisation de toutes les clés de Sésame. Elle doit correspondre exactement à `versions.json` :

- l'ensemble des couples (empreinte, dictionnaire `env_override`) de la condition, une clause par version, est **identique** à l'ensemble des couples (`imageDigest`, `envDigest`) des lignes `en_service` ;
- la clause `dbgstat` est présente ;
- les valeurs de `hwmodel` correspondent à `hwmodels`.

Un contrôle CI lit la condition avec `gcloud` et échoue au moindre écart. Une empreinte autorisée dans KMS mais absente de `versions.json` signifie qu'une image non publiée pourrait desceller des clés (`THREAT_MODEL.md` §2, limite 2).

Le script public `build/verifier-journaux.sh` vérifie aussi le passé. Il lit le bucket d'audit public (journaux d'administration de Google exportés et bornes du registre) et signale, entre autres (liste complète : `ARCHITECTURE.md` §8.3) : tout appel aux clés qui ne vient pas d'une empreinte `en_service` dans l'historique de `versions.json` à cette date ; toute modification d'IAM, de pool, de fournisseur, de journalisation ou de verrou sans commit de `versions.json` correspondant ; tout import de matériel de clé ; tout trou de plus de 3 heures dans les bornes. La CI l'exécute chaque jour et publie son résultat ; n'importe qui peut l'exécuter.

## 8. Contrôles CI du dépôt

| Contrôle | Échoue si |
|---|---|
| Double construction | les deux digests diffèrent |
| Budget | plus de 6 000 lignes de Go hors tests et hors bibliothèques épinglées |
| Vecteurs | les vecteurs de test diffèrent de ceux du client de référence |
| `versions.json` | le fichier n'est pas conforme à son schéma |
| `hwmodels` de l'image | la constante `hwmodels` compilée dans l'image diffère des `hwmodels` de sa ligne de `versions.json` (`ARCHITECTURE.md` §3) |
| KMS ⇔ `versions.json` | les couples (empreinte, dictionnaire `env_override`) de la condition, la clause `dbgstat` ou les valeurs de `hwmodel` ne correspondent pas (§7) |
| Dépendances | une dépendance réseau hors de la liste d'`ARCHITECTURE.md` |
| Image | un shell ; une variable de lanceur factice lue hors du paquet de test |
| Scripts publics | `build/verifier-attestation.sh` ou `build/verifier-journaux.sh` accepte un argument manquant de sa signature (`README.md`) ; le premier accepte un jeton d'un autre projet ou d'un autre dépôt d'images, le second ne signale pas un appel aux clés d'un sujet fédéré d'un autre projet |
| Borne du registre | borne non signée ou non déposée toutes les 10 minutes en création seule ; pas d'échec fermé sans borne ; une version activée de la clé de signature refusée |
| Journaux (chaque jour) | `build/verifier-journaux.sh` signale un appel d'une identité inconnue, un surplus, une modification inattendue ou un trou ; le résultat est publié |

## 9. Actions manuelles de l'opérateur

| Action | Quand |
|---|---|
| Comptes de publication (Apple Developer, Google Play Console, GitHub : organisation et dépôt `sesame`, branche `main` protégée ; Google Cloud) sous passkey, sans SMS, codes de récupération hors ligne | avant la première version |
| Publier une version (§4) ou en retirer une (§5) | chaque version |
| Lire le résultat de `build/verifier-journaux.sh` ; chaque écart déclenche une alerte par email et est expliqué publiquement dans ce dépôt ; aucun écart ne s'efface | chaque jour |
| Recréer les VM sur l'image Confidential Space courante (l'attribut `STABLE` dure environ 6 mois) | au moins tous les 6 mois |
| Faire entrer la racine Confidential Space suivante dans les clients, par une version | avant l'échéance de la racine, renouvelée tous les 10 ans par Google |
| Restaurer le registre : toujours par une reprise publiée dans une nouvelle version de Sésame (`ARCHITECTURE.md`, « Registre ») | sur perte du registre |
