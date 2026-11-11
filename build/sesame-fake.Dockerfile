# Image de sesame-fake (décisions 0063 et 0071), distincte de celle de
# cmd/sesame : ce fichier seul construit cmd/sesame-fake, avec le lanceur
# factice et les doubles d'internal/factice ; le Dockerfile de cmd/sesame ne
# les lie jamais (contrôle « Lanceur factice hors de l'image »). Jamais une
# version de Sésame. La configuration du client se monte au chemin -config.
FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS construction
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY . .
RUN go mod verify \
 && go build -trimpath -buildvcs=false -ldflags=-buildid= -o /sortie/sesame-fake ./cmd/sesame-fake

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
ENV SESAME_FAKE_LAUNCHER=1
COPY --from=construction /sortie/sesame-fake /sesame-fake
EXPOSE 8090 8091
ENTRYPOINT ["/sesame-fake"]
