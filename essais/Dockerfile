# Image jetable des essais sur Confidential Space (essais/README.md,
# décision 0075). Jamais une version de Sésame : elle ne contient que
# essais/cmd/essai. Mêmes images de base, épinglées par digest, que le
# Dockerfile de cmd/sesame ; contexte : la racine du dépôt.
FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS construction
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY . .
RUN go mod verify \
 && go build -trimpath -buildvcs=false -ldflags=-buildid= -o /sortie/essai ./essais/cmd/essai

# Distroless statique, sans shell, utilisateur sans privilège.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
# Politique de lancement de l'essai : journaux redirigés vers Cloud Logging
# même sur l'image Confidential Space de production (l'essai du jeton exige
# dbgstat disabled-since-boot, donc pas l'image de débogage) ; commande non
# modifiable. Une seule variable admise, ESSAI_VARIABLE, sans effet sur le
# programme : sans variable passée, le jeton ne porte pas
# submods.container.env_override et le client de référence le refuse
# (ARCHITECTURE.md §2.3) ; les VM la reçoivent par tee-env-ESSAI_VARIABLE.
LABEL "tee.launch_policy.log_redirect"="always" \
      "tee.launch_policy.allow_cmd_override"="false" \
      "tee.launch_policy.allow_env_override"="ESSAI_VARIABLE"
COPY --from=construction /sortie/essai /essai
# Port ouvert par le lanceur (EXPOSE) ; le pare-feu du VPC ne l'ouvre qu'au
# tunnel IAP.
EXPOSE 8080/tcp
ENTRYPOINT ["/essai"]
