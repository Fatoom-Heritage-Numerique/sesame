<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0060 · Événements d'une borne par période ; rescellement d'une autre KTS à génération égale ; suppression d'identité réessayée sans réservation

**Date** : 2026-10-29 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau)

## Contexte et menace

Plusieurs comportements n'étaient pas écrits, ou l'étaient de deux façons. Ce que porte `evenements` dans une borne : les événements depuis la borne précédente ou un état courant, avec ou sans dédoublonnage, et ce que vaut `vuLe`. Si un rescellement à génération égale peut sceller une KTS différente, comme le fait la reprise d'une rotation sur un autre appareil, et s'il ferme des sessions. Comment se réessaie « une fois par jour » la suppression d'une vérification d'identité, alors que la tâche est horaire et que l'enregistrement ne garde pas la date du dernier essai, et si ce réessai est réservé comme les envois. Si un dépôt `ouverture` du PIN, qui ne change aucun champ recopié, écrit au registre. Ce que dit le contrôle anti-retour au premier dépôt du PIN. Ce que vaut « aucun effet » d'`/etat` quand la lecture rejette la ligne. Enfin, la limite de `grappe/rejoindre` (2 par minute) égalait le rythme de renouvellement d'une VM sans grappe (toutes les 30 secondes) : au moindre décalage, une demande légitime recevait `429`.

## Décision

- **Événements d'une borne** : une borne porte ce que l'instance a vu depuis sa borne précédente, une entrée par couple `type`, `ressource`, avec `vuLe` à la première occurrence de la période, sans autre plafond que le nombre de ressources distinctes. Un fait qui dure revient, une fois, dans chaque borne de la période où il est vu.
- **Rescellement à génération égale** : la KTS reçue remplace la KTS scellée, même si elle en diffère, sans fermer aucune session. Seul le porteur de la clé de vie inscrite le peut, et aucune session ne s'ouvre à une génération dont le client ne sert pas encore les chartes.
- **Suppression chez le vérificateur d'identité** : le réessai a lieu à chaque passage de la tâche horaire, pendant 7 jours depuis `depuisLe`, sans réservation : la suppression est irréversible, et deux demandes ont l'effet d'une seule.
- **Dépôt `ouverture`** : rien n'est écrit au registre ; la réponse suit la seule validation par la grappe.
- **Premier dépôt du PIN** : `reessayer_plus_tard` y est possible, comme avant un dépôt dans une autre grappe (décision 0058), et `API.md` le dit.
- **`/etat`** : aucune écriture ; une ligne rejetée y est publiée et alertée comme à toute lecture. Le `200` de `/vie` sur une ligne rejetée compte pour le client comme une preuve de vie réussie : seule l'alerte de Sésame prévient le titulaire.
- **`grappe/rejoindre`** : 4 demandes par minute et par `nodeId`, le double du rythme de renouvellement.

## Alternatives écartées

- **Borne qui porte un état courant, ou plafond d'entrées** : un état à tenir entre les bornes, ou une règle de troncature à écrire et à vérifier par le script public, pour un volume borné par les ressources réellement en défaut.
- **Date du dernier essai dans `redactionsEnAttente[]`, ou réservation du réessai** : un champ et une transaction de plus pour éviter une demande en double que le vérificateur traite sans dommage.
- **Écriture signée pour un dépôt `ouverture`** : deux signatures et une mise à jour de la tête de chaîne pour ne rien changer.
- **Refus d'une autre KTS à génération égale** : l'appareil perdu pendant une rotation ne pourrait plus la reprendre.

Pratique reprise : la suppression d'une session de vérification est irréversible, et la session supprimée reste lisible, ses données remplacées (https://docs.stripe.com/api/identity/verification_sessions/redact).

## Conséquences et limites

Une ligne rejetée lue souvent apparaît dans chaque borne de sa période. Pendant une pause de la grappe, plusieurs VM peuvent demander la même suppression. Un titulaire dont la ligne est rejetée ne voit rien dans l'app ; il reçoit l'alerte de Sésame.

## Documents concernés

`API.md` §2.3, §2.5, §2.12, §4.1, §6, §7 ; `ARCHITECTURE.md` §2.2, §8.1, §11 ; `THREAT_MODEL.md`, limite 25 ; `SECURITY.md`.
