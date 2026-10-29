<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0059 · État du PIN lu sur la copie locale ; jeton fédéré gardé jusqu'à son expiration ; fenêtre d'une rotation interrompue

**Date** : 2026-10-29 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau)

## Contexte et menace

Cinq comportements n'étaient pas écrits. D'où une VM qui n'est pas leader tient `pinPresent`, `pinBloque` et le compteur des évaluations quand elle sert `/vie` ou `/etat` hors pause. Ce qui remet ce compteur à zéro. Combien de temps Sésame garde le jeton d'accès obtenu par l'échange STS, dont dépend le délai réel d'un retrait d'empreinte. Où se placent les limites d'appels d'`/enroler`. Quand partent les rappels de 3 et 6 jours d'un signalement. Une remarque relevait aussi qu'une rotation de KTS ou une migration interrompue entre `/enroler` et l'enregistrement chez le client laisse une fenêtre sans aucune limite écrite.

## Décision

- **État du PIN hors pause** : la VM qui reçoit `/vie` ou `/etat`, leader ou non, lit `pinPresent`, `pinBloque` et le compteur dans sa propre copie appliquée de l'état répliqué, sans relais ni lecture au leader. Avec quelques battements de retard au plus, une évaluation toute récente n'apparaît qu'à l'`etat` suivant, ou l'alerte locale montre de nouveau des évaluations déjà vues, cas déjà admis par `API.md` §2.5.
- **Compteur des évaluations** : il part de zéro à la création de l'enregistrement (premier dépôt, dont un `redepot` sur un enregistrement absent) et n'est remis à zéro par rien d'autre, ni `nouveau_pin` ni `ouverture`.
- **Jeton fédéré** : Sésame garde en mémoire le jeton d'accès de son identité fédérée jusqu'à son expiration, à la durée par défaut d'une heure, et ne refait l'échange qu'à ce moment, comme les bibliothèques clientes de Google. Une empreinte retirée de la condition coupe une instance au plus 7 minutes de propagation plus une heure après ; pour une faille, les VM sont arrêtées aussitôt, sans l'attendre. Le script public compare chaque appel aux clés à la condition de l'heure et des 7 minutes qui le précèdent.
- **Limites d'`/enroler`** : la limite par IP se vérifie après l'`idToken`, la limite par `uid` après la preuve, comme sur une route signée par la clé de vie.
- **Rappels J3 et J6 d'un signalement** : à `signaleLe` + 72 h et + 144 h exactement, envoyés au premier passage de la tâche horaire qui suit ; le client suit la même convention pour ses propres rappels.
- **Rotation ou migration interrompue** : écrite comme un point de la limite de disponibilité (limite 8). La fenêtre se referme au déverrouillage suivant, qui reprend la rotation dès l'ouverture du coffre.

## Alternatives écartées

- **Lecture linéarisable au leader (ReadIndex) ou relais de `/vie` et `/etat`** : un aller-retour de plus à chaque preuve de vie pour une alerte qui tolère déjà un retard d'une vérification.
- **Compteur remis à zéro par `nouveau_pin`** : le repère de l'app deviendrait supérieur au compteur, traité comme `0`, et l'alerte locale montrerait toutes les évaluations déjà vues.
- **Aucun cache du jeton, ou durée raccourcie à 10 minutes** : un échange STS à chaque appel, ou un réglage de plus, pour avancer un retrait hors faille qui n'a pas besoin d'être plus rapide ; un retrait pour faille arrête déjà les VM.
- **Borner la fenêtre d'une rotation** (alerte sur une rotation qui dure, reprise prioritaire côté serveur) : un mécanisme nouveau pour une fenêtre de quelques secondes, refermée par le prochain déverrouillage.

Pratiques reprises : lectures « serializable » d'etcd, servies sans passer par le consensus et possiblement en retard sur le quorum (https://etcd.io/docs/v3.5/learning/api_guarantees/) ; durée par défaut d'une heure du jeton d'accès d'un compte de service usurpé par la fédération d'identité, réglable de 10 minutes à 12 heures (https://google.aip.dev/auth/4117).

## Conséquences et limites

Un retrait d'empreinte hors faille prend jusqu'à environ 67 minutes au lieu de 7. Une alerte locale d'évaluation peut arriver une vérification plus tard. Un titulaire qui meurt pendant une rotation interrompue, sans rouvrir son app, ne transmet rien.

## Documents concernés

`API.md` §2.3 ; `ARCHITECTURE.md` §4, §8.3, §9.1, §9.2, §11.6, §11.10 ; `THREAT_MODEL.md`, limites 8 et 19 ; `RELEASE.md` §5.
