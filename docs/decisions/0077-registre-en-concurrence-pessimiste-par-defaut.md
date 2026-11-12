<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), essais de Confidential Space -->

# 0077 · Registre sur une base Firestore en concurrence par défaut, pessimiste

**Date** : 2026-11-12 · **Statut** : actée (choix technique délégué, sur les relevés de la séance d'essais de la décision 0075)

## Contexte et menace

Chaque écriture du registre est une transaction qui relit le document unique `chaine/tete` et le réécrit (`ARCHITECTURE.md` §6.2) : les écritures des trois VM doivent s'ordonner sur ce document, sinon deux écritures de même époque fourcheraient la chaîne d'empreintes. Le texte de §6.2 attribuait cet ordre à la « concurrence optimiste » de Firestore, alors qu'une base `standard` est créée en concurrence `PESSIMISTIC` (verrous pris à la lecture) ; le mode `OPTIMISTIC` (contrôle des lectures à la validation) se choisit par la ressource de la base (`concurrencyMode`).

Dans les deux modes, une transaction en lecture et écriture est sérialisable : un conflit la fait échouer (`ABORTED`) et le client la rejoue. Aucune garantie de §6 ne repose sur l'un des deux modes en particulier ; reste à savoir lequel tient le mieux le débit d'un document unique, que `sesame-fake` ne peut pas mesurer (registre en mémoire).

Mesure de la séance (`essais/README.md`, route `/firestore`) : base `standard` d'essai vide en `europe-west9`, trois clients (un par VM) ensemble, 60 s par palier, au plus 16 transactions simultanées par client ; chaque transaction relit `chaine/tete`, le réécrit et crée une ligne en création seule, rejouée jusqu'à 5 fois sur `ABORTED`. Débits visés en tout : 6, 15, 30, 60 et 120 par seconde.

- **30 écritures par seconde d'un document unique sont tenues sans aucun échec dans les deux modes** (1 800 transactions validées sur 1 800 à 30/s, 900 sur 900 à 15/s) ; latence médiane d'une transaction validée de 24 à 80 ms selon le palier, p99 de 57 à 163 ms hors premier palier et hors saturation.
- **Plafond d'environ 50 à 56 écritures par seconde** : à 60/s visés, 56,0/s tenus en `PESSIMISTIC` (237 échecs après 5 essais sur 3 600), 50,3/s en `OPTIMISTIC` (582 échecs).
- **En surcharge, le mode optimiste s'effondre** : à 120/s visés, 53,6/s tenus en `PESSIMISTIC`, 25,5/s en `OPTIMISTIC`, dont la concurrence des clients a saturé (une partie des transactions n'a pas pu être lancée).
- Au tout premier palier de la base (6/s, `PESSIMISTIC`), 16 échecs sur 360, non reproduits ensuite ; `OPTIMISTIC` a tenu 360 sur 360 à ce palier.
- Aucune collision (deux lignes de même époque) dans aucun relevé.

## Décision

- **La base du registre garde le mode de concurrence par défaut d'une base `standard`, `PESSIMISTIC`.** Il fait au moins aussi bien que `OPTIMISTIC` à chaque palier, sauf le tout premier, et se dégrade moins en surcharge. Rien n'est réglé à la création de la base au-delà du défaut.
- `ARCHITECTURE.md` §6.2 décrit l'ordre des écritures par ce qui vaut dans les deux modes : Firestore sérialise les transactions d'un même document, une transaction en conflit est rejouée en entier, `ecritLe`, époque et empreinte recalculés.
- Le débit du registre se lit contre ce plafond : ses écritures continues (`vuLe` de chaque instance toutes les 60 s, §6.1, preuves de vie et actes des titulaires et des proches) sont loin de 30 par seconde.

## Alternatives écartées

- **Passer la base en `OPTIMISTIC`** : aucun gain mesuré au débit utile, un effondrement en surcharge, et un réglage de plus à poser et à contrôler à chaque création de base.
- **Répartir la tête sur plusieurs documents** : la chaîne d'empreintes exige un ordre total des écritures ; le débit mesuré n'en donne aucune raison.
- **Ne rien écrire et laisser §6.2 en l'état** : le texte attribuait la garantie à un mode que la base n'a pas.

## Conséquences et limites

- Le débit mesuré est celui d'une base d'essai vide, à un instant donné, depuis trois clients d'essai : il fonde une marge, pas une garantie (décision 0075). Il se remesure sur l'image et ses adaptateurs réels, à l'étape de l'infrastructure.
- En `PESSIMISTIC`, une transaction peut attendre un verrou avant de valider ; son `ecritLe`, posé avant la validation, doit rester à moins de 2 minutes de l'`update_time` que Firestore attribue (§6.3, « Fraîcheur de chaque ligne ») : les latences mesurées en sont très loin.

## Documents concernés

`ARCHITECTURE.md` §6.2 ; `essais/README.md`.
