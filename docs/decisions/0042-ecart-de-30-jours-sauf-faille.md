<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0042 · Deux versions à moins de 30 jours d'écart seulement pour une faille

**Date** : 2026-10-28 · **Statut** : actée (alignement sur la règle du client, sans changement de cette règle) · précise [0017](0017-protocole-de-pin-grappe-de-trois-vm.md)

## Contexte et menace

La règle d'exploitation interdit deux versions de Sésame à moins de 30 jours d'écart, parce que chaque nouvelle grappe rend des essais de PIN et que la règle des 30 jours borne ce cas. L'exception était écrite « sauf retrait d'urgence », alors que la règle du client dit « sauf faille » et que la procédure distingue le retrait pour une faille de tout autre retrait d'urgence.

## Décision

- L'exception ne vaut que pour un retrait pour une faille. Tout autre retrait d'urgence retire la version de KMS aussitôt, mais la version suivante attend l'échéance des 30 jours.

## Alternatives écartées

- **Garder « sauf retrait d'urgence »** : formulation plus large que la règle du client, qui ouvrait plus de grappes neuves que la règle des 30 jours ne l'annonce.

## Conséquences et limites

Un défaut sans faille de sécurité se corrige au plus tôt 30 jours après la version précédente. Les titulaires entrés dans une grappe depuis moins de 30 jours restent bloqués jusqu'à l'échéance, comme avant (`THREAT_MODEL.md`, limite 19).

## Documents concernés

`ARCHITECTURE.md` §6.4, §9.3, §11.6 ; `RELEASE.md` ; `THREAT_MODEL.md` §2 (19).
