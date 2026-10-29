<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0050 · Remplacement d'un membre de la grappe : l'absent est retiré avant la promotion de la remplaçante

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, d'après les pratiques établies de Raft) · précise [0017](0017-protocole-de-pin-grappe-de-trois-vm.md) · **revue depuis** ([0057](0057-place-inscrite-et-instance-qui-refuse-de-servir.md))

## Contexte et menace

Le remplacement automatique d'un membre absent était décrit dans cet ordre : remplaçante ajoutée comme apprenante, promue votante, puis ancien membre retiré. Entre la promotion et le retrait, la grappe comptait quatre votants, dont l'absent, alors que le même texte annonçait qu'elle ne dépasse jamais trois votants, un par zone. Un autre passage parlait de « consensus conjoint », un mécanisme que la grappe n'utilise pas. Ce n'était pas une faille, puisque l'ancien membre reste minoritaire, mais la séquence Raft à écrire n'était pas fixée.

## Décision

- Une seule modification de membres à la fois, chacune appliquée avant la suivante (changement d'un seul membre, sans consensus conjoint), dans cet ordre :
  1. la remplaçante est ajoutée comme **apprenante**, sans droit de vote ;
  2. une fois son journal rattrapé, l'**ancien membre absent est retiré** ;
  3. la remplaçante est **promue** votante.
- La grappe ne compte donc jamais plus de trois votants. Entre le retrait et la promotion, elle tient à deux votants présents, qui gardent la majorité. Si la promotion échoue, le leader la retente : la remplaçante reste apprenante jusque-là.
- Les conditions d'admission ne changent pas : elles portent sur le membre absent, encore inscrit quand la remplaçante demande à entrer.

## Alternatives écartées

- **Promouvoir avant de retirer** : quatre votants, dont un absent, pendant la transition, avec une majorité de trois sur trois présents. La tolérance aux pannes est la même, mais le plafond de trois votants n'est plus vrai.
- **Changement conjoint unique** (promotion et retrait dans une même modification) : un mécanisme de plus à écrire et à tester, sans gain pour une grappe de trois.
- **Retirer l'absent avant d'ajouter la remplaçante** : pratique d'etcd pour un membre en panne. Ici, l'admission se vérifie contre le votant absent encore inscrit ; le retirer d'abord obligerait à garder sa place hors de la liste des membres.

Sources : documentation d'etcd sur la reconfiguration (un membre en panne se retire avant que son remplaçant vote ; un apprenant ne compte pas dans le quorum tant qu'il n'est pas promu), https://etcd.io/docs/v3.5/op-guide/runtime-configuration/ et https://etcd.io/docs/v3.5/learning/design-learner/ ; thèse de Diego Ongaro, *Consensus: Bridging Theory and Practice*, §4.1 (changement d'un seul membre à la fois), https://github.com/ongardie/dissertation.

## Conséquences et limites

Rien ne change pour les titulaires. Pendant un remplacement, une seconde perte arrête la grappe, comme avant (`THREAT_MODEL.md`, limite 19).

## Documents concernés

`ARCHITECTURE.md` §9.3 ; `API.md` §4.1 et §4.2.
