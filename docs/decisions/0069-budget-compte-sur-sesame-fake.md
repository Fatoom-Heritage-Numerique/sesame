<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0069 · Budget de code compté sur ce que lient `cmd/sesame` et `cmd/sesame-fake`, hors doubles

**Date** : 2026-11-11 · **Statut** : actée (choix technique délégué par l'opérateur) · applique [0063](0063-sesame-fake-contrat-executable.md) et [0015](0015-budget-de-code-6000-lignes.md)

## Contexte et menace

Le plafond dur de 6 000 lignes (décision 0015) se contrôle sur les paquets que lie `cmd/sesame` (`RELEASE.md` §8). Jusqu'aux adaptateurs réels (étapes 3 à 5), `cmd/sesame` n'a rien à brancher sur le code des décisions et ne le lie pas : le contrôle compterait quelques lignes pendant que le code des décisions grandit dans `sesame-fake`, qui le partage (décision 0063, « le budget de code se mesure dès cette étape sur les décisions »). Menace : un dépassement découvert seulement au branchement de l'image, quand il faut retirer du code déjà écrit et testé.

## Décision

- `outils/decompte-lignes` compte les paquets de ce module que lient `cmd/sesame` **et** `cmd/sesame-fake` (`go list -deps`), hors `internal/factice/` (lanceur factice et doubles) et hors l'assemblage `cmd/sesame-fake` lui-même ; les règles de décompte ne changent pas (lignes non vides hors commentaires, hors `_test.go`, hors modules épinglés).
- Le contrôle ne peut que compter davantage : tout paquet que lie l'image reste compté.

## Alternatives écartées

- **Lier les décisions dans `cmd/sesame` avant ses adaptateurs** : le binaire de l'image assemblerait du code qu'il ne sert pas, sans adaptateur honnête à brancher.
- **Compter tout `internal/` hors `factice`** : un paquet écrit mais lié par aucun binaire compterait, et un outil lié par aucun binaire passerait pour du code de l'enclave.

## Conséquences et limites

- Le chiffre publié par la CI (`-detail`) est celui du code des décisions dès l'étape 3.
- `internal/factice/` et `cmd/sesame-fake` restent hors budget : ce qui s'y écrit ne doit rien décider (contrôle « Lanceur factice »).

## Documents concernés

`RELEASE.md` §8 ; `CONTRIBUTING.md` (« Règles du code », « Outils et commandes »).
