<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0017 · Protocole de PIN en mémoire, sur une grappe de trois VM à majorité, et règle des 30 jours

**Date** : 2026-10-24 · **Statut** : actée (validée par l'opérateur) · revoit [0006](0006-verification-en-direct-et-par-cache.md) · **revue depuis** ([0018](0018-amorcage-commande-d-une-grappe-neuve.md), [0042](0042-ecart-de-30-jours-sauf-faille.md), [0043](0043-nouveau-pin-apres-recuperation-et-annulation.md), [0050](0050-remplacement-d-un-membre-sans-quatrieme-votant.md))

## Contexte et menace

Sur un nouvel appareil, le titulaire rouvre la copie serveur de sa clé maître avec son PIN. Si le secret qui vérifie le PIN était scellé sous une clé KMS du projet de Sésame, l'administrateur de ce projet pourrait autoriser une autre image, desceller ce secret et chercher hors ligne chaque PIN de 6 chiffres. Si une VM seule pouvait accepter une jumelle, il pourrait couper le réseau et obtenir deux copies de l'état, chacune avec 10 essais par titulaire.

## Décision

- Tout ce qui vérifie un PIN (clé OPRF, essais restants, délai) vit **uniquement en mémoire** des VM de Sésame, répliqué, jamais écrit ni scellé, jamais transmis à une autre version.
- Trois VM, une par zone, forment une grappe Raft qui ne répond à un dépôt ni à une évaluation qu'après l'accord de la majorité ; un membre n'entre ou ne sort que par la majorité.
- Chaque dépôt est signé par la clé de vie du titulaire ; un essai est décompté avant l'évaluation, 10 essais par code PIN, compteur conservé au redépôt du même PIN.
- Un titulaire ne change de grappe qu'au plus une fois tous les 30 jours (`grappePin` au registre), et jamais deux versions de Sésame ne se suivent à moins de 30 jours, sauf retrait d'urgence.

## Alternatives écartées

- **Secret du PIN scellé sous KMS** : une image autorisée en douce le descellerait.
- **Deux VM qui se copient l'état** : une coupure du réseau suffisait à dupliquer les essais.
- **Séquestre de clés chez Apple ou Google** : écarté, la grappe est maintenue.
- **OPAQUE** : il ne remplace pas le compteur d'essais tenu en mémoire.

## Conséquences et limites

Deux VM perdues en même temps effacent tous les enregistrements du PIN : le titulaire rouvre sur un appareil connu ou par son code de récupération, puis redépose (`THREAT_MODEL.md`, limite 19). Une nouvelle grappe rend au plus 10 essais de plus par titulaire et par période de 30 jours, ce qui se voit dans les bornes du registre (0021).

## Documents concernés

`ARCHITECTURE.md` §9 ; `API.md` (`pin/deposer`, `pin/evaluer`, §4) ; `THREAT_MODEL.md` §2 (19).
