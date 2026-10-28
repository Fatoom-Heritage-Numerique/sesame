<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0049 · Retrait d'urgence hors faille : l'arrêt jusqu'à la version suivante est écrit, sans procédure nouvelle

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · précise [0042](0042-ecart-de-30-jours-sauf-faille.md)

## Contexte et menace

Un retrait d'urgence retire l'empreinte de la version de la condition KMS aussitôt. Pour une faille, ses VM sont arrêtées aussitôt et la version corrective peut suivre sans attendre. Hors faille, la décision 0042 fait attendre la version suivante jusqu'à l'échéance des 30 jours, et les VM de la version retirée tournent jusque-là. Aucun texte ne disait ce qu'elles font alors sans KMS.

## Décision

- La procédure ne change pas. Son effet est écrit : sans KMS, la version retirée ne signe plus rien (`registre-sign`, `session-sign`) et ne descelle plus rien. Aucune preuve de vie ni annulation n'est inscrite ; aucune ouverture, lettre ni transmission n'a lieu ; aucun dépôt du PIN ; aucune borne n'est déposée.
- Le contrôle anti-retour échoue donc 2 heures après la dernière borne, et la version suivante est une reprise publiée, comme après tout arrêt de plus de 2 heures.
- Un signalement de décès arrivé à échéance pendant ce temps n'a pas pu être annulé : à la reprise, `deces/annuler` répond `deja_transmis` et la transmission suit son cours, comme pour toute indisponibilité de Sésame (le mécanisme échoue ouvert).

## Alternatives écartées

- **Laisser l'empreinte dans KMS jusqu'à la version suivante quand le retrait n'est pas une faille** : une seconde sorte de retrait, à décider dans l'urgence, et une version jugée assez mauvaise pour être retirée continuerait de desceller.
- **Admettre une version suivante avant 30 jours pour tout retrait** : des grappes neuves en plus, donc des essais de PIN en plus, contraire à la décision 0042.
- **Suspendre les échéances des signalements pendant l'arrêt** : un mécanisme et un état nouveaux au registre.

## Conséquences et limites

Un retrait hors faille coûte jusqu'à 30 jours d'arrêt de Sésame et une reprise publiée ; il se réserve aux défauts qui justifient cet arrêt. Un faux décès transmis ne donne rien à l'opérateur (`THREAT_MODEL.md`, limites 8 et 9).

## Documents concernés

`ARCHITECTURE.md` §11.6 ; `RELEASE.md` §5 ; `THREAT_MODEL.md` §2 (8).
