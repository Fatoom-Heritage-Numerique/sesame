<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0045 · Version de signature fixée par l'image, une clause de la condition KMS par version en service

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option la plus simple qui tient la garantie annoncée) · précise [0021](0021-registre-signe-et-borne-publique.md), [0026](0026-cles-session-sign-dans-la-liste-des-versions.md) et [0028](0028-reglages-du-client-env-override-entier.md)

## Contexte et menace

Une signature asymétrique de Cloud KMS vise toujours une version précise de la clé : une clé asymétrique n'a pas de version principale. Rien ne disait avec quelle version de `session-sign` ou de `registre-sign` Sésame signe. Or les clients n'acceptent que les `kid` de `clesSession`, compilés chez eux, et les VM ne lisent les versions de ces clés qu'au démarrage. Une VM recréée en cours de service, qui prendrait par exemple la version la plus récente, pouvait signer avec un `kid` que les clients ne connaissent pas encore, ou écrire des lignes que les VM démarrées avant elle ne savent pas vérifier.

Par ailleurs, la condition du fournisseur du pool comparait `env_override` à un seul dictionnaire. Or la mise en service d'une version garde deux versions `en_service` à la fois, et leurs dictionnaires diffèrent dès qu'une variable change.

## Décision

- La version de `session-sign` et celle de `registre-sign` avec lesquelles Sésame signe sont des constantes de l'image, publiées avec son code. Une VM qui démarre refuse de servir si l'une d'elles n'est pas activée, ou n'est pas générée par KMS (`importJob`), comme pour toute version importée.
- La vérification ne change pas : toute version activée générée par KMS, lue au démarrage. Une version ajoutée, ou importée, en cours de service n'est donc ni utilisée pour signer, ni acceptée, par les VM déjà démarrées ; le contrôle de démarrage la juge au démarrage suivant.
- Une nouvelle version de l'une de ces deux clés ne sert donc qu'à partir d'une nouvelle version de Sésame, publiée après les clients qui portent son `kid` dans `clesSession`.
- La condition du fournisseur porte une clause par version `en_service` : `(image_digest == "<empreinte>" && env_override == {…})`, reliées par `||`, les autres termes restant communs. Le contrôle CI compare l'ensemble des couples (empreinte, empreinte du dictionnaire) de la condition à celui des lignes `en_service` de `versions.json` (`imageDigest`, `envDigest`).

## Alternatives écartées

- **Signer avec la version activée la plus récente** : une VM recréée après la création d'une version signerait avec un `kid` encore absent des clients, et les VM plus anciennes ignoreraient ses lignes du registre.
- **Relire les versions de KMS en cours de service** : un appel et un état de plus, pour un cas que la publication d'une version règle déjà.
- **Un seul dictionnaire `env_override`, commun aux versions en service** : interdisait de changer une variable attestée d'une version à l'autre sans arrêt du service.

## Conséquences et limites

Une rotation de `session-sign` ou de `registre-sign` suit l'ordre d'une mise en service : clients d'abord, puis version de Sésame. La condition grandit d'une clause pendant le recouvrement de deux versions, sous la borne de 4 096 caractères.

## Documents concernés

`ARCHITECTURE.md` §4, §6 ; `RELEASE.md` §7.
