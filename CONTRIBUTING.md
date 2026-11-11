<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), « Sésame : l'enclave » (code minimal, budget de code, image reproductible et dépôt public) -->

# Contribuer à Sésame

Merci de lire ce dépôt, de le reconstruire et de le critiquer : c'est pour cela qu'il est public. Avant d'ouvrir une PR, voici ce qui compte.

## Ce qu'est ce dépôt

Sésame est une **zone critique** : chaque ligne peut ouvrir ou fermer la clé d'un contenu que quelqu'un destinait à ses proches. La spécification de référence est tenue par l'opérateur ; ce dépôt en est la reprise publique (`ARCHITECTURE.md`, `API.md`, `THREAT_MODEL.md`, `RELEASE.md`) et fait foi pour le code. Quand le code et ces documents divergent, c'est un bug : le document ou le code est corrigé dans la même PR.

Le dépôt est construit dans cet ordre : documentation, contrat exécutable (OpenAPI, vecteurs, `sesame-fake`), cœur, fournisseurs, infrastructure, version 1.0.0 (`README.md`, « État du projet »). Une PR qui saute une étape sera redirigée.

## Langue

La prose est en **français** : documents, commentaires, messages de commit, descriptions de PR, issues. Le code et ses identifiants sont en anglais, comme les API qu'il appelle ; les termes métier (charte, enveloppe, titulaire, proche, preuve de vie, annulation, signalement, borne, grappe) ne se traduisent pas et apparaissent tels quels dans les noms (`Charte`, `Enveloppe`, `PreuveDeVie`).

## Règles du code

- **Go**, dernière version stable épinglée dans `go.mod` et dans la CI ; module unique ; binaire statique.
- **Cryptographie de la bibliothèque standard** (`crypto/ecdh`, `crypto/aes`, `crypto/ed25519`, `crypto/hkdf`, `crypto/mlkem`, et `crypto/ecdsa`, `crypto/rsa`, `crypto/x509` pour vérifier l'attestation et les signatures du registre). Deux exceptions cryptographiques, épinglées : `github.com/cloudflare/circl` pour l'OPRF du protocole de PIN et `golang.org/x/crypto/argon2` pour l'Argon2id de l'index des lettres d'invitation. Aucune autre bibliothèque cryptographique.
- **Dépendances** : SDK Google officiels (KMS, Firestore, Secret Manager, Cloud Storage, Cloud Logging) ; `go.etcd.io/raft/v3`, épinglée, pour la grappe du protocole de PIN ; `net/http` brut pour Twilio Verify, Twilio Messaging, SendGrid, Stripe Identity et Pingen. Toute nouvelle dépendance se justifie dans la PR et entre dans les listes d'`ARCHITECTURE.md` §2.4 (bibliothèques) et §2.2 (réseau) ; la CI refuse une dépendance réseau hors liste.
- **Adresses des fournisseurs en dur** dans le code, jamais lues au démarrage. Ce qui est propre à un client va dans des variables attestées (identifiants de compte, adresse de la passerelle) et, pour les textes et les gabarits des lettres, dans un document lu au démarrage et attesté par son empreinte, elle-même variable attestée (`ARCHITECTURE.md` §2.3) ; les clés d'API et ce document sont dans Secret Manager. **Rien de propre à un client dans ce dépôt** : ni identifiant, ni nom de projet, de bucket ou de compte, ni texte de marque. Les noms de ressources sont écrits en `<projet-sesame>`, `<bucket-audit>`, `<bucket-inscrits>`, `<bucket-ecritures>`, `<compte-charge>`, `<compte-deces>`, `<compte-exploitation>`, `<cle-codes-deces>`, `<secret-textes>`, `<registre-images>`, `<numero-projet-sesame>` ; les valeurs concrètes sont celles du déploiement de Fatoom et ne s'écrivent nulle part dans ce dépôt. Cette règle porte sur le dépôt seul : la page de transparence du client (pour Fatoom : fatoom.fr/sesame) publie les valeurs des arguments des scripts publics, `<numero-projet-sesame>`, `<registre-images>` et `<bucket-audit>`, dont les deux derniers contiennent le nom du projet, pour que chacun puisse exécuter ces scripts (`README.md`, étape 4) ; les autres valeurs ne sont publiées nulle part.
- **Budget** : de l'ordre de 5 000 à 6 000 lignes de Go visées, plafond dur de 6 000 lignes hors tests et hors bibliothèques épinglées (`ARCHITECTURE.md` §2.2), contrôlé en CI. Une PR qui le dépasse doit retirer autant qu'elle ajoute.
- **Journaux** : jamais de clé, de code, de numéro, d'email, de nom, de libellé ni de corps de requête (`ARCHITECTURE.md` §7). Les identifiants sont masqués (`uidMasque`).
- **Lanceur factice** (`SESAME_FAKE_LAUNCHER`) et doubles de KMS, Twilio, SendGrid, Stripe, Pingen, Firestore, Cloud Logging : uniquement dans le paquet de test ; le build échoue si la variable est lue ailleurs.
- Pas de rétrocompatibilité tant qu'aucune version n'est en production : un format se remplace, il ne se versionne pas.

## Formats et vecteurs

Les formats (`ARCHITECTURE.md` : hiérarchie et formats des clés, canal, borne du registre) sont partagés avec les clients, octet pour octet ; `versions.json` (`RELEASE.md` §2) est compilé dans les clients. Les vecteurs de test sont **générés par le client de référence** et recopiés ici octet pour octet, avec un fichier `SHA256SUMS` de leurs empreintes ; la CI échoue si une copie ne correspond pas à `SHA256SUMS`, et la CI du client de référence échoue si ses fichiers diffèrent des copies de ce dépôt.

Fichiers repris de `packages/crypto/vectors/` du dépôt `Fatoom-Heritage-Numerique/Fatoom`, ceux dont Sésame refait les calculs (la liste, et ce que le code Go recalcule pour chaque groupe, sont tenus par le client de référence : `docs/technique/tests.md`, « Vecteurs repris par le dépôt `sesame` ») :

- `sesame.json` : canal, `instanceId`, jetons de paire, reçus, jeton de session et requêtes à la passerelle, KTS et dérivations, enveloppes, charte, code d'accès et son index, messages signés par `cleVie`, dépôts du PIN, fiche d'alerte, scellés, registre et borne ;
- `primitives.json` : vecteurs publiés de chaque primitive, avec les refus de bord (secret X25519 nul, signature Ed25519 `S + ℓ`, clé ML-KEM-768 hors module) ;
- `json.json` : JSON canonique RFC 8785 et refus d'une chaîne hors NFC ;
- `attestation.json` : vérification d'un jeton PKI fabriqué sous une racine de test, et format de `versions.json` (`listeVersions`).

Ils se lisent tels quels en Go (`encoding/json`, `encoding/hex`) : JSON en UTF-8 aux clés triées ; `{"vecteurs": [{"nom", "entrees", "sorties"}], "refus": [{"nom", "entrees", "erreur"}]}`, `nom` de la forme `<opération>/<cas>` ; octets en hexadécimal minuscule, et à côté la chaîne base64 ou base64url exacte quand le protocole transporte la valeur ainsi ; texte UTF-8 en chaîne JSON telle quelle. Chaque vecteur donne les clés de test qu'il faut (graines, clés AES tenant lieu des clés KMS, `cleOprf`) ; les signatures P-256 des vecteurs sont déterministes (RFC 6979), le code les vérifie sans chercher à les reproduire. Les fichiers sont recopiés octet pour octet, sous leur nom, et le commit qui les recopie cite le commit du client d'où ils viennent ; la CI compare l'empreinte SHA-256 de chaque copie à celle du fichier sur la branche `main` du client et échoue au moindre écart.

Un changement de format :

1. se décrit d'abord (menace, décision, alternatives écartées) dans une issue, puis dans un fichier de `docs/decisions/` ;
2. est validé par l'opérateur avant tout code ;
3. change les documents de ce dépôt, le code et les vecteurs dans la même PR ;
4. est livré des deux côtés (Sésame et clients) dans la même série de PR.

On ne modifie jamais un vecteur à la main.

## Ce qu'une PR contient

- Un titre et une description en français : ce qui change, pourquoi, quelle section d'`ARCHITECTURE.md` ou d'`API.md` est concernée.
- Les tests : chaque décision de Sésame (charte périmée, rôle insuffisant, signalement trop récent, enveloppe à AAD étranger, défi rejoué, annulation d'un autre signalement, …) a son test unitaire ; la lettre est comparée octet pour octet ; les fournisseurs sont rejoués par des doubles : Twilio et SendGrid en mémoire, Stripe et Pingen en doubles HTTP locaux (`ARCHITECTURE.md` §2.4).
- Les documents mis à jour si le changement les rend faux, et un fichier `docs/decisions/` si une décision est prise ou revue (une décision revue est un **nouveau** fichier, jamais une réécriture).
- La CI verte : build reproductible (double construction), budget, vecteurs, schéma de `versions.json`, cohérence KMS ⇔ `versions.json`, dépendances, image sans shell, borne du registre (`RELEASE.md` §8).

Une PR qui touche l'attestation, le canal, KMS, le registre, le protocole de PIN, les protocoles ou `versions.json` est revue par les propriétaires du dépôt (`CODEOWNERS`) et n'est fusionnée qu'avec leur accord explicite. Tant qu'il n'y a qu'un administrateur, une telle PR cite une décision écrite (`docs/decisions/`) et reste publique au moins 7 jours avant sa fusion ; dès qu'une seconde personne de confiance existe, elle approuve aussi. L'équipe des propriétaires (`CODEOWNERS`) n'a aujourd'hui qu'un membre : l'auteur d'une telle PR en est aussi l'approbateur, et la décision écrite et les 7 jours de publicité tiennent lieu de second regard.

## Vérifier et signaler

- Vous avez reconstruit une version et l'empreinte diffère : ouvrez une issue avec le gabarit **Rapport de vérification**.
- Vous avez trouvé une faille : **ne l'écrivez pas dans une issue publique**, suivez `SECURITY.md`.
- Vous avez une question ou une proposition : une issue ordinaire, en français.

## Licence

En soumettant une contribution, vous acceptez qu'elle soit distribuée sous la licence du dépôt (Apache 2.0, section 5 : toute contribution soumise intentionnellement est sous les termes de la licence). Aucun accord de contribution séparé ni sign-off n'est demandé.
