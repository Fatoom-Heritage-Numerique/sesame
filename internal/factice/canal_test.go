package factice_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
)

// Cas « Enveloppe, statut et canal » et « GET /attestation » de
// routes-etat.md (API.md §1.1 à §1.3, §2.1).

func TestAttestationNominale(t *testing.T) { // ATT-01, API.md §2.1
	e := newEnv(t)
	for _, p := range []string{"a", "b", "c"} {
		o := e.attest(p)
		if len(o) != 4 {
			t.Fatalf("champs de GET /attestation : %v", o)
		}
	}
}

func TestAttestationIndisponible(t *testing.T) { // ATT-02, API.md §2.1
	e := newEnv(t)
	c := e.open("a")
	e.f.LauncherDown = true
	if err := e.f.Restart(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		w := e.do("GET", "/attestation", "198.51.100.9", nil)
		if w.Code == 200 {
			continue // une autre instance, dont le jeton est déjà obtenu
		}
		clearErr(t, w, 503, "attestation_indisponible")
	}
	clearErr(t, c.post(c.envelope(1, "defi", jcs.Object{"uid": "u1"})), 503, "attestation_indisponible")
}

func TestAttestationLimite(t *testing.T) { // ATT-03, API.md §2.1, §7
	instanceMode = "a"
	defer func() { instanceMode = "" }()
	e := newEnv(t)
	for i := range 60 {
		if w := e.do("GET", "/attestation", "203.0.113.5", nil); w.Code != 200 {
			t.Fatalf("appel %d refusé : %d", i+1, w.Code)
		}
	}
	w := e.do("GET", "/attestation", "203.0.113.5", nil)
	clearErr(t, w, 429, "trop_d_appels")
	if !strings.Contains(w.Body.String(), `"reessayerLe":"2026-11-11T10:01:00.000Z"`) {
		t.Fatalf("reessayerLe : %s", w.Body)
	}
	if w := e.do("GET", "/attestation", "203.0.113.6", nil); w.Code != 200 {
		t.Fatal("autre IP refusée")
	}
}

func TestEnveloppeEtStatut(t *testing.T) { // ENV-01, ENV-10, API.md §1.1, §1.3
	e := newEnv(t)
	c := e.open("b")
	ok(t, c.call("defi", jcs.Object{"uid": "u1"}))
	expect(t, c.raw("route/inconnue", jcs.Object{}), 400, "message_refuse")
	for _, p := range []string{"/attestation", "/n/a/canal", "/n/b/canal", "/n/c/canal"} {
		w := e.do("OPTIONS", p, "198.51.100.7", nil)
		h := w.Header()
		if w.Code != 204 || h.Get("Access-Control-Allow-Methods") != "GET, POST" || h.Get("Access-Control-Allow-Headers") != "Content-Type" ||
			h.Get("Access-Control-Max-Age") != "86400" || h.Get("Access-Control-Allow-Origin") != "*" || h.Get("Cache-Control") != "no-store" {
			t.Fatalf("OPTIONS %s : %d %v", p, w.Code, h)
		}
	}
}

func TestCheminUnique(t *testing.T) { // ENV-02, API.md §1.1
	e := newEnv(t)
	c := e.open("a")
	env := c.envelope(1, "defi", jcs.Object{"uid": "u1"})
	for _, p := range []string{"/canal", "/n/d/canal", "/n/a/defi", "/defi", "/n/a/canal/x", "/n/a/canal?x=1"} {
		if w := e.do("POST", p, c.ip, env); w.Code != 404 {
			t.Fatalf("%s servi : %d", p, w.Code)
		}
	}
	if w := e.do("GET", "/n/a/canal", c.ip, nil); w.Code != 405 {
		t.Fatalf("GET sur le canal : %d", w.Code)
	}
	c.n = 1
	ok(t, c.openResponse(1, c.post(env)))
}

func TestCorpsHorsContrat(t *testing.T) { // ENV-03, API.md §1.7
	e := newEnv(t)
	c := e.open("c")
	for _, plain := range []string{
		`{"corps":{"uid":"u1"},"route":"inconnue"}`,
		`{"corps":{"uid":"u1","x":1},"route":"defi"}`,
		`{"corps":{},"route":"defi"}`,
		`{"corps":{"uid":null},"route":"defi"}`,
		`{"corps":{"uid":12},"route":"defi"}`,
		`{"corps":{"uid":"u1","uid":"u2"},"route":"defi"}`,
		`{"corps":{"defi":"AAAA","signature":"` + strings.Repeat("A", 86) + `==","uid":"u1"},"route":"vie"}`,
		"{\"corps\":{\"email\":\"é@x.fr\",\"generation\":1,\"signature\":\"" + strings.Repeat("A", 86) + "==\",\"telephone\":null,\"uid\":\"u1\"},\"route\":\"alerte/fiche\"}",
		`{"corps":{"uid":"` + strings.Repeat("a", 65<<10) + `"},"route":"defi"}`,
		`{"route":"defi","corps":{"uid":"u1"}}`,
	} {
		c.n++
		expect(t, c.openResponse(c.n, c.post(c.sealed(c.n, []byte(plain)))), 400, "message_refuse")
	}
}

func TestInstanceRedemarree(t *testing.T) { // ENV-04, API.md §1.2
	e := newEnv(t)
	c := e.open("a")
	ok(t, c.call("defi", jcs.Object{"uid": "u1"}))
	if err := e.f.Restart(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	clearErr(t, c.post(c.envelope(2, "defi", jcs.Object{"uid": "u1"})), 410, "instance_inconnue")
	ok(t, e.open("a").call("defi", jcs.Object{"uid": "u1"}))
}

func TestCanalInvalide(t *testing.T) { // ENV-05, ENV-06, ENV-07, ENV-08, API.md §1.2, §1.3
	e := newEnv(t)
	c := e.open("b")
	big := bytes.Repeat([]byte("a"), 400<<10+1)
	clearErr(t, c.post(big), 400, "canal_invalide")
	bad := c.envelope(1, "defi", jcs.Object{"uid": "u1"})
	bad = bytes.Replace(bad, []byte(`"body":"`), []byte(`"body":"AAAA`), 1)
	clearErr(t, c.post(bad), 400, "canal_invalide")
	for _, n := range []int64{0, 1 << 32} {
		env := bytes.Replace(c.envelope(1, "defi", jcs.Object{"uid": "u1"}), []byte(`"n":1`), []byte(`"n":`+itoa(n)), 1)
		clearErr(t, c.post(env), 400, "canal_invalide")
	}
	// ENV-06 : la requête indéchiffrable n'a pas consommé n = 1.
	c.n = 0
	ok(t, c.call("defi", jcs.Object{"uid": "u1"}))
	// ENV-07 : rejeu à l'identique, puis même n, puis n inférieur.
	env := c.envelope(5, "defi", jcs.Object{"uid": "u1"})
	ok(t, c.openResponse(5, c.post(env)))
	clearErr(t, c.post(env), 400, "canal_invalide")
	clearErr(t, c.post(c.envelope(5, "defi", jcs.Object{"uid": "u2"})), 400, "canal_invalide")
	clearErr(t, c.post(c.envelope(3, "defi", jcs.Object{"uid": "u2"})), 400, "canal_invalide")
	// ENV-08 : trous admis.
	ok(t, c.openResponse(9, c.post(c.envelope(9, "defi", jcs.Object{"uid": "u1"}))))
	// kemCt ou ivPrefixe changé pour un même cliPub.
	c2 := *c
	c2.pref = []byte{9, 9, 9, 9}
	clearErr(t, c2.post(c2.envelope(10, "defi", jcs.Object{"uid": "u1"})), 400, "canal_invalide")
	c3 := *c
	c3.kemCt = e.open("b").kemCt
	clearErr(t, c3.post(c3.envelope(10, "defi", jcs.Object{"uid": "u1"})), 400, "canal_invalide")
	// Secret X25519 nul : point d'ordre faible.
	c4 := e.open("b")
	c4.cliPub = make([]byte, 32)
	clearErr(t, c4.post(c4.envelope(1, "defi", jcs.Object{"uid": "u1"})), 400, "canal_invalide")
}

func TestCanalFerme(t *testing.T) { // ENV-05 (canal fermé), ENV-14, API.md §1.2
	e := newEnv(t)
	c := e.open("a")
	ok(t, c.call("defi", jcs.Object{"uid": "u1"}))
	e.f.Clock.Advance(time.Hour)
	tick(e)
	clearErr(t, c.post(c.envelope(2, "defi", jcs.Object{"uid": "u1"})), 400, "canal_invalide")
	// Filtre de taille fixe : 10 000 canaux fermés, mémoire constante.
	in := e.f.Server.Instances[0]
	for i := range 2000 {
		c := e.open("a")
		ok(t, c.call("defi", jcs.Object{"uid": "u" + itoa(int64(i))}))
	}
	if in.Channels() != 2000 {
		t.Fatalf("canaux ouverts : %d", in.Channels())
	}
	e.f.Clock.Advance(time.Hour)
	tick(e)
	if in.Channels() != 0 {
		t.Fatal("canaux inactifs non fermés")
	}
}

func TestLimiteParIP(t *testing.T) { // ENV-09, ENV-11, API.md §1.1, §7
	e := newEnv(t)
	for i := range 4 {
		c := e.open("a")
		c.ip = "192.0.2.1"
		for j := range 30 {
			if i*30+j >= 120 {
				break
			}
			ok(t, c.call("defi", jcs.Object{"uid": "u1"}))
		}
	}
	c := e.open("a")
	c.ip = "192.0.2.1"
	r := c.call("defi", jcs.Object{"uid": "u1"})
	expect(t, r, 429, "trop_d_appels")
	if str(sub(r, "details"), "reessayerLe") != "2026-11-11T11:00:00.000Z" {
		t.Fatalf("reessayerLe : %v", r)
	}
	// Seule l'avant-dernière valeur de X-Forwarded-For compte.
	c.ip = "192.0.2.1, 192.0.2.2"
	ok(t, c.call("defi", jcs.Object{"uid": "u1"}))
	c.ip = "192.0.2.3, 192.0.2.1"
	expect(t, c.call("defi", jcs.Object{"uid": "u1"}), 429, "trop_d_appels")
}

func itoa(n int64) string {
	b, _ := jcs.Marshal(n)
	return string(b)
}

// tick fait passer les tâches de fond de chaque instance à l'heure fixe.
func tick(e *env) {
	for _, in := range e.f.Server.Instances {
		var a, b, c, d time.Time
		in.Tick(context.Background(), &a, &b, &c, &d)
	}
}
