<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# Décisions

Un fichier par décision, en ajout seul : une décision revue est un **nouveau** fichier qui cite l'ancien, jamais une réécriture. Le corps d'une décision ne se réécrit pas ; seuls les renvois (sections, documents), les identifiants (noms de champs, de routes, de limites) et la ligne « revue depuis » se mettent à jour, pour que la décision reste lisible avec les documents actuels. La ligne de statut d'une décision revue s'écrit « **revue depuis** », suivie des décisions qui la revoient, sans date ni résumé : ce qui a changé est écrit dans ces décisions. C'est ici que vit l'historique du projet ; les autres documents décrivent l'état cible sans dates ni numéros de version.

Chaque fichier suit le même gabarit : date et statut, contexte et menace, décision, alternatives écartées (telles que consignées au moment de la décision ; quand le registre d'origine n'en consigne pas, le fichier le dit), conséquences et limites, documents de ce dépôt concernés.

| # | Décision | Date |
|---|---|---|
| [0001](0001-enclave-confidential-space.md) | Les clés des contenus destinés s'ouvrent dans une enclave au code public | 2026-10-12 · revue depuis 0014 |
| [0002](0002-kms-hsm-cle-de-transmission-par-titulaire.md) | Cloud KMS HSM conditionné à l'attestation, une clé de transmission par titulaire | 2026-10-12 · revue depuis 0014 |
| [0003](0003-registre-minimal-tenu-par-sesame.md) | Un registre minimal tenu par Sésame : 7 jours, preuve de vie, générations | 2026-10-12 · revue depuis 0020, 0021, 0022, 0024, 0039 |
| [0004](0004-second-facteur-envoye-et-verifie-par-sesame.md) | Le dernier code est envoyé et vérifié par Sésame | 2026-10-12 · revue depuis 0029 |
| [0005](0005-manifeste-signe-publie-7-jours-avant.md) | Manifeste signé sur clé matérielle, publié 7 jours avant, retrait immédiat, sans voie rapide | 2026-10-12 · revue depuis 0014 |
| [0006](0006-verification-en-direct-et-par-cache.md) | Vérification de Sésame en direct pour enrôler et changer une charte, par manifeste en cache pour destiner | 2026-10-12 · revue depuis 0014, 0017, 0020, 0023 |
| [0007](0007-depot-public-separe.md) | Un dépôt public séparé du client | 2026-10-12 · revue depuis 0014 |
| [0008](0008-lettre-et-identite-dans-sesame.md) | Lettre au proche sans téléphone ni email et vérification d'identité menées par Sésame | 2026-10-12 · revue depuis 0027, 0036 |
| [0009](0009-cle-de-release-sur-cle-materielle.md) | Une clé de release sur clé matérielle au départ, nombre de signatures gravé dans les clients | 2026-10-12 · revue depuis 0014 |
| [0010](0010-fournisseurs-nommes-dans-le-code.md) | Fournisseurs nommés dans le code, réglages du client en variables attestées, clés d'API dans Secret Manager | 2026-10-12 · revue depuis 0014, 0023, 0028, 0029, 0058 |
| [0011](0011-go-binaire-statique-distroless.md) | Go, binaire statique, image distroless, scripts gcloud ; ordre de construction | 2026-10-12 · revue depuis 0015, 0016 |
| [0012](0012-canaux-d-envoi-non-croises.md) | Canaux d'envoi non croisés : un fournisseur piraté peut ouvrir après un décès vérifié | 2026-10-12 · revue depuis 0013 |
| [0013](0013-canaux-d-envoi-croises.md) | Canaux d'envoi croisés : code d'accès par email, dernier code par SMS | 2026-10-22 |
| [0014](0014-liste-des-versions-compilee.md) | Liste des versions compilée dans les clients, mise à jour forcée | 2026-10-25 · revue depuis 0025, 0026, 0032 |
| [0015](0015-budget-de-code-6000-lignes.md) | Budget de code porté à 6 000 lignes de Go | 2026-10-26 |
| [0016](0016-bibliotheques-oprf-argon2-raft.md) | Bibliothèques admises : `circl` pour l'OPRF, `x/crypto/argon2`, `go.etcd.io/raft/v3` | 2026-10-24 et 2026-10-27 |
| [0017](0017-protocole-de-pin-grappe-de-trois-vm.md) | Protocole de PIN en mémoire, sur une grappe de trois VM à majorité, et règle des 30 jours | 2026-10-24 · revue depuis 0018, 0042, 0043, 0050 |
| [0018](0018-amorcage-commande-d-une-grappe-neuve.md) | Amorçage d'une grappe neuve commandé par l'exploitant | 2026-10-27 · revue depuis 0055 |
| [0019](0019-canal-hybride-x25519-ml-kem.md) | Canal chiffré hybride X25519 et ML-KEM-768 vers l'enclave | 2026-10-23 |
| [0020](0020-annulation-explicite-et-transmission-definitive.md) | Annulation explicite d'un signalement de décès, transmission définitive après l'échéance | 2026-10-22 et 2026-10-26 |
| [0021](0021-registre-signe-et-borne-publique.md) | Registre en ajout seul signé par `registre-sign`, borne publique du registre | 2026-10-26 · revue depuis 0040, 0045 |
| [0022](0022-reprise-publiee-avec-ou-sans-sauvegarde.md) | Export horaire, toute restauration par une reprise publiée, ré-enrôlement d'un coffre sans ligne | 2026-10-27 · revue depuis 0040, 0051 |
| [0023](0023-alertes-envoyees-par-sesame-et-fiche-d-alerte.md) | Alertes envoyées par Sésame lui-même, fiche d'alerte scellée par Sésame | 2026-10-24 |
| [0024](0024-signe-de-vie-transmission-par-inactivite.md) | Signe de vie : transmission par inactivité | 2026-10-22 |
| [0025](0025-noms-francais-des-champs.md) | Noms français des champs des formats publiés | 2026-10-27 · revue depuis 0032 |
| [0026](0026-cles-session-sign-dans-la-liste-des-versions.md) | Clés publiques de `session-sign` dans la liste des versions | 2026-10-27 · revue depuis 0044, 0045 |
| [0027](0027-lettre-et-identite-precisees.md) | Lettre et identité : compte Pingen distinct, trois champs, bascule, lettre de secours, suppression chez Stripe | 2026-10-23 · revue depuis 0034, 0046 |
| [0028](0028-reglages-du-client-env-override-entier.md) | Réglages du client : dictionnaire `env_override` exigé en entier, textes longs dans l'image | 2026-10-26 · revue depuis 0033, 0045 |
| [0029](0029-dernier-code-par-email-envoye-par-sesame.md) | Dernier code par email tiré et envoyé par Sésame lui-même ; Twilio Verify pour le SMS seulement | 2026-10-27 · revue depuis 0033 |
| [0030](0030-preuve-des-signalements-insee.md) | Preuve des signalements de la source `insee` | 2026-10-27 |
| [0031](0031-refus-du-role-sans-invalider-la-charte.md) | Refus du rôle depuis le portail sans invalider la charte | 2026-10-27 |
| [0032](0032-chemin-du-depot-d-images-hors-de-la-liste.md) | Chemin du dépôt d'images hors de la liste des versions | 2026-10-27 |
| [0033](0033-textes-du-client-attestes-par-leur-empreinte.md) | Textes du client réglés au lancement, attestés par leur empreinte | 2026-10-27 · revue depuis 0058 |
| [0034](0034-lettre-de-secours-pour-un-seul-canal.md) | Lettre de secours pour un proche joint par un seul canal | 2026-10-28 |
| [0035](0035-ce-qu-expose-une-faille-de-l-enclave.md) | Ce qu'expose une faille de l'enclave, écrit sans promesse de plus | 2026-10-28 |
| [0036](0036-code-d-invitation-scelle-au-registre.md) | Code d'invitation des lettres scellé au registre par Sésame | 2026-10-28 · revue depuis 0041 |
| [0037](0037-essais-des-codes-comptes-au-registre.md) | Essais des codes d'un proche comptés au registre | 2026-10-28 · revue depuis 0038, 0048 |
| [0038](0038-blocage-de-15-minutes-apres-le-5e-code-faux.md) | Blocage de 15 minutes pleines après le 5ᵉ code faux | 2026-10-28 · revue depuis 0048 |
| [0039](0039-ouvertures-sans-detail-des-contenus.md) | Ouvertures sans le détail des contenus ouverts | 2026-10-28 |
| [0040](0040-bornes-en-echec-hors-du-controle-anti-retour.md) | Les bornes déposées en échec ne comptent pas dans le contrôle anti-retour | 2026-10-28 · revue depuis 0051 |
| [0041](0041-code-d-invitation-verifie-par-son-scelle.md) | Code d'invitation vérifié par son scellé, sans empreinte au registre | 2026-10-28 |
| [0042](0042-ecart-de-30-jours-sauf-faille.md) | Deux versions à moins de 30 jours d'écart seulement pour une faille | 2026-10-28 · revue depuis 0049, 0052 |
| [0043](0043-nouveau-pin-apres-recuperation-et-annulation.md) | Après le code de récupération, le nouveau PIN attend la grappe ; l'annulation d'un décès, non | 2026-10-28 · revue depuis 0047 |
| [0044](0044-requetes-a-la-passerelle-sans-kid.md) | Requêtes à la passerelle vérifiées contre chaque clé de `clesSession` | 2026-10-28 |
| [0045](0045-version-de-signature-fixee-par-l-image.md) | Version de signature fixée par l'image, une clause de la condition KMS par version en service | 2026-10-28 |
| [0046](0046-nouvel-envoi-d-une-lettre-au-passage-suivant.md) | Nouvel envoi d'une lettre au passage de la tâche qui relève l'erreur | 2026-10-28 |
| [0047](0047-recuperation-dans-les-30-jours-d-un-changement-de-grappe.md) | Dans les 30 jours d'un changement de grappe, le code de récupération ne rouvre pas le coffre sur un nouvel appareil ; l'annulation d'un décès reste possible | 2026-10-28 |
| [0048](0048-tout-code-refuse-decompte.md) | Tout code refusé est décompté, code juste compris | 2026-10-28 |
| [0049](0049-retrait-d-urgence-hors-faille-ecrit.md) | Retrait d'urgence hors faille : l'arrêt jusqu'à la version suivante est écrit, sans procédure nouvelle | 2026-10-28 · revue depuis 0051 |
| [0050](0050-remplacement-d-un-membre-sans-quatrieme-votant.md) | Remplacement d'un membre de la grappe : l'absent est retiré avant la promotion de la remplaçante | 2026-10-28 · revue depuis 0057 |
| [0051](0051-arret-de-plus-de-2-heures-et-premiere-version.md) | Après un arrêt de plus de 2 heures, et à la toute première version, une reprise publiée sans mécanisme nouveau | 2026-10-28 · revue depuis 0052 |
| [0052](0052-reprise-apres-une-panne-sans-attendre-30-jours.md) | Une reprise publiée après une panne de plus de 2 heures n'attend pas l'échéance des 30 jours | 2026-10-28 |
| [0053](0053-recopie-au-registre-avant-la-reponse-du-pin.md) | L'état du PIN est recopié au registre avant la réponse ; sans KMS, ni dépôt ni évaluation | 2026-10-28 · revue depuis 0054 |
| [0054](0054-essai-decompte-sans-recopie-signale.md) | Un essai décompté dont la recopie échoue est signalé ; un retrait d'urgence ne décompte aucun essai | 2026-10-28 |
| [0055](0055-amorcage-par-place-et-pairs-derives-du-nom.md) | Amorçage d'une grappe sur trois places, quelle que soit la zone ; pairs dérivés du nom de la VM | 2026-10-28 · revue depuis 0057 |
| [0056](0056-ordre-des-controles-et-precisions-de-la-grappe.md) | Ligne lue avant la signature `cleVie` ; `grappePin` non défait ; rythme des admissions et des alertes | 2026-10-29 |
| [0057](0057-place-inscrite-et-instance-qui-refuse-de-servir.md) | Place de chaque membre inscrite dans l'état répliqué ; instance qui refuse de servir ; contrôles avant `grappePin` | 2026-10-29 · revue depuis 0058 |
| [0058](0058-aucun-refus-au-demarrage-ne-fait-sortir-le-processus.md) | Aucun refus au démarrage ne fait sortir le processus ; ligne relue avant une ouverture ; premier dépôt du PIN | 2026-10-29 |
| [0059](0059-lectures-locales-jeton-federe-et-fenetre-de-rotation.md) | État du PIN lu sur la copie locale ; jeton fédéré gardé jusqu'à son expiration ; fenêtre d'une rotation interrompue | 2026-10-29 |
| [0060](0060-borne-rescellement-et-reessais-sans-reservation.md) | Événements d'une borne par période ; rescellement d'une autre KTS à génération égale ; suppression d'identité réessayée sans réservation | 2026-10-29 |
| [0061](0061-nature-du-signalement-notifications-en-memoire-et-pertes-d-une-restauration.md) | Nature d'un signalement ; notifications réessayées en mémoire ; pertes d'une restauration assumées | 2026-10-29 |
| [0062](0062-fusion-sans-delai-avant-la-mise-en-service.md) | Fusion sans délai tant qu'aucune version n'est en service | 2026-11-11 |
| [0063](0063-sesame-fake-contrat-executable.md) | `sesame-fake` : le code des décisions avec des doubles, deux modes, configuration reçue du client | 2026-11-11 |
| [0064](0064-openapi-et-vecteurs-du-contrat.md) | OpenAPI en JSON vérifiée contre `sesame-fake` ; vecteurs sous `testdata/vecteurs/` ; `compteurPin` généré | 2026-11-11 |
| [0065](0065-ci-du-contrat-executable.md) | CI du contrat exécutable ; contrôles de l'infrastructure écrits d'avance ; noms du déploiement cherchés hors de ce dépôt | 2026-11-11 · revue depuis 0066 |
| [0066](0066-copies-des-vecteurs-hors-de-la-part-structurelle.md) | Copies des vecteurs hors de la part structurelle du contrôle des noms | 2026-11-11 |
| [0067](0067-circl-et-x-crypto-verifiees-puis-epinglees.md) | `circl` v1.6.5 et `golang.org/x/crypto` v0.58.0 vérifiées sur leurs vecteurs, puis épinglées | 2026-11-11 |
