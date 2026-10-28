<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0011 · Go, binaire statique, image distroless, scripts gcloud ; ordre de construction

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur) · Résout une question ouverte du client · **revue depuis** ([0015](0015-budget-de-code-6000-lignes.md), [0016](0016-bibliotheques-oprf-argon2-raft.md))

## Contexte et menace

Sésame doit être reproductible octet pour octet, petit (moins de 5 000 lignes hors tests) et auditable par des tiers. Le langage et la base d'image décident de la taille de la surface à auditer et de la facilité à obtenir une image identique sur deux machines.

## Décision

- **Go**, dernière version stable épinglée dans `go.mod` et dans la CI, module unique, **binaire statique** (`-trimpath`, `SOURCE_DATE_EPOCH`), cryptographie de la bibliothèque standard, SDK Google officiels (KMS, Firestore, Secret Manager), `net/http` brut pour les fournisseurs.
- Image de base **`gcr.io/distroless/static-debian12`** épinglée par digest, `Dockerfile` multi-étapes, conteneur de build épinglé.
- Infrastructure par **scripts `gcloud` versionnés** plutôt que Terraform.
- **Ordre de construction** : documentation du dépôt public ; contrat exécutable (OpenAPI, vecteurs, `sesame-fake`) qui débloque le client, l'app et le portail ; cœur ; fournisseurs ; infrastructure ; version 1.0.0.

## Alternatives écartées

- **TypeScript / Node** : image reproductible difficile, surface npm.
- **Terraform** : au profit de scripts `gcloud` lisibles et sans état distant, pour une infrastructure de quelques ressources.

## Conséquences et limites

La reproductibilité est vérifiée par double construction en CI (`RELEASE.md` §1.2). Le budget de 5 000 lignes est un contrôle CI. Les contributions suivent l'ordre de construction (`CONTRIBUTING.md`).

## Documents concernés

`ARCHITECTURE.md` §2.2, §2.4 ; `RELEASE.md` §1 ; `CONTRIBUTING.md` ; `README.md` (« État du projet »).
