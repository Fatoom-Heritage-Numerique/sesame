<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0023 · Alertes envoyées par Sésame lui-même, fiche d'alerte scellée par Sésame

**Date** : 2026-10-24 · **Statut** : actée (validée par l'opérateur) · revoit [0006](0006-verification-en-direct-et-par-cache.md) et [0010](0010-fournisseurs-nommes-dans-le-code.md)

## Contexte et menace

Si toutes les alertes au titulaire passaient par le client, la seule voie indépendante serait la réponse à la preuve de vie, qui n'arrive qu'au déverrouillage. Un client compromis, ou une erreur honnête, pourrait laisser passer un faux décès chez un titulaire qui n'ouvre pas l'app pendant 7 jours, et cacher les ouvertures du contact de confiance ou les essais de son PIN.

## Décision

- Le titulaire envoie à Sésame une **fiche d'alerte** (email, téléphone facultatif) signée par sa clé de vie ; Sésame la scelle sous la clé KMS `alerte-seal` et la garde dans son registre. Le client n'en garde rien.
- Sésame alerte lui-même le titulaire, par ses propres comptes Twilio Messaging et SendGrid, par email et, si la fiche porte un téléphone, par SMS : signalement inscrit (puis 3 et 6 jours après), ouverture par le contact de confiance de son vivant, chaque essai de PIN (au plus un message par heure), ligne rejetée.
- Une alerte refusée est réessayée 24 h, puis publiée dans la borne (`alerte_refusee`) ; l'échéance d'un signalement ne bouge pas.
- L'app vérifie aussi chaque jour, coffre verrouillé, l'état que Sésame signe (`etat`).

## Alternatives écartées

- **Fiche gardée par le client** : il pourrait empêcher précisément l'alerte d'un faux décès.
- **Repousser l'échéance quand une alerte échoue** : l'administrateur pourrait retenir une transmission due.

## Conséquences et limites

Un administrateur peut couper la sortie de Sésame vers ses fournisseurs ou suspendre ses comptes : les alertes de Sésame se taisent, pas celles du client ni les réponses de `vie` et `etat` (`THREAT_MODEL.md`, limite 8).

## Documents concernés

`ARCHITECTURE.md` §11.10 ; `API.md` (`alerte/fiche`, `etat`) ; `THREAT_MODEL.md` §2 (8).
