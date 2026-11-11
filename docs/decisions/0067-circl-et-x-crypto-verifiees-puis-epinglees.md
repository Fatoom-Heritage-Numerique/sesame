<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0067 · `circl` v1.6.5 et `golang.org/x/crypto` v0.58.0 vérifiées sur leurs vecteurs, puis épinglées

**Date** : 2026-11-11 · **Statut** : actée (choix technique délégué par l'opérateur) · applique [0016](0016-bibliotheques-oprf-argon2-raft.md)

## Contexte et menace

La décision 0016 admet deux exceptions cryptographiques à la bibliothèque standard : `github.com/cloudflare/circl` pour l'OPRF du protocole de PIN (RFC 9497, suite `ristretto255-SHA512`) et `golang.org/x/crypto/argon2` pour l'Argon2id de l'index des lettres d'invitation (`ARCHITECTURE.md` §2.4, §9.1, §10.4). Une version fautive ou altérée de l'une d'elles évaluerait le PIN ou étirerait le code autrement que les clients : un titulaire ne rouvrirait plus son coffre, ou une lettre ne retrouverait plus son proche. `ARCHITECTURE.md` §2.4 exige de vérifier `circl` sur les vecteurs de l'annexe A.1.1 de la RFC 9497 et sur le vecteur `depotPin` avant de l'épingler.

## Décision

- **`github.com/cloudflare/circl` v1.6.5**, la dernière version publiée sur `proxy.golang.org` (`https://proxy.golang.org/github.com/cloudflare/circl/@latest` : v1.6.5, commit `cfa7c70defd831ffb0792ab2af560bfef43d60ca`, étiquette `v1.6.5`), paquet `oprf` seul, mode OPRF, suite `SuiteRistretto255`. Vérifiée par le test `internal/pin` :
  - annexe A.1.1 de la RFC 9497 (texte de `https://www.rfc-editor.org/rfc/rfc9497.txt`, valeurs recopiées dans le test) : `DeriveKey(Seed, KeyInfo)` rend `skSm` ; pour les deux vecteurs, l'aveuglement déterministe par `Blind` rend `BlindedElement`, l'évaluation de Sésame (`pin.Evaluate`) rend `EvaluationElement`, et `Finalize` rend `Output` ;
  - vecteur `depotPin` du client de référence (`testdata/vecteurs/sesame.json`) : `pin.Evaluate(cleOprf, elementAveugle)` rend `evaluation` pour les trois motifs ; un élément identité ou hors forme est refusé.
- **`golang.org/x/crypto` v0.58.0**, la dernière version publiée (`https://proxy.golang.org/golang.org/x/crypto/@latest` : v0.58.0, commit `8fdf07b3114278fb3b78c1cf1814e08bd851eb4c`), paquet `argon2` seul. Vérifiée par les vecteurs `argon2id` de `primitives.json` (deux calculs et cinq refus de paramètres, que `primitives.Argon2id` fait avant d'appeler la bibliothèque, qui sinon ajusterait ou paniquerait) et `indexAcces` de `sesame.json` (m = 65 536 Kio, t = 3, p = 1, sel `fatoom-lookup-v1`).
- **Modules requis**, épinglés avec elles par `go.sum`, tous nommés dans `ARCHITECTURE.md` §2.4 : `github.com/bwesterb/go-ristretto` v1.2.4 (requis par `circl`) et `golang.org/x/sys` v0.49.0 (requis par `golang.org/x/crypto` v0.58.0, version que retient la sélection de Go ; v0.50.0 existe mais n'est requise par aucun module).
- Une version suivante de l'une de ces bibliothèques n'entre que par une nouvelle décision, après les mêmes vérifications.

## Alternatives écartées

- **Se fier aux tests de `circl`**, qui portent les vecteurs de la RFC : ils vérifient la bibliothèque, pas l'usage qu'en fait Sésame (clé lue en 32 octets, élément reçu, sortie compressée).
- **Recalculer en Go l'aveuglement du PIN depuis le tirage du vecteur** : c'est une opération du client seul, et sa réduction du tirage en scalaire est propre à sa bibliothèque ; Sésame ne reçoit que l'élément aveuglé.

## Conséquences et limites

- `cmd/sesame` ne lie pas encore ces paquets ; le contrôle des dépendances (`outils/controle-dependances`) les couvrira dès que le code des routes les liera.
- Ces vérifications portent sur le comportement, pas sur l'absence de porte dérobée : `go.sum` garantit seulement que chacun construit avec les mêmes octets.

## Documents concernés

`ARCHITECTURE.md` §2.4 ; `go.mod`, `go.sum`.
