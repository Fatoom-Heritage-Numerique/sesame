<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0037 · Essais des codes d'un proche comptés au registre

**Date** : 2026-10-28 · **Statut** : actée (validée par l'opérateur) · **revue depuis** ([0038](0038-blocage-de-15-minutes-apres-le-5e-code-faux.md), [0048](0048-tout-code-refuse-decompte.md))

## Contexte et menace

Les limites par proche étaient décomptées par le leader de la grappe, en mémoire, sans réplication : un changement de leader les remettait à zéro. Qui tient le réseau, l'exploitant compris, pouvait donc provoquer des élections et dépasser les 5 essais par 15 minutes du dernier code, limite pourtant spécifiée ; pour le dernier code par email, aucun fournisseur ne garde de limite propre.

## Décision

- Les essais faux des codes d'un proche (code d'accès ou d'invitation qui le désigne, dernier code, vérification d'identité) sont comptés au registre, par proche : `essais/{contactId}` (`essaisCode`, `essaisCodeDepuisLe`, `essaisDernierCode`, `essaisDernierCodeDepuisLe`) pour les codes, comme `essaisIdentite` dans `identites/{contactId}` pour l'identité.
- Chaque essai faux est une écriture signée par `registre-sign`, faite avant la réponse ; un essai faux que le registre n'a pas inscrit reçoit `registre_indisponible`, jamais la réponse d'un code faux.
- Les autres limites de débit restent en mémoire : celles par `uid`, par proche (sessions simultanées) et par compte de service chez le leader, celles par IP, par canal et par session dans la VM qui reçoit.

## Alternatives écartées

- **Écrire le contournement comme une limite assumée** : la limite du dernier code n'aurait pas tenu contre l'exploitant.
- **N'accepter le dernier code par email que dans la session qui l'a envoyé** : cela ne bornait ni le dernier code par SMS ni le code d'accès.

## Conséquences et limites

Une écriture KMS et Firestore de plus par essai faux. Ni un redémarrage des VM ni des élections ne remettent les compteurs à zéro ; qui administre la base du registre peut encore effacer le document des essais d'un proche, écriture faite hors de l'enclave qui reste au journal non public des écritures du registre (`THREAT_MODEL.md`, limite 22).

## Documents concernés

`API.md` §2.14, §2.15, §6, §7 ; `ARCHITECTURE.md` §6.1, §9.3, §11.2 ; `THREAT_MODEL.md` §2 (22).
