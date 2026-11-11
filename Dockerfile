# Image de cmd/sesame (RELEASE.md §1, ARCHITECTURE.md §2.1 et §2.4).
# Se construit par build/reproduire.sh, qui fixe la date (SOURCE_DATE_EPOCH),
# la version de BuildKit et le format de sortie : une autre commande ne donne
# pas la même empreinte.
#
# Étage de construction : Go de go.mod, dans une image épinglée par digest
# (index multi-architecture de golang:1.27.4-bookworm). GOTOOLCHAIN=local
# interdit tout téléchargement d'une autre chaîne de compilation.
FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS construction
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY . .
RUN go mod verify \
 && go build -trimpath -buildvcs=false -ldflags=-buildid= -o /sortie/sesame ./cmd/sesame

# Étage final : distroless statique, sans shell ni gestionnaire de paquets,
# épinglé par digest (index multi-architecture de
# gcr.io/distroless/static-debian12:nonroot), utilisateur sans privilège.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
# Politique de lancement de Confidential Space (ARCHITECTURE.md §2.1) : les
# variables d'ARCHITECTURE.md §2.3, aucune autre commande, journaux non
# redirigés, aucune surveillance de la mémoire.
LABEL "tee.launch_policy.allow_env_override"="VERIFICATION_SERVICE,COURRIER_ORGANISATION,EMAIL_EXPEDITEUR,PASSERELLE_URL,SESAME_AUDIENCE,SMS_EXPEDITEUR,IDTOKEN_PROJET,COMPTE_DECES,COMPTE_EXPLOITATION,SESAME_TEXTES" \
      "tee.launch_policy.allow_cmd_override"="false" \
      "tee.launch_policy.log_redirect"="never" \
      "tee.launch_policy.hardened_monitoring"="none" \
      "tee.launch_policy.debug_monitoring"="none"
COPY --from=construction /sortie/sesame /sesame
ENTRYPOINT ["/sesame"]
