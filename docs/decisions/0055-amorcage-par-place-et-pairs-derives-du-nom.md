<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0055 · Amorçage d'une grappe sur trois places, quelle que soit la zone ; pairs dérivés du nom de la VM

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · revoit [0018](0018-amorcage-commande-d-une-grappe-neuve.md)

## Contexte et menace

L'amorçage d'une grappe neuve exigeait trois VM dans trois zones distinctes (`amorcage_refuse` pour deux VM dans une même zone). Or l'option A n'a que deux zones pour trois VM, et le déplacement d'une place pendant l'indisponibilité longue d'une zone met deux VM dans une zone. Toute nouvelle version, qui démarre par un amorçage, et toute sortie d'un effacement total y étaient donc impossibles. Par ailleurs, rien ne disait d'où une VM tient les noms de ses pairs : `env_override` est identique sur les trois VM, et pendant un passage de version six VM coexistent dans les mêmes zones.

## Décision

- **Une VM par place** : l'amorçage exige exactement trois VM sans grappe, attestées, de même `image_digest`, une par place (préfixe `-a`, `-b`, `-c`, dernière lettre du nom). Leur zone n'est pas contrôlée : elle est inscrite dans la configuration initiale, triée par zone puis par `nodeId`. La grappe compte au plus trois votants, un par place.
- **Noms** : les trois VM d'une version portent un même radical suivi de leur place ; le radical change à chaque version, si bien que les six VM d'un passage de version ont six noms distincts.
- **Pairs** : une VM lit son nom dans son jeton et en dérive ceux de ses deux pairs (même radical, autre dernière lettre). Elle joint chacun à son adresse DNS interne zonale, en essayant les zones de la région dans l'ordre. Aucune liste de pairs n'est configurée.
- **Réglages initiaux** de Raft et du TLS entre VM, révisables avec les tests de charge : valeurs par défaut d'etcd 3.6 pour les ticks, l'instantané et la compaction ; TLS 1.3 seulement, clé ECDSA P-256 tirée au démarrage.

## Alternatives écartées

- **Une liste de places (nom, zone) passée à `grappe/amorcer`** : un champ de plus à vérifier, pour une information que les jetons des pairs donnent déjà.
- **Garder « une par zone » et interdire l'amorçage en option A** : une option matérielle choisie par l'opérateur ne pourrait plus recevoir de version.
- **Une liste de pairs en variable attestée** : elle changerait le dictionnaire `env_override`, donc la condition KMS, à chaque renommage.

Pratiques reprises : etcd amorce une grappe par une liste initiale statique de membres et, après la perte de la majorité, en démarre une nouvelle sur les hôtes choisis par l'opérateur, sans contrôle de zone (etcd.io, « Clustering Guide » et « Disaster recovery ») ; la zone des membres est une affaire de disponibilité laissée à l'opérateur. Google Compute Engine nomme chaque VM `<nom>.<zone>.c.<projet>.internal` dans son DNS interne zonal (docs.cloud.google.com/compute/docs/internal-dns).

## Conséquences et limites

Rien n'empêche l'opérateur d'amorcer trois VM dans une même zone : la perte de cette zone serait alors un effacement total, qui se voit dans les bornes comme tout effacement. Cela n'ajoute aucun essai : la règle des 30 jours par titulaire borne toute grappe neuve.

## Documents concernés

`API.md` §4.3 ; `ARCHITECTURE.md` §9.3.
