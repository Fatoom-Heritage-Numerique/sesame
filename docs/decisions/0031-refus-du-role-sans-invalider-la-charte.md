<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0031 · Refus du rôle depuis le portail sans invalider la charte

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur)

## Contexte et menace

Un refus du rôle par le contact de confiance, depuis une session du portail, fermait son code chez Sésame (`chartes` à `-1`) jusqu'à ce que l'app republie une charte. Si le titulaire mourait avant, ce proche ne pouvait plus rien ouvrir, alors qu'il devait recevoir à son décès un code valable comme tout proche.

## Décision

- Le refus retire le rôle `gardien` sans invalider la charte : Sésame l'inscrit pour la génération courante (`rolesRefuses`), ferme la session et prévient le client (`roleRefuse`).
- La charte de cette génération vaut ensuite rôle `proche` : aucune ouverture du vivant du titulaire, et son code ouvre à la transmission comme celui de tout proche.
- Une génération plus récente publiée par le titulaire (réinvitation, autre contact de confiance) remplace la charte refusée. `-1` ne signifie plus qu'une suppression du proche, définitive.

## Alternatives écartées

- **Fermer le code jusqu'à la republication** (état d'avant) : un décès entre-temps privait ce proche de ce qui lui est destiné.

## Conséquences et limites

Le client met la fiche du proche à jour ; rien n'est à republier pour que le code vaille à la transmission.

## Documents concernés

`API.md` (`session/refuser-role`, `session/ouvrir`, `session/ouvrir-cles`, `charte/generation`) ; `ARCHITECTURE.md` §6.1, §10.5, §11.2.
