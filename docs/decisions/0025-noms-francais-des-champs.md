<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0025 · Noms français des champs des formats publiés

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur) · revoit [0014](0014-liste-des-versions-compilee.md)

## Contexte et menace

Les formats partagés avec les clients mêlaient des noms anglais et français. Aucun format n'étant encore en service, les noms pouvaient changer sans rétrocompatibilité.

## Décision

Les champs des formats publiés sont nommés en français : ceux de `versions.json` (`publieLe`, `statut`, `retireLe`, `referenceImage`, `notes`), ceux de la charte d'un proche (`telephone`, `creeLe`) et la date d'enrôlement renvoyée par `/enroler` (`enroleLe`). Restent en anglais les noms déjà fixés par le contrat dans les données du client (`sealedKts`, `ktsGeneration`, `cleViePub`, `imageDigest`).

## Alternatives écartées

Le registre d'origine ne consigne pas d'alternative.

## Conséquences et limites

Les vecteurs de test et les schémas sont régénérés ; aucun format n'est versionné tant qu'aucune version n'est en service (`CONTRIBUTING.md`).

## Documents concernés

`ARCHITECTURE.md` §10.4, §11.6 ; `RELEASE.md` §2 ; `API.md` (`enroler`).
