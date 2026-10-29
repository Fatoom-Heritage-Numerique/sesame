<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0054 · Un essai décompté dont la recopie échoue est signalé ; un retrait d'urgence ne décompte aucun essai

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · précise [0053](0053-recopie-au-registre-avant-la-reponse-du-pin.md)

## Contexte et menace

La décision 0053 fait répondre `pin_indisponible_temporaire` quand la recopie au registre échoue après le décompte d'un essai, « sans évaluation ni alerte », l'essai restant décompté. Trois textes disaient pourtant qu'un essai n'est décompté sans réponse que si le leader tombe entre la validation et sa réponse, et que `pin_indisponible_temporaire` ne décompte rien. La promesse « chaque essai décompté est signalé au titulaire », sur laquelle reposent les limites 19 (i) et (j) de `THREAT_MODEL.md`, était fausse pour ce cas. Enfin, 0053 disait qu'un retrait d'urgence hors faille laissait consommer des essais en silence pendant jusqu'à 30 jours.

## Décision

- **Une seule réponse sur le décompte** : l'essai dont la recopie échoue juste après la validation par la majorité reste décompté. C'est, comme la chute du leader entre la validation et sa réponse, une panne au moment précis de l'essai, le sens sûr : un compteur ne remonte jamais sans ouverture réussie ni nouveau PIN. Hors le 10ᵉ essai (`pin_bloque`), ces deux cas sont les seuls où un essai est décompté sans évaluation.
- **Signalé comme les autres** : l'alerte d'essai part du décompte validé par la majorité, pas de la recopie. Elle suit le regroupement par heure et les réessais pendant 24 h de toute alerte. Seul un leader qui redémarre avant de l'envoyer la perd, comme toute alerte dont le réessai est tenu en mémoire (`THREAT_MODEL.md`, limite 8).
- **Retrait d'urgence** : la ligne du registre se lit sous l'identité fédérée, comme les clés. Une fois l'empreinte retirée de la condition du fournisseur, l'échange STS est refusé et la lecture de la ligne, qui précède le décompte, échoue : `pin_indisponible_temporaire`, aucun essai décompté. La phrase de 0053 sur des essais consommés pendant un retrait hors faille ne vaut donc pas.

## Alternatives écartées

- **Rendre l'essai à la recopie suivante**, par une entrée de compensation : un type d'entrée de plus dans l'état répliqué, pour un cas de quelques millisecondes.
- **Recopier avant la validation par la majorité** : la copie lue pendant une pause pourrait annoncer un essai que la grappe n'a pas validé, voire un plafond qui n'est pas atteint.
- **Ne pas signaler l'essai** et l'écrire comme limite : une exception à « chaque essai est signalé », alors que l'alerte part déjà du leader qui a décompté.

Pratiques reprises : chez Signal (Secure Value Recovery) comme dans les puces de séquestre d'Apple et de Google, le compteur d'essais est décrémenté avant l'évaluation et ne remonte qu'après une réussite ; Raft valide une entrée par la majorité avant d'y donner suite (Ongaro et Ousterhout, §5.3).

## Conséquences et limites

Le titulaire peut perdre un essai sans évaluation sur une panne brève du registre ou de KMS ; il en est averti par l'alerte d'essai. Un retrait d'urgence ne coûte aucun essai.

## Documents concernés

`API.md` §2.13 ; `ARCHITECTURE.md` §9.3, §11.6 et §11.7 ; `THREAT_MODEL.md`, limite 19 (i) et (j).
