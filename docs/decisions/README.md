<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# Décisions

Un fichier par décision, en ajout seul : une décision revue est un **nouveau** fichier qui cite l'ancien, jamais une réécriture. Le corps d'une décision ne se réécrit pas ; seuls les renvois (sections, documents), les identifiants (noms de champs, de routes, de limites) et la ligne « revue depuis » se mettent à jour, pour que la décision reste lisible avec les documents actuels. La ligne de statut d'une décision revue s'écrit « **revue depuis** », suivie des décisions qui la revoient, sans date ni résumé : ce qui a changé est écrit dans ces décisions. C'est ici que vit l'historique du projet ; les autres documents décrivent l'état cible sans dates ni numéros de version.

Chaque fichier suit le même gabarit : date et statut, contexte et menace, décision, alternatives écartées (telles que consignées au moment de la décision ; quand le registre d'origine n'en consigne pas, le fichier le dit), conséquences et limites, documents de ce dépôt concernés.

| # | Décision | Date |
|---|---|---|
| [0001](0001-enclave-confidential-space.md) | Les clés des contenus destinés s'ouvrent dans une enclave au code public, pas chez le client | 2026-10-12 · revue depuis 0014 |
| [0002](0002-kms-hsm-cle-de-transmission-par-titulaire.md) | Libération des clés par Cloud KMS HSM conditionné à l'attestation, une clé de transmission par titulaire | 2026-10-12 · revue depuis 0014 |
| [0003](0003-registre-minimal-tenu-par-sesame.md) | Registre minimal tenu par Sésame : 7 jours, preuve de vie, générations de chartes | 2026-10-12 · revue depuis 0020, 0021, 0022, 0024 |
| [0004](0004-second-facteur-envoye-et-verifie-par-sesame.md) | Le dernier code est envoyé et vérifié par Sésame, pas par le client | 2026-10-12 · revue depuis 0029 |
| [0005](0005-manifeste-signe-publie-7-jours-avant.md) | Manifeste signé sur clé matérielle, publié 7 jours avant, retrait immédiat, sans voie rapide | 2026-10-12 · revue depuis 0014 |
| [0006](0006-verification-en-direct-et-par-cache.md) | Vérification de Sésame en direct pour enrôler et changer une charte, par manifeste en cache pour destiner | 2026-10-12 · revue depuis 0014, 0017, 0020, 0023 |
| [0007](0007-depot-public-separe.md) | Un dépôt public séparé du client | 2026-10-12 · revue depuis 0014 |
| [0008](0008-lettre-et-identite-dans-sesame.md) | Lettre au proche sans téléphone ni email et vérification d'identité menées par Sésame | 2026-10-12 · revue depuis 0027 |
| [0009](0009-cle-de-release-sur-cle-materielle.md) | Une clé de release sur clé matérielle au départ, nombre de signatures gravé dans les clients | 2026-10-12 · revue depuis 0014 |
| [0010](0010-fournisseurs-nommes-dans-le-code.md) | Fournisseurs nommés dans le code, réglages du client en variables attestées, clés d'API dans Secret Manager | 2026-10-12 · revue depuis 0014, 0023, 0028, 0029 |
| [0011](0011-go-binaire-statique-distroless.md) | Go, binaire statique, image distroless, scripts gcloud ; ordre de construction | 2026-10-12 · revue depuis 0015, 0016 |
| [0012](0012-canaux-d-envoi-non-croises.md) | Canaux d'envoi non croisés : un fournisseur piraté peut ouvrir après un décès vérifié | 2026-10-12 · revue depuis 0013 |
| [0013](0013-canaux-d-envoi-croises.md) | Canaux d'envoi croisés : code d'accès par email, dernier code par SMS | 2026-10-22 |
| [0014](0014-liste-des-versions-compilee.md) | Liste des versions compilée dans les clients, mise à jour forcée ; plus de manifeste signé, de clé de release ni de délai de 7 jours | 2026-10-25 · revue depuis 0025, 0026, 0032 |
| [0015](0015-budget-de-code-6000-lignes.md) | Budget de code porté à 6 000 lignes de Go | 2026-10-26 |
| [0016](0016-bibliotheques-oprf-argon2-raft.md) | Bibliothèques admises : `circl` pour l'OPRF, `x/crypto/argon2`, `go.etcd.io/raft/v3` | 2026-10-24 et 2026-10-27 |
| [0017](0017-protocole-de-pin-grappe-de-trois-vm.md) | Protocole de PIN en mémoire, sur une grappe de trois VM à majorité, et règle des 30 jours | 2026-10-24 · revue depuis 0018 |
| [0018](0018-amorcage-commande-d-une-grappe-neuve.md) | Amorçage d'une grappe neuve commandé par l'exploitant | 2026-10-27 |
| [0019](0019-canal-hybride-x25519-ml-kem.md) | Canal chiffré hybride X25519 et ML-KEM-768 vers l'enclave | 2026-10-23 |
| [0020](0020-annulation-explicite-et-transmission-definitive.md) | Annulation explicite d'un signalement de décès, transmission définitive après l'échéance | 2026-10-22 et 2026-10-26 |
| [0021](0021-registre-signe-et-borne-publique.md) | Registre en ajout seul signé par `registre-sign`, borne publique du registre | 2026-10-26 |
| [0022](0022-reprise-publiee-avec-ou-sans-sauvegarde.md) | Export horaire, toute restauration par une reprise publiée, ré-enrôlement d'un coffre sans ligne | 2026-10-27 |
| [0023](0023-alertes-envoyees-par-sesame-et-fiche-d-alerte.md) | Alertes envoyées par Sésame lui-même, fiche d'alerte scellée par Sésame | 2026-10-24 |
| [0024](0024-signe-de-vie-transmission-par-inactivite.md) | Signe de vie : transmission par inactivité | 2026-10-22 |
| [0025](0025-noms-francais-des-champs.md) | Noms français des champs des formats publiés | 2026-10-27 · revue depuis 0032 |
| [0026](0026-cles-session-sign-dans-la-liste-des-versions.md) | Clés publiques de `session-sign` dans la liste des versions | 2026-10-27 |
| [0027](0027-lettre-et-identite-precisees.md) | Lettre et identité : compte Pingen distinct, trois champs, bascule, lettre de secours, suppression chez Stripe | 2026-10-23 |
| [0028](0028-reglages-du-client-env-override-entier.md) | Réglages du client : dictionnaire `env_override` exigé en entier, textes longs dans l'image | 2026-10-26 · revue depuis 0033 |
| [0029](0029-dernier-code-par-email-envoye-par-sesame.md) | Dernier code par email tiré et envoyé par Sésame lui-même ; Twilio Verify pour le SMS seulement | 2026-10-27 · revue depuis 0033 |
| [0030](0030-preuve-des-signalements-insee.md) | Preuve des signalements de la source `insee` | 2026-10-27 |
| [0031](0031-refus-du-role-sans-invalider-la-charte.md) | Refus du rôle depuis le portail sans invalider la charte | 2026-10-27 |
| [0032](0032-chemin-du-depot-d-images-hors-de-la-liste.md) | Chemin du dépôt d'images hors de la liste des versions | 2026-10-27 |
| [0033](0033-textes-du-client-attestes-par-leur-empreinte.md) | Textes du client réglés au lancement, attestés par leur empreinte | 2026-10-27 |
