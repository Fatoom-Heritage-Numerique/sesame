<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0044 · Requêtes à la passerelle vérifiées contre chaque clé de `clesSession`

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option la plus simple qui tient la garantie annoncée) · précise [0026](0026-cles-session-sign-dans-la-liste-des-versions.md)

## Contexte et menace

Les requêtes de Sésame à la passerelle du client portent six en-têtes, dont une signature `session-sign` nue, sans `kid`. Le texte de la passerelle disait pourtant qu'elle lisait la clé publique « par `kid` » dans `clesSession`, avec « `kid` inconnu ⇒ `401` ». Les autres pièces signées par `session-sign` (jetons, reçus, réponses de `etat`, bornes) portent bien un `kid`.

## Décision

- La passerelle vérifie `X-Sesame-Signature` contre chaque clé publique de `clesSession` de la liste des versions compilée, et l'accepte si l'une d'elles la vérifie. Aucune ne la vérifie ⇒ `401`.
- Les en-têtes de la requête et la chaîne signée ne changent pas.

## Alternatives écartées

- **Un septième en-tête `X-Sesame-Kid`** : un en-tête, un contrôle et des vecteurs de plus pour choisir entre une ou deux clés, que la liste compilée borne déjà ; une valeur de `kid` n'ajoute rien à la sécurité, car seules les clés de la liste sont essayées.

## Conséquences et limites

Une vérification ECDSA de plus par requête pendant une rotation de `session-sign`, où la liste porte deux clés. La pratique est courante quand une signature n'a pas de `kid` (RFC 7515 fait du `kid` une indication facultative).

## Documents concernés

`API.md` §5.1.
