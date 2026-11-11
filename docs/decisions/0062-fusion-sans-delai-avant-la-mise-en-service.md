<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0062 · Fusion sans délai tant qu'aucune version n'est en service

**Date** : 2026-11-11 · **Statut** : actée (décision de l'opérateur)

## Contexte et menace

`CONTRIBUTING.md` demandait qu'une PR qui touche l'attestation, le canal, KMS, le registre, le protocole de PIN, les protocoles ou `versions.json` reste publique au moins 7 jours avant sa fusion, tant qu'il n'y a qu'un administrateur, et qu'un changement de format soit d'abord décrit dans une issue. Ce délai tient lieu de second regard : il laisse à chacun le temps de relire un changement avant qu'il puisse servir à un titulaire.

Aucune version de Sésame n'est en service : `versions.json` n'a aucune ligne `en_service`, aucune clé n'est scellée, aucun titulaire n'est enrôlé. Un changement fusionné ne peut donc atteindre personne. Pendant la construction du dépôt, le délai n'apporte pas la protection qu'il vise et multiplie par sept jours chaque étape.

## Décision

- **Avant la première mise en service** (aucune ligne `en_service` dans `versions.json`) : une PR se fusionne dès que sa CI est verte et que l'opérateur l'accepte, sans délai de publicité et sans issue préalable. La décision écrite dans `docs/decisions/` reste exigée pour toute décision prise ou revue : c'est elle qui explique publiquement le choix.
- **À partir de la première mise en service** : la règle de `CONTRIBUTING.md` s'applique en entier, délai de 7 jours compris, et la version mise en service est relue en entier avant d'entrer dans `versions.json` (`RELEASE.md`).

## Alternatives écartées

- **Garder les 7 jours pendant la construction** : aucun titulaire à protéger, et un contrat exécutable découpé en petites PR prendrait des mois d'attente pure.
- **Retirer la règle pour de bon** : l'opérateur de Sésame est aussi celui du client ; une fois en service, le délai public est le second regard qui reste.

## Conséquences et limites

- L'historique du dépôt avant la version 1.0.0 n'a pas été soumis au délai de publicité : ce qui fait foi pour un titulaire est le code de la version mise en service, que chacun reconstruit et relit (`README.md`).
- Le passage d'un régime à l'autre ne dépend pas d'une date mais d'un fait vérifiable : la première ligne `en_service` de `versions.json`.

## Documents concernés

- `CONTRIBUTING.md` : « Formats et vecteurs », « Ce qu'une PR contient » ;
- `ARCHITECTURE.md` : §2.
