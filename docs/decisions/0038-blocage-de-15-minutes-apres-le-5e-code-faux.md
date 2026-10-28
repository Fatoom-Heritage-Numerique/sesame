<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0038 · Blocage de 15 minutes pleines après le 5ᵉ code faux

**Date** : 2026-10-28 · **Statut** : actée (validée par l'opérateur) · revoit [0037](0037-essais-des-codes-comptes-au-registre.md)

## Contexte et menace

Les essais faux des codes d'un proche étaient comptés dans une fenêtre de 15 minutes ouverte au premier code faux, et le refus durait jusqu'à la fin de cette fenêtre. Après cinq codes faux saisis en fin de fenêtre, le proche n'attendait donc que quelques secondes, alors que le client annonce au proche, après 5 essais, une saisie bloquée 15 minutes.

## Décision

- Le 5ᵉ code faux d'une fenêtre de 15 minutes pose un blocage de 15 minutes pleines, compté depuis ce code faux, même si la fenêtre prend fin avant. Le code d'accès ou d'invitation et le dernier code ont chacun leur blocage.
- Sésame garde l'heure de fin du blocage au registre, dans le document des essais du proche (`essais/{contactId}` : `codeBloqueJusquau`, `dernierCodeBloqueJusquau`), écrite et signée avec le 5ᵉ code faux, avant la réponse. Pendant le blocage, un essai est refusé sans vérifier le code (`trop_d_essais`, `details.reessayerLe` = fin du blocage). Le 5ᵉ code faux reçoit encore la réponse d'un code faux, avec `essaisRestants: 0`.
- Les codes faux d'une IP suivent la même règle, comptés en mémoire par la VM qui reçoit.
- Le premier code faux après la fin du blocage ouvre une nouvelle fenêtre.

## Alternatives écartées

- **Garder le refus jusqu'à la fin de la fenêtre** : la durée du blocage dépendait du moment des essais, et ne tenait pas l'annonce faite au proche.
- **Garder l'heure du blocage en mémoire** : un redémarrage ou un changement de leader l'aurait effacée, comme les compteurs avant la décision 0037.

## Conséquences et limites

Le rythme des essais reste au plus 5 par 15 minutes (`THREAT_MODEL.md`, limite 22). L'heure du blocage est écrite avec le 5ᵉ code faux : aucune écriture de plus. Qui administre la base du registre peut encore effacer le document des essais d'un proche, blocage compris, écriture qui reste au journal non public des écritures du registre (limite 22).

## Documents concernés

`API.md` §2.14, §2.15, §6, §7 ; `ARCHITECTURE.md` §6.1, §11.2 ; `THREAT_MODEL.md` §2 (22).
