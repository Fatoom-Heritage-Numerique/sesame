<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0046 · Nouvel envoi d'une lettre au passage de la tâche qui relève l'erreur

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option la plus simple, sans champ nouveau) · précise [0027](0027-lettre-et-identite-precisees.md)

## Contexte et menace

Une lettre en `erreur` chez le prestataire d'impression devait repartir « le lendemain ». Aucun champ de `lettres/{contactId}` ne date l'erreur ni la prochaine tentative : la tâche horaire ne pouvait pas savoir quand reprendre.

## Décision

- Le passage de la tâche horaire qui relève une `erreur` incrémente `erreurs` et, à la 1ʳᵉ et à la 2ᵉ erreur, réserve aussitôt le nouvel envoi (nouvelle lettre, même code), comme tout envoi. À la 3ᵉ, plus aucun envoi : `lettreEchouee`.

## Alternatives écartées

- **Un champ `prochainEnvoiLe`** : un champ signé de plus pour un délai sans effet sur la sécurité ; le plafond de trois erreurs borne déjà les envois.

## Conséquences et limites

Une erreur due à une adresse fausse se répète au plus deux fois, à une heure d'écart, avant `lettreEchouee` ; le support la traite comme avant.

## Documents concernés

`ARCHITECTURE.md` §11.9.
