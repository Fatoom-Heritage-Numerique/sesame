<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0006 · Vérification de Sésame en direct pour enrôler et changer une charte, par manifeste en cache pour destiner

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur) · **revue depuis** ([0014](0014-liste-des-versions-compilee.md), [0017](0017-protocole-de-pin-grappe-de-trois-vm.md), [0020](0020-annulation-explicite-et-transmission-definitive.md), [0023](0023-alertes-envoyees-par-sesame-et-fiche-d-alerte.md))

## Contexte et menace

Si chaque fois qu'un utilisateur destine un contenu l'app devait joindre Sésame et vérifier son attestation, destiner coûterait une attente réseau, et Sésame injoignable bloquerait l'usage quotidien. Mais une app qui ne vérifierait jamais Sésame enverrait sa clé de transmission à n'importe quelle image.

## Décision

Destiner un contenu est une opération **locale** : l'app chiffre les enveloppes sous la clé de transmission et les envoie au client, sans appel à Sésame. Sésame n'est joint, **avec attestation vérifiée en direct**, qu'à l'enrôlement (la KTS lui est confiée), à la rotation, et à chaque changement de droits d'un proche (publication de génération, rare) ; en arrière-plan pour la preuve de vie (au plus une fois par jour) ; et sur le portail. Le manifeste est gardé en cache 24 h et rafraîchi en arrière-plan.

## Alternatives écartées

- **Vérifier Sésame à chaque destination** : attente réseau sur le parcours le plus fréquent, pour une opération qui n'envoie rien à Sésame.

## Conséquences et limites

Destiner 100 fichiers à 100 proches coûte 10 000 AES-GCM sur l'appareil et un appel au client. Sésame injoignable n'empêche ni de destiner ni de retirer un destinataire ; il empêche d'enrôler, de révoquer un proche, d'ouvrir sur le portail (`THREAT_MODEL.md` §2, limite 8). Le client ne peut pas vérifier l'AAD des enveloppes : c'est Sésame qui refuse, à l'ouverture, une enveloppe copiée d'un contenu ou d'un proche à l'autre.

## Documents concernés

`ARCHITECTURE.md` §1, §11.1, §11.5, §11.6.
