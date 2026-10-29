<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0058 · Aucun refus au démarrage ne fait sortir le processus ; ligne relue avant une ouverture ; premier dépôt du PIN

**Date** : 2026-10-29 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · revoit [0010](0010-fournisseurs-nommes-dans-le-code.md), [0033](0033-textes-du-client-attestes-par-leur-empreinte.md) et [0057](0057-place-inscrite-et-instance-qui-refuse-de-servir.md)

## Contexte et menace

Trois cas de démarrage refusé (variable attestée manquante ou vide, nom de VM qui ne se termine pas par `-a`, `-b` ou `-c`, document des textes refusé) faisaient « refuser de démarrer » l'instance, alors que la décision 0057 avait déjà constaté qu'un processus qui sort boucle. Sur l'image de production de Confidential Space, une charge qui sort arrête la VM, ou la redémarre selon `tee-restart-policy` (`Never`, `Always` ou `OnFailure`, sans nombre maximal d'essais) ; un groupe d'instances géré répare toute VM arrêtée en la recréant. Rien ne borne donc les redémarrages, et seuls les textes refusés étaient journalisés. Deux comportements n'étaient pas écrits non plus : la réponse des routes de session qui relisent la ligne du titulaire quand elle est rejetée, absente ou `supprime`, et le premier dépôt du PIN d'un titulaire qui n'a pas encore de `grappePin`.

## Décision

- **Refus au démarrage** : une instance dont la configuration attestée est refusée (variable attestée manquante ou vide, nom de VM hors des trois places, document des textes d'une autre empreinte ou incomplet) se met dans l'état de l'instance qui refuse de servir (0057) : le processus ne sort pas, répond `attestation_indisponible` à toute route, ne demande pas à joindre de grappe et ne dépose aucune borne. Elle journalise `configuration_refusee`, qui remplace l'événement des textes refusés : les alertes à l'exploitant restent au nombre de sept. L'exploitant recrée la VM une fois la cause levée.
- **Ligne relue avant une ouverture** : `/session/ouvrir-cles`, et `/session/verifier` comme `/session/identite/verifier` avant d'inscrire une ouverture, ferment la session (`sessionTerminee`) et répondent `session_expiree` si la ligne du titulaire est rejetée, absente ou `supprime` ; aucune ouverture n'est inscrite, aucun jeton n'est émis.
- **Premier dépôt du PIN** : sans `grappePin`, le dépôt est admis comme un changement de grappe sans échéance à attendre : contrôle anti-retour exigé, puis `grappePin` écrit avec la grappe en service et `depuisLe` à l'heure du dépôt.

## Alternatives écartées

- **Sortir avec un code stable et borner les redémarrages** : ni le lanceur de Confidential Space ni le groupe d'instances géré n'offrent de borne ; couper la réparation du groupe retirerait aussi le remplacement automatique d'un membre.
- **Politique `Never`** : une VM arrêtée est quand même recréée par son groupe, et une vraie panne du processus ne serait plus relancée.
- **Un événement par cause** : trois noms d'alerte pour une seule action de l'exploitant, qui lit la cause dans le journal.
- **`registre_indisponible` ou `titulaire_indisponible` pour une ligne rejetée pendant une session** : le portail a déjà un seul traitement de fin de session ; un code de plus ne change rien pour le proche.
- **Premier dépôt admis sans contrôle anti-retour** : un registre ramené avant le premier dépôt ferait passer pour un premier dépôt le changement de grappe d'un titulaire entré depuis moins de 30 jours.

Pratiques reprises : codes de retour du lanceur de Confidential Space (0 et 1 arrêtent la VM, 3 la redémarre selon `tee-restart-policy`, https://docs.cloud.google.com/confidential-computing/confidential-space/docs/monitor-debug) et valeurs de cette politique (https://docs.cloud.google.com/confidential-computing/confidential-space/docs/deploy-workloads) ; réparation d'une VM arrêtée par son groupe d'instances géré (https://docs.cloud.google.com/compute/docs/instance-groups/about-repair) ; échec fermé.

## Conséquences et limites

Une instance mal configurée reste visible (alerte immédiate, votant absent) sans tourner en boucle ; la grappe tient avec les deux autres votants. Un proche dont la ligne du titulaire est rejetée pendant sa session la perd à sa requête suivante, ce qui ne sert que la disponibilité : l'alerte `ligne_rejetee` part déjà au titulaire.

## Documents concernés

`API.md` §1.2, §2.15, §2.17, §2.19, §4.3 et table des codes d'erreur ; `ARCHITECTURE.md` §2.3, §4, §9.3 ; `THREAT_MODEL.md`, limites 19 et 22.
