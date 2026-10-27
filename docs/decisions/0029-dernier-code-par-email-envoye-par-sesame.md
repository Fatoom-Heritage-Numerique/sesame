<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0029 · Dernier code par email tiré et envoyé par Sésame lui-même ; Twilio Verify pour le SMS seulement

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur) · revoit [0004](0004-second-facteur-envoye-et-verifie-par-sesame.md) et [0010](0010-fournisseurs-nommes-dans-le-code.md)

## Contexte et menace

Le dernier code d'un proche sans téléphone partait par le canal email de Twilio Verify, avec un gabarit réglé chez le fournisseur. Ce réglage échappe à l'attestation : qui administre le compte pourrait modifier le gabarit pour recopier le code ailleurs. L'exploitant, qui peut connaître le code d'accès (`THREAT_MODEL.md`, limite 3), tiendrait alors les deux facteurs d'un proche à email seul.

## Décision

- Sans téléphone dans la charte, Sésame tire lui-même le dernier code (6 chiffres, valable 10 minutes, gardé dans l'état de la session), l'envoie par l'API Mail Send de SendGrid, avec un texte de l'image et l'adresse d'expédition attestée de ses alertes, et le vérifie lui-même.
- Twilio Verify ne sert qu'au dernier code par SMS.

## Alternatives écartées

- **Garder le canal email de Verify et écrire une limite assumée** : le code d'un proche à email seul resterait à portée d'un réglage du compte.

## Conséquences et limites

Le compte SendGrid de Sésame reste réglé hors de l'attestation : ses réglages qui copieraient ou réécriraient un envoi sont désactivés. Un proche à email seul reçoit toujours ses deux codes par email (`THREAT_MODEL.md`, limite 13, et 0013).

## Documents concernés

`API.md` (`session/ouvrir`, `session/verifier`) ; `ARCHITECTURE.md` §2.2, §2.3, §11.2 ; `THREAT_MODEL.md` §2 (13).
