<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), « Sésame : principe et modèle de menace » et « Versions et alertes de Sésame » ; planche SiteSesameV2 -->

# Sésame

Sésame est le programme qui garde les clés de transmission de Fatoom. Il tourne dans une enclave Google Cloud Confidential Space, son code est public, et chacun peut vérifier que la version qui tourne est bien celle publiée ici.

Fatoom est un coffre-fort numérique : fichiers et mots de passe chiffrés sur le téléphone, et des proches qui recevront ce qui leur est destiné après le décès du titulaire. Pour que ces proches puissent ouvrir ce qui leur revient avec un simple code, quelque chose doit pouvoir déverrouiller ces contenus le moment venu. Ce n'est pas Fatoom : c'est Sésame, un petit programme qui tourne dans une enclave, et dont le code est public.

Sésame est conçu comme un service générique de garde de clés de transmission. Fatoom en est le premier client ; `ARCHITECTURE.md` et `API.md` décrivent le service indépendamment de lui.

## Où lire quoi

Ce dépôt ne contient pas de cahier des charges ni de document technique uniques : la spécification de référence est tenue par l'opérateur, et chaque document ici en reprend une partie, pour un lecteur précis. En langage courant :

| Vous voulez savoir… | Lisez |
|---|---|
| ce que Sésame fait, ne fait pas, et comment le vérifier | cette page |
| contre qui il protège, et ce qu'il ne protège pas (sans détour) | `THREAT_MODEL.md` |
| pourquoi il est fait comme ça, et ce qu'on a écarté | `docs/decisions/` (un fichier par décision) |
| comment il fonctionne, étape par étape | `ARCHITECTURE.md` |
| ce que Sésame et le client s'envoient exactement | `API.md` |
| comment une version est construite, publiée et retirée | `RELEASE.md` |
| comment contribuer, comment signaler une faille | `CONTRIBUTING.md`, `SECURITY.md` |

Les trois premiers disent le **quoi** et le **pourquoi** ; les suivants disent le **comment**. Chaque fichier commence par un commentaire qui indique la section de la spécification dont il dérive, pour que les deux restent alignés.

## Pour qui

- **Les utilisateurs de Fatoom**, qui veulent comprendre à quoi sert Sésame et ce qu'il ne fera jamais : cette page, puis `THREAT_MODEL.md`.
- **Les développeurs et les auditeurs**, qui veulent lire le code, reconstruire l'image et comparer son empreinte à celle publiée : les quatre étapes ci-dessous, puis `ARCHITECTURE.md`, `API.md` et `RELEASE.md`.
- **Les chercheurs en sécurité**, qui ont trouvé une faille : `SECURITY.md`.

## Qu'est-ce qu'une enclave ?

C'est une machine virtuelle dont la mémoire est chiffrée par le processeur lui-même. Sésame tourne dans Google Cloud Confidential Space, sur des processeurs AMD SEV, à Paris. Google, qui héberge le serveur, ne voit pas cette mémoire. Fatoom, qui le loue, n'a ni shell, ni accès SSH, ni port de débogage pour regarder à l'intérieur.

Une précision, sans détour : AMD SEV chiffre la mémoire mais ne protège pas son **intégrité**. Qui contrôle l'hyperviseur (Google, ou qui l'aurait compromis) pourrait la manipuler pour en extraire des secrets ; des attaques de ce type sont publiées. Intel TDX l'empêche : c'est l'option de montée en gamme, admise dès la première version de Sésame. Fatoom n'a pas cet accès à l'hyperviseur.

## Le parcours d'une clé

1. **Sur votre téléphone.** Quand vous destinez un fichier ou un mot de passe à un proche, sa clé est chiffrée sous votre clé de transmission. Cette clé, Sésame seul peut la desceller.
2. **En sommeil.** Tant que rien ne se passe, Sésame ne touche à rien : les clés restent chiffrées dans votre coffre-fort.
3. **Le moment venu.** Sésame n'ouvre que dans trois cas : un décès vérifié dont il a compté lui-même les 7 jours sans annulation de votre part ; une transmission par inactivité (signe de vie) dont il mesure aussi le délai ; votre contact de confiance, qui a prouvé à Sésame son code d'accès et un second code envoyé par Sésame.
4. **Chez votre proche.** Sésame ouvre uniquement les clés qui lui sont destinées et les lui transmet, chiffrées pour son navigateur. Les fichiers s'ouvrent chez lui, jamais chez Fatoom.

## Ce que Sésame peut faire, et ce qu'il ne fera jamais

**Il peut :**

- Ouvrir les clés de ce que vous destinez à un proche, une fois la transmission vérifiée.
- Ouvrir pour votre contact de confiance ce que vous lui avez destiné, après son code d'accès et un second code.

**Il ne peut pas :**

- Ouvrir ce que vous gardez pour vous, destiné à personne : il n'a jamais ces clés.
- Ouvrir pour un proche ce qui ne lui est pas destiné.
- Transmettre avant 7 jours, ou malgré votre annulation.
- Changer de comportement sans que son empreinte change.

## Ne nous croyez pas sur parole : vérifiez

Promettre « nous ne lisons pas vos fichiers » ne suffit pas. Avec Sésame, chacun peut contrôler ce qui tourne.

- **Un code public.** Le code de Sésame est publié en intégralité sur GitHub. Chacun peut le lire, l'auditer et le reconstruire.
- **Une empreinte unique.** Chaque version produit une empreinte (SHA-256). La moindre modification du code la change complètement.
- **Des clés verrouillées sur cette empreinte.** Le coffre de clés de Google (Cloud KMS) ne descelle qu'au programme dont l'empreinte est autorisée, et un contrôle automatique de ce dépôt vérifie que cette autorisation correspond aux versions publiées.
- **Une preuve à chaque échange.** Avant d'envoyer quoi que ce soit, l'app et fatoom.fr/acces demandent à l'enclave son attestation, un jeton signé par Google, et vérifient l'empreinte qu'il porte.

## Vérifiez Sésame en 4 étapes

Ces étapes s'adressent aux développeurs et aux auditeurs. Pour vous, l'app fait la dernière automatiquement. `<version>` et `<empreinte>` sont à lire dans `versions.json` (section suivante) ou sur fatoom.fr/sesame.

### 1. Récupérez le code

Clonez le dépôt public de Sésame, à la version indiquée dans `versions.json`.

```bash
git clone https://github.com/Fatoom-Heritage-Numerique/sesame
cd sesame
git checkout v<version>
git tag -v v<version>          # le tag est signé par une clé SSH (RELEASE.md §3)
```

`git tag -v` vérifie une signature SSH si la clé publique figure dans votre fichier `allowed_signers` (`gpg.ssh.allowedSignersFile`).

### 2. Reconstruisez l'image

Le build est reproductible : en suivant les instructions du dépôt, vous obtenez exactement la même image que celle qui tourne. Le script construit dans un conteneur de build épinglé par digest ; il ne dépend ni de votre version de Go ni de votre système.

```bash
./build/reproduire.sh v<version>
# Affiche : sha256:<empreinte obtenue>
```

### 3. Comparez l'empreinte

L'empreinte de votre image doit être identique, caractère pour caractère, à celle affichée sur fatoom.fr/sesame et dans `versions.json`.

```bash
jq -r '.versions[] | select(.tag == "v<version>") | .imageDigest' versions.json
```

Si elles diffèrent, ouvrez un rapport de vérification (gabarit d'issue « Rapport de vérification ») avec votre environnement et les deux empreintes.

### 4. Vérifiez l'attestation

L'enclave fournit une attestation : un jeton signé par Google, qui porte l'empreinte du programme en cours d'exécution et les clés publiques du canal chiffré. L'app Fatoom et fatoom.fr/acces la vérifient automatiquement avant chaque échange. Vous pouvez la demander vous-même :

```bash
./build/verifier-attestation.sh https://sesame.fatoom.fr versions.json
# GET /attestation, vérifie la chaîne de certificats jusqu'à la racine Confidential Space de Google,
# chaque claim exigé (ARCHITECTURE.md, « Attestation ») et que l'empreinte attestée est en_service
```

Ces scripts sont livrés avec le code ; leur contrat est décrit par ce `README`.

### Et après : les journaux publics

Une étape de plus pour qui veut contrôler l'opérateur lui-même : `build/verifier-journaux.sh` lit le bucket d'audit public de Sésame et vérifie que chaque descellement vient d'une version publiée (`RELEASE.md` §7). La CI de ce dépôt l'exécute chaque jour et publie son résultat.

## La liste des versions

`versions.json` est la liste des versions de Sésame : pour chacune, l'empreinte de l'image, le matériel admis, la date de publication et le statut (`en_service` ou `retiree`). Elle ne change que par un commit sur la branche `main` protégée de ce dépôt : son historique est public.

- La liste est **compilée** dans l'app, le portail des proches et la passerelle de Fatoom. Ils ne téléchargent rien et refusent toute version absente de la liste ou retirée.
- Une nouvelle version n'est mise en service qu'une fois l'app qui la contient disponible dans les deux boutiques. La version minimale de l'app est alors relevée : une app plus ancienne demande sa mise à jour.
- Une version peut être retirée immédiatement : elle ne descelle plus rien.
- Le coffre de clés (Cloud KMS) ne descelle qu'aux empreintes `en_service` ; un contrôle automatique du dépôt vérifie que les deux listes sont identiques.

Format exact et procédure : `RELEASE.md`.

## Les limites, sans détour

- **Le matériel et Google.** Sésame repose sur AMD SEV, sur Confidential Space et sur le service d'attestation de Google. Une faille exposerait les clés présentes en mémoire au moment de la faille, jamais ce que vous gardez pour vous. AMD SEV ne protégeant pas l'intégrité de la mémoire, Google, ou qui contrôlerait ses hyperviseurs, pourrait la manipuler pour en extraire ces clés. C'est le compromis des services les plus exigeants : sans enclave, Fatoom pourrait tout lire.
- **L'opérateur administre les clés.** Fatoom administre le projet Google Cloud de Sésame, avec un seul administrateur aujourd'hui. Il pourrait autoriser une autre image dans le coffre de clés : elle n'aurait besoin d'aucun client pour desceller les clés de transmission existantes. Ce n'est pas impossible ; c'est **visible**. L'opération laisse une trace dans les journaux d'administration de Google, exportés vers un bucket public que personne ne peut vider avant 10 ans sans supprimer Sésame entier, et le script public la signale.
- **Les comptes de publication.** Il n'y a pas de délai d'annonce entre la publication d'une version et sa mise en service. Qui tiendrait à la fois les accès Google Cloud de Sésame et un compte qui publie l'app, le site ou ce dépôt pourrait faire accepter une image malveillante. Ces comptes se connectent par passkey, sans SMS.
- **Le portail est servi par Fatoom.** Le portail des proches est un JavaScript servi par Fatoom, qui y compile aussi la liste des versions. Un Fatoom compromis qui servirait un portail modifié pourrait lire ce qu'un proche y ouvre. L'app, publiée par les boutiques, n'est pas concernée.
- **La disponibilité.** Fatoom peut tout bloquer : empêcher une annulation ou une alerte de Sésame d'arriver, au prix de bloquer aussi les ouvertures des proches. Passé l'échéance, une transmission sur décès est définitive.

Toutes les limites, adversaire par adversaire : `THREAT_MODEL.md` §2.

## État du projet

Sésame est **en construction**. Ce dépôt ne contient encore que sa documentation : ni code, ni scripts de `build/`, ni `versions.json` ; aucune version n'est publiée et aucune enclave ne tourne. L'app Fatoom ne promet Sésame qu'une fois livré.

Ordre de construction :

1. **Documentation** : ce dépôt tel que vous le lisez (en cours).
2. **Contrat exécutable** : OpenAPI, vecteurs de test partagés avec les clients, `sesame-fake` (un double local de Sésame pour développer l'app, le portail et la passerelle sans enclave).
3. **Cœur** : attestation, canal chiffré, KMS, registre, protocoles, grappe du protocole de PIN.
4. **Fournisseurs** : SMS et email, alertes, vérification d'identité, lettre.
5. **Infrastructure** : scripts `gcloud` du projet, du trousseau, du pool d'identités, des machines, du bucket d'audit.
6. **Version 1.0.0** inscrite à `versions.json`, compilée dans les clients, puis mise en service.

Les décisions prises et les alternatives écartées sont dans `docs/decisions/`.

## Documents

| Document | Contenu |
|---|---|
| `ARCHITECTURE.md` | Principe, enclave, attestation, libération des clés par KMS, canal chiffré, registre, journalisation, hiérarchie des clés, protocoles pas à pas |
| `API.md` | Contrat de chaque route de Sésame et des routes que le client expose à Sésame |
| `THREAT_MODEL.md` | Ce qui est protégé contre qui, et les limites assumées |
| `RELEASE.md` | Build reproductible, format de `versions.json`, mise en service, retrait, contrôles CI |
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
