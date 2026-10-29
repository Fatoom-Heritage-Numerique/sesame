<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0018 · Amorçage d'une grappe neuve commandé par l'exploitant

**Date** : 2026-10-27 · **Statut** : actée (décision de l'opérateur) · revoit [0017](0017-protocole-de-pin-grappe-de-trois-vm.md) · **revue depuis** ([0055](0055-amorcage-par-place-et-pairs-derives-du-nom.md))

## Contexte et menace

Une nouvelle version de Sésame, ou un effacement total, demande une grappe neuve et vide. Si une VM sans grappe pouvait en amorcer une d'elle-même, une simple coupure du réseau suffirait à faire naître une grappe parallèle, qui rendrait 10 essais de plus contre chaque PIN.

## Décision

- Aucune VM n'amorce de grappe d'elle-même : l'exploitant appelle `grappe/amorcer` sur l'une de trois VM sans grappe, une par zone, avec le jeton OIDC du compte d'exploitation, dont l'adresse est une variable attestée.
- La forme suit l'amorçage statique de Raft : la même configuration initiale (identifiants de nœud, zones, clés du canal entre VM, empreinte), réservée par les deux autres VM après vérification mutuelle de leurs attestations (`grappe/preparer`), puis démarrage des trois en état `new` (`grappe/demarrer`).
- `grappeId` est l'empreinte de la configuration initiale ; la grappe neuve est publiée dans la borne de chaque VM et l'amorçage est journalisé avec l'adresse du compte.

## Alternatives écartées

- **Amorçage spontané par une VM qui ne trouve pas de grappe** : une coupure du réseau suffirait.
- **Commande sans attestation mutuelle des pairs** : une VM d'une autre empreinte pourrait entrer dans la configuration initiale.

## Conséquences et limites

La commande ne donne ni état, ni essai, ni clé. Amorcer une grappe parallèle demande de tenir trois VM sans grappe, ce que seul l'exploitant peut faire : c'est « démarrer une nouvelle grappe », borné par la règle des 30 jours et visible dans les bornes (`THREAT_MODEL.md`, limite 19 (f)).

## Documents concernés

`ARCHITECTURE.md` §9.3 ; `API.md` §4.3.
