<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0066 · Copies des vecteurs hors de la part structurelle du contrôle des noms

**Date** : 2026-11-11 · **Statut** : actée (choix technique délégué par l'opérateur) · revoit [0065](0065-ci-du-contrat-executable.md)

## Contexte et menace

La décision 0065 fait vérifier par ce dépôt, dans chaque fichier suivi, que tout nom de ressource du déploiement (bucket, chemin d'image, compte de service, projet, hôte `run.app`, nombre de 12 chiffres) s'écrit sous la forme `<…>`. Les copies des vecteurs (`testdata/vecteurs/`, `CONTRIBUTING.md`, « Formats et vecteurs ») sont des fichiers suivis dont le contenu est fixé par le client de référence, octet pour octet, et contrôlé par `SHA256SUMS`. Un vecteur porte des valeurs concrètes que le code compare : un numéro de projet de test de 12 chiffres, des chemins d'image, des octets en hexadécimal qui peuvent former 12 chiffres. Sous la règle de 0065, la CI échouerait sur ces copies, et la seule façon de la faire passer serait de les modifier à la main, ce qui est interdit.

## Décision

- La part structurelle du contrôle « Noms de ressources » (`outils/controle-noms`) ne lit pas les fichiers que nomme `testdata/vecteurs/SHA256SUMS` ; elle imprime leur liste à chaque exécution. `SHA256SUMS` ne peut nommer que les quatre copies de `CONTRIBUTING.md`, dans `testdata/vecteurs/` (test du paquet `internal/vecteurs`) : l'exclusion ne s'étend à aucun autre fichier. `compteurPin.json`, écrit par ce dépôt, reste contrôlé.
- Ces copies restent couvertes par la recherche des noms réels que fait la CI du client sur ce dépôt (décision 0065) : un nom réel dans un vecteur se corrige dans le générateur du client, puis par une nouvelle recopie.

## Alternatives écartées

- **Liste des valeurs admises dans les copies** : elle écrirait les valeurs mêmes que le contrôle doit écarter, serait à refaire à chaque recopie, et resterait aveugle aux noms réels.
- **Exiger du générateur du client des formes `<…>`** : un vecteur doit porter des valeurs que le code compare (préfixe d'un chemin d'image, numéro de projet) ; un gabarit n'en est pas une.
- **Modifier les copies** : interdit, un vecteur ne s'écrit jamais à la main.

## Conséquences et limites

- Un nom réel dans une copie de vecteurs n'est trouvé que par la CI du client, après la recopie : la revue de la PR de recopie le prévient avant.
- Le contrôle ne lit pas ce qu'il exclut : son message le dit, pour qu'aucune exécution verte ne soit lue comme couvrant les copies.

## Documents concernés

`RELEASE.md` §8 ; `CONTRIBUTING.md` (« Outils et commandes »).
