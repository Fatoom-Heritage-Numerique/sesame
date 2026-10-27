<!-- Source : Fatoom-Heritage-Numerique/Fatoom docs/technique/sesame/ -->

# 0007 · Un dépôt public séparé du client

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur)

## Contexte et menace

La promesse de Sésame est « ne nous croyez pas sur parole : vérifiez ». Elle n'a de sens que si le code qui tourne est lisible par tous, reconstructible à l'identique, et séparé de ce que le client ne publie pas.

## Décision

Sésame vit dans un dépôt GitHub **public** et **séparé** du dépôt du client : `Fatoom-Heritage-Numerique/sesame`. Il contient le code, le `Dockerfile`, le script de build reproductible, le manifeste, les vecteurs de test recopiés depuis le client et la procédure de vérification en 4 étapes. Il ne contient rien de propre au client (identifiants, textes, adresses) : décision 0010. Sésame y est décrit comme un service générique de garde de clés de transmission dont Fatoom est le premier client.

## Alternatives écartées

Le registre d'origine ne consigne pas d'alternative : la séparation découle de la publication (le dépôt du client est privé et contient ses secrets de configuration, ses écrans et ses règles métier).

## Conséquences et limites

Le code public est aussi lisible par un attaquant : la sécurité ne repose sur aucun secret du code, seulement sur l'attestation, KMS, les clés des titulaires et la clé de release. Les vecteurs de test doivent rester identiques des deux côtés : la CI du dépôt échoue s'ils diffèrent. La spécification de référence reste tenue par l'opérateur ; ce dépôt en est la reprise publique et fait foi pour le code.

## Documents concernés

`README.md`, `CONTRIBUTING.md`, `ARCHITECTURE.md` §2.3.
