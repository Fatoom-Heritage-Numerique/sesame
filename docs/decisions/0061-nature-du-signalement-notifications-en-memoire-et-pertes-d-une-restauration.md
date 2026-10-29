<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0061 · Nature d'un signalement ; notifications réessayées en mémoire ; pertes d'une restauration assumées

**Date** : 2026-10-29 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau)

## Contexte et menace

Trois points restaient sans réponse écrite.

- « Signalement de même nature » n'était défini nulle part. Si `insee` et `acte` étaient deux natures, deux décès non annulés pourraient coexister. `/deces/annuler` n'accepte que le signalement courant : l'autre décès irait à son échéance et serait transmis malgré l'annulation.
- Le réessai de 24 h des notifications au client ne disait ni où il est tenu, ni s'il est signé de nouveau. Or la passerelle refuse un horodatage de plus de 5 minutes. Une notification `signalementAnnule` perdue laisse le client envoyer les codes, si l'app n'a pas pu le prévenir elle-même.
- Une reprise avec sauvegarde perd au plus la dernière heure d'écritures, mais rien ne disait ce que devient une ligne restaurée en retard. Une annulation, une génération de charte ou une rotation de cette heure peuvent être perdues.

## Décision

- **Nature** : un signalement est d'une de deux natures, le décès (sources `insee` et `acte` confondues) ou l'inactivité. Un `acte` reçu pendant un `insee` non annulé renvoie le signalement `insee`, sans inscrire la nouvelle pièce. Un titulaire n'a donc jamais deux décès en cours, et l'annulation vise toujours le seul en cours.
- **Notifications** : chaque essai est signé de nouveau, avec un `X-Sesame-Date` frais et le même `X-Sesame-Request-Id`. Le réessai n'est tenu qu'en mémoire de la VM qui a commencé l'envoi, comme celui des alertes au titulaire : un redémarrage l'arrête, sans journal de perte. Côté client, l'app retente sa propre notification de l'annulation à chaque déverrouillage jusqu'à l'échéance. Elle ne la rappelle pas sur la seule vue d'une annulation dans `/vie` ou `/etat`.
- **Restauration** : les pertes d'une ligne restaurée sont une limite assumée, écrite dans la limite 11 avec la façon dont le titulaire retrouve chacune :
  - une annulation se refait depuis l'alerte que `/vie` et `/etat` montrent de nouveau ;
  - une génération de charte se refait par une nouvelle modification du proche ;
  - une rotation perdue ne se rattrape pas.
- **Fiche d'alerte** : `generation` reste tirée de l'horloge de l'appareil, sans borne de Sésame. Une horloge en avance fige la fiche pendant la durée de cette avance, et la documentation le dit.

## Alternatives écartées

- **Une nature par source** : il faudrait que l'annulation vise tous les décès en cours, ce qui change `/deces/annuler`.
- **Réessai des notifications tenu au registre** : il faut une collection, une réservation et une tâche, pour un cas qui exige à la fois un redémarrage pendant le réessai et un client que l'app n'a pas pu prévenir avant l'échéance. Les codes envoyés alors n'ouvrent rien.
- **Rappel de la notification d'annulation par l'app à chaque lecture d'une annulation** : l'app ne sait pas si le client l'a reçue. Elle rappellerait la fonction à chaque déverrouillage pendant 7 jours.
- **Ré-enrôlement d'une ligne restaurée d'une génération inférieure contre la clé fournie par le client**, comme pour une ligne absente : cela élargit la limite 54 et change `/enroler`. La question est laissée à l'opérateur.
- **Borne de `generation` par l'horloge de Sésame** : c'est un contrôle de plus dans `/alerte/fiche`. La question est laissée à l'opérateur.

Pratique reprise :

- un expéditeur de webhooks signe de nouveau chaque essai avec un horodatage frais, sous le même identifiant d'événement, pour que le destinataire déduplique (https://www.getconvoy.io/blog/generating-stripe-like-webhook-signatures) ;
- une sauvegarde planifiée perd les écritures qui suivent son export (https://firebase.google.com/docs/firestore/disaster-recovery).

## Conséquences et limites

- Un titulaire dont une rotation tombe dans l'heure perdue par une restauration ne transmet plus rien : ses fichiers et ses mots de passe restent.
- Si la VM qui notifie une annulation redémarre et que l'app n'a pas pu prévenir le client, les proches reçoivent l'annonce d'un décès, avec des codes qui n'ouvrent rien.
- Une fiche d'alerte envoyée depuis une horloge en avance ne change plus pendant la durée de cette avance.

## Documents concernés

- `API.md` : §2.8, §2.15, §4.1 et §5.5 ;
- `ARCHITECTURE.md` : §2, §10 et §11 ;
- `THREAT_MODEL.md` : limites 8, 11 et 12 ;
- `README.md` ;
- `RELEASE.md`.
