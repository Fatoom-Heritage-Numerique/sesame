<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0026 · Clés publiques de `session-sign` dans la liste des versions

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur) · revoit [0014](0014-liste-des-versions-compilee.md) · **revue depuis** ([0044](0044-requetes-a-la-passerelle-sans-kid.md), [0045](0045-version-de-signature-fixee-par-l-image.md))

## Contexte et menace

Les clients vérifient les jetons de session, les reçus, les réponses de `etat` et les requêtes à la passerelle signés par la clé KMS `session-sign`. Une clé publique téléchargée au démarrage, ou épinglée à part, serait une seconde ancre de confiance, hors de la liste des versions.

## Décision

`versions.json` porte `clesSession` : par `kid` (`session-sign/<version KMS>`), la clé publique (`cle`) et sa date (`depuis`). Les clients la lisent dans la liste compilée ; une rotation de la clé ajoute une entrée et passe par une nouvelle version des clients.

## Alternatives écartées

- **Clé publique téléchargée** : un client trompé sur la source accepterait une autre clé.
- **Clé épinglée à part de la liste** : deux listes à garder cohérentes.

## Conséquences et limites

Une rotation de `session-sign` attend la publication des clients qui contiennent la nouvelle entrée.

## Documents concernés

`RELEASE.md` §2 ; `ARCHITECTURE.md` §4, §11.6 ; `API.md` §1.6, §5.1.
