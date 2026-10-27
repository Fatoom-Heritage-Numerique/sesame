<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0022 · Export horaire, toute restauration par une reprise publiée, ré-enrôlement d'un coffre sans ligne

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur) · revoit [0003](0003-registre-minimal-tenu-par-sesame.md)

## Contexte et menace

La décision 0003 prévoyait une sauvegarde chaque nuit et un ré-enrôlement des coffres après une perte. Une restauration silencieuse ferait oublier des faits à Sésame ; un ré-enrôlement sans preuve laisserait le client sceller une clé de son choix.

## Décision

- Le registre est exporté **chaque heure**, à un instant donné. **Toute** restauration passe par une **reprise publiée** : une nouvelle version de Sésame dont l'image porte l'époque et l'empreinte de reprise, qui vérifie la sauvegarde, réécrit chaque ligne, recrée les marqueurs d'inscription et publie `reprise` dans sa borne.
- Avec ou sans sauvegarde, un coffre dont la ligne manque (aucune sauvegarde, ou coffre enrôlé après le dernier export) se ré-enrôle pendant la reprise, par une rotation dont la preuve se vérifie contre la `cleViePub` que fournit le client (`cleViePubAttendue`), avec un nouveau code et une génération de charte supérieure pour chaque proche. La reprise dure tant que sa version est en service et finit à la version suivante (`reprise_absente`).

## Alternatives écartées

- **Restauration sans nouvelle version** : rejetée ligne par ligne par le contrôle de fraîcheur.
- **Ré-enrôlement sans preuve** : le client pourrait sceller une clé de son choix.

## Conséquences et limites

Le client pourrait substituer une autre `cleViePub` pendant la reprise : limite assumée (`THREAT_MODEL.md`, limite 54) ; le client alerte le titulaire du ré-enrôlement. Les écritures postérieures à la sauvegarde sont perdues (limite 11).

## Documents concernés

`ARCHITECTURE.md` §6.4, §11.8 ; `API.md` (`enroler`, `vie`, `etat`) ; `THREAT_MODEL.md` §2 (11, 54).
