<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0068 · Contrôle NFC par `golang.org/x/text/unicode/norm` v0.44.0, épinglée

**Date** : 2026-11-11 · **Statut** : actée (choix technique délégué par l'opérateur)

## Contexte et menace

Sésame refuse une chaîne saisie par une personne qui n'est pas en forme normale C (`API.md` §1.7) : un email de charte, un nom, une fiche d'alerte, et toute chaîne d'un message signé passée au JSON canonique (`internal/jcs`). Le client de référence normalise par `String.prototype.normalize` de son moteur. Menace : **deux implémentations qui ne refusent pas les mêmes chaînes**. Une chaîne que le client signe et que Sésame refuse fait échouer une route pour un titulaire ; une chaîne que Sésame accepte alors qu'elle n'est pas en NFC laisse deux écritures d'un même texte passer pour différentes, dans une zone critique. Le contrôle de `internal/jcs` reposait sur des tables écrites dans ce dépôt par un générateur, avec une limite connue : un caractère précomposé suivi d'une marque de classe combinatoire inférieure à celles de sa décomposition (« ṧ » puis la cédille U+0327) était accepté, alors que la normalisation réordonne ces marques.

## Décision

- Le contrôle NFC est `norm.NFC.IsNormalString` de **`golang.org/x/text` v0.44.0**, paquet `unicode/norm` seul (avec `transform`, qu'il importe), extension officielle de Go au même titre que `golang.org/x/crypto`. Version lue sur `proxy.golang.org` (`https://proxy.golang.org/golang.org/x/text/@latest` : v0.44.0, commit `6070ac32abcdd1670599eeb3ee4d513c8184a513`) ; elle porte les tables d'Unicode 17.0.0, la version de la bibliothèque standard de Go 1.27.4.
- Vérifiée par le test de `internal/jcs` : vecteurs `jsonCanonique` de `json.json` (refus d'une chaîne hors NFC compris), cas de la limite connue (précomposé suivi d'une marque de classe inférieure ⇒ refusé), et version Unicode de `norm` égale à `unicode.Version`. Vérifiée aussi, hors du dépôt, sur 587 346 chaînes (chaque point de code, et des suites de marques combinantes) contre `String.prototype.normalize` de Node 24 (ICU 78.3, Unicode 17.0) : aucun écart.
- Les tables écrites dans ce dépôt et leur générateur (`outils/generer-nfc`) sont retirés.
- Le module entre dans la liste d'`ARCHITECTURE.md` §2.4 et dans celle du contrôle des dépendances (`outils/controle-dependances`). Une version suivante n'entre que par une nouvelle décision, après les mêmes vérifications.

## Alternatives écartées

- **Tables maison générées depuis la base Unicode** : un algorithme écrit une seconde fois, ici, pour une seule question ; la limite connue montre qu'une version complète demanderait la décomposition entière de chaque caractère, soit refaire `unicode/norm` dans le budget de code.
- **Normaliser au lieu de refuser** : le contrat refuse (`API.md` §1.7) ; un texte signé ne se réécrit pas.

## Conséquences et limites

- Une dépendance de plus dans l'image, épinglée par `go.sum`. `cmd/sesame` la liera avec le code des routes ; le budget de 6 000 lignes ne la compte pas (bibliothèque épinglée) et baisse des tables retirées.
- Une montée de version de Go qui change `unicode.Version` fait échouer le test de `internal/jcs` jusqu'à une version de `x/text` aux mêmes tables, après une nouvelle décision. Le client de référence suit la version Unicode de son moteur : un écart entre les deux reste possible à chaque montée de version, et se voit par les vecteurs `json.json` régénérés.

## Documents concernés

`ARCHITECTURE.md` §2.4 ; `CONTRIBUTING.md` (« Règles du code », « Outils et commandes ») ; `go.mod`, `go.sum`.
