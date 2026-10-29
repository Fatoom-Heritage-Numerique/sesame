<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0039 · Ouvertures sans le détail des contenus ouverts

**Date** : 2026-10-28 · **Statut** : actée (validée par l'opérateur) · précise [0003](0003-registre-minimal-tenu-par-sesame.md)

## Contexte et menace

Chaque ouverture inscrite au registre gardait la liste des contenus dont un proche avait ouvert la clé (`{type, id}`, complétée à chaque ouverture de clés), et `/vie` et `/etat` la rendaient à l'app. Le client n'affiche jamais ce détail au titulaire : il lui montre seulement qu'un proche a ouvert ce qui lui est destiné, et quand. Cette liste ajoutait au registre, que la décision 0003 veut minimal, une trace durable de ce que chaque proche a ouvert, sans usage.

## Décision

- Le registre ne garde pas la liste des contenus ouverts : une ouverture ne dit que qui a ouvert (le proche et son rôle) et quand.
- Les éléments d'`ouvertures` des réponses de `/vie` et de `/etat` n'ont pas de champ `contenus` : `contactId`, `role` et `ouvertLe` seulement.
- `/session/ouvrir-cles` n'écrit rien au registre pour les clés qu'il ouvre.
- Le navigateur de la visite reste dans la seule notification d'ouverture au client (`ouvertureNotifiee`), jamais au registre ; la borne du registre ne publie rien des ouvertures.

## Alternatives écartées

Le registre d'origine ne consigne pas d'alternative.

## Conséquences et limites

Le titulaire qui revient après une transmission sait qui a ouvert et quand, jamais quels contenus. Pendant une session, l'enclave sait encore quelles clés elle ouvre, en mémoire seulement : une faille de l'enclave l'exposerait comme le reste de l'état des sessions ouvertes (`THREAT_MODEL.md` §2, limite 1).

## Documents concernés

`ARCHITECTURE.md` §1, §6.1, §11.2 ; `API.md` §2.4, §2.5, §2.19, §5.5.
