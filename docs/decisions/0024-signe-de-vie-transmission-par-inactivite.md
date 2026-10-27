<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0024 · Signe de vie : transmission par inactivité

**Date** : 2026-10-22 · **Statut** : actée (validée par l'opérateur) · revoit [0003](0003-registre-minimal-tenu-par-sesame.md)

## Contexte et menace

Une option du client transmet aux proches quand le titulaire ne donne plus signe de vie pendant un délai qu'il choisit. Si le client en était seul juge, il pourrait transmettre un titulaire actif, ou un signe de vie pourrait se fabriquer par un lien dans un email.

## Décision

- **Un seul signe de vie : le déverrouillage**, qui envoie à Sésame une preuve de vie signée par la clé de vie, sur un défi. Aucun bouton ni lien ne vaut signe de vie, et le client ne relaie rien.
- Le client tient le réglage, relance et inscrit le signalement `inactivite` ; Sésame mesure les 7 derniers jours et refuse l'inscription si sa dernière preuve de vie (initialisée à l'enrôlement) date de moins de 3 mois et 30 jours, ou pendant un signalement de décès non annulé.
- Une preuve de vie annule un signalement d'inactivité ; après la transmission, la première preuve de vie le clôt et coupe les ouvertures.

## Alternatives écartées

- **Lien ou bouton de confirmation dans un email** : un tiers qui lit la boîte du titulaire ferait taire le signe de vie.

## Conséquences et limites

Sésame ne connaît pas le délai choisi : un client compromis peut avancer une transmission jusqu'au plancher (`THREAT_MODEL.md`, limite 15).

## Documents concernés

`ARCHITECTURE.md` §11.4 ; `API.md` (`vie`, `deces/signaler`) ; `THREAT_MODEL.md` §2 (15).
