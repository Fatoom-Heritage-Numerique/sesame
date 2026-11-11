<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0071 · `go.etcd.io/raft/v3` v3.7.0 essayée sur les cas de l'essai jetable, puis épinglée ; grappe de `sesame-fake`

**Date** : 2026-11-11 · **Statut** : actée (choix technique délégué par l'opérateur) · applique [0016](0016-bibliotheques-oprf-argon2-raft.md), [0017](0017-protocole-de-pin-grappe-de-trois-vm.md), [0018](0018-amorcage-commande-d-une-grappe-neuve.md), [0055](0055-amorcage-par-place-et-pairs-derives-du-nom.md)

## Contexte et menace

La grappe du protocole de PIN (`ARCHITECTURE.md` §9.3) repose sur Raft. L'essai jetable a été fait avec un prototype sur v3.6.0 ; la spécification demande d'épingler v3.7.0 après avoir refait sur elle les cas de cet essai. Une bibliothèque de consensus fautive pourrait valider sans majorité (deux copies de l'état, donc 20 essais par titulaire) ou paniquer sur un message reçu (effacement total si les trois répliques tombent ensemble).

## Décision

- **`go.etcd.io/raft/v3` v3.7.0**, dernière version publiée sur `proxy.golang.org` (`https://proxy.golang.org/go.etcd.io/raft/v3/@latest` : v3.7.0, commit `b867cf13f6bc0dae21204302df97bc2355c3af55`, étiquette `v3.7.0` ; liste `@v/list` : v3.6.0, v3.7.0-beta.0, -rc.0, -rc.1, v3.7.0), `RawNode` et `MemoryStorage` seuls, `PreVote`, `CheckQuorum`, `DisableProposalForwarding`, `StepDownOnRemoval`, journal de la bibliothèque jeté (`io.Discard`).
- **Module transitif** : `google.golang.org/protobuf` v1.36.11, version que requiert `raft/v3` v3.7.0 (son `go.mod`) et que retient la sélection de Go ; v1.36.12 existe (`@latest`) mais aucun module ne la requiert. Les modules de test de Raft (`github.com/cockroachdb/datadriven`, `github.com/stretchr/testify` et leurs dépendances) ne sont liés par aucun binaire. `ARCHITECTURE.md` §2.4 et `outils/controle-dependances` nomment déjà `google.golang.org/protobuf`.
- **Ce qui a été essayé, comment** : les cas de l'essai jetable sont rejoués sur v3.7.0 par les tests de `internal/factice` (`grappe_test.go`), trois instances dans un même processus sur le réseau entre VM simulé en mémoire (`factice.Reseau` : coupures, pertes déterministes, lots retenus), horloge fixe et ticks passés par le test, aucune attente réelle :
  - écriture validée par la majorité avant toute réponse (`TestPartitions`, tests `pin/*`) ;
  - VM coupée : sans majorité, aucune validation, `pin_indisponible_temporaire` de chaque côté, aucun essai décompté ; un contre deux : la majorité continue ; rattrapage et état identique au retour (`TestPartitions`) ;
  - leader arrêté : nouvelle élection, aucune écriture perdue (`TestArretDuLeader`) ;
  - VM recréée vide sous le même nom avec un nouvel identifiant : refusée avant 2 minutes d'absence, puis apprenante, ancien membre retiré, promue, copie complète reçue (`TestRemplacement`) ;
  - connexion sans échange des jetons de paire, jeton refusé : `403 pair_refuse` (`TestTransportEntreVM`) ;
  - amorçage commandé, deux amorçages, deux `grappeId` (`TestAmorcage`).
  Le passage à une VM TDX (option A) et la migration à chaud ne se rejouent pas hors de Google Cloud (lot 9).
- **Ce que la grappe ajoute à Raft** (`internal/grappe`) : identifiant de Raft tiré des 8 premiers octets de l'identifiant de nœud ; messages reçus seulement d'un pair attesté, de l'identifiant que lie son attestation, non retiré, ni local ni proposition ; `Step` sous `recover` (un message qui ferait paniquer la bibliothèque est rejeté et journalisé) ; instantané binaire de l'état (lu sans panique, forme fausse rejetée) toutes les 10 000 entrées et au moins une fois par heure, journal compacté aux 5 000 dernières ; une VM qui rejoint répond au pair qu'elle a entendu avant d'avoir une configuration.
- **`sesame-fake`** : en grappe locale, les trois instances sont reliées par le réseau simulé et la grappe est amorcée au démarrage par la décision de `grappe/amorcer` (`Grappe.Amorcer`), appelée dans le processus avec l'adresse du compte d'exploitation de la configuration, sans jeton OIDC : la configuration du client ne porte pas de clé privée de l'émetteur Google de test. En mode instance (trois conteneurs), les routes entre VM sont servies sur une écoute à part (`ecouteGrappe`), en HTTP entre conteneurs, l'identifiant de connexion et la clé du certificat entre VM dans deux en-têtes propres à `sesame-fake` ; le TLS mutuel est à l'adaptateur de l'image (lot 9).

## Alternatives écartées

- **`raft.Node`** (goroutines de la bibliothèque) : ses horloges internes rendent les tests de panne dépendants du temps réel ; `RawNode` laisse le tick et l'envoi au code de Sésame.
- **Garder v3.6.0**, celle de l'essai : la spécification épingle v3.7.0.
- **Instantané en JSON ou `encoding/gob`** : plus gros, et `gob` n'est pas fait pour une entrée non fiable.

## Conséquences et limites

- Le code de la grappe compte au budget (`outils/decompte-lignes`) dès que `sesame-fake` le lie : le plafond de 6 000 lignes est dépassé (décision au porteur).
- Les essais sur trois vraies VM (zones, migration à chaud, dérive d'horloge, TDX) restent ceux de `RELEASE.md` §8 et de la spécification, au lot 9.
- Une version suivante de Raft n'entre que par une nouvelle décision, après les mêmes essais.

## Documents concernés

`ARCHITECTURE.md` §2.4, §9.3 ; `CONTRIBUTING.md` (« Outils et commandes ») ; `go.mod`, `go.sum`.
