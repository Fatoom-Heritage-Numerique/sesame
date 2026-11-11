#!/usr/bin/env bash
# Grappe du PIN sur trois conteneurs de sesame-fake (build/grappe.compose.yaml,
# décision 0071) : construit l'image de sesame-fake, démarre les trois
# instances et Toxiproxy, crée un proxy par sens de chaque paire, joue les
# pannes (go test -tags conteneurs ./internal/factice) : amorçage, relais au
# leader, pause, un contre deux, retour, puis conteneur c redémarré vide et
# admis après 2 minutes d'absence de son votant. Il faut docker compose, curl
# et go. GARDER=1 laisse les conteneurs en place ; SANS_CONSTRUCTION=1 reprend
# l'image sesame-fake:grappe déjà construite.
set -euo pipefail
racine=$(git rev-parse --show-toplevel)
cd "$racine"
compose=(docker compose -f build/grappe.compose.yaml)
fin() { [ "${GARDER:-}" = 1 ] || "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true; }
trap fin EXIT

[ "${SANS_CONSTRUCTION:-}" = 1 ] || "${compose[@]}" build sesame-a
"${compose[@]}" up -d --no-build
for i in $(seq 1 60); do curl -fsS http://127.0.0.1:28474/version >/dev/null 2>&1 && break; sleep 1; done
vms=(a b c)
for f in 0 1 2; do
  for t in 0 1 2; do
    [ "$f" = "$t" ] && continue
    nom="${vms[$f]}-${vms[$t]}"
    port=$((18000 + 100 * (f + 1) + t + 1))
    curl -fsS -X POST http://127.0.0.1:28474/proxies \
      -d "{\"name\":\"$nom\",\"listen\":\"0.0.0.0:$port\",\"upstream\":\"sesame-${vms[$t]}:8091\",\"enabled\":true}" >/dev/null
  done
done
for p in 28090 28091 28092; do
  for i in $(seq 1 60); do curl -fsS "http://127.0.0.1:$p/attestation" >/dev/null 2>&1 && break; sleep 1; done
done
export SESAME_CONTENEURS="a=http://127.0.0.1:28090,b=http://127.0.0.1:28091,c=http://127.0.0.1:28092"
export TOXIPROXY=http://127.0.0.1:28474
go test -count=1 -tags conteneurs -run '^TestGrappeSurConteneurs$' -v ./internal/factice
"${compose[@]}" restart sesame-c
go test -count=1 -tags conteneurs -run '^TestGrappeSurConteneursRemplacement$' -v -timeout 15m ./internal/factice
"${compose[@]}" logs --no-color | grep -c '' >/dev/null
echo "grappe.sh : grappe sur trois conteneurs, pannes jouées"
