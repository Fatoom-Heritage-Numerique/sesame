<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0009 · Une clé de release sur clé matérielle au départ, nombre de signatures gravé dans les clients

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur après explication du rôle de la signature) · **remplacée le 2026-10-25 par [0014](0014-liste-des-versions-compilee.md)**

## Contexte et menace

Le manifeste (décision 0005) doit prouver qu'il vient du porteur du produit, hors de portée d'un pirate des serveurs : si la clé qui le signe vivait sur une machine connectée, un logiciel malveillant la copierait et publierait un manifeste autorisant une image de son choix.

## Décision

**Une** clé de release Ed25519, générée sur une **clé matérielle** (YubiKey ou équivalent), détenue par le porteur du produit, hors de tout service en ligne : le secret ne quitte jamais la clé, signer exige sa présence physique. Une signature est exigée tant qu'il n'y a pas de seconde personne de confiance ; dès qu'elle existe, une seconde clé est ajoutée et le nombre exigé passe à 2 **par une version des clients** : `requiredSignatures` est gravé dans l'app, le portail et la passerelle, pas lu dans le manifeste. Jamais de clé de release dans un fichier, un dépôt, une CI ou un service en ligne.

## Alternatives écartées

- **Une clé dans un fichier sur un PC** : copiable par un logiciel malveillant.
- **Deux clés tenues par une seule personne** : la seconde signature ne protégerait de rien.

## Conséquences et limites

Une clé volée suffit jusqu'à l'ajout de la seconde (`THREAT_MODEL.md` §2, limite 12) ; compensé par le secret non extractible, le délai de 7 jours (une image malveillante est visible avant de servir) et la détection par quiconque reconstruit l'image. La CI vérifie des signatures et n'en produit aucune. Rotation ou ajout de clé : une version des clients (clé ajoutée, puis ancienne retirée une version plus tard).

## Documents concernés

`RELEASE.md` §3 ; `THREAT_MODEL.md` §2 (12).
