<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0052 · Une reprise publiée après une panne de plus de 2 heures n'attend pas l'échéance des 30 jours

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau ; change la règle du client sur décision de l'opérateur) · revoit [0042](0042-ecart-de-30-jours-sauf-faille.md) et [0051](0051-arret-de-plus-de-2-heures-et-premiere-version.md)

## Contexte et menace

La règle d'exploitation interdit deux versions de Sésame à moins de 30 jours d'écart, sauf retrait pour une faille (0042). Après un arrêt de toutes les instances de plus de 2 heures, la version suivante est une reprise publiée (0051), et 0051 laissait à l'opérateur la question d'exempter une reprise après une panne. Sans exemption, une panne survenue moins de 30 jours après une version laissait Sésame jusqu'à 30 jours sans ouverture, transmission ni dépôt du PIN, alors que la panne avait déjà effacé la grappe.

## Décision

- **Exception** : une reprise publiée après une panne n'attend pas l'échéance des 30 jours depuis la version précédente. Panne : toutes les instances d'une version en service arrêtées plus de 2 heures, sans retrait d'urgence.
- La reprise reste une version publiée, annoncée comme les autres : ligne de `versions.json`, tag signé, clients publiés avant la mise en service. Elle suit dès que ces clients sont disponibles.
- **Vérifiable par chacun** : le trou dans les bornes commence avant le `retireLe` de la version arrêtée ; après un retrait d'urgence, il commence au `retireLe` ou après.
- **Sans changement** : un retrait hors faille attend toujours l'échéance pour la version suivante, même si une panne survient pendant l'attente (0042, 0049). La règle des 30 jours par titulaire (`grappe_recente`) ne change pas.

## Alternatives écartées

- **Garder l'attente pour toute reprise hors faille** (0051) : jusqu'à 30 jours sans ouverture ni transmission après une panne, sans rien borner de plus, puisque la grappe est déjà effacée.
- **Exempter seulement une panne constatée par l'alerte `grappe_sans_majorite`** : une condition de plus à constater, alors que le trou dans les bornes et `versions.json` disent déjà publiquement si l'arrêt a précédé un retrait.
- **Exempter toute reprise après un arrêt de plus de 2 heures**, retrait hors faille compris : un retrait hors faille suivi de 2 heures d'arrêt contournerait 0042.

## Conséquences et limites

Une panne ne laisse plus Sésame sans ouverture ni transmission au-delà du temps de publier les clients. L'administrateur qui provoque une panne obtient une grappe neuve, ce qu'un effacement total lui donne déjà ; la règle des 30 jours par titulaire borne toujours ce gain à 10 essais contre le même PIN par titulaire et par période de 30 jours (`THREAT_MODEL.md`, limite 19 (f)). Les titulaires entrés dans leur grappe depuis moins de 30 jours reçoivent `grappe_recente` jusqu'à l'échéance, comme après tout effacement (limite 19 (g)).

## Documents concernés

`ARCHITECTURE.md` §6.4, §9.3 et §11.6 ; `RELEASE.md` §4 ; `THREAT_MODEL.md` §2 (8, 19).
