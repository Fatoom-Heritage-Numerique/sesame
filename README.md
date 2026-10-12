<!-- Source : fatoom-app docs/TRD.md §16.1, §16.3.2, §16.5.5, §16.9 ; maquette « Fatoom · Maquettes », planches SiteSesame, SiteSecurite (03 · Sésame), OnbSecuriteInfo -->

# Sésame

Sésame est le programme qui garde les clés de transmission de Fatoom. Il tourne dans une enclave Google Cloud Confidential Space, son code est public, et chacun peut vérifier que la version qui tourne est bien celle publiée ici.

Fatoom est un coffre-fort numérique : fichiers et mots de passe chiffrés sur le téléphone, et des proches qui recevront ce qui leur est destiné après le décès du titulaire. Pour que ces proches puissent ouvrir ce qui leur revient avec un simple code, quelque chose doit pouvoir déverrouiller ces contenus le moment venu. Ce n'est pas Fatoom : c'est Sésame, un petit programme qui tourne dans une enclave sécurisée, et dont le code est public.

Sésame est conçu comme un service générique de garde de clés de transmission. Fatoom en est le premier client ; `ARCHITECTURE.md` et `API.md` décrivent le service indépendamment de lui.

## Pour qui

- **Les utilisateurs de Fatoom**, qui veulent comprendre à quoi sert Sésame et ce qu'il ne fera jamais : cette page, puis `THREAT_MODEL.md`.
- **Les développeurs et les auditeurs**, qui veulent lire le code, reconstruire l'image et comparer son empreinte à celle publiée : les quatre étapes ci-dessous, puis `ARCHITECTURE.md`, `API.md` et `RELEASE.md`.
- **Les chercheurs en sécurité**, qui ont trouvé une faille : `SECURITY.md`.

## Qu'est-ce qu'une enclave ?

C'est une zone d'un serveur isolée par le processeur lui-même. Sa mémoire est chiffrée en permanence : ni Google, qui héberge le serveur, ni Fatoom, qui le loue, ne peuvent regarder à l'intérieur ni modifier ce qui s'y exécute.

Sésame tourne dans Google Cloud Confidential Space. C'est une approche comparable à celle d'Apple et de Signal pour protéger leurs propres services.

## Le parcours d'une clé

1. **Sur votre téléphone.** Quand vous destinez un fichier ou un mot de passe à un proche, sa clé est chiffrée pour Sésame. Seul Sésame pourra la lire.
2. **En sommeil.** Tant que rien ne se passe, Sésame ne touche à rien : les clés restent chiffrées dans votre coffre-fort.
3. **Le moment venu.** Après la détection du décès et les 7 jours de vérification, ou quand votre contact de confiance saisit son code et le code SMS, Sésame vérifie la demande.
4. **Chez votre proche.** Sésame ouvre uniquement les clés qui lui sont destinées et les lui transmet, chiffrées pour son navigateur. Les fichiers s'ouvrent chez lui, jamais chez Fatoom.

## Ce que Sésame peut faire, et ce qu'il ne fera jamais

**Il peut :**

- Ouvrir les clés de ce que vous destinez à un proche, une fois la transmission vérifiée.
- Ouvrir pour votre contact de confiance ce que vous lui avez confié, après son code d'accès et un code SMS.

**Il ne peut pas :**

- Ouvrir ce que vous gardez pour vous, destiné à personne : il n'a jamais ces clés.
- Remettre une clé à Fatoom, ou à quiconque n'y a pas droit.
- Changer de comportement sans que son empreinte change.

## Ne nous croyez pas sur parole : vérifiez

Promettre « nous ne lisons pas vos fichiers » ne suffit pas. Avec Sésame, c'est la technique qui l'empêche, et chacun peut le contrôler.

- **Un code public.** Le code de Sésame est publié en intégralité sur GitHub. Chacun peut le lire, l'auditer et le reconstruire.
- **Une empreinte unique.** Chaque version produit une empreinte (SHA-256). La moindre modification du code la change complètement.
- **Des clés verrouillées sur cette empreinte.** Les clés de déchiffrement ne sont délivrées qu'au programme dont l'empreinte est publiée. Une version modifiée, même par nous, n'y aurait pas accès.
- **Une preuve à chaque échange.** Avant d'envoyer quoi que ce soit, l'app et fatoom.fr/acces demandent à l'enclave une attestation signée par le processeur, et vérifient l'empreinte.

## Vérifiez Sésame en 4 étapes

Ces étapes s'adressent aux développeurs et aux auditeurs. Pour vous, l'app fait la dernière automatiquement. Les commandes ci-dessous sont celles du dépôt ; `<version>` et `<empreinte>` sont à lire dans le manifeste (section suivante) ou sur fatoom.fr/sesame.

### 1. Récupérez le code

Clonez le dépôt public de Sésame, à la version indiquée dans le manifeste.

```bash
git clone https://github.com/Fatoom-Heritage-Numerique/sesame
cd sesame
git checkout v<version>
git tag -v v<version>          # le tag est signé ; la clé publique est dans RELEASE.md
```

### 2. Reconstruisez l'image

Le build est reproductible : en suivant les instructions du dépôt, vous obtenez exactement la même image que celle qui tourne chez nous. Le script construit dans un conteneur de build épinglé par digest ; il ne dépend ni de votre version de Go ni de votre système.

```bash
./build/reproduire.sh v<version>
# Affiche : sha256:<empreinte obtenue>
```

### 3. Comparez l'empreinte

L'empreinte de votre image doit être identique, caractère pour caractère, à celle affichée sur fatoom.fr/sesame et dans `manifeste.json`.

```bash
./build/verifier-manifeste.sh manifeste.json v<version> sha256:<empreinte obtenue>
# Vérifie les signatures du manifeste, puis que la ligne v<version> porte cette empreinte
```

Si elles diffèrent, ouvrez un rapport de vérification (gabarit d'issue « Rapport de vérification ») avec votre environnement et les deux empreintes.

### 4. Vérifiez l'attestation

L'enclave fournit une attestation signée par le processeur, qui contient l'empreinte du programme en cours d'exécution. L'app Fatoom et fatoom.fr/acces la vérifient automatiquement avant chaque échange. Vous pouvez la demander vous-même :

```bash
./build/verifier-attestation.sh https://sesame.fatoom.fr manifeste.json
# GET /attestation, vérifie la signature Google, chaque claim exigé (ARCHITECTURE.md §3)
# et que l'empreinte attestée est « en service » dans le manifeste
```

Ces trois scripts sont livrés avec le code (étape 2 du plan de construction, ci-dessous) ; leur contrat est décrit dans `RELEASE.md`.

## Le manifeste

Le manifeste est la liste signée des versions de Sésame : pour chacune, l'empreinte de l'image, la date de publication, la date de mise en service et le statut (`annoncee`, `en_service`, `retiree`). Il est signé sur une clé matérielle, hors de tout serveur, et publié **7 jours avant** toute mise en service, ici (`manifeste.json`) et sur fatoom.fr/sesame.

- L'app, le portail des proches et la passerelle de Fatoom refusent toute version absente du manifeste, annoncée mais pas encore en service, ou retirée.
- Le coffre de clés matériel (Cloud KMS) ne libère les clés qu'aux empreintes `en_service` ; un contrôle automatique du dépôt vérifie que les deux listes sont identiques.
- Une version peut être retirée immédiatement, jamais ajoutée plus vite : un correctif attend 7 jours, les transmissions patientent.

Format exact et procédure de signature : `RELEASE.md`.

## Les limites, sans détour

Sésame repose sur la sécurité du processeur et de Google Cloud Confidential Space. Une faille dans ce matériel pourrait, en théorie, exposer ce qu'il protège. C'est le compromis accepté par les services les plus exigeants, et ce que vous gardez pour vous ne passe jamais par Sésame.

Toutes les limites, adversaire par adversaire : `THREAT_MODEL.md`.

## État du projet

Sésame est **en construction**. Aucune version n'est publiée, le manifeste est vide, aucune enclave ne tourne. L'app Fatoom ne promet Sésame qu'une fois livré.

Ordre de construction :

1. **Documentation** : ce dépôt tel que vous le lisez (en cours).
2. **Contrat exécutable** : OpenAPI, vecteurs de test partagés avec les clients, `sesame-fake` (un double local de Sésame pour développer l'app, le portail et la passerelle sans enclave).
3. **Cœur** : attestation, canal chiffré, KMS, registre, protocoles.
4. **Fournisseurs** : SMS et email, vérification d'identité, lettre.
5. **Infrastructure** : scripts `gcloud` du projet, du trousseau, du pool d'identités, des machines.
6. **Version 1.0.0** annoncée au manifeste, en service 7 jours plus tard.

Les décisions prises et les alternatives écartées sont dans `docs/decisions/`.

## Documents

| Document | Contenu |
|---|---|
| `ARCHITECTURE.md` | Principe, enclave, attestation, libération des clés par KMS, canal chiffré, registre, journalisation, hiérarchie des clés, protocoles pas à pas |
| `API.md` | Contrat de chaque route de Sésame et des routes que le client expose à Sésame |
| `THREAT_MODEL.md` | Ce qui est protégé contre qui, et les limites assumées |
| `RELEASE.md` | Build reproductible, signature du manifeste, publication, retrait, format de `manifeste.json` |
| `CONTRIBUTING.md` | Comment contribuer, ce qu'une PR doit contenir |
| `SECURITY.md` | Signaler une faille |
| `docs/decisions/` | Une décision par fichier, avec les alternatives écartées |

## Liens

- Code et empreinte, vus par les utilisateurs : <https://fatoom.fr/sesame>
- Portail des proches : <https://fatoom.fr/acces>
- Comment Fatoom protège vos données : <https://fatoom.fr/securite>
- Signaler une faille : `SECURITY.md`

## Licence

Apache 2.0 (`LICENSE`).
