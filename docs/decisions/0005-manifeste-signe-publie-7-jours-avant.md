<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0005 · Manifeste signé sur clé matérielle, publié 7 jours avant, retrait immédiat, sans voie rapide

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur) · **revue depuis** ([0014](0014-liste-des-versions-compilee.md)) : remplacée ; plus de manifeste signé ni de délai de 7 jours, la liste des versions `versions.json` est compilée dans les clients et une version se met en service par mise à jour forcée

## Contexte et menace

L'attestation prouve quelle image tourne ; elle ne dit pas si cette image est **celle que l'on a promise**. Sans liste de référence, l'opérateur pourrait déployer une image modifiée, ou un pirate de ses serveurs le faire à sa place, et les clients l'accepteraient. Il faut une liste des versions légitimes que ni les serveurs de l'opérateur ni ceux du client ne puissent altérer, et que chacun puisse lire à l'avance.

## Décision

Un **manifeste** signé liste les versions de Sésame (empreinte, référence d'image, `envDigest`, dates, statut). Il est signé sur une **clé matérielle** hors de tout serveur, publié dans ce dépôt et sur la page de transparence du client, **7 jours avant** toute mise en service (`activeFrom ≥ publishedAt + 7 j`). L'app, le portail et la passerelle refusent toute empreinte absente, annoncée ou retirée ; la liaison IAM de KMS n'autorise une empreinte qu'à sa mise en service et doit rester identique aux lignes `en_service` (contrôle CI). Une version se **retire immédiatement**. Il n'y a **pas de voie rapide** : un correctif attend 7 jours, les transmissions patientent.

## Alternatives écartées

Le registre d'origine ne consigne pas d'alternative pour cette décision. La spécification exclut explicitement : une voie rapide pour les correctifs (elle rendrait la publication préalable contournable) ; un manifeste servi par l'infrastructure du client qui sert les données (il est hébergé hors de celle-ci) ; un nombre de signatures exigé **lu dans le manifeste** (il est gravé dans les clients, sinon un manifeste à une signature pourrait l'abaisser).

## Conséquences et limites

Une image malveillante est visible publiquement 7 jours avant de pouvoir servir. Un correctif de sécurité attend lui aussi 7 jours ; en attendant, la version vulnérable est retirée et Sésame est indisponible (`THREAT_MODEL.md` §2, limite 8). La sécurité du manifeste repose sur la clé de release : décision 0009. Les clients gardent le manifeste en cache 24 h et le refusent au-delà de 30 jours.

## Documents concernés

`RELEASE.md` ; `ARCHITECTURE.md` §3, §11.6 ; `README.md` (« La liste des versions »).
