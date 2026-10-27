<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0014 · Liste des versions compilée dans les clients, mise à jour forcée

**Date** : 2026-10-25 · **Statut** : actée (décision explicite de l'opérateur) · remplace [0005](0005-manifeste-signe-publie-7-jours-avant.md) et [0009](0009-cle-de-release-sur-cle-materielle.md) · **revue depuis** ([0025](0025-noms-francais-des-champs.md), [0026](0026-cles-session-sign-dans-la-liste-des-versions.md))

## Contexte et menace

Les clients doivent refuser toute version de Sésame qui n'a pas été publiée. Les décisions 0005 et 0009 confiaient cette liste à un manifeste signé sur une clé matérielle, re-signé tous les 30 jours, annoncé 7 jours avant toute mise en service.

## Décision

Le manifeste signé, sa re-signature, la clé de release, le numéro de séquence, l'expiration et l'annonce de 7 jours sont **retirés**. Les versions autorisées sont listées dans `versions.json` (ce dépôt, branche `main` protégée : `version`, `imageDigest`, `referenceImage`, `envDigest`, `hwmodels`, `tag`, `commit`, `publieLe`, `statut` ∈ `en_service` | `retiree`, `retireLe`, `notes`) et **compilées** dans l'app, le portail et la passerelle du client au moment du build : ils ne téléchargent rien et ne vérifient aucune signature. Une empreinte absente ou `retiree`, ou un `hwmodel` hors de `hwmodels`, est refusée.

Mise en service : tag et build reproductible, ligne `en_service` ; versions des clients qui la contiennent publiées et disponibles dans les deux boutiques ; l'IAM KMS autorise l'empreinte (la liste d'abord, l'IAM ensuite) et la version démarre ; la version minimale de l'app est relevée (mise à jour forcée) ; l'ancienne version est arrêtée. Retrait d'urgence : ligne `retiree`, empreinte retirée de KMS immédiatement, version minimale relevée.

## Alternatives écartées

Le manifeste signé et la clé de release des décisions 0005 et 0009, retirés par cette décision.

## Conséquences et limites

L'ancre de confiance devient l'ensemble des comptes qui publient l'app, le site, les fonctions du client et ce dépôt, connectés par passkey sans SMS. Qui contrôle à la fois les accès Google Cloud de Sésame et l'un de ces comptes peut faire accepter une image malveillante par les clients de cette publication : `THREAT_MODEL.md` §2, limite 12. Plus de délai d'annonce : une version corrective sert dès que les clients qui la contiennent sont disponibles.

## Documents concernés

`RELEASE.md`, `README.md`, `ARCHITECTURE.md` ; `THREAT_MODEL.md` §2 (8, 12) ; `SECURITY.md` ; `CODEOWNERS`.
