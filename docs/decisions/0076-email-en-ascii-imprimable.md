<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0076 · Email de la charte et de la fiche d'alerte en ASCII imprimable seul

**Date** : 2026-11-11 · **Statut** : actée (décision de l'opérateur)

## Contexte et menace

L'email de la charte d'un proche (`ARCHITECTURE.md` §10.4) et celui de la fiche d'alerte (`API.md` §2.7) étaient admis en Unicode : le client les normalisait (NFC, espaces de bord retirés, tout en minuscules) et Sésame refusait un email différent de sa forme normalisée, recalculée de son côté. Le client et Sésame ne calculent pas cette forme avec la même bibliothèque, et elles ne s'accordent pas sur tous les caractères : la minuscule de `İ` (I majuscule pointé), le sigma final, les espaces Unicode retirés au bord (U+0085, U+FEFF), les caractères ajoutés par une version récente d'Unicode, que l'une connaît et l'autre pas.

Menace : **un email accepté d'un côté et refusé de l'autre**. Une charte écrite par l'app, dont Sésame refuse l'email, ne s'ouvre plus : le proche perd son accès sans que rien ne l'ait signalé à l'écriture. Une fiche d'alerte refusée laisse le titulaire sans alerte de Sésame.

## Décision

- L'email de la charte et de la fiche d'alerte n'est admis qu'en ASCII imprimable, de 0x21 à 0x7E.
- Le client retire de la saisie les espaces ASCII de bord (U+0020, U+0009, U+000A, U+000D), met en minuscules les seules lettres `A` à `Z`, sans autre transformation, puis refuse l'email vide ou qui contient un autre caractère. Le groupe de vecteurs `normaliserEmail` de `sesame.json` porte ce calcul.
- Sésame ne normalise rien : il exige l'email déjà sous cette forme, au moins un caractère, tous de 0x21 à 0x7E, aucun de `A` à `Z` (`cles.EmailNormalized`). Sinon la charte est refusée et la fiche d'alerte aussi (`message_refuse`), avec les codes du contrat, inchangés.
- Le test de `internal/cles` réclame `normaliserEmail`, que Sésame ne calcule pas : chaque email rendu par le client passe le contrôle de forme, aucune saisie refusée ne le passe. Les refus nouveaux des groupes `charte` et `ficheAlerte` sont rejoués par leurs tests.
- La règle s'écrit à un seul endroit, `ARCHITECTURE.md` §10 ; §10.4 et `API.md` §2.7 y renvoient.
- Aucune version n'est en service : la décision s'applique sans attente (décision 0062).

## Alternatives écartées

- **Garder Unicode avec une règle écrite caractère par caractère** (table des minuscules et des espaces figée dans le contrat) : chaque implémentation devrait recopier la table au lieu d'appeler sa bibliothèque, et la table vieillirait à chaque version d'Unicode ; l'écart réapparaîtrait au premier caractère oublié.
- **Normaliser côté Sésame** (accepter la saisie et la réécrire) : un texte signé ou scellé ne se réécrit pas, et Sésame recalculerait la même forme avec sa propre bibliothèque, ce qui laisse la menace entière.

## Conséquences et limites

- **Une adresse à accents ou dans un autre alphabet** (adresse internationalisée, RFC 6531) **est refusée** : le titulaire ou son proche doit donner une adresse en ASCII, ou se passer de l'email (téléphone, ou lettre pour un proche sans téléphone ni email).
- Seules `A` à `Z` sont mises en minuscules : deux adresses qui ne diffèrent que par la casse de leur partie locale sont confondues, comme avant.
- Le contrôle de Sésame ne dépend plus de `golang.org/x/text` ni de la version Unicode pour l'email ; le contrôle NFC des autres chaînes saisies reste celui de la décision 0068.

## Documents concernés

`ARCHITECTURE.md` §10 et §10.4 ; `API.md` §2.7 ; `CONTRIBUTING.md` (« Formats et vecteurs ») ; `testdata/vecteurs/sesame.json` et `SHA256SUMS`.
