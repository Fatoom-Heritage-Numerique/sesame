<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0064 · OpenAPI en JSON vérifiée contre `sesame-fake` ; vecteurs recopiés sous `testdata/vecteurs/` ; `compteurPin` généré

**Date** : 2026-11-11 · **Statut** : actée (décision de l'opérateur)

## Contexte et menace

`API.md` est la publication du contrat ; l'OpenAPI, les vecteurs et `sesame-fake` en dérivent. Trois descriptions d'un même contrat divergent si aucun test ne les compare. Les vecteurs viennent du client de référence, dont le dépôt est privé : ce dépôt ne peut pas relire leur source, et une copie modifiée à la main ferait accepter à Sésame un autre format que les clients.

## Décision

- **OpenAPI** : `api/openapi.json`, OpenAPI 3.1 en JSON (lu par `encoding/json`, sans dépendance). Elle décrit `GET /attestation`, `OPTIONS` et `POST /n/{a|b|c}/canal` avec leurs enveloppes et leurs erreurs en clair (`API.md` §1.2, §1.3), et chaque corps chiffré comme schéma, sous `x-sesame-routes` : corps de la requête, réponse `200` et codes d'erreur de chaque route du canal, plus les routes entre VM (§4) et la passerelle (§5).
- **Concordance** : un test du paquet de test vérifie que les routes de `x-sesame-routes` sont celles que sert `sesame-fake`, que chaque requête et chaque réponse des tests des routes se valide contre son schéma (validateur réduit écrit dans le paquet de test : types, champs requis, aucun champ en plus, énumérations, motifs, longueurs, bornes), et que les routes et codes d'erreur d'`API.md` (titres du §2, tableau du §6) sont ceux de l'OpenAPI.
- **Vecteurs** : les quatre fichiers de `CONTRIBUTING.md` (« Formats et vecteurs ») sous `testdata/vecteurs/`, avec `SHA256SUMS` ; le commit qui les recopie cite le commit du client. Chaque groupe est relu par un test Go du paquet qui porte son format ; un nom de vecteur qu'aucun test ne lit fait échouer la suite.
- **`compteurPin`** : vecteur propre à ce dépôt, `testdata/vecteurs/compteurPin.json`, écrit par `outils/generer-compteurpin`, qui calcule les réponses attendues depuis la table des délais d'`API.md` (§2.13) sans importer le code du protocole de PIN ; la CI le relance et échoue sur un écart. Il n'entre pas dans `SHA256SUMS`, qui ne porte que les copies.
- **Ordre d'un changement de format** : la PR du client change son générateur ; la PR de ce dépôt recopie les fichiers de ce commit, régénère `SHA256SUMS`, change le code Go et se fusionne la première ; la PR du client épingle ensuite le commit de fusion de ce dépôt. Le contrôle du client compare ses fichiers à ce commit épinglé : aucune `main` ne passe au rouge.

## Alternatives écartées

- **OpenAPI en YAML** : une dépendance de plus pour la lire en Go.
- **Générer l'OpenAPI depuis le code** : le contrat suivrait le code, alors que c'est le code qui suit le contrat.
- **Vecteurs relus depuis le dépôt du client** : il est privé.
- **`compteurPin` écrit à la main** : interdit pour tout vecteur.

## Conséquences et limites

- Le validateur réduit ne couvre que les mots-clés que l'OpenAPI emploie ; un mot-clé de plus dans l'OpenAPI fait échouer le test tant qu'il ne le lit pas.
- La concordance entre la spécification du client et `API.md` reste une relecture humaine (`CONTRIBUTING.md`).

## Documents concernés

`API.md` (tête) ; `CONTRIBUTING.md` (« Formats et vecteurs », « Outils et commandes ») ; `RELEASE.md` §8.
