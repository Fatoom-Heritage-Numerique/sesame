<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0028 · Réglages du client : dictionnaire `env_override` exigé en entier, textes longs dans l'image

**Date** : 2026-10-26 · **Statut** : actée (décision de l'opérateur) · revoit [0010](0010-fournisseurs-nommes-dans-le-code.md) · **revue depuis** ([0033](0033-textes-du-client-attestes-par-leur-empreinte.md), [0045](0045-version-de-signature-fixee-par-l-image.md))

## Contexte et menace

La décision 0010 mettait les réglages du client, textes compris, dans des variables attestées. Un essai a montré qu'une condition KMS qui vérifie chaque variable séparément laisse déchiffrer une image lancée avec une variable de plus ; et la condition, bornée à 4 096 caractères, ne peut pas porter des textes longs.

## Décision

- La condition KMS exige l'égalité de **tout** le dictionnaire `env_override`, en une seule clause, dont `envDigest` est l'empreinte publiée.
- Les variables ne gardent que des identifiants et des adresses courts ; les textes (SMS, emails, gabarits des lettres) sont des fichiers de l'image, attestés par son empreinte.

## Alternatives écartées

- **Une clause par variable** : une variable ajoutée passait (vérifié à l'essai).
- **Une clause par variable et la taille du dictionnaire** : deux mécanismes à garder cohérents.

## Conséquences et limites

Changer un texte ou un réglage est une nouvelle version de Sésame.

## Documents concernés

`ARCHITECTURE.md` §2.3, §4 ; `RELEASE.md` §2.
