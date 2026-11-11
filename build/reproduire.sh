#!/usr/bin/env bash
# Construction reproductible de l'image de cmd/sesame (RELEASE.md §1).
#
#   ./build/reproduire.sh            construit le commit courant (HEAD)
#   ./build/reproduire.sh vX.Y.Z     construit le tag, qui doit être HEAD
#
# Imprime sur la sortie standard une seule ligne : sha256:<empreinte>, digest
# du manifeste de l'image (celui que porte imageDigest dans versions.json).
#
# Ce qui rend la construction reproductible :
#   - le contexte est l'arbre du commit (git archive), sans fichier local ;
#   - SOURCE_DATE_EPOCH vaut la date du commit : dates de l'image et des
#     fichiers des couches (rewrite-timestamp) ;
#   - BuildKit est une image épinglée par digest, dans un constructeur dédié ;
#   - le Dockerfile épingle ses images de base par digest et construit le
#     binaire avec -trimpath, -buildvcs=false et un identifiant de build vide ;
#   - ni attestation de provenance ni SBOM, aucun cache.
#
# Il faut docker avec buildx, et git. SORTIE_OCI=<fichier> garde l'archive OCI
# de l'image (pour outils/controle-image) ; BUILDKITD_CONFIG=<fichier> passe une
# configuration à BuildKit (par exemple l'autorité d'un proxy), sans effet sur
# l'empreinte.
set -euo pipefail

BUILDKIT='moby/buildkit:v0.34.2@sha256:c5f8862f52499773af85bc07695b2379ef08e8002352b0df2c4864c91d78dabe'
CONSTRUCTEUR='sesame-reproduire-v0.34.2'
PLATEFORME='linux/amd64'

erreur() { echo "reproduire.sh : $*" >&2; exit 1; }

[ $# -le 1 ] || erreur "usage : $0 [vX.Y.Z]"
racine=$(git rev-parse --show-toplevel)
cd "$racine"

commit=$(git rev-parse --verify 'HEAD^{commit}')
if [ $# -eq 1 ]; then
  tag=$1
  commit_tag=$(git rev-parse --verify --quiet "refs/tags/$tag^{commit}") || erreur "tag $tag introuvable"
  [ "$commit_tag" = "$commit" ] || erreur "HEAD n'est pas $tag : git checkout $tag"
fi
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "reproduire.sh : modifications locales ignorées ; construit le commit $commit" >&2
fi
SOURCE_DATE_EPOCH=$(git log -1 --format=%ct "$commit")
export SOURCE_DATE_EPOCH

travail=$(mktemp -d)
trap 'rm -rf "$travail"' EXIT
mkdir "$travail/contexte"
git archive --format=tar "$commit" | tar -x -C "$travail/contexte"

if ! docker buildx inspect "$CONSTRUCTEUR" > /dev/null 2>&1; then
  options=(--name "$CONSTRUCTEUR" --driver docker-container --driver-opt "image=$BUILDKIT")
  if [ -n "${HTTPS_PROXY:-}" ]; then
    # Valeurs entre guillemets : buildx lit chaque option comme un champ CSV.
    options+=(--driver-opt network=host --driver-opt "\"env.HTTPS_PROXY=$HTTPS_PROXY\"" --driver-opt "\"env.NO_PROXY=${NO_PROXY:-}\"")
  fi
  if [ -n "${BUILDKITD_CONFIG:-}" ]; then
    options+=(--buildkitd-config "$BUILDKITD_CONFIG")
  fi
  docker buildx create "${options[@]}" > /dev/null
fi

docker buildx build \
  --builder "$CONSTRUCTEUR" \
  --platform "$PLATEFORME" \
  --no-cache \
  --pull \
  --provenance=false \
  --sbom=false \
  --build-arg SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" \
  --output "type=oci,dest=$travail/image.tar,rewrite-timestamp=true" \
  "$travail/contexte" >&2

empreinte=$(tar -xOf "$travail/image.tar" index.json | grep -o '"digest":"sha256:[0-9a-f]\{64\}"' | cut -d'"' -f4)
[ "$(printf '%s\n' "$empreinte" | wc -l)" -eq 1 ] && [ -n "$empreinte" ] || erreur "index.json de l'archive : attendu une seule image"

if [ -n "${SORTIE_OCI:-}" ]; then
  cp "$travail/image.tar" "$SORTIE_OCI"
fi
echo "$empreinte"
