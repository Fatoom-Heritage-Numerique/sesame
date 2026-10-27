<!-- Source : Fatoom-Heritage-Numerique/Fatoom docs/technique/sesame/ -->

# 0002 · Cloud KMS HSM conditionné à l'attestation, une clé de transmission par titulaire

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur)

## Contexte et menace

L'enclave doit pouvoir ouvrir les enveloppes des contenus destinés, et seulement elle. La question est où vit le secret qui le permet, et ce qu'une faille mémoire de l'enclave (processeur, pile Confidential Space) exposerait.

## Décision

Chaque titulaire a une **clé de transmission** (KTS, 32 octets) tirée sur son téléphone, protégée par sa clé maître, et confiée une fois à Sésame qui la **scelle** sous une clé **Cloud KMS HSM** (`kts-seal`, AES-256-GCM, non exportable). Le scellé est rangé par le client ; il ne se descelle que par une charge attestée, car la liaison IAM de la clé porte une condition sur l'empreinte de l'image. L'enclave ne détient aucune clé longue durée : elle descelle une KTS le temps d'une session, puis l'efface. Tout ce que le titulaire destine est chiffré, sur son téléphone, sous des clés dérivées de sa KTS (`ARCHITECTURE.md` §8).

## Alternatives écartées

- **Une clé de service privée, scellée et gardée en mémoire de l'enclave** : plus rapide côté serveur, mais une faille mémoire exposerait toutes les enveloppes de tous les titulaires jusqu'à rotation.
- **Une enveloppe RSA par clé de contenu, déchiffrée par KMS** : un appel KMS par clé ouverte, et de la cryptographie RSA dans l'app ; coût et surface inutiles.

## Conséquences et limites

Une faille mémoire n'expose que les KTS des titulaires ouverts pendant la session, jamais les enveloppes stockées ni ce qui est gardé pour soi (`THREAT_MODEL.md` §2, limite 1). L'IAM de la clé est administrée par l'opérateur de Sésame : un administrateur pourrait autoriser une autre image ; compensé par le projet séparé, l'absence de compte humain déchiffreur, le journal d'audit public et le contrôle CI KMS ⇔ manifeste (limite 2). La rotation de KTS re-chiffre toutes les enveloppes et chartes sur le téléphone (`ARCHITECTURE.md` §9.4).

## Documents concernés

`ARCHITECTURE.md` §4, §8 ; `RELEASE.md` §5 ; `THREAT_MODEL.md` §2 (1, 2).
