<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0020 · Annulation explicite d'un signalement de décès, transmission définitive après l'échéance

**Date** : 2026-10-22 (annulation explicite) et 2026-10-26 (transmission définitive) · **Statut** : actée (validée par l'opérateur) · revoit [0003](0003-registre-minimal-tenu-par-sesame.md)

## Contexte et menace

La décision 0003 faisait d'une preuve de vie, envoyée à chaque déverrouillage, l'annulation d'un signalement de décès. Un tiers qui tient le téléphone déverrouillé d'un titulaire décédé annulerait alors sans le vouloir, et un titulaire déclaré mort à tort, qui rouvre son app après l'échéance, couperait des accès déjà ouverts.

## Décision

- Seul le geste « Je suis en vie, annuler la transmission » annule un signalement de décès : un message signé par la clé de vie, lié à ce signalement et à un défi de Sésame, depuis n'importe quel appareil où le titulaire ouvre son coffre. Un déverrouillage n'annule rien.
- Après l'échéance, la transmission sur décès est **définitive** : ni un déverrouillage, ni une preuve de vie, ni une annulation ne coupent les ouvertures. Seule une transmission par inactivité se clôt par une preuve de vie (0024).

## Alternatives écartées

- **Preuve de vie qui annule** (0003) : annulation involontaire, et retour arrière après une transmission faite.

## Conséquences et limites

Un titulaire qui ouvre son app pendant les 7 jours sans toucher ce bouton est transmis (`THREAT_MODEL.md`, limite 7). L'annulation ne transite jamais par le client.

## Documents concernés

`ARCHITECTURE.md` §10.5, §11.3 ; `API.md` (`deces/annuler`, `vie`) ; `THREAT_MODEL.md` §2 (7).
