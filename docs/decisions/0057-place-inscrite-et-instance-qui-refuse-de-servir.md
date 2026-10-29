<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0057 · Place de chaque membre inscrite dans l'état répliqué ; instance qui refuse de servir ; contrôles avant `grappePin`

**Date** : 2026-10-29 · **Statut** : actée (prise sur consigne de l'opérateur : l'option qui complexifie le moins le système, sans mécanisme nouveau) · précise [0050](0050-remplacement-d-un-membre-sans-quatrieme-votant.md) et [0055](0055-amorcage-par-place-et-pairs-derives-du-nom.md) · **revue depuis** ([0058](0058-aucun-refus-au-demarrage-ne-fait-sortir-le-processus.md))

## Contexte et menace

Le remplacement automatique admettait une VM sur la **zone** d'un votant absent, alors que la grappe tient « un votant par place » (préfixe `-a`, `-b`, `-c`) et que la configuration initiale ne portait ni nom de VM ni place. En option A, ou après le déplacement d'une place, une zone porte deux VM : une VM `-b` créée dans la zone d'un votant `-a` absent aurait pu prendre sa place, et le préfixe de routage `/n/b/` serait devenu ambigu. Quatre comportements n'étaient pas fixés : l'état d'une instance qui refuse de servir sur une version importée d'une clé de signature (arrêt du processus, ou instance vivante qui ne sert pas), le moment des refus `pin_bloque` et « mémoire à 90 % » par rapport à l'écriture de `grappePin`, le sort d'un `redepot` sur un enregistrement présent au seuil de mémoire, et qui décompte la limite de `/grappe/rejoindre`, que les demandes renouvelées toutes les 30 secondes atteignent exactement.

## Décision

- **Place inscrite** : la configuration initiale porte, pour chaque membre, `vm`, le nom de sa VM (`submods.gce.instance_name` du jeton), qui fixe sa place ; les membres y sont triés par `vm`. L'état répliqué garde depuis l'amorçage la liste `{vm, zone}` des membres, que seule `/grappe/membres` modifie. Une VM n'est admise automatiquement que si son nom est celui d'un votant injoignable depuis au moins 2 minutes et sa zone celle que l'état répliqué inscrit pour ce nom ; l'ancien membre de ce nom est retiré avant la promotion. Deux membres ne portent jamais le même nom.
- **Instance qui refuse de servir** (`version_importee`) : le processus ne sort pas, si bien que son groupe zonal ne la recrée pas en boucle ; elle répond à toute route `attestation_indisponible`, comme avant d'avoir obtenu son jeton, ne demande pas à joindre de grappe et ne dépose aucune borne. L'exploitant la recrée une fois la cause levée. `textes_refuses` reste un refus de démarrer.
- **Contrôles avant `grappePin`** : le plafond (`pin_bloque`), la mémoire à 90 % et `grappe_recente` sont vérifiés sans rien écrire avant la transaction qui écrit `grappePin` ; un refus laisse `grappePin` intact. L'erreur rendue suit toujours l'ordre des contrôles commun (`API.md` §6).
- **Mémoire à 90 %** : seul un dépôt qui ajoute un enregistrement absent est refusé ; `ouverture`, `redepot` et `nouveau_pin` sur un enregistrement présent restent admis.
- **Limite de `/grappe/rejoindre`** : décomptée par la VM qui reçoit la demande ; un `trop_d_appels` n'est pas un refus d'admission, ni journalisé ni alerté, et la VM sans grappe renouvelle sa demande 30 secondes plus tard.
- **Signatures de `session-sign`** : une seule forme, `R ‖ S` sur 64 octets, y compris `X-Sesame-Signature`, convertie depuis la forme DER que rend KMS.

## Alternatives écartées

- **Dériver la place de la seule zone** : faux en option A et après un déplacement, où une zone porte deux places.
- **Arrêter le processus** sur `version_importee` : le groupe zonal le recréerait sans fin, avec une alerte `grappe_votant_absent` à chaque tour, sans rien corriger.
- **Défaire `grappePin`** après un refus pour mémoire pleine : une écriture de compensation de plus, alors que vérifier avant d'écrire suffit (décision 0056).
- **Relever la limite de `/grappe/rejoindre`**, ou espacer les demandes : un réglage de plus pour un refus sans effet, la demande suivante passant.
- **Garder DER pour la passerelle** : deux encodages d'une même clé, deux jeux de vecteurs, pour aucun gain.

Pratiques reprises : la composition des membres tenue dans l'état répliqué et changée une modification à la fois (Raft, reconfiguration d'un membre à la fois, `etcd` qui identifie chaque membre par son nom) ; l'encodage `R ‖ S` des signatures ECDSA des JWS (RFC 7518 §3.4).

## Conséquences et limites

Une VM recréée sous un autre nom n'entre pas sans commande de l'exploitant. Une instance qui refuse de servir reste visible (alerte immédiate, votant absent) jusqu'à ce que l'exploitant la recrée ; pendant ce temps la grappe tient avec les deux autres votants.

## Documents concernés

`API.md` §4.1, §4.3, §5 (`X-Sesame-Signature`), §2.12 et table des codes d'erreur ; `ARCHITECTURE.md` §4 et §9.3.
