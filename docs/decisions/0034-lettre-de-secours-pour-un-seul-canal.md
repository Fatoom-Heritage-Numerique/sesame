<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0034 · Lettre de secours pour un proche joint par un seul canal

**Date** : 2026-10-28 · **Statut** : actée (validée par l'opérateur) · revoit [0027](0027-lettre-et-identite-precisees.md)

## Contexte et menace

La décision 0027 faisait partir la lettre de secours quand l'email et le SMS d'un proche avaient tous deux échoué. Elle ne disait rien d'un proche qui n'a qu'un seul canal (email seul ou téléphone seul) et une adresse postale : si ce canal échoue, le proche n'était plus relancé.

## Décision

Pour un proche qui a une adresse postale, la lettre de secours part quand chacun de ses canaux a échoué techniquement après les nouveaux essais : l'email et le SMS s'il a les deux, sinon son seul canal. Une absence d'ouverture ne la déclenche toujours pas.

## Alternatives écartées

- **Aucune lettre pour un proche à un seul canal** : il ne serait plus prévenu, alors que son adresse est connue.

## Conséquences et limites

Une lettre de plus peut partir pour un proche à un seul canal. Le code reste le même que celui du canal échoué.

## Documents concernés

`API.md` §2.10 et §5.4 ; `ARCHITECTURE.md` §11.2 et §11.9.
