<!-- Source : fatoom-app docs/TRD.md §16.3.1, §16.3.2, §16.8.1, §16.8.4, §13.1 -->

# Contribuer à Sésame

Merci de lire ce dépôt, de le reconstruire et de le critiquer : c'est pour cela qu'il est public. Avant d'ouvrir une PR, voici ce qui compte.

## Ce qu'est ce dépôt

Sésame est une **zone critique** : chaque ligne peut ouvrir ou fermer la clé d'un contenu que quelqu'un destinait à ses proches. La spécification de référence est tenue par l'opérateur ; ce dépôt en est la reprise publique (`ARCHITECTURE.md`, `API.md`, `THREAT_MODEL.md`, `RELEASE.md`) et fait foi pour le code. Quand le code et ces documents divergent, c'est un bug : le document ou le code est corrigé dans la même PR.

Le dépôt est construit dans cet ordre : documentation, contrat exécutable (OpenAPI, vecteurs, `sesame-fake`), cœur, fournisseurs, infrastructure, version 1.0.0 (`README.md`, « État du projet »). Une PR qui saute une étape sera redirigée.

## Langue

La prose est en **français** : documents, commentaires, messages de commit, descriptions de PR, issues. Le code et ses identifiants sont en anglais, comme les API qu'il appelle ; les termes métier (charte, enveloppe, titulaire, proche, gardien, preuve de vie, signalement) ne se traduisent pas et apparaissent tels quels dans les noms (`Charte`, `Enveloppe`, `PreuveDeVie`).

## Règles du code

- **Go**, dernière version stable épinglée dans `go.mod` et dans la CI ; module unique ; binaire statique.
- **Cryptographie de la bibliothèque standard** (`crypto/ecdh`, `crypto/aes`, `crypto/ed25519`, `crypto/ecdsa`, `golang.org/x/crypto/hkdf`). Aucune autre bibliothèque cryptographique.
- **Dépendances** : SDK Google officiels (KMS, Firestore, Secret Manager) ; `net/http` brut pour Twilio Verify, Stripe Identity et Pingen. Toute nouvelle dépendance se justifie dans la PR et entre dans la liste d'`ARCHITECTURE.md` §2.1 ; la CI refuse une dépendance réseau hors liste.
- **Adresses des fournisseurs en dur** dans le code, jamais lues au démarrage. Ce qui est propre à un client (identifiants de compte, textes, gabarit de lettre, adresse de la passerelle) est une variable attestée ; les clés d'API sont dans Secret Manager. **Rien de propre à un client dans ce dépôt** : ni identifiant, ni texte de marque, ni adresse.
- **Budget** : moins de 5 000 lignes de Go hors tests. Une PR qui le dépasse doit retirer autant qu'elle ajoute.
- **Journaux** : jamais de clé, de code, de numéro, d'email, de nom, de libellé ni de corps de requête (`ARCHITECTURE.md` §7). Les identifiants sont masqués (`uidMasque`).
- **Lanceur factice** (`SESAME_FAKE_LAUNCHER`) et doubles de KMS, Twilio, Stripe, Pingen, Firestore : uniquement dans le paquet de test ; le build échoue si la variable est lue ailleurs.
- Pas de rétrocompatibilité tant qu'aucune version n'est en production : un format se remplace, il ne se versionne pas. Cette règle sera levée à la première mise en service.

## Formats et vecteurs

Les formats (`ARCHITECTURE.md` §8, canal §5, manifeste `RELEASE.md` §2) sont partagés avec les clients, octet pour octet. Les vecteurs de test sont **générés par le client de référence** et recopiés ici ; la CI échoue s'ils diffèrent. Un changement de format :

1. se décrit d'abord (menace, décision, alternatives écartées) dans une issue, puis dans un fichier de `docs/decisions/` ;
2. est validé par l'opérateur avant tout code ;
3. change les documents de ce dépôt, le code et les vecteurs dans la même PR ;
4. est livré des deux côtés (Sésame et clients) dans la même série de PR.

On ne modifie jamais un vecteur à la main.

## Ce qu'une PR contient

- Un titre et une description en français : ce qui change, pourquoi, quelle section d'`ARCHITECTURE.md` ou d'`API.md` est concernée.
- Les tests : chaque décision de Sésame (charte périmée, rôle insuffisant, signalement trop récent, enveloppe à AAD étranger, nonce rejoué, …) a son test unitaire ; la lettre est comparée octet pour octet ; les fournisseurs sont rejoués par des doubles HTTP locaux.
- Les documents mis à jour si le changement les rend faux, et un fichier `docs/decisions/` si une décision est prise ou revue (une décision revue est un **nouveau** fichier, jamais une réécriture).
- La CI verte : build reproductible (double construction), vecteurs, manifeste, dépendances, image sans shell, budget.

Une PR qui touche l'attestation, le canal, KMS, le registre, les protocoles ou le manifeste est revue par les propriétaires du dépôt (`CODEOWNERS`) et n'est fusionnée qu'avec leur accord explicite. Personne, l'opérateur compris, ne fusionne seul une telle PR sans qu'une décision écrite n'existe.

## Vérifier et signaler

- Vous avez reconstruit une version et l'empreinte diffère : ouvrez une issue avec le gabarit **Rapport de vérification**.
- Vous avez trouvé une faille : **ne l'écrivez pas dans une issue publique**, suivez `SECURITY.md`.
- Vous avez une question ou une proposition : une issue ordinaire, en français.

## Licence

En soumettant une contribution, vous acceptez qu'elle soit distribuée sous la licence du dépôt (Apache 2.0, section 5 : toute contribution soumise intentionnellement est sous les termes de la licence). Aucun accord de contribution séparé ni sign-off n'est demandé.
