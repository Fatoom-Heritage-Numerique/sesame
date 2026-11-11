<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0072 · Limites de débit et tailles mesurées sur `sesame-fake` : aucune révision technique

**Date** : 2026-11-11 · **Statut** : actée (choix technique délégué par l'opérateur) ; les limites que voit un proche ou un titulaire restent à l'opérateur · applique `API.md` §7 (« Révision des valeurs initiales »)

## Contexte et menace

`API.md` §7 donne des valeurs initiales, à réviser au contrat exécutable : une limite vaut au moins le double de l'usage du parcours légitime le plus chargé de sa route ; une taille se fixe sur la mémoire mesurée d'une instance, rapportée aux 1,38 Gio du conteneur. Une limite trop basse bloque un titulaire ou un proche ; trop haute, elle laisse épuiser une ressource.

## Décision

- **Mesures** (méthode dans `CONTRIBUTING.md`, « Outils et commandes ») :
  - appels par route et par clé de limite des parcours joués par les tests du contrat (`MESURES_APPELS=1`), hors tests qui éprouvent une limite ou un refus : au plus, par uid, 11 `etat` (sur un jour), 8 `enroler`, 6 `defi`, 6 `pin/deposer` et 15 `pin/evaluer` (suite du vecteur `compteurPin`, sur 41 heures, donc au plus 8 dans une heure), 5 `vie`, 4 `deces/signaler`, 1 pour `alerte/fiche`, `charte/generation`, `deces/annuler`, `lettre/secours`, `titulaire/supprimer` ; par canal ou par session, au plus 6 `session/verifier`, 3 `session/ouvrir` et `session/ouvrir-cles`, 2 `session/identite/demarrer` et `verifier`, 1 `session/identite/bascule`, `session/fermer`, `grappe/amorcer` ;
  - grappe : une VM sans grappe demande à entrer toutes les 30 secondes à chaque autre place, soit 2 demandes par minute et par `nodeId` à une même VM ; un amorçage fait un `grappe/preparer` et un `grappe/demarrer` par VM, en quelques millisecondes en local ;
  - mémoire : tas Go d'un nœud (`go test -tags mesures`) : 6 Mo pour 10⁴ enregistrements du PIN, 35 Mo pour 10⁵, 294 Mo pour 10⁶ (20,8 % de 1,38 Gio, dont un instantané de 67 Mo et un journal de 5 001 entrées), 5 Mo pour 10⁵ dépôts répétés sur 10³ titulaires.
- **Aucune limite technique n'est révisée** : chaque valeur du §7 vaut au moins le double de l'usage mesuré ou du parcours légitime le plus chargé qu'on peut en déduire (`grappe/rejoindre` 4 / min pour 2, `grappe/amorcer` 2 / min pour 1, `grappe/preparer` et `grappe/demarrer` 2 / min pour 1, `session/fermer` 10 / h pour 1, `defi` 30 / h par canal pour 6) ; la réservation de 2 minutes dépasse de loin un amorçage. Les seuils de mémoire de 70 % et 90 % restent : 10⁶ titulaires en occupent 21 %.
- Les limites que voit un proche ou un titulaire (renvois du dernier code, sessions simultanées, identité, essais du PIN, `pin/deposer`, `pin/evaluer`, `vie`, `etat`) ne changent que par l'opérateur : aucune n'est atteinte à moitié par les parcours mesurés.

## Alternatives écartées

- **Relever par prudence** les limites de la grappe : rien ne le justifie, et une limite plus haute laisse plus de demandes à journaliser.

## Conséquences et limites

- Les parcours mesurés sont ceux des tests de ce dépôt ; ceux de l'app et du portail se mesurent chez le client contre `sesame-fake`, et peuvent rouvrir une révision.
- L'empreinte mémoire de 10⁶ titulaires (≈ 300 Mo) dépasse l'estimation initiale de 200 Mo de la spécification, sans approcher les seuils.

## Documents concernés

`API.md` §7 (inchangé) ; `CONTRIBUTING.md` (« Outils et commandes »).
