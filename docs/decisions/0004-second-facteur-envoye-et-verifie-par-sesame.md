<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0004 · Le dernier code est envoyé et vérifié par Sésame

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur) · **revue depuis** ([0029](0029-dernier-code-par-email-envoye-par-sesame.md))

## Contexte et menace

Le client reçoit les codes d'accès des proches pour pouvoir les envoyer (email de désignation, SMS, envois au décès). Un client compromis connaît donc les codes. Si le second facteur était envoyé et vérifié par lui, il pourrait se faire passer pour un proche, et surtout pour le contact de confiance, qui peut ouvrir du vivant du titulaire.

## Décision

Le **dernier code** est envoyé par **Sésame** (Twilio Verify, appelé depuis l'enclave) vers le numéro ou l'email **fixés par le téléphone du titulaire dans la charte**, et vérifié par Sésame. La charte est chiffrée et authentifiée sous la clé de transmission : le client ne peut ni la lire ni la modifier, donc ni rediriger le code ni le lire. Le code d'accès seul n'ouvre rien.

## Alternatives écartées

- **Second facteur vérifié par le client, qui l'attesterait à Sésame** : un client compromis, qui connaît les codes qu'il envoie, ouvrirait ce qui est destiné au contact de confiance.

## Conséquences et limites

Sésame embarque un client REST pour le fournisseur de SMS et d'email, et ses identifiants dans Secret Manager sous condition d'attestation. Le client conserve malgré tout les codes d'accès (scellés sous sa propre clé KMS) : limite assumée et compensée par ce second facteur (`THREAT_MODEL.md` §2, limite 3). Un fournisseur d'envoi piraté qui lit les deux facteurs d'un même canal reste un risque : décisions 0012 puis 0013.

## Documents concernés

`ARCHITECTURE.md` §11.2 ; `API.md` (`session/ouvrir`, `session/verifier`) ; `THREAT_MODEL.md` §2 (3, 13).
