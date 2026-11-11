<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0073 · Seule la dernière évaluation notée sur le canal vaut pour un dépôt `ouverture`

**Date** : 2026-11-11 · **Statut** : actée (choix technique délégué par l'opérateur) · applique `API.md` §2.12 et §2.13

## Contexte et menace

`API.md` §2.13 : « Sésame note sur le canal la dernière évaluation, pour un dépôt `ouverture` » ; §2.12 : `ouverture` « seulement juste après une évaluation sur le même canal, pour le même `elementAveugle` ». Un dépôt `ouverture` remet le compteur à 10 sans changer de clé. Le code gardait chaque évaluation du canal pendant 5 minutes, comme un défi : une évaluation antérieure du même canal restait admise. Le contrat ne donne aucune durée.

## Décision

- Le canal garde une seule évaluation notée : la dernière réussie (réponse `200` de `pin/evaluer`, servie ici ou relayée au leader), avec son `uid` ; chaque évaluation remplace la précédente.
- Tout dépôt `ouverture` qui arrive au contrôle de l'évaluation la consomme, qu'il soit admis ou refusé (`message_refuse` pour un autre élément) : un second dépôt demande une nouvelle évaluation.
- Aucune durée propre : la note vit avec le canal (fermé après 1 h sans requête, `API.md` §7). Le dépôt porte de toute façon un défi de 5 minutes, et seul le titulaire signe un dépôt (`cleVie`).

## Alternatives écartées

- **Garder 5 minutes** chaque évaluation : contraire à « la dernière », et une durée que le contrat n'écrit pas.
- **Consommer seulement sur un dépôt admis** : un dépôt refusé laisserait la note réutilisable ; la consommer à chaque essai est plus strict et ne coûte rien à l'app, qui dépose juste après son évaluation.

## Conséquences et limites

- Un dépôt `ouverture` refusé, ou qui échoue pendant une pause, ne se rattrape pas sur la même évaluation : c'est le comportement déjà écrit pour la pause (l'essai reste décompté jusqu'à la prochaine ouverture réussie ou au prochain `nouveau_pin`).

## Documents concernés

`API.md` §2.12, §2.13 (inchangés) ; `internal/canal` (`Note`, `Noted`), `internal/routes/pin.go`.
