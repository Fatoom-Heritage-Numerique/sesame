<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0048 · Tout code refusé est décompté, code juste compris

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · précise [0037](0037-essais-des-codes-comptes-au-registre.md) et [0038](0038-blocage-de-15-minutes-apres-le-5e-code-faux.md)

## Contexte et menace

`session/ouvrir` répond le même `code_refuse` quel que soit le motif, pour qu'un porteur de code n'apprenne rien de l'état du titulaire. La réponse porte pourtant `details.essaisRestants`, le plus petit des essais restants de l'IP et du proche. Seuls les codes faux étaient décomptés : un code juste refusé pour l'état de l'accès (charte révoquée, proche avant l'échéance, signalement clos, invitation expirée) laissait `essaisRestants` inchangé, ce qui disait au porteur que son code était valable et que l'accès était fermé. Le client, lui, annonce au proche que tout code refusé est décompté.

## Décision

- Pour une IP, tout `code_refuse` est un code faux, que le code désigne un proche ou non.
- Pour un proche, tout code qui le désigne (invitation trouvée, ou charte servie par la passerelle) et que Sésame refuse est un code faux, quel que soit le motif, code juste compris : il est inscrit au document des essais du proche, avec la même fenêtre et le même blocage (0038).
- `essaisRestants` baisse donc de la même façon dans tous les cas : le refus ne distingue jamais un code valable d'un code faux.
- Rien d'autre ne change : routes, champs, réponses et limites restent ceux du contrat.

## Alternatives écartées

- **Ne décompter que les codes faux** : la baisse ou non d'`essaisRestants` révèle la validité du code.
- **Retirer `essaisRestants` de la réponse** : change le contrat et l'écran des essais restants du portail.
- **Décompter un code juste refusé pour l'IP seule** : `essaisRestants`, plus petit reste de l'IP et du proche, ne baisse alors pas quand le proche a moins d'essais restants que l'IP.

## Conséquences et limites

Un proche qui saisit son code juste alors que son accès est fermé se bloque 15 minutes au 5ᵉ essai, comme avec un code faux ; son accès étant fermé, il n'y perd rien. Chaque code juste refusé coûte une écriture signée au registre, comme un code faux. Les essais d'une IP sont comptés en mémoire par la VM qui reçoit : une IP qui s'adresse aux trois VM a jusqu'à trois fois ces essais ; la protection tient au code lui-même et à l'Argon2id de son index (`THREAT_MODEL.md`, limite 22).

## Documents concernés

`API.md` §2.14 ; `ARCHITECTURE.md` §6.1.
