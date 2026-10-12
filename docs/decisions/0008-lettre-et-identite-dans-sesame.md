<!-- Source : fatoom-app docs/TRD.md §12 (2026-10-12, « Proche prévenu par lettre : tout dans Sésame »), §16.5.8 ; docs/PRD.md §9 (R-DECES-03, R-PORTAIL-01), §10 (Q-30) -->

# 0008 · Lettre au proche sans téléphone ni email et vérification d'identité menées par Sésame

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur : « tout sur Sésame, en réduisant au maximum la taille du code ») · Résout la question Q-30 du client

## Contexte et menace

Un proche qui n'a ni téléphone ni email reçoit son code par lettre, au décès. Le canal postal ne prouve rien : le second facteur est une vérification d'identité (pièce et selfie). Si le client imprimait la lettre, il connaîtrait le code ; s'il menait la vérification d'identité, il pourrait l'attester à tort. Dans les deux cas, un client compromis ouvrirait les contenus destinés aux proches « courrier ».

## Décision

Tout se passe dans **Sésame** : il tire le **code d'invitation**, produit la lettre (PDF texte d'une page, générateur maison déterministe), la poste par Pingen via une organisation **distincte** de celle du client, à l'**adresse scellée dans la charte** par le titulaire ; il crée et relit la session Stripe Identity avec une clé restreinte et compare la pièce au prénom, au nom et à la date de naissance scellés dans la charte. Le client ne connaît ni le code ni le résultat ; il reçoit seulement les notifications de suivi (lettre envoyée, statut, échecs) pour l'historique et le support.

## Alternatives écartées

- **Le client atteste l'identité** auprès de Sésame : un client compromis ouvrirait les contenus des proches « courrier ».
- **Le contact de confiance se porte garant** du proche : humain, et dépend d'un gardien joignable.
- **HTML envoyé tel quel à Pingen** au lieu d'un PDF maison : non confirmé par Pingen ; à vérifier, le générateur PDF disparaît si c'est possible.
- Avant la décision : une clé d'attestation d'identité côté client (`IDENTITY_ATTEST_KEY`) avait été envisagée ; abandonnée avant d'exister.

## Conséquences et limites

Sésame embarque trois appels REST pour Pingen, deux pour Stripe, un générateur PDF d'environ 150 lignes, et une tâche horaire (envoi à `transmissibleLe`, suivi, renvois J+15 et J+45, trois erreurs puis échec). Pingen voit la lettre et son code ; Stripe voit la pièce et le selfie ; après une première identité vérifiée, la lettre seule rouvre l'accès pendant 12 mois (`THREAT_MODEL.md` §2, limite 4). Le client ne peut que retenir une charte « courrier », jamais la modifier : un déni de service visible.

## Documents concernés

`ARCHITECTURE.md` §9.8 ; `API.md` §2.7, §3.2, §3.3 ; `THREAT_MODEL.md` §2 (4).
