<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0053 · L'état du PIN est recopié au registre avant la réponse ; sans KMS, ni dépôt ni évaluation

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · **revue depuis** ([0054](0054-essai-decompte-sans-recopie-signale.md))

## Contexte et menace

Le leader de la grappe recopie l'état du PIN dans la ligne du registre (`pinDeposeLe`, `pinBloqueLe`, `pinEvaluations`) après chaque changement validé par la majorité. Pendant une pause de la grappe, `/vie` et `/etat` répondent cette copie. Rien ne disait si la recopie précédait la réponse, ni ce que recevait le client quand elle échouait. Un appel à KMS ou un échange STS refusé n'avait pas de code d'erreur. Enfin, après un retrait d'urgence hors faille, les VM tournent sans KMS jusqu'à la version suivante (0049) : la grappe, en mémoire, pouvait encore évaluer le PIN alors qu'aucune alerte ne pouvait partir au titulaire (fiche scellée par `alerte-seal`), soit jusqu'à 10 essais silencieux par titulaire.

## Décision

- **Recopie avant la réponse** : un dépôt ou un essai décompté est validé par la majorité, puis recopié au registre, puis seulement évalué et répondu. Une recopie impossible (registre illisible, transaction en conflit, KMS ou STS indisponible) fait répondre `pin_indisponible_temporaire`, sans évaluation ni alerte. L'essai décompté le reste ; la recopie suivante écrit le compteur entier et le rattrape.
- **Sans KMS, ni dépôt ni évaluation** : la ligne du registre est signée par `registre-sign`. Après un retrait d'urgence, la recopie est donc impossible, et aucune évaluation n'est rendue. La règle ci-dessus suffit, sans contrôle de plus.
- **Codes d'erreur** : un appel à KMS (`Decrypt`, `AsymmetricSign`) ou un échange STS refusé ou sans réponse reçoit `registre_indisponible`, et `pin_indisponible_temporaire` sur les routes du PIN. Aucun code nouveau.

## Alternatives écartées

- **Répondre d'abord, recopier ensuite** : la copie lue pendant une pause pourrait mentir sur un plafond atteint ou des évaluations faites, et une évaluation pourrait partir sans que rien d'inscrit n'en garde la trace.
- **Vérifier KMS avant le décompte** : un contrôle de plus, à chaque essai, pour un cas déjà fermé par la recopie signée.
- **Un code d'erreur propre aux pannes de KMS** : le client ne ferait rien de plus que « réessayer plus tard », ce que disent déjà les deux codes 503 existants.

Pratiques reprises : Raft ne répond au client qu'après la validation de l'entrée par la majorité (Ongaro et Ousterhout, « In Search of an Understandable Consensus Algorithm », §5.3) ; les enclaves de Signal font de même pour les essais de PIN (« Technology Preview for secure value recovery », signal.org/blog/secure-value-recovery) ; les API de Google Cloud se réessaient sur une erreur temporaire (`503`) avec une attente exponentielle tronquée (docs.cloud.google.com/iam/docs/retry-strategy).

## Conséquences et limites

Un essai dont la recopie échoue est perdu pour celui qui le tente, sans rien lui apprendre. Pendant un retrait d'urgence hors faille, qui tient une session du titulaire peut encore consommer ses essais sans évaluation ; la version suivante, sur une grappe neuve, n'en garde rien (`THREAT_MODEL.md`, limite 19 (j)).

## Documents concernés

`API.md` §2.12, §2.13 et §6 ; `ARCHITECTURE.md` §9.2 et §11.6.
