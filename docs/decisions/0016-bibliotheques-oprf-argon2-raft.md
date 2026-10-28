<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0016 · Bibliothèques admises : `circl` pour l'OPRF, `x/crypto/argon2`, `go.etcd.io/raft/v3`

**Date** : 2026-10-24 (`circl`, Raft) et 2026-10-27 (`x/crypto/argon2`) · **Statut** : actée (décision de l'opérateur) · revoit [0011](0011-go-binaire-statique-distroless.md)

## Contexte et menace

La décision 0011 limitait la cryptographie à la bibliothèque standard de Go. Le protocole de PIN exige un OPRF (RFC 9497) et un consensus entre trois VM (0017) ; l'index des lettres d'invitation exige Argon2id. Aucun des trois n'existe dans la bibliothèque standard, et les écrire soi-même serait la part la plus risquée du code.

## Décision

Trois exceptions, épinglées par version et somme de contrôle : `github.com/cloudflare/circl` pour l'OPRF (suite ristretto255 seule), `golang.org/x/crypto/argon2` (extension officielle de Go) pour l'Argon2id de l'index des lettres d'invitation, `go.etcd.io/raft/v3` pour le consensus de la grappe, avec son seul stockage en mémoire. Aucune autre bibliothèque cryptographique.

## Alternatives écartées

- **Écrire l'OPRF, Argon2id ou Raft soi-même** : plus de code à auditer, sans gain de confiance.
- Le registre d'origine ne consigne pas d'autre bibliothèque comparée.

## Conséquences et limites

Toute nouvelle dépendance se justifie dans une PR et entre dans la liste d'`ARCHITECTURE.md` §2.4 ; la CI refuse une dépendance réseau hors liste. La version de Raft est retenue après un nouvel essai des cas de panne.

## Documents concernés

`ARCHITECTURE.md` §2.4, §9.3 ; `CONTRIBUTING.md`.
