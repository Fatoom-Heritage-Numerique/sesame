<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0036 · Code d'invitation des lettres scellé au registre par Sésame

**Date** : 2026-10-28 · **Statut** : actée (validée par l'opérateur) · revoit [0008](0008-lettre-et-identite-dans-sesame.md) · **revue depuis** ([0041](0041-code-d-invitation-verifie-par-son-scelle.md))

## Contexte et menace

Les renvois de J+15 et J+45 et le nouvel envoi après une erreur d'impression portent le même code que la première lettre. Or le registre ne gardait du code que son index (`lookup`) et son `codeVerifier` : rien ne disait où Sésame retrouvait ce code pour réimprimer la lettre, ni ce qui arrivait si une VM tombait entre la réservation d'un envoi au registre et la commande chez Pingen. Le code ne doit jamais passer par le client.

## Décision

- Sésame scelle le code d'invitation d'une lettre sous une clé KMS qui lui est propre (`invitation-seal` ; HSM, non exportable, sous la même condition d'attestation que les autres clés du trousseau) et le garde dans `lettres/{contactId}` (`codeInvitationScelle`, AAD `"fatoom-sesame-invitation|1|" ‖ uid ‖ "|" ‖ contactId ‖ "|" ‖ lookup`).
- Il ne le descelle que pour réimprimer la même lettre : renvois de J+15 et J+45, nouvel envoi après une erreur, reprise d'un envoi interrompu avant Pingen. Aucune route ne le rend et aucune notification ne le porte.
- Chaque descellement est un `Decrypt` inscrit au journal « Data Access » de KMS, exporté au bucket public d'audit.
- Une lettre qui n'est pas une réimpression (lettre de secours, lettre d'une nouvelle transmission) porte un nouveau code, scellé à son tour ([0034](0034-lettre-de-secours-pour-un-seul-canal.md)).

## Alternatives écartées

- **Tirer un nouveau code à chaque envoi et réécrire `invitations/{lookup}`** : un proche qui a reçu deux lettres ne saurait pas laquelle vaut, et chaque renvoi invaliderait la lettre déjà reçue.

## Conséquences et limites

Le registre garde un scellé de plus par proche « courrier », que seule la charge attestée ouvre. Une image autorisée en douce dans KMS descellerait ces codes comme les KTS, sans rien gagner de plus (`THREAT_MODEL.md`, limite 2). Une VM qui tombe après la commande chez Pingen et avant l'écriture de son identifiant peut faire poster deux fois la même lettre, jamais une lettre à un autre code.

## Documents concernés

`ARCHITECTURE.md` §1, §4, §6.1, §11.9 ; `API.md` (`lettre/secours`) ; `THREAT_MODEL.md` §2 (2).
