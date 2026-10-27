<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0027 · Lettre et identité : compte Pingen distinct, trois champs, bascule, lettre de secours, suppression chez Stripe

**Date** : 2026-10-23 · **Statut** : actée (validée par l'opérateur) · revoit [0008](0008-lettre-et-identite-dans-sesame.md)

## Contexte et menace

La décision 0008 confiait à Sésame la lettre et la vérification d'identité d'un proche sans téléphone ni email. Restaient ouverts : qui peut lire les lettres, sur quoi porte la comparaison, que faire pour un proche dont les coordonnées ne marchent plus, et combien de temps Stripe garde la pièce et le selfie.

## Décision

- **Compte** Pingen distinct de celui du client, pas seulement une organisation : un accès API voit tout son compte.
- Identité comparée sur **trois champs obligatoires** de la charte (prénom, nom, date de naissance), noms comparés avec tolérance, date à l'identique.
- « Je n'ai plus ce numéro / cet email » : après une transmission arrivée à terme, la session bascule sur la même vérification d'identité.
- **Lettre de secours** quand le client constate l'échec technique de l'email et du téléphone d'un proche qui a une adresse dans sa charte.
- Cinq appels REST pour Pingen et trois pour Stripe, dont la suppression de la pièce et du selfie (`redact`) dès le verdict.

## Alternatives écartées

Le registre d'origine ne consigne pas d'alternative au-delà de celles de 0008.

## Conséquences et limites

Les échecs d'identité et le blocage de 24 h sont tenus par proche au registre. Après une première identité vérifiée, la lettre seule rouvre l'accès pendant 12 mois (`THREAT_MODEL.md`, limite 4).

## Documents concernés

`ARCHITECTURE.md` §11.2, §11.9 ; `API.md` (`lettre/secours`, `session/identite/*`) ; `THREAT_MODEL.md` §2 (4, 21).
