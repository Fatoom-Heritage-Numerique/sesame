<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0040 · Les bornes déposées en échec ne comptent pas dans le contrôle anti-retour

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option la plus simple qui tient la garantie annoncée) · précise [0021](0021-registre-signe-et-borne-publique.md) et [0022](0022-reprise-publiee-avec-ou-sans-sauvegarde.md)

## Contexte et menace

Le contrôle anti-retour compare l'époque du registre lu à l'époque la plus haute des bornes déposées par toutes les instances dans les 3 dernières heures, et échoue si la plus récente a plus de 2 heures. Chaque instance dépose sa borne toutes les 10 minutes, même en échec, avec l'époque du registre qu'elle lit. Deux cas défaisaient donc la garantie annoncée, « un arrêt de plus de 2 heures ne se termine que par une reprise publiée » :

- une instance redémarrée après un arrêt de toutes les instances de plus de 2 heures déposait une borne fraîche, et le contrôle repassait seul ;
- devant un registre remis en arrière, les instances en échec déposaient des bornes à l'époque basse ; 3 heures plus tard, les bornes plus hautes sortaient de la fenêtre, et le retour arrière passait.

Il manquait aussi la façon dont une version en reprise passe le contrôle, son époque restaurée étant plus basse que les bornes récentes.

## Décision

- Une borne déposée pendant que le contrôle de son instance échoue porte l'événement `registre_ancien`, à chaque dépôt tant que dure l'échec, et ne compte ni dans le maximum ni dans l'âge de la plus récente borne. Une instance en échec ne remplace jamais la référence par l'époque qu'elle lit.
- Une version en reprise publiée ne compte que les bornes de son propre `imageDigest`. Tant qu'elle n'en a déposé aucune, sa référence est l'époque de sa constante `reprise` (0 sans sauvegarde), vérifiée au démarrage.

Aucun champ, aucun composant : l'événement `registre_ancien` et l'`imageDigest` sont déjà dans la borne, la constante `reprise` déjà dans l'image.

## Alternatives écartées

- **Une borne sans `registre` en échec** : change le format de la borne et ses vecteurs, pour le même effet que l'événement déjà publié.
- **Un état d'échec gardé en mémoire jusqu'à une reprise** : perdu au redémarrage, il ne couvre pas le premier cas.
- **Un contrôle de monotonie des époques dans le script public** : utile pour voir un retour arrière, mais il ne l'empêche pas ; avec cette décision, un retour arrière reste en échec fermé, et la suite des bornes publiées le montre déjà.

Pratiques reprises : un client de TUF garde la plus haute version de métadonnées vue et refuse toute version plus basse ; Android Verified Boot garde un index de version dans un stockage résistant aux manipulations et refuse de démarrer une version plus ancienne ; le client de la base de sommes de contrôle de Go garde la dernière tête d'arbre vue et traite une incohérence comme une erreur de sécurité. Dans chaque cas, la référence ne descend jamais du fait de ce qui est lu ; c'est ce que la règle ci-dessus garantit pour les bornes.

## Conséquences et limites

Un arrêt de toutes les instances de plus de 2 heures, ou un échec qui dure plus de 2 heures sur toutes les instances, ne se termine que par une reprise publiée, comme annoncé. Une image autorisée en douce dans KMS peut toujours signer une borne (`THREAT_MODEL.md`, limite 2) : elle ne gagne rien de plus qu'avant.

## Documents concernés

`ARCHITECTURE.md` §6.3.
