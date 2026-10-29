<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0056 · Ligne lue avant la signature `cleVie` ; `grappePin` non défait ; rythme des admissions et des alertes

**Date** : 2026-10-29 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau)

## Contexte et menace

L'ordre des contrôles commun (`API.md` §6) vérifiait la signature `cleVie` avant de lire la ligne du registre, alors que seule cette ligne porte la `cleViePub` qui la vérifie : une ligne absente aurait reçu `message_refuse` au lieu de `titulaire_indisponible`, et deux implémentations conformes auraient répondu différemment. Quatre comportements n'étaient pas fixés : la portée de `grappeId: null` reçu de `/defi`, l'état laissé par une écriture de `grappePin` que la majorité ne valide pas ensuite, le rythme des demandes d'entrée d'une VM sans grappe et de l'alerte `grappe_admission_refusee` qui en découle, et l'heure portée par l'alerte d'essai de PIN regroupée. L'alerte J0 d'un signalement n'était ni inscrite ni protégée contre un signalement renvoyé.

## Décision

- **Ordre des contrôles** : pour une route signée par `cleVie`, la ligne du registre est lue avant la signature, puis la limite par `uid` est décomptée, comme pour `/enroler`. Le défi passe avant la signature.
- **`grappeId: null`** ne fait reprendre à `GET /attestation` qu'avant un dépôt du PIN ; avant `/vie`, `/deces/annuler` ou `/enroler`, servies par toute VM, l'app n'en tient pas compte.
- **`grappePin` écrit, majorité en échec** : rien n'est défait. Le dépôt retenté dans la grappe inscrite est admis ; un dépôt dans une autre grappe attend l'échéance des 30 jours (`THREAT_MODEL.md`, limite 19 (g)).
- **VM sans grappe** : elle renouvelle sa demande d'entrée toutes les 30 secondes ; `grappe_admission_refusee` est journalisé au plus une fois par heure par VM qui refuse.
- **Alerte d'essai de PIN** : envoyée au premier essai, avec sa date et son heure ; les essais de l'heure qui suit y sont regroupés sans message de plus, et leur nombre n'est pas donné.
- **Alerte J0** : inscrite dans `alertesEnvoyeesLe[]` par l'écriture qui crée le signalement ; un `/deces/signaler` renvoyé, qui rend le signalement existant, ne la renvoie pas.

## Alternatives écartées

- **Défaire `grappePin`** après un échec de la majorité : une écriture de compensation de plus, qui peut elle-même échouer, pour un cas déjà borné par la règle des 30 jours.
- **Compter les essais dans l'alerte de PIN**, ou l'envoyer à la fin de l'heure : un minuteur et un texte attesté de plus, alors qu'un seul essai inconnu appelle déjà un changement de PIN.
- **Exclure le passage de version de l'alerte d'admission** : Sésame devrait distinguer un déploiement d'une tentative ; la limite horaire suffit.

Pratiques reprises : l'ordre « identifier la clé, puis vérifier la signature » de toute vérification par clé publique désignée (JWS, RFC 7515 §5.2, la clé étant choisie avant la vérification) ; le plafond de fréquence des notifications des alertes fondées sur les journaux de Cloud Monitoring (`notificationRateLimit`).

## Conséquences et limites

Un titulaire dont le dépôt échoue pendant une pause, juste après l'écriture de sa nouvelle grappe, ne peut changer de grappe avant 30 jours ; il le peut dans celle-ci dès son retour. Un exploitant reçoit au plus un email par heure et par VM pendant un passage de version ou avant un amorçage.

## Documents concernés

`API.md` §2.2, §2.4, §2.6, §2.7, §2.9, §6 et §7 ; `ARCHITECTURE.md` §9.2, §9.3 et §11.10 ; `THREAT_MODEL.md`, limite 19 (g).
