<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0035 · Ce qu'expose une faille de l'enclave, écrit sans promesse de plus

**Date** : 2026-10-28 · **Statut** : actée (validée par l'opérateur)

## Contexte et menace

La limite 1 disait qu'une faille de l'enclave n'expose jamais ce que le titulaire garde pour lui. Or la même faille expose les clés OPRF de la grappe du PIN (limite 19 (a)). Combinées à la copie serveur de la clé maître rangée par le client, elles permettent une recherche hors ligne d'un PIN à 6 chiffres et l'ouverture du coffre entier.

## Décision

Le protocole ne change pas. La limite 1 dit ce qui est vrai : une faille de l'enclave seule expose les KTS des sessions ouvertes ; combinée à la copie serveur de la clé maître, elle permet une recherche hors ligne du PIN et l'ouverture du coffre, « gardé pour vous » compris.

## Alternatives écartées

- **Seconde enveloppe de la copie serveur sous une clé KMS que seule l'enclave attestée invoque** : une lecture de la mémoire de l'enclave ne suffirait plus, mais l'opérateur a retenu de ne pas changer le protocole.

## Conséquences et limites

La protection du coffre contre une faille de l'enclave repose sur le secret de la copie serveur chez le client. Le choix peut être revu par une décision nouvelle.

## Documents concernés

`THREAT_MODEL.md` (limites 1 et 19) ; `README.md` (« Les limites, sans détour ») ; `ARCHITECTURE.md` §4.
