<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0021 · Registre en ajout seul signé par `registre-sign`, borne publique du registre

**Date** : 2026-10-26 · **Statut** : actée (validée par l'opérateur) · revoit [0003](0003-registre-minimal-tenu-par-sesame.md) · **revue depuis** ([0040](0040-bornes-en-echec-hors-du-controle-anti-retour.md), [0045](0045-version-de-signature-fixee-par-l-image.md))

## Contexte et menace

La décision 0003 authentifiait chaque ligne du registre par un HMAC sous KMS. Un HMAC empêche de modifier une ligne, mais pas de restaurer une sauvegarde ancienne, d'effacer un titulaire ni de recopier une ligne ancienne : une annulation « Je suis en vie » disparaîtrait, un code révoqué se rouvrirait. Un HMAC sous KMS coûte aussi un appel facturé par ligne lue.

## Décision

- Chaque ligne est signée par la clé KMS asymétrique `registre-sign` à l'écriture et vérifiée **localement** avec sa clé publique ; une version importée de la clé fait refuser de servir.
- Pierres tombales, marqueur d'inscription dans un bucket verrouillé, `ecritLe` comparé à l'`update_time` de Firestore pour chaque ligne, époque et chaîne d'empreintes.
- Toutes les 10 minutes, chaque instance dépose une **borne** signée par `session-sign` dans un bucket public à rétention verrouillée : époque et empreinte du registre, identité de la grappe, événements ; le contrôle anti-retour refuse d'ouvrir sur un registre plus ancien que la dernière borne.

## Alternatives écartées

- **HMAC sous KMS** (0003) : un appel par lecture, et rien contre une restauration.
- **Clé HMAC descellée en mémoire** : un secret global durable de plus, exposé à une faille de l'enclave.
- **Tiers séquestre** : écarté ; la garantie est qu'une manœuvre se voit, pas qu'elle soit impossible.

## Conséquences et limites

L'administrateur garde le pouvoir de bloquer et perd celui de faire oublier un fait à Sésame. Une image autorisée en douce peut signer une borne, mais seulement élever le maximum (échec fermé pour tous) ou combler un trou après 3 heures sans instance honnête (`THREAT_MODEL.md`, limites 2 et 11).

## Documents concernés

`ARCHITECTURE.md` §4, §6, §8.
