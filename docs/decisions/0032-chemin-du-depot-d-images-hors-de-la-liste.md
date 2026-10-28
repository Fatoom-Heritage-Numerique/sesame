<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0032 · Chemin du dépôt d'images hors de la liste des versions

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur) · revoit [0014](0014-liste-des-versions-compilee.md) et [0025](0025-noms-francais-des-champs.md)

## Contexte et menace

`versions.json` vit dans ce dépôt, qui ne cite aucun nom de projet, de bucket ni de compte. Le champ `referenceImage` des décisions 0014 et 0025 portait la référence complète de l'image, donc le chemin du dépôt d'images de l'exploitant, qui contient le nom de son projet : la première ligne publiée l'aurait écrit ici.

## Décision

- `versions.json` désigne l'image par `imageDigest` seul : `referenceImage` est retiré. Une version porte `version`, `imageDigest`, `envDigest`, `hwmodels`, `tag`, `commit`, `publieLe`, `statut`, `retireLe` et `notes` ; la liste porte aussi `clesSession`.
- Le chemin du dépôt d'images, que `submods.container.image_reference` doit préfixer, est une constante compilée dans les clients.
- Les scripts publics reçoivent ce chemin en argument, avec le numéro du projet et le nom du bucket d'audit (`<registre-images>`, `<numero-projet-sesame>`, `<bucket-audit>` dans ce dépôt) ; le client publie leurs valeurs sur sa page de transparence.

## Alternatives écartées

- **Admettre le chemin dans `versions.json`** : ce dépôt citerait le nom du projet de l'exploitant, contre sa règle (`CONTRIBUTING.md`).

## Conséquences et limites

L'empreinte suffit à désigner l'image ; le chemin ne sert qu'à refuser une image lancée depuis un autre dépôt. Changer de dépôt d'images demande une nouvelle version des clients.

## Documents concernés

`RELEASE.md` §2 ; `ARCHITECTURE.md` §3, §11.6 ; `README.md` (« Vérifiez Sésame en 4 étapes »).
