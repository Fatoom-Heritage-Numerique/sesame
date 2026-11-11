<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0063 · `sesame-fake` : le code des décisions avec des doubles, deux modes, configuration reçue du client

**Date** : 2026-11-11 · **Statut** : actée (décision de l'opérateur)

## Contexte et menace

Le contrat exécutable (étape 2 de l'ordre de construction, `README.md`) doit permettre au client de construire son app, son portail et sa passerelle sans enclave. Un faux écrit à part dériverait du vrai : un client testé contre lui ne prouverait rien sur Sésame. Un lanceur factice laissé dans l'image publiée ferait accepter des jetons signés par une clé de test. Un faux qui ne sert pas le protocole de PIN sans trois machines rendrait les tests de l'app lourds.

## Décision

- **Un module, deux binaires.** `cmd/sesame` (l'image) et `cmd/sesame-fake` lient le même code des décisions (`internal/` hors `factice`, `gcp` et `fournisseurs`), qui ne voit que les interfaces de `internal/ports/` (lanceur, KMS, base, stockage, secrets, journal, horloge, aléa, passerelle, SMS, email, identité, courrier). `sesame-fake` les remplace par `internal/factice/` ; l'image, par les adaptateurs réels de `internal/gcp/` et `internal/fournisseurs/`, écrits à l'étape 3 et suivantes. L'arborescence est dans `ARCHITECTURE.md` §2.4.
- **Lanceur factice confiné.** `SESAME_FAKE_LAUNCHER` n'est lu que dans `internal/factice/`, et `cmd/sesame` ne lie aucun paquet de `internal/factice/` : la CI l'impose (`go list -deps`, `RELEASE.md` §8).
- **Deux modes**, choisis par la configuration : *grappe locale*, un processus qui tient trois instances `a`, `b`, `c` sur un réseau entre VM simulé en mémoire, sert les trois préfixes `/n/{a|b|c}/canal` et amorce sa grappe au démarrage par la route `grappe/amorcer`, sous un jeton d'exploitation de test : le PIN est servi dès le démarrage ; *instance*, une instance par processus, qui ne s'amorce jamais seule, pour la grappe sur trois conteneurs (`docker compose`, Toxiproxy). Le contrat ne change pas : trois membres, amorçage commandé, `grappeId: null` sur une instance sans grappe.
- **Configuration reçue du client**, en JSON, à un chemin donné au démarrage : chaîne d'attestation de test (racine, intermédiaire et sa clé privée), clé `session-sign` de test et son `kid`, numéro de projet et chemin du dépôt d'images de test, dictionnaire `env_override` de test, émetteurs des jetons à vérifier (émulateur ou projet réel du client), adresse de la passerelle, comptes de test. Aucune valeur d'un client n'est dans ce dépôt ; les tests du dépôt ont leur propre configuration de test sous `testdata/`. Ce sont des clés de test, publiques et nommées comme telles.
- **Mémoire bornée comme l'enclave** : registre, canaux, sessions et filtre de Bloom aux tailles du contrat ; un redémarrage efface tout.
- **Image publiée** : la CI construit l'image de `sesame-fake` à chaque fusion sur `main` et la publie sur le registre de conteneurs de GitHub du dépôt ; un client l'épingle par son empreinte. Elle n'est jamais une version de Sésame et n'entre jamais dans `versions.json`.

## Alternatives écartées

- **Un faux écrit à part** (serveur de réponses enregistrées) : il dériverait du contrat et ne partagerait pas le code des décisions (`ARCHITECTURE.md` §2.2, budget mesuré sur le faux).
- **Une grappe à un seul membre pour les tests de l'app** : configuration à un membre hors contrat (le contrat en exige trois) ; le mode grappe locale garde trois membres dans un processus.
- **Une instance sans grappe pour l'app** : elle ne sert pas le PIN, que l'app exige dès la création du coffre.
- **Le lanceur factice dans l'image derrière une variable** : une variable de lancement n'est pas une garantie ; le binaire de l'image ne contient pas le code.

## Conséquences et limites

- Le budget de code se mesure dès cette étape sur les décisions (`RELEASE.md` §8).
- Le réseau simulé du mode grappe locale ne remplace pas les tests sur trois conteneurs ni l'essai sur trois VM avant la version 1.0.0.
- Un client garde la configuration de test de `sesame-fake` hors de ses builds de production : c'est à lui de le contrôler.

## Documents concernés

`ARCHITECTURE.md` §2.4 ; `CONTRIBUTING.md` (« Outils et commandes ») ; `RELEASE.md` §8 ; `README.md` (« État du projet »).
