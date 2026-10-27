<!-- Source : Fatoom-Heritage-Numerique/Fatoom docs/technique/sesame/ -->

# 0010 · Fournisseurs nommés dans le code, réglages du client en variables attestées, clés d'API dans Secret Manager

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur) · **revue depuis** : réglages du client en `env_override` dont tout le dictionnaire est exigé par la condition KMS, textes longs dans des fichiers de l'image ; plus de délai de 7 jours ([0014](0014-liste-des-versions-compilee.md))

## Contexte et menace

Sésame appelle des fournisseurs externes (SMS et email, identité, courrier). Si l'adresse d'un fournisseur se lisait dans une variable au démarrage, qui contrôle la VM pourrait la remplacer par la sienne et recevoir le dernier code à la place du proche, sans que l'empreinte de l'image change. À l'inverse, le dépôt public ne doit contenir ni identifiant de compte ni texte de marque du client.

## Décision

- Les fournisseurs sont **nommés dans le code public** (Twilio Verify, Stripe Identity, Pingen), leurs adresses de base **écrites en dur**, jamais lues au démarrage.
- Ce qui est propre au client (identifiant du service de vérification, identifiant d'organisation du courrier, nom et adresse de l'expéditeur, textes du SMS et gabarit de la lettre, adresse de la passerelle, émetteur des jetons d'identité, compte de service autorisé) va dans des **variables d'environnement attestées** (`tee-env-*`, présentes dans `submods.container.env`) ; Sésame refuse de démarrer si l'une manque, et le manifeste publie `envDigest`, le `SHA-256` de l'ensemble, que les clients comparent au jeton.
- Les **clés d'API** sont dans Secret Manager du projet Sésame, sous la même condition d'attestation que KMS.

## Alternatives écartées

- **Adresses des fournisseurs en variables d'environnement** : détournables par qui contrôle la VM.
- **Adaptateurs génériques avec un fichier de réglages scellé** : plus de code, et les fournisseurs seraient devinables malgré tout.

## Conséquences et limites

Changer de fournisseur est une nouvelle version de Sésame, publiée 7 jours avant. Un texte de lettre ou un expéditeur ne se change pas en douce : `envDigest` changerait. Le dépôt reste indépendant du client : un autre client fournit ses propres variables.

## Documents concernés

`ARCHITECTURE.md` §2.2 ; `RELEASE.md` §2 (`envDigest`) ; `CONTRIBUTING.md`.
