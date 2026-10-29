<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0033 · Textes du client réglés au lancement, attestés par leur empreinte

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur) · revoit [0028](0028-reglages-du-client-env-override-entier.md) et [0029](0029-dernier-code-par-email-envoye-par-sesame.md) · **revue depuis** ([0058](0058-aucun-refus-au-demarrage-ne-fait-sortir-le-processus.md))

## Contexte et menace

La décision 0028 mettait les textes (SMS, emails, gabarits des lettres) dans des fichiers de l'image. Or l'image se construit depuis ce dépôt, qui ne contient aucun texte de marque : les textes du premier client y seraient entrés. La condition KMS, bornée à 4 096 caractères et qui exige tout le dictionnaire `env_override`, ne peut pas non plus les porter comme variables.

## Décision

- Les textes forment un seul document en JSON canonique, réglé par le client, hors de l'image.
- La variable attestée `SESAME_TEXTES` porte son empreinte, `"sha256:"` ‖ hex(SHA-256) ; elle entre dans `env_override`, donc dans la condition KMS et dans l'`envDigest` publié.
- Le document est rangé dans Secret Manager du projet de Sésame et lu au démarrage par la seule charge attestée ; Sésame refuse de démarrer si son empreinte diffère ou si un texte attendu manque.
- Le dernier code par email (0029) et les alertes partent avec ces textes attestés.

## Alternatives écartées

- **Fichiers de l'image** (0028) : ce dépôt contiendrait les textes du client.
- **Textes en variables** : la condition KMS dépasserait 4 096 caractères.

## Conséquences et limites

Changer un texte reste une nouvelle version de Sésame. Une nouvelle version du secret sans nouvelle version de Sésame ne fait qu'empêcher le démarrage, et le script public la signale. Ce dépôt ne publie pas le contenu des textes.

## Documents concernés

`ARCHITECTURE.md` §2.3, §4, §8.3, §11.9, §11.10 ; `CONTRIBUTING.md`.
