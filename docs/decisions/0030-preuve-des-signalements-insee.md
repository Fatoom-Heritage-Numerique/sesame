<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0030 · Preuve des signalements de la source `insee`

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur)

## Contexte et menace

Un signalement de la source `acte` porte l'empreinte du fichier de l'acte, inscrite au registre : une fausse inscription laisse une pièce à comparer. Un signalement de la source `insee` ne portait rien : un client compromis pouvait inscrire un décès inventé sans laisser de trace vérifiable.

## Décision

Un signalement `insee` porte l'empreinte `SHA-256` de la ligne du fichier mensuel des décès qui désigne le titulaire (octets de la ligne, sans sa fin de ligne) et le nom de ce fichier. Sésame les inscrit au registre (`empreintePreuve`, `preuveNom` ; `empreintePreuve` vaut aussi pour l'acte) et les cite dans ses alertes au titulaire. Chacun peut recalculer l'empreinte sur le fichier publié.

## Alternatives écartées

- **Retirer la promesse** « pas de décès inscrit sans INSEE ni acte » et l'écrire comme limite assumée : la source `insee` serait restée sans trace.

## Conséquences et limites

Sésame ne vérifie toujours pas le décès lui-même : il mesure les 7 jours et exige l'absence d'annulation (0003, 0020). Une ligne inventée ne correspond à aucune ligne du fichier publié (`THREAT_MODEL.md`, limite 18).

## Documents concernés

`API.md` (`deces/signaler`) ; `ARCHITECTURE.md` §6.1, §11.3, §11.10 ; `THREAT_MODEL.md` §2 (18).
