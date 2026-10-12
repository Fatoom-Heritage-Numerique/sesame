<!-- Source : fatoom-app docs/TRD.md §16.9 ; maquette « Fatoom · Maquettes », planche SiteSesame (« Vous avez trouvé une faille ? ») -->

# Signaler une faille

Vous avez trouvé une faille ? Écrivez-nous avant de la rendre publique, nous vous répondrons rapidement.

## Où

- **Email** : <support@fatoom.fr>, objet commençant par `[Sésame]`.
- Si le rapport contient de quoi ouvrir une clé ou contourner une vérification, chiffrez-le : demandez-nous une clé publique par un premier message sans détail, nous vous la renvoyons sous 72 h.
- **Pas d'issue publique** pour une faille, même mineure : le gabarit d'issue « Faille » existe seulement pour les points déjà corrigés et publiés, ou pour une faille dont l'exploitation est impossible sans l'enclave et que nous avons convenu ensemble de traiter en public.

## Ce que nous nous engageons à faire

| Étape | Délai |
|---|---|
| Accusé de réception | sous **72 h** |
| Premier diagnostic : reproduit, non reproduit, hors périmètre | sous **7 jours** |
| Point d'avancement | toutes les 2 semaines tant que le sujet est ouvert |
| Correctif | selon `RELEASE.md` : une version corrective attend **7 jours** entre son annonce et sa mise en service ; une version vulnérable peut être **retirée immédiatement**, les transmissions patientent |
| Publication | une fois le correctif en service, nous publions la faille, sa portée et votre nom si vous le souhaitez, dans `docs/decisions/` ou dans les notes de la version |

## Périmètre

Nous intéresse tout ce qui contredit `THREAT_MODEL.md` : une clé ouverte pour quelqu'un qui n'y a pas droit, une transmission avant 7 jours ou malgré une preuve de vie, un code révoqué qui ouvre encore, une attestation acceptée à tort par un client, une image non publiée qui obtient une clé de KMS, une fuite dans les journaux, un écart entre le code et les documents de ce dépôt, un build non reproductible.

Ne nous intéresse pas, parce que c'est assumé et écrit : les limites de `THREAT_MODEL.md` §2 (matériel et Google, IAM administrée par l'opérateur, codes d'accès reçus par le client, lettre vue par l'imprimeur, métadonnées en clair, disponibilité, collusion, horloge, perte du registre, clé de release unique, fournisseurs d'envoi). Si vous pensez qu'une de ces limites est plus grave que ce que nous écrivons, dites-le quand même : c'est une discussion, pas une faille.

## Ce que nous demandons

- Ne pas accéder à des données qui ne sont pas les vôtres ; pour tester, utilisez un coffre et des proches que vous avez créés.
- Ne pas déclencher d'envoi réel (SMS, email, lettre, vérification d'identité) vers des personnes qui n'ont pas accepté de participer.
- Nous laisser le temps du correctif avant toute publication ; nous ne demanderons jamais plus que les délais ci-dessus plus les 7 jours du manifeste.

## Bug bounty

Il n'y a **pas de programme de récompense** pour l'instant. Nous remercions publiquement, si vous le souhaitez, et nous disons ce que votre rapport a changé.
