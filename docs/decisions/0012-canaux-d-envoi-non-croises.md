<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0012 · Canaux d'envoi non croisés : un fournisseur piraté peut ouvrir après un décès vérifié

**Date** : 2026-10-12 · **Statut** : actée comme limite assumée (décision de l'opérateur : « garder tel quel ») · **revue depuis** ([0013](0013-canaux-d-envoi-croises.md)), qui la remplace

## Contexte et menace

Au décès, le code d'accès part par email **et** SMS à chaque proche qui a ces coordonnées, et le dernier code part par l'un des deux. Un attaquant qui lit tout ce qui passe chez l'opérateur SMS, ou chez l'opérateur email, tient donc les deux facteurs d'un proche joint par ce canal ; de même pour le contact de confiance (code et dernier code par SMS). Le risque a été exposé à l'opérateur avec le cadre réglementaire : le client reste responsable de traitement vis-à-vis des utilisateurs, quelle que soit la faute du sous-traitant, dont la responsabilité est plafonnée.

## Décision

Le modèle est **conservé** : code d'accès par tous les canaux numériques renseignés, dernier code par l'un d'eux. Un seul fournisseur d'envoi compromis peut donc ouvrir, après un décès vérifié par Sésame, ce qui est destiné aux proches joints par ce canal. Il ne peut rien avant une transmission vérifiée, rien de ce qui est gardé pour soi, et rien sur un proche « courrier » (identité).

## Alternatives écartées (pour l'instant)

- **Croiser les canaux** : code d'accès par email, dernier code par SMS ; amenderait la règle produit « tous les canaux renseignés ».
- **Un second opérateur SMS**, pour que la compromission d'un seul ne suffise pas.

## Conséquences et limites

C'est la limite 13 de `THREAT_MODEL.md` §2. Raison retenue : simplicité et respect de la règle « personne n'est oublié ». La question est rouverte avant la première mise en service ; une révision sera un nouveau fichier de ce dossier et modifiera `ARCHITECTURE.md` §11.2 et §11.3.

## Documents concernés

`THREAT_MODEL.md` §2 (13) ; `ARCHITECTURE.md` §11.2.
