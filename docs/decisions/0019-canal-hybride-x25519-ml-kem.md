<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0019 · Canal chiffré hybride X25519 et ML-KEM-768 vers l'enclave

**Date** : 2026-10-23 · **Statut** : actée (validée par l'opérateur)

## Contexte et menace

TLS est terminé par le répartiteur de charge, hors de l'enclave. Le canal transporte la clé de transmission à l'enrôlement, les clés ouvertes pour les proches et le protocole de PIN, alors que les enveloppes restent stockées des décennies : un trafic enregistré aujourd'hui ne doit pas pouvoir être déchiffré par un futur calculateur quantique.

## Décision

Toute route, sauf `GET /attestation`, passe par un canal chiffré de bout en bout vers la clé de session attestée de l'instance : X25519 et ML-KEM-768 combinés à la manière de X-Wing (sûr si l'un des deux secrets l'est), une clé par sens, un seul chemin `POST /n/{a|b|c}/canal` dont le nom de route est dans le corps chiffré. Les deux clés publiques de l'instance sont liées à l'attestation par `eat_nonce`.

## Alternatives écartées

- **TLS seul** : terminé hors de l'enclave, il ne protège rien à lui seul.
- **X25519 seul** : exposé à un trafic enregistré puis déchiffré plus tard.

## Conséquences et limites

Le répartiteur ne distingue pas une annulation d'une ouverture par un proche : bloquer l'une bloque l'autre. Les clients ont besoin d'une implémentation de ML-KEM-768, vérifiée par des vecteurs communs.

## Documents concernés

`ARCHITECTURE.md` §3, §5 ; `API.md` §1.
