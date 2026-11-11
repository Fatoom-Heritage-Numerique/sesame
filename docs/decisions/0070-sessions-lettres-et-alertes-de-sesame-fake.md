<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0070 · Sessions du portail, lettres et alertes : choix d'écriture du code des décisions

**Date** : 2026-11-11 · **Statut** : actée (choix technique délégué par l'opérateur) · applique [0063](0063-sesame-fake-contrat-executable.md)

## Contexte et menace

Les routes du portail, la lettre de secours, les lettres d'invitation et les alertes au titulaire entrent dans le code des décisions que partagent `sesame-fake` et l'image (décision 0063). Le contrat (`API.md`) fixe leur comportement ; quelques points d'écriture n'y sont pas dits : la saisie d'un code hors NFC, la forme des valeurs placées dans les textes attestés, la façon de rattacher une lettre à sa transmission, les bornes de mémoire, et ce que remplacent les doubles de Stripe et de Pingen. Menace : un choix implicite qui ouvrirait un essai non décompté, une lettre en double, ou une mémoire sans borne.

## Décision

- **Code d'accès hors NFC** : le corps entier reste en JSON canonique, NFC compris (`API.md` §1.7). Une saisie hors NFC porte forcément un caractère hors de l'alphabet du code : le corps est refusé (`message_refuse`) sans essai décompté, ce qui ne dit rien d'un code ; le portail l'affiche comme `code_refuse`.
- **Valeurs des textes attestés** : un texte porte entre accolades les clés que Sésame remplace (`ARCHITECTURE.md` §2.3) ; la première ligne d'un texte d'email est son objet. Sésame n'ajoute aucun texte à ceux du document.
- **Lettre d'une transmission** : une lettre appartient à la transmission en cours si son premier envoi accepté (ou, avant lui, sa réservation) est postérieur au `signaleLe` de son signalement ; une lettre d'une transmission close ne compte donc pas comme « déjà commandée » (`API.md` §2.10), sans champ de plus au registre.
- **Sessions** : l'état d'une session vit dans `internal/sessions`, verrouillé pendant chaque appel, retiré une seule fois de sa table, ce qui garantit une seule `sessionTerminee` ; au plus 100 000 sessions par instance (au-delà, `trop_d_appels`).
- **Codes faux par IP** : comptés en mémoire, au plus 100 000 IP ; table pleine sans entrée expirée, l'IP est tenue pour bloquée (échec fermé).
- **Tâche horaire** : sans grappe, chaque VM la fait, une heure après son démarrage puis chaque heure ; elle parcourt au plus 100 000 titulaires et 1 000 proches par titulaire ; chaque envoi est réservé au registre en transaction avant d'être fait.
- **Réessais** : notifications et alertes partagent un même réessai en mémoire (`passerelle.Retrier` : 1 min, doublé, plafonné à 1 h, 24 h au plus, 10 000 envois en attente).
- **Doubles de Stripe et de Pingen** : en mémoire, au niveau des ports `Identite` et `Courrier`, comme ceux de Twilio et SendGrid ; les doubles HTTP qui rejouent des réponses enregistrées arrivent avec les clients REST de ces fournisseurs, adaptateurs réels de l'image.

## Alternatives écartées

- **Lire le code d'accès sans contrôle NFC** : une exception au JSON canonique dans le seul corps de `session/ouvrir`, pour une saisie qui ne peut de toute façon pas être un code.
- **Ajouter un `signalementId` à `lettres/{contactId}`** : un champ hors du registre décrit par la spécification, que la date de l'envoi remplace.
- **Doubles HTTP de Stripe et Pingen dès maintenant** : ils exigeraient d'écrire maintenant les clients REST de l'image, qui comptent au budget de code.

## Conséquences et limites

- Le générateur de la configuration de test du client écrit ses textes avec ces clés pour que ses tests les retrouvent.
- Une instance redémarrée perd ses sessions, ses codes faux par IP et ses réessais en cours, comme le contrat le prévoit ; les codes faux par proche restent au registre.

## Documents concernés

`API.md` §1.7 ; `ARCHITECTURE.md` §2.3, §2.4 ; `README.md` (« État du projet »).
