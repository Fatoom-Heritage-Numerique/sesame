<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# Décisions

Un fichier par décision, en ajout seul : une décision revue est un **nouveau** fichier qui cite l'ancien, jamais une réécriture. C'est ici que vit l'historique du projet ; les autres documents décrivent l'état cible sans dates ni numéros de version.

Chaque fichier suit le même gabarit : date et statut, contexte et menace, décision, alternatives écartées (telles que consignées au moment de la décision ; quand le registre d'origine n'en consigne pas, le fichier le dit), conséquences et limites, documents de ce dépôt concernés.

| # | Décision | Date |
|---|---|---|
| [0001](0001-enclave-confidential-space.md) | Les clés des contenus destinés s'ouvrent dans une enclave au code public, pas chez le client | 2026-10-12 |
| [0002](0002-kms-hsm-cle-de-transmission-par-titulaire.md) | Libération des clés par Cloud KMS HSM conditionné à l'attestation, une clé de transmission par titulaire | 2026-10-12 |
| [0003](0003-registre-minimal-tenu-par-sesame.md) | Registre minimal tenu par Sésame : 7 jours, preuve de vie, générations de chartes | 2026-10-12 |
| [0004](0004-second-facteur-envoye-et-verifie-par-sesame.md) | Le dernier code est envoyé et vérifié par Sésame, pas par le client | 2026-10-12 |
| [0005](0005-manifeste-signe-publie-7-jours-avant.md) | Manifeste signé sur clé matérielle, publié 7 jours avant, retrait immédiat, sans voie rapide | 2026-10-12 · remplacée par 0014 |
| [0006](0006-verification-en-direct-et-par-cache.md) | Vérification de Sésame en direct pour enrôler et changer une charte, par manifeste en cache pour destiner | 2026-10-12 |
| [0007](0007-depot-public-separe.md) | Un dépôt public séparé du client | 2026-10-12 |
| [0008](0008-lettre-et-identite-dans-sesame.md) | Lettre au proche sans téléphone ni email et vérification d'identité menées par Sésame | 2026-10-12 |
| [0009](0009-cle-de-release-sur-cle-materielle.md) | Une clé de release sur clé matérielle au départ, nombre de signatures gravé dans les clients | 2026-10-12 · remplacée par 0014 |
| [0010](0010-fournisseurs-nommes-dans-le-code.md) | Fournisseurs nommés dans le code, réglages du client en variables attestées, clés d'API dans Secret Manager | 2026-10-12 |
| [0011](0011-go-binaire-statique-distroless.md) | Go, binaire statique, image distroless, scripts gcloud ; ordre de construction | 2026-10-12 |
| [0012](0012-canaux-d-envoi-non-croises.md) | Canaux d'envoi non croisés : un fournisseur piraté peut ouvrir après un décès vérifié (limite assumée, à revoir) | 2026-10-12 · remplacée par 0013 |
| [0013](0013-canaux-d-envoi-croises.md) | Canaux d'envoi croisés : code d'accès par email, dernier code par SMS | 2026-10-22 |
| [0014](0014-liste-des-versions-compilee.md) | Liste des versions compilée dans les clients, mise à jour forcée ; plus de manifeste signé, de clé de release ni de délai de 7 jours | 2026-10-25 |
