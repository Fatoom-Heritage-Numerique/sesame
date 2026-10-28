<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0043 · Après le code de récupération, le nouveau PIN attend la grappe ; l'annulation d'un décès, non

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option la plus simple qui tient la garantie annoncée, sans changement des règles du client) · précise [0017](0017-protocole-de-pin-grappe-de-trois-vm.md)

## Contexte et menace

Le code de récupération est vérifié sur l'appareil, sans Sésame. Mais le client exige ensuite un nouveau PIN, déposé chez Sésame (`nouveau_pin`) avant toute écriture, et ce dépôt passe par la grappe du protocole de PIN. Plusieurs textes disaient pourtant le code de récupération « toujours possible » pendant une pause de la grappe. Un titulaire sans appareil connu ne pouvait donc pas, pendant une pause, rouvrir son coffre sur un nouvel appareil. Surtout, rien ne disait s'il pouvait encore annuler un décès signalé, alors que l'annulation ne passe pas par la grappe.

## Décision

- Le code de récupération ne rouvre le coffre sur un nouvel appareil qu'une fois le nouveau PIN déposé. Pendant une pause de la grappe, le dépôt reçoit `pin_indisponible_temporaire` : l'app propose « Réessayer », rien n'est écrit chez le client, et le code qui vient de servir reste valide.
- L'annulation d'un signalement de décès reste possible dans cet état. Le code de récupération a ouvert la clé maître dans la mémoire de l'appareil, d'où la `cleVie` qui signe l'annulation. `etat` (`idToken` seul) et `deces/annuler` (signature `cleVie`) sont servies par chaque VM joignable, sans la grappe. L'annulation n'écrit rien chez le client ; la clé maître reste en mémoire seulement, jusqu'au dépôt du nouveau PIN ou à l'abandon du parcours.
- Rien ne change chez Sésame : routes, formats et contrôles restent ceux du contrat.

## Alternatives écartées

- **Rouvrir le coffre sans nouveau PIN pendant une pause, et différer le PIN** : un état de plus chez le client (coffre ouvert sans PIN déposé), contraire à la règle du client qui exige le nouveau PIN avant toute écriture.
- **Écrire seulement une limite « pas d'annulation sans la grappe »** : la règle du client promet l'annulation depuis n'importe quel appareil après le code de récupération, et rien chez Sésame ne l'empêche.

## Conséquences et limites

Une pause de la grappe retarde la réouverture d'un coffre par le code de récupération, jamais l'annulation d'un décès. Il en va de même pour tout refus du dépôt du nouveau PIN (`THREAT_MODEL.md`, limite 19).

## Documents concernés

`ARCHITECTURE.md` §11.7 ; `THREAT_MODEL.md` §2 (19).
