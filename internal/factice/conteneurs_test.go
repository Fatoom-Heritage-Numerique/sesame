//go:build conteneurs

package factice_test

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
)

// Grappe du PIN sur trois conteneurs de sesame-fake en mode instance
// (build/grappe.compose.yaml, Toxiproxy entre chaque paire), jouée par
// build/grappe.sh : go test -tags conteneurs. SESAME_CONTENEURS donne les
// adresses (a=…,b=…,c=…), TOXIPROXY l'API de Toxiproxy. Chaque conteneur a son
// propre registre en mémoire : un titulaire n'est servi hors du PIN que par
// l'instance où il est enrôlé, le leader de la grappe lit le sien.

func remoteEnv(t *testing.T) *env {
	e := newEnv(t)
	e.f.Clock.Set(time.Now())
	e.remote = map[string]string{}
	for _, kv := range strings.Split(os.Getenv("SESAME_CONTENEURS"), ",") {
		k, v, _ := strings.Cut(kv, "=")
		e.remote[k] = v
	}
	if len(e.remote) != 3 {
		t.Fatal("SESAME_CONTENEURS attendu : a=<adresse>,b=<adresse>,c=<adresse>")
	}
	return e
}

// proxy active ou coupe le lien de from vers to (proxy Toxiproxy « from-to »).
func proxy(t *testing.T, from, to string, on bool) {
	b, _ := jcs.Marshal(jcs.Object{"enabled": on})
	r, err := http.Post(os.Getenv("TOXIPROXY")+"/proxies/"+from+"-"+to, "application/json", bytes.NewReader(b))
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("toxiproxy %s-%s : %v", from, to, err)
	}
	r.Body.Close()
}

func isolate(t *testing.T, p string, cut bool) {
	for _, o := range []string{"a", "b", "c"} {
		if o != p {
			proxy(t, p, o, !cut)
			proxy(t, o, p, !cut)
		}
	}
}

// eventually rappelle f jusqu'à ce qu'il rende vrai, au plus d (temps réel :
// élections et battements des conteneurs).
func eventually(t *testing.T, d time.Duration, what string, f func() bool) {
	for end := time.Now().Add(d); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
		if f() {
			return
		}
	}
	t.Fatalf("%s : pas avant %s", what, d)
}

func TestGrappeSurConteneurs(t *testing.T) { // « Grappe du PIN » : amorçage, relais, pause, un contre deux, retour
	e := remoteEnv(t)
	o := ok(t, e.open("a").call("grappe/amorcer", jcs.Object{"jetonExploitation": e.oidc(e.cfg.EnvOverride["COMPTE_EXPLOITATION"])}))
	id := str(o, "grappeId")
	for _, p := range []string{"b", "c"} {
		if g := ok(t, e.open(p).call("defi", jcs.Object{"uid": "u"}))["grappeId"]; g != id {
			t.Fatalf("%s : grappeId %v", p, g)
		}
	}
	h, c := newTitulaire("u-conteneurs"), e.open("a") // enrôlé sur a, le leader qui a amorcé
	c.enrol(h)
	x := element(t)
	ok(t, e.open("b").depot(h, "nouveau_pin", x)) // relayé au leader
	ok(t, e.open("c").evaluer(h, x))
	for _, p := range []string{"a", "b", "c"} { // pause : toute VM coupée des autres
		isolate(t, p, true)
	}
	eventually(t, 10*time.Second, "pause", func() bool { return e.open("b").evaluer(h, x)["statut"] == 503.0 })
	for _, p := range []string{"a", "b", "c"} {
		isolate(t, p, false)
	}
	eventually(t, 20*time.Second, "reprise", func() bool { return e.open("c").evaluer(h, x)["statut"] == 200.0 })
	isolate(t, "c", true) // un contre deux : la majorité continue
	eventually(t, 20*time.Second, "majorité", func() bool { return e.open("b").evaluer(h, x)["statut"] == 200.0 })
	expect(t, e.open("c").evaluer(h, x), 503, "pin_indisponible_temporaire")
	isolate(t, "c", false)
}

func TestGrappeSurConteneursRemplacement(t *testing.T) { // « Une VM perdue » : conteneur c redémarré vide, admis après 2 minutes
	e := remoteEnv(t)
	eventually(t, 6*time.Minute, "remplaçante admise", func() bool {
		a := ok(t, e.open("a").call("defi", jcs.Object{"uid": "u"}))["grappeId"]
		return a != nil && ok(t, e.open("c").call("defi", jcs.Object{"uid": "u"}))["grappeId"] == a
	})
	h := newTitulaire("u-conteneurs-2")
	e.open("a").enrol(h)
	x := element(t)
	eventually(t, 30*time.Second, "dépôt", func() bool { return e.open("c").depot(h, "nouveau_pin", x)["statut"] == 200.0 })
}
