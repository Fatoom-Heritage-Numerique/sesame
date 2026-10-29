<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0051 · Après un arrêt de plus de 2 heures, et à la toute première version, une reprise publiée sans mécanisme nouveau

**Date** : 2026-10-28 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · précise [0022](0022-reprise-publiee-avec-ou-sans-sauvegarde.md), [0040](0040-bornes-en-echec-hors-du-controle-anti-retour.md) et [0049](0049-retrait-d-urgence-hors-faille-ecrit.md) · **revue depuis** ([0052](0052-reprise-apres-une-panne-sans-attendre-30-jours.md))

## Contexte et menace

Le contrôle anti-retour échoue quand aucune borne valide n'a été déposée depuis plus de 2 heures, et seule une reprise publiée en sort (0040). Trois cas restaient flous :

- la **version corrective** d'un retrait pour une faille était décrite comme une mise en service ordinaire, alors que ses VM arrêtées aussitôt ne déposent plus de borne et que la publication des apps dans les deux boutiques dure plus de 2 heures ;
- une **reprise sans restauration** (registre intact, bornes trop anciennes : panne de toutes les instances, retrait d'urgence) n'avait pas de constante `reprise` définie, la constante ne décrivant qu'une sauvegarde restaurée ou l'absence de sauvegarde ;
- la **toute première version** démarre sans aucune borne : le contrôle échoue, et rien ne disait comment elle en sort.

## Décision

- **Arrêt de plus de 2 heures** : quel qu'en soit le motif (retrait pour une faille, retrait hors faille, panne de toutes les instances), la version qui suit est une reprise publiée. Pour un retrait pour une faille, la version corrective l'est donc en pratique toujours.
- **Reprise sans restauration** : la constante `reprise` porte `{"epoque", "empreinte"}` de la tête de la chaîne du registre en place à l'arrêt. Cette époque est égale ou supérieure à celle de la dernière borne valide, et quiconque peut la comparer à la dernière borne publiée. Les étapes du démarrage d'une reprise s'appliquent au registre en place comme à une sauvegarde restaurée : vérification des `sig` et de la chaîne, réécriture de chaque ligne, marqueurs, événement `reprise` dans la borne.
- **Règle des 30 jours** : une reprise est une nouvelle version. Hors faille, elle attend l'échéance des 30 jours depuis la version précédente, comme toute version. Jusque-là, Sésame refuse ouvertures, transmissions et changements de grappe, et accepte encore preuves de vie et annulations tant que ses instances tournent avec KMS.
- **Toute première version** : elle porte `reprise: {"sansSauvegarde": true}`. Sa référence est l'époque 0 tant qu'elle n'a déposé aucune borne, puis ses bornes deviennent la référence. Aucun marqueur d'inscription n'existe avant elle, si bien que `cleViePubAttendue` n'y sert à personne. Sa reprise prend fin, comme toute autre, à la version suivante.

## Alternatives écartées

- **Exception « bucket d'audit sans aucune borne » pour la première version** : une règle de plus dans le contrôle, qu'un registre et un bucket vidés pourraient invoquer. La constante compilée est une ancre de confiance publique, comme les métadonnées racines livrées avec le client dans TUF.
- **Constante propre à une reprise sans restauration** (par exemple `{"sansRestauration": true}`) : un troisième format, alors que `{"epoque", "empreinte"}` décrit déjà exactement l'état repris.
- **Exempter de la règle des 30 jours une reprise après une panne** : cette règle borne les grappes neuves, donc les essais de PIN rendus. L'exemption reste à décider par l'opérateur, hors de cette décision.
- **Un compteur monotone matériel ou un journal de transparence externe** (Rekor, Certificate Transparency) pour la référence : un composant de plus. Les bornes horodatées par le stockage verrouillé en tiennent déjà lieu.

Sources : spécification de TUF, §2.1.1 et §5.2-5.3 (métadonnées racines de confiance livrées avec le client, chaque version vérifiée contre la précédente), https://theupdateframework.github.io/specification/latest/ ; Certificate Transparency, RFC 6962, https://www.rfc-editor.org/rfc/rfc6962.

## Conséquences et limites

Un arrêt de plus de 2 heures coûte une reprise publiée, donc une grappe neuve et le redépôt du PIN de chaque titulaire. Moins de 30 jours après la version précédente, et hors faille, il laisse Sésame sans ouverture ni transmission jusqu'à l'échéance. Un signalement de décès arrivé à échéance pendant un arrêt où Sésame ne sert plus rien est transmis à la reprise sans avoir pu être annulé (`THREAT_MODEL.md`, limite 8).

## Documents concernés

`ARCHITECTURE.md` §6.3, §6.4 et §11.6 ; `RELEASE.md` §5 ; `SECURITY.md` ; `THREAT_MODEL.md` §2 (8).
