<!-- Source : spécification de l'opérateur (dépôt Fatoom-Heritage-Numerique/Fatoom), décisions prises sur Sésame -->

# 0065 · CI du contrat exécutable ; contrôles de l'infrastructure écrits d'avance ; noms du déploiement cherchés hors de ce dépôt

**Date** : 2026-11-11 · **Statut** : actée (décision de l'opérateur)

## Contexte et menace

`RELEASE.md` §8 liste les contrôles de la CI. Certains n'ont de sens qu'avec du code (double construction, budget, vecteurs), d'autres qu'avec l'infrastructure ou une version (condition KMS, `versions.json`, scripts publics, journaux). Le contrôle « dépôt public sans nom du déploiement » cherche des noms qu'un dépôt public ne peut pas écrire en clair sans les publier.

## Décision

- **Dès le contrat exécutable**, sur chaque PR et sur `main` : `gofmt -l` vide, `go vet ./...`, `go test -race ./...`, `go mod verify`, `govulncheck ./...` (`golang.org/x/vuln`, lancé par `go run` à version épinglée, outil hors de l'image) ; double construction ; budget ; vecteurs par `SHA256SUMS` et `compteurPin` régénéré ; lanceur factice hors de l'image (`go list -deps ./cmd/sesame`) ; image sans shell et étiquettes de la politique de lancement ; dépendances (paquets liés par `cmd/sesame` dans la liste d'`ARCHITECTURE.md` §2.4, modules transitifs compris) ; concordance de l'OpenAPI (décision 0064) ; noms de ressources, part structurelle (ci-dessous). Sur `main`, publication de l'image de `sesame-fake` (décision 0063).
- **À l'infrastructure et à la première version** (étapes 5 et 6) : schéma de `versions.json`, `hwmodels` de l'image, cohérence KMS ⇔ `versions.json`, scripts publics, borne du registre sur une instance réelle, exécution quotidienne de `build/verifier-journaux.sh`. Ils sont écrits dans `RELEASE.md` §8 dès maintenant, avec l'étape qui les met en place.
- **Noms du déploiement** : ce dépôt ne fait que la part structurelle : tout `gs://`, chemin `-docker.pkg.dev/`, adresse `.iam.gserviceaccount.com`, chemin `projects/…`, hôte `run.app` et nombre de 12 chiffres d'un fichier suivi doit être sous la forme `<…>` des noms de ressources (`CONTRIBUTING.md`, « Règles du code »). La recherche des noms réels se fait dans la CI privée du client, qui les connaît et lit ce dépôt public.
- **Outils** : la version de Go vient de `go.mod` (`go` et `toolchain`) ; les actions GitHub sont épinglées par l'empreinte de leur commit, la version en commentaire ; la double construction tourne dans deux jobs sur deux exécuteurs et compare leurs digests ; les contrôles propres au dépôt sont des programmes Go de `outils/`, sans outil tiers.

## Alternatives écartées

- **Noms en clair dans un secret de la CI de ce dépôt** : une PR venue d'une bifurcation ne reçoit pas les secrets, et le contrôle serait sauté là où il sert le plus.
- **Empreintes des noms dans ce dépôt** : une empreinte de nom court se retrouve hors ligne, et la recherche d'une sous-chaîne par empreinte n'est pas fiable.
- **Actions épinglées par version majeure** : une étiquette se déplace ; l'empreinte du commit ne bouge pas.
- **Attendre l'infrastructure pour écrire tous les contrôles** : le contrat exécutable serait fusionné sans double construction ni budget.

## Conséquences et limites

- Un nom réel fusionné ici n'est trouvé que par la CI suivante du client, après sa publication : la part structurelle et la revue le préviennent avant.
- `govulncheck` dépend d'une base de failles publique : un résultat change sans changement du code ; un échec se lit avant toute fusion.

## Documents concernés

`RELEASE.md` §8 ; `CONTRIBUTING.md` (« Outils et commandes ») ; `ARCHITECTURE.md` §2.4.
