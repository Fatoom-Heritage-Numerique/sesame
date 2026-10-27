<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0001 · Les clés des contenus destinés s'ouvrent dans une enclave au code public

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur, question par question) · **revue depuis** ([0014](0014-liste-des-versions-compilee.md)) : le manifeste est remplacé par la liste des versions `versions.json`, compilée dans les clients

## Contexte et menace

Dans le modèle initial du client, destiner un fichier à un proche envoyait la clé du fichier en clair au serveur, qui la rangeait sous une clé qu'il détenait ; les mots de passe étaient chiffrés par le serveur. Le serveur pouvait donc lire les contenus destinés et les mots de passe. La promesse « Fatoom ne peut pas lire ce que vous destinez » n'était pas tenue techniquement : elle reposait sur l'honnêteté de l'opérateur, de ses dépendances et de ses administrateurs.

## Décision

Les clés de ce qui est destiné aux proches ne s'ouvrent que dans **Sésame**, un programme au code public exécuté dans une enclave Google Cloud Confidential Space, dont l'app et le portail vérifient l'empreinte et à laquelle seule un coffre matériel (Cloud KMS) libère les clés. Le serveur du client range et transporte des objets qu'il ne sait ni lire ni fabriquer. Ce qui n'est destiné à personne ne traverse jamais Sésame. Sésame est spécifié entièrement avant tout code ; le modèle serveur est transitoire et l'app ne promet Sésame qu'une fois livré.

Critère retenu par l'opérateur : rien ne doit coûter à l'utilisateur final ; à coût nul, l'option la plus sûre. « Fatoom ne peut pas lire ce que vous destinez » doit rester vrai même contre Fatoom.

## Alternatives écartées

- **Déchiffrement dans le navigateur avec une clé dérivée du code d'accès** : le code (60 bits) deviendrait la seule protection des contenus destinés, et le serveur, qui connaît les codes qu'il envoie, pourrait tout lire.
- **Garder le chiffrement serveur** : la promesse resterait une promesse.

## Conséquences et limites

Sésame ajoute une base de confiance : le processeur, la pile Confidential Space et le service d'attestation de Google (`THREAT_MODEL.md` §2, limite 1). Il impose un canal applicatif par-dessus TLS, un manifeste, un registre, une clé de transmission par titulaire : décisions 0002 à 0007. Les contenus « gardés pour vous » ne changent pas : jamais de clé hors du téléphone.

## Documents concernés

`README.md`, `ARCHITECTURE.md` §1, `THREAT_MODEL.md`.
