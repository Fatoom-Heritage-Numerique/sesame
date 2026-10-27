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
- **Cryptographie de la bibliothèque standard** (`crypto/ecdh`, `crypto/aes`, `crypto/ed25519`, `crypto/hkdf`, `crypto/mlkem`, et `crypto/ecdsa`, `crypto/rsa`, `crypto/x509` pour vérifier l'attestation et les signatures du registre). Deux exceptions, épinglées : `github.com/cloudflare/circl` pour l'OPRF du protocole de PIN et `golang.org/x/crypto/argon2` pour l'Argon2id de l'index des lettres d'invitation. Aucune autre bibliothèque cryptographique.
- **Dépendances** : SDK Google officiels (KMS, Firestore, Secret Manager, Cloud Storage) ; `go.etcd.io/raft/v3`, épinglée, pour la grappe du protocole de PIN ; `net/http` brut pour Twilio Verify, Twilio Messaging, SendGrid, Stripe Identity et Pingen. Toute nouvelle dépendance se justifie dans la PR et entre dans la liste d'`ARCHITECTURE.md` §2.2 ; la CI refuse une dépendance réseau hors liste.
- **Adresses des fournisseurs en dur** dans le code, jamais lues au démarrage. Ce qui est propre à un client va dans des variables attestées (identifiants de compte, adresse de la passerelle) et, pour les textes et le gabarit de la lettre, dans des fichiers de l'image, attestés par son empreinte (`ARCHITECTURE.md` §2.3) ; les clés d'API sont dans Secret Manager. **Rien de propre à un client dans ce dépôt** : ni identifiant, ni nom de projet, de bucket ou de compte, ni texte de marque. Les noms de ressources sont écrits en `<projet-sesame>`, `<bucket-audit>`, `<bucket-inscrits>`, `<bucket-ecritures>`, `<compte-charge>`, `<compte-deces>`, `<compte-exploitation>`, `<cle-codes-deces>` ; les valeurs concrètes sont celles du déploiement de Fatoom, non publiées.
- **Budget** : de l'ordre de 5 000 à 6 000 lignes de Go visées, plafond dur de 6 000 lignes hors tests et hors bibliothèques épinglées (`ARCHITECTURE.md` §2.2), contrôlé en CI. Une PR qui le dépasse doit retirer autant qu'elle ajoute.
- **Journaux** : jamais de clé, de code, de numéro, d'email, de nom, de libellé ni de corps de requête (`ARCHITECTURE.md` §7). Les identifiants sont masqués (`uidMasque`).
- **Lanceur factice** (`SESAME_FAKE_LAUNCHER`) et doubles de KMS, Twilio, SendGrid, Stripe, Pingen, Firestore : uniquement dans le paquet de test ; le build échoue si la variable est lue ailleurs.
- Pas de rétrocompatibilité tant qu'aucune version n'est en production : un format se remplace, il ne se versionne pas. Cette règle sera levée à la première mise en service.

## Formats et vecteurs

Les formats (`ARCHITECTURE.md` : hiérarchie et formats des clés, canal, borne du registre) sont partagés avec les clients, octet pour octet ; `versions.json` (`RELEASE.md` §2) est compilé dans les clients. Les vecteurs de test sont **générés par le client de référence** et recopiés ici ; la CI échoue s'ils diffèrent. Un changement de format :

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
