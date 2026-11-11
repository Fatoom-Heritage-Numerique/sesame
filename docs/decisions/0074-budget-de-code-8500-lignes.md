<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0074 · Budget de code porté à 8 500 lignes de Go

**Date** : 2026-11-11 · **Statut** : actée (décision de l'opérateur) · revoit [0015](0015-budget-de-code-6000-lignes.md)

## Contexte et menace

Le budget de code existe pour qu'un auditeur puisse relire en entier le code qui ouvre les clés : au-delà d'une certaine taille, personne ne relit plus tout, et un défaut se cache dans la masse. La décision 0015 fixait un plafond dur de 6 000 lignes de Go, sur une estimation ; elle demandait de remesurer le chiffre sur le Sésame factice, puis sur l'image.

Un plafond trop bas a sa propre menace : pour le tenir, on est tenté de sortir du code du binaire attesté (le mettre dans le client, dans un service à côté, ou dans une bibliothèque tierce que personne ne compte), ou de retirer des contrôles (fraîcheur des lignes, anti-retour, refus typés). Les deux affaiblissent ce que le budget devait protéger.

## Mesure qui la fonde

`go run ./outils/decompte-lignes -detail`, sur la branche de la grappe du PIN (règles de comptage de la décision 0069 : paquets de ce module que lient `cmd/sesame` et `cmd/sesame-fake`, hors `internal/factice/` et hors l'assemblage `cmd/sesame-fake`, hors `_test.go`, lignes non vides hors commentaires) :

| Paquet | Lignes |
|---|---|
| `internal/routes` | 2 857 |
| `internal/grappe` | 1 076 |
| `internal/registre` | 608 |
| `internal/jcs` | 364 |
| `internal/canal` | 347 |
| `internal/attestation` | 340 |
| `internal/cles` | 208 |
| `internal/primitives` | 205 |
| `internal/jetons` | 135 |
| `internal/vie` | 133 |
| `internal/passerelle` | 132 |
| `internal/ports` | 100 |
| `internal/sessions` | 94 |
| `internal/pin` | 84 |
| `internal/lettres` | 83 |
| `cmd/sesame` | 9 |
| **Total** | **6 775** |

Ce code des décisions est complet pour toutes les routes d'`API.md` et la grappe ; il ne contient aucun adaptateur réel. Restent à écrire, aux étapes 3 et 4 (`README.md`, « État du projet »), les adaptateurs de l'image : Confidential Space (socket du lanceur), KMS, Firestore, Cloud Storage, Secret Manager, Cloud Logging, STS et IAM Credentials, TLS mutuel entre VM, clients REST de Twilio, SendGrid, Stripe et Pingen, et l'assemblage de `cmd/sesame`. **Estimation, non mesurée** : de l'ordre de 1 000 à 1 450 lignes, soit 7 775 à 8 225 lignes en tout.

## Décision

- **Plafond dur de 8 500 lignes** de Go hors tests et hors bibliothèques épinglées, contrôlé par la CI (`outils/decompte-lignes`), avec les règles de comptage de la décision 0069, inchangées.
- Le plafond laisse environ 275 à 725 lignes au-delà de l'estimation des adaptateurs : une marge pour l'écart d'une estimation, pas une réserve à remplir. Une PR qui le dépasse retire autant qu'elle ajoute (`CONTRIBUTING.md`), comme avant.
- Le chiffre est remesuré à chaque PR (le job « Budget de code » imprime le détail par paquet), et une dernière fois sur l'image de la version 1.0.0.
- Tant qu'aucune version n'est en service, cette décision se fusionne sans délai (décision 0062).

## Alternatives écartées

- **Simplifier le contrat de la grappe** (moins de contrôles à l'admission, au relais ou au remplacement d'un membre) pour revenir sous 6 000 : chacun de ces contrôles répond à une menace écrite (`ARCHITECTURE.md` §9.3, décisions 0050, 0055 à 0057) ; les retirer pour un chiffre affaiblirait ce que le budget doit rendre relisible.
- **Sortir les adaptateurs du décompte** : ils sont dans le binaire attesté et voient les clés en clair (KMS, canal) ou les codes (fournisseurs) ; un auditeur doit les lire autant que les décisions. Le décompte ne peut que compter davantage (décision 0069).
- **Garder 6 000 et laisser la CI rouge** jusqu'à l'image : un contrôle toujours rouge n'est plus lu.

## Conséquences et limites

- 8 500 lignes restent lisibles par une personne en quelques jours, mais c'est plus que les 6 000 annoncées jusque-là : l'audit coûte plus.
- L'estimation des adaptateurs peut se révéler fausse ; un dépassement se tranche alors par une nouvelle décision, sur une nouvelle mesure, jamais en sortant du code du décompte.

## Documents concernés

`ARCHITECTURE.md` §2.2 ; `CONTRIBUTING.md` (« Règles du code », « Outils et commandes ») ; `RELEASE.md` §8 ; `README.md` (« État du projet ») ; `outils/decompte-lignes`.
