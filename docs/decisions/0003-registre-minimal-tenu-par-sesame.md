<!-- Source : Fatoom-Heritage-Numerique/Fatoom docs/technique/sesame/ -->

# 0003 · Un registre minimal tenu par Sésame : 7 jours, preuve de vie, générations

**Date** : 2026-10-12 · **Statut** : actée (validée par l'opérateur) · **revue depuis** : annulation explicite d'un signalement au lieu de la preuve de vie, transmission sur décès définitive après l'échéance, registre en ajout seul signé par `registre-sign` et borné par une borne publique ; l'état actuel est dans `ARCHITECTURE.md`

## Contexte et menace

Trois garanties ne peuvent pas reposer sur le client : la mesure des 7 jours de vérification après un signalement de décès, l'annulation de ce signalement par le titulaire vivant, et le refus d'un code d'accès révoqué. Si le client en était seul juge, un client compromis pourrait raccourcir le délai, bloquer une annulation ou faire revivre un code.

## Décision

Sésame tient lui-même un **registre** dans son propre projet, accessible uniquement à la charge attestée : signalements de décès (datés par **son** horloge, `transmissibleLe = signaleLe + 7 j`), **preuves de vie** envoyées directement par l'app à chaque déverrouillage, génération courante de chaque charte et de la KTS. Chaque ligne est authentifiée par un HMAC (clé KMS HSM) : une ligne modifiée hors de l'enclave est ignorée. Le registre ne contient ni clé, ni contenu, ni coordonnée.

Sésame ne vérifie pas lui-même le décès : il mesure 7 jours et exige l'absence de preuve de vie. Un client compromis peut au pire déclencher une fausse alerte.

## Alternatives écartées

- **Un reçu signé sans état** : Sésame signerait la date du signalement et vérifierait le reçu à l'ouverture ; mais l'annulation devrait transiter par le client, qui pourrait la bloquer.
- **L'attestation du client seule** (« le décès est vérifié, ouvre ») : Sésame deviendrait un décor ; toute la garantie reposerait à nouveau sur l'honnêteté du client.

## Conséquences et limites

Sésame a un état, petit (quelques Ko par titulaire), sauvegardé chaque nuit ; sa perte ne détruit aucune clé mais remet à zéro signalements et générations : les coffres se ré-enrôlent (`THREAT_MODEL.md` §2, limite 11). Les 7 jours dépendent de l'horloge de la VM (limite 10). Un titulaire qui n'ouvre pas son app pendant 7 jours après un faux signalement est transmis (limite 7). Toute action sur une charte passe d'abord par Sésame : si Sésame est injoignable, l'action échoue et l'ancien code reste valide (`ARCHITECTURE.md` §9.4).

## Documents concernés

`ARCHITECTURE.md` §6, §8.5, §9.3, §9.4 ; `API.md` §2.3, §2.4, §2.12 ; `THREAT_MODEL.md` §2 (7, 10, 11).
