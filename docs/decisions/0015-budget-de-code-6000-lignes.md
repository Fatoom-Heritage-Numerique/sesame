<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0015 · Budget de code porté à 6 000 lignes de Go

**Date** : 2026-10-26 · **Statut** : actée (décision de l'opérateur) · revoit [0011](0011-go-binaire-statique-distroless.md) · **revue depuis** ([0074](0074-budget-de-code-8500-lignes.md))

## Contexte et menace

La décision 0011 bornait Sésame à moins de 5 000 lignes de Go hors tests, pour rester auditable. La grappe Raft du protocole de PIN (0017), les alertes envoyées par Sésame (0023) et la borne du registre (0021) ajoutent du code que ce plafond ne laissait pas écrire sans retirer de contrôle.

## Décision

Le budget visé est de l'ordre de 5 000 à 6 000 lignes de Go, hors tests et hors bibliothèques épinglées, avec un **plafond dur de 6 000 lignes** contrôlé par la CI. Le chiffre est remesuré sur le Sésame factice, puis sur l'image, et remplace alors l'estimation.

## Alternatives écartées

- **Garder 5 000 lignes** : il aurait fallu retirer la grappe ou des contrôles du registre.
- Le registre d'origine ne consigne pas d'autre alternative.

## Conséquences et limites

Une PR qui dépasse le plafond doit retirer autant qu'elle ajoute (`CONTRIBUTING.md`). Le code de la bibliothèque Raft n'entre pas dans le budget, mais sa version est épinglée et revue (0016).

## Documents concernés

`ARCHITECTURE.md` §2.2 ; `CONTRIBUTING.md` ; `RELEASE.md` §8.
