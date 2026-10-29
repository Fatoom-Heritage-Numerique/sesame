<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0041 · Code d'invitation vérifié par son scellé, sans empreinte au registre

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option la plus simple qui tient la garantie annoncée) · précise [0036](0036-code-d-invitation-scelle-au-registre.md)

## Contexte et menace

Chaque invitation gardait au registre `codeVerifier`, un SHA-256 sans clé ni sel d'un code de 60 bits. Les lectures du registre ne sont pas journalisées. Qui lit le registre (l'administrateur du projet de Sésame) pouvait donc retrouver le code hors ligne, en 2⁶⁰ SHA-256 au plus : coûteux mais faisable, et sans aucun appel KMS. Après une vérification d'identité, le code seul rouvre l'accès pendant 12 mois. Cela contredisait « connu de Sésame seul » et l'objectif d'une opération qui ne peut pas rester invisible.

## Décision

- Le registre ne garde aucune empreinte du code d'invitation : `invitations/{lookup}` perd `codeVerifier`.
- Sur le parcours « courrier », Sésame vérifie le code saisi en descellant `codeInvitationScelle` de la dernière lettre du proche, déjà au registre, et en le comparant à temps constant au code reçu normalisé. Un code différent est un code faux du proche, compté comme avant.
- Le scellé sert donc à deux usages : vérifier un code saisi et réimprimer la même lettre. Chaque usage est un `Decrypt` au journal public de KMS.

Aucun champ, aucune clé, aucun composant ajouté : un champ disparaît, et le contrôle qui recomparait le code descellé à `codeVerifier` avant une réimpression disparaît avec lui (l'AAD lie déjà le scellé au titulaire, au proche et à l'invitation).

## Alternatives écartées

- **Écrire la limite sans rien changer** : la garantie annoncée tombait.
- **Un MAC KMS du code** : une clé de plus pour le même résultat que le scellé existant.
- **Recalculer l'Argon2id du `lookup` à chaque essai** : 64 Mio par essai sur des VM de 2 Go, que le contrat écarte du chemin chaud.
- **Exiger l'identité à chaque session** : alourdit le parcours du proche sans fermer la recherche hors ligne.

Pratique reprise : pour un secret de faible entropie, le secret qui en empêche la recherche hors ligne reste hors de la base, dans un HSM ou un coffre de secrets (OWASP, *Password Storage Cheat Sheet*, « pepper »). Ici, le code lui-même reste dans le HSM, scellé.

## Conséquences et limites

Un essai de code par le parcours « courrier » coûte un `Decrypt` KMS ; les essais sont déjà bornés (5 par 15 minutes par proche et par IP). Une image autorisée en douce dans KMS descellerait les codes comme les KTS, avec une trace au journal public (`THREAT_MODEL.md`, limite 2).

## Documents concernés

`ARCHITECTURE.md` §1, §4, §6.1, §11.2, §11.9 ; `API.md` §2.14, §6.
