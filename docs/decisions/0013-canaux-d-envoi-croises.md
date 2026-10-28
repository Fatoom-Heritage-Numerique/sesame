<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0013 · Canaux d'envoi croisés : code d'accès par email, dernier code par SMS

**Date** : 2026-10-22 · **Statut** : actée (décision de l'opérateur, septième tour) · remplace [0012](0012-canaux-d-envoi-non-croises.md)

## Contexte et menace

Avec la décision 0012, un attaquant qui lisait tout ce qui passe chez un seul fournisseur d'envoi tenait les deux facteurs d'un proche joint par ce canal, et pouvait ouvrir ce qui lui est destiné après un décès vérifié. La question avait été laissée ouverte avant la production.

## Décision

Pour un proche qui a un email et un téléphone, les canaux sont **croisés** : le code d'accès part par email, le dernier code par SMS ; il faut alors compromettre les deux comptes d'envoi. L'opérateur email appartient au même groupe que l'opérateur SMS : les deux canaux reposent sur deux infrastructures et deux comptes distincts (celui de Sésame pour le dernier code), pas sur deux sociétés.

## Alternatives écartées

- **Garder le modèle de 0012** (code par tous les canaux, dernier code par l'un d'eux) : un seul fournisseur compromis suffisait.
- **Un second opérateur SMS** : écarté pour la simplicité.

## Conséquences et limites

Risque accepté : un proche à email seul reçoit code et dernier code par email, un proche à téléphone seul les deux par SMS ; un opérateur compromis ouvre alors ce qui lui est destiné après un décès vérifié. Il ne peut rien avant une transmission vérifiée par Sésame, rien de ce qui est gardé pour soi, et rien sur un proche « courrier » (identité). Le client reste responsable de traitement vis-à-vis des utilisateurs.

## Documents concernés

`THREAT_MODEL.md` §2 (13) ; `ARCHITECTURE.md` §11.2.
