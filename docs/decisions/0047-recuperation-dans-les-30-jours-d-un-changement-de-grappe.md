<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0047 · Dans les 30 jours d'un changement de grappe, le code de récupération ne rouvre pas le coffre sur un nouvel appareil ; l'annulation d'un décès reste possible

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · précise [0043](0043-nouveau-pin-apres-recuperation-et-annulation.md)

## Contexte et menace

La décision 0043 fait attendre le dépôt du nouveau PIN pour rouvrir un coffre par le code de récupération. Ce dépôt passe aussi par le contrôle du changement de grappe : un titulaire entré dans sa grappe depuis moins de 30 jours reçoit `grappe_recente` s'il dépose dans une autre grappe, après une nouvelle version ou un effacement total. Plusieurs textes disaient pourtant que, dans cette fenêtre, le code de récupération rouvrait le coffre sur un nouvel appareil. Un titulaire sans appareil connu ne pouvait en fait pas le rouvrir avant l'échéance.

## Décision

- Dans les 30 jours qui suivent l'entrée du titulaire dans sa grappe, après une nouvelle version ou un effacement total, ni le PIN ni le code de récupération ne rouvrent le coffre sur un nouvel appareil. Le code se vérifie sur l'appareil, mais le dépôt `nouveau_pin` qu'il exige reçoit `grappe_recente` jusqu'à l'échéance : l'app propose « Réessayer », rien n'est écrit chez le client, et le code qui vient de servir reste valide.
- Le coffre s'ouvre toujours sur un appareil connu, sans Sésame.
- L'annulation d'un signalement de décès reste possible dans cet état, comme pendant une pause de la grappe (0043) : la clé maître ouverte en mémoire par le code donne la clé de vie qui signe `deces/annuler`, route qui ne passe pas par le contrôle du changement de grappe.
- Rien ne change chez Sésame : routes, formats et contrôles restent ceux du contrat.

## Alternatives écartées

- **Admettre `nouveau_pin` dans une autre grappe avant 30 jours** : il exige la clé maître, donc aucun essai de plus contre l'ancien PIN, mais c'est une exception de plus à la règle des 30 jours, au contrat et dans les contrôles de la grappe.
- **Rouvrir le coffre sans nouveau PIN et différer son dépôt à l'échéance** : un état de plus chez le client, contraire à la règle du client qui exige le nouveau PIN avant toute écriture.

## Conséquences et limites

Un titulaire qui, dans la fenêtre, n'a plus d'appareil connu attend l'échéance pour rouvrir son coffre ; il peut annuler un décès signalé entre-temps. Cas rare, borné par la règle « jamais deux versions de Sésame à moins de 30 jours », sauf retrait pour une faille (`THREAT_MODEL.md`, limite 19 (g)).

## Documents concernés

`ARCHITECTURE.md` §9.3 et §11.7 ; `THREAT_MODEL.md` §2 (19 (g)).
