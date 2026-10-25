<!-- Source : Fatoom-Heritage-Numerique/Fatoom docs/TRD.md §16.5.5, §16.9 #12, registre §12 (2026-10-25 ; revu le 2026-10-26, second audit, S-01) ; docs/PRD.md R-PARTAGE-09, R-COMPTE-17 -->

# 0014 · Liste des versions compilée dans les clients, mise à jour forcée

**Date** : 2026-10-25 · **Statut** : actée (décision explicite de l'opérateur) · remplace [0005](0005-manifeste-signe-publie-7-jours-avant.md) et [0009](0009-cle-de-release-sur-cle-materielle.md)

## Contexte et menace

Les clients doivent refuser toute version de Sésame qui n'a pas été publiée. Les décisions 0005 et 0009 confiaient cette liste à un manifeste signé sur une clé matérielle, re-signé tous les 30 jours, annoncé 7 jours avant toute mise en service.

## Décision

Le manifeste signé, sa re-signature, la clé de release, le numéro de séquence, l'expiration et l'annonce de 7 jours sont **retirés**. Les versions autorisées sont listées dans `versions.json` (ce dépôt, branche `main` protégée : `imageDigest`, `imageReference`, `envDigest`, `hwmodels`, `tag`, `commit`, `publishedAt`, `status` ∈ `en_service` | `retiree`, `retiredAt`, `notes`) et **compilées** dans l'app, le portail et la passerelle du client au moment du build : ils ne téléchargent rien et ne vérifient aucune signature. Une empreinte absente ou `retiree`, ou un `hwmodel` hors de `hwmodels`, est refusée.

Mise en service : tag et build reproductible, ligne `en_service` ; versions des clients qui la contiennent publiées et disponibles dans les deux boutiques ; l'IAM KMS autorise l'empreinte (la liste d'abord, l'IAM ensuite) et la version démarre ; la version minimale de l'app est relevée (mise à jour forcée) ; l'ancienne version est arrêtée. Retrait d'urgence : ligne `retiree`, empreinte retirée de KMS immédiatement, version minimale relevée.

## Alternatives écartées

Le manifeste signé et la clé de release des décisions 0005 et 0009, retirés par cette décision.

## Conséquences et limites

L'ancre de confiance devient l'ensemble des comptes qui publient l'app, le site, les fonctions du client et ce dépôt, connectés par passkey sans SMS. Qui contrôle à la fois les accès Google Cloud de Sésame et l'un de ces comptes peut faire accepter une image malveillante par les clients de cette publication : `THREAT_MODEL.md` §2, limite 12. Plus de délai d'annonce : une version corrective sert dès que les clients qui la contiennent sont disponibles.

## Documents concernés

`RELEASE.md`, `README.md`, `ARCHITECTURE.md` (à réécrire, voir leurs bandeaux) ; `THREAT_MODEL.md` §2 (8, 12) ; `SECURITY.md` ; `CODEOWNERS`.
