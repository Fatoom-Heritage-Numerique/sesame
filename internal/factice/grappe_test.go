package factice_test

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/grappe"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
)

// Cas de « Grappe du PIN » (tests.md de Fatoom) et des routes grappe/* de
// routes-etat.md, sur le réseau entre VM simulé en mémoire : aucune goroutine
// d'envoi, le test fait passer les messages et les ticks (horloge et réseau
// simulés, aucune attente réelle).

func manual(o *factice.Options) { o.Manual = true }

// run exécute f dans une goroutine et fait passer messages et ticks jusqu'à
// sa fin.
func (e *env) run(f func() jcs.Object, ticks bool) jcs.Object {
	e.t.Helper()
	done := make(chan jcs.Object, 1)
	go func() { done <- f() }()
	for i := 0; i < 200_000; i++ {
		select {
		case r := <-done:
			return r
		default:
		}
		if e.f.Pump(1); ticks && i%50 == 0 {
			e.f.TickGrappe()
		}
		runtime.Gosched()
	}
	e.t.Fatal("aucune réponse")
	return nil
}

// ticks fait passer n ticks de Raft à toutes les instances.
func (e *env) ticks(n int) {
	for range n {
		e.f.TickGrappe()
	}
}

func (e *env) g(p string) *grappe.Grappe {
	for _, in := range e.f.Server.Instances {
		if in.ID[:1] == p {
			return in.Grappe()
		}
	}
	return nil
}

func (e *env) events(name string) int {
	n := 0
	for _, ev := range e.f.Journal.Events() {
		if ev.Event == name {
			n++
		}
	}
	return n
}

func (e *env) amorcer(p string) jcs.Object {
	c := e.open(p)
	return e.run(func() jcs.Object {
		return c.call("grappe/amorcer", jcs.Object{"jetonExploitation": e.oidc(e.cfg.EnvOverride["COMPTE_EXPLOITATION"])})
	}, false)
}

// leader rend le préfixe du leader, après des ticks au besoin.
func (e *env) leader() string {
	for range 500 {
		for _, p := range []string{"a", "b", "c"} {
			if g := e.g(p); g != nil && g.Leader() {
				return p
			}
		}
		e.f.TickGrappe()
	}
	e.t.Fatal("aucun leader")
	return ""
}

func TestAmorcage(t *testing.T) { // GAM-01, GAM-02, GAM-03, GAM-04, GAM-11, GAM-12, GAM-13, DEF-03, API.md §4.3, §4.1
	e := newEnv(t, manual)
	c := e.open("b")
	if o := ok(t, c.call("defi", jcs.Object{"uid": "u"})); o["grappeId"] != nil { // VM sans grappe
		t.Fatalf("défi : %v", o)
	}
	if id, n := e.g("b").Borne(); id != nil || n != 0 {
		t.Fatal("borne d'une VM sans grappe")
	}
	h, _ := enrolled(t, e, "u-sg")
	expect(t, e.open("c").evaluer(h, element(t)), 503, "pin_indisponible_temporaire")
	e.ticks(grappe.JoinTicks) // demandes d'entrée renouvelées : refusee par des VM sans grappe (GAM-13)
	for i := 0; i < 100_000 && e.events("grappe_admission_refusee") == 0; i++ {
		runtime.Gosched()
	}
	if e.events("grappe_admission_refusee") == 0 {
		t.Fatal("refus non journalisé")
	}
	expect(t, c.call("grappe/amorcer", jcs.Object{"jetonExploitation": e.oidc("autre@example.org")}), 401, "jeton_invalide")
	o := ok(t, e.amorcer("b"))
	id := str(o, "grappeId")
	if o["votants"] != 3.0 || len(id) != 64 || e.g("a").ID() != id || e.g("c").ID() != id || e.events("grappe_amorcee") != 1 {
		t.Fatalf("amorçage : %v", o)
	}
	if gid, n := e.g("c").Borne(); gid != id || n != 3 {
		t.Fatal("borne de la grappe neuve")
	}
	expect(t, e.amorcer("a"), 409, "amorcage_refuse") // VM déjà dans une grappe (GAM-04)
	r := e.amorcer("b")
	expect(t, r, 409, "amorcage_refuse")
	again := func() jcs.Object {
		return c.call("grappe/amorcer", jcs.Object{"jetonExploitation": e.oidc(e.cfg.EnvOverride["COMPTE_EXPLOITATION"])})
	}
	expect(t, e.run(again, false), 409, "amorcage_refuse")
	expect(t, e.run(again, false), 429, "trop_d_appels") // 3ᵉ en une minute d'une IP
	f := newEnv(t, manual)                               // VM recréées : autre grappeId (GAM-11)
	if str(ok(t, f.amorcer("a")), "grappeId") == id {
		t.Fatal("même grappeId")
	}
}

// pinCluster rend une grappe amorcée et un titulaire qui a déposé.
func pinCluster(t *testing.T) (*env, *titulaire, []byte) {
	e := newEnv(t, manual)
	ok(t, e.amorcer("a"))
	h, c := enrolled(t, e, "u-g")
	x := element(t)
	ok(t, e.run(func() jcs.Object { return c.depot(h, "nouveau_pin", x) }, false))
	return e, h, x
}

func TestPartitions(t *testing.T) { // « Deux VM injoignables mais vivantes », VIE-02, ETA-06, ENV-15, API.md §2.4, §2.5
	e, h, x := pinCluster(t)
	ok(t, e.run(func() jcs.Object { return e.open("b").evaluer(h, x) }, false))
	for _, p := range []string{"a", "b", "c"} {
		e.f.Net.Isolate("sesame-test-"+p, true)
	}
	e.ticks(40) // élection sans majorité ; PreVote : aucun changement de terme
	for _, p := range []string{"a", "b", "c"} {
		c := e.open(p)
		expect(t, e.run(func() jcs.Object { return c.evaluer(h, x) }, true), 503, "pin_indisponible_temporaire")
		if o := ok(t, c.life(h)); o["pinPresent"] != true || o["pinBloque"] != false { // dernier état connu, au registre
			t.Fatalf("vie en pause : %v", o)
		}
		if p := e.payload(str(ok(t, c.etat(h, nil, 0)), "etat"), false); p["evaluationsDepuis"] != 1.0 {
			t.Fatalf("etat en pause : %v", p)
		}
	}
	if e.events("grappe_sans_majorite") < 3 {
		t.Fatal("pause non journalisée")
	}
	if r, _ := e.g("a").Record(h.uid); r.Evals != 1 {
		t.Fatal("essai décompté pendant la pause")
	}
	for _, p := range []string{"a", "b", "c"} {
		e.f.Net.Isolate("sesame-test-"+p, false)
	}
	e.f.Net.Isolate("sesame-test-"+e.leader(), true) // un contre deux : le côté majoritaire continue
	e.ticks(60)
	var maj string
	for _, p := range []string{"a", "b", "c"} {
		if e.g(p).Leader() {
			maj = p
		}
	}
	if maj == "" {
		maj = e.leader()
	}
	ok(t, e.run(func() jcs.Object { return e.open(maj).evaluer(h, x) }, true))
	for _, p := range []string{"a", "b", "c"} {
		e.f.Net.Isolate("sesame-test-"+p, false)
	}
	e.ticks(60)
	for _, p := range []string{"a", "b", "c"} { // retour : état identique sur les trois
		if r, _ := e.g(p).Record(h.uid); r.Evals != 2 || r.Left != 8 {
			t.Fatalf("%s après la coupure : %+v", p, r)
		}
	}
}

func TestRemplacement(t *testing.T) { // « Une VM perdue », « Renouvellement d'un membre », GRJ-01, GRJ-02, API.md §4.1
	e, h, x := pinCluster(t)
	lost := "c"
	if e.leader() == "c" {
		lost = "b"
	}
	e.f.Net.Remove("sesame-test-" + lost)
	if err := e.f.Restart(context.Background(), lost); err != nil { // recréée vide sous le même nom, même zone
		t.Fatal(err)
	}
	e.ticks(grappe.JoinTicks + 1) // demande avant 2 minutes d'absence : refusée
	e.run(func() jcs.Object { e.ticks(1); return nil }, false)
	if e.g(lost).ID() != "" || e.events("grappe_admission_refusee") == 0 {
		t.Fatal("admise avant 2 minutes")
	}
	ok(t, e.run(func() jcs.Object { return e.open(e.leader()).evaluer(h, x) }, true)) // la grappe sert à deux votants
	for range 40 {
		e.f.Clock.Advance(grappe.JoinTicks * grappe.Tick) // horloge murale et ticks avancent ensemble
		e.ticks(grappe.JoinTicks)
		e.run(func() jcs.Object { e.ticks(1); return nil }, false)
		if e.g(lost).ID() != "" && !e.g(lost).Paused() {
			break
		}
	}
	e.ticks(200) // rattrapage, retrait de l'ancien, promotion
	if e.g(lost).ID() != e.g("a").ID() {
		t.Fatal("remplaçante non admise")
	}
	if id, n := e.g(e.leader()).Borne(); id == nil || n != 3 {
		t.Fatalf("votants : %d", n)
	}
	if r, _ := e.g(lost).Record(h.uid); r.Evals != 1 || r.Left != 9 { // copie complète reçue
		t.Fatalf("remplaçante : %+v", r)
	}
}

func TestTransportEntreVM(t *testing.T) { // « Transport », GRJ-01, API.md §3, §4
	e, _, _ := pinCluster(t)
	g := e.g("a")
	ctx := context.Background()
	if st, b := g.Serve(ctx, "inconnue", []byte("cle"), "/interne/raft", nil); st != 403 || !strings.Contains(string(b), "pair_refuse") {
		t.Fatalf("raft avant l'échange : %d", st) // toute requête avant la fin de l'échange
	}
	if st, _ := g.Serve(ctx, "x", []byte("cle"), "/interne/attestation", []byte(`{"jwt":"a.b.c","nodeId":"AAAAAAAAAAAAAAAAAAAAAA"}`)); st != 403 {
		t.Fatalf("jeton sans nonce : %d", st)
	}
	if st, _ := g.Serve(ctx, "x", []byte("cle"), "/interne/attestation", []byte(`{"noncePaire":"court"}`)); st != 403 {
		t.Fatalf("nonce hors forme : %d", st)
	}
	st, b := g.Serve(ctx, "y", []byte("cle"), "/interne/attestation", []byte(`{"noncePaire":"`+strings.Repeat("A", 43)+`"}`))
	if st != 200 || !strings.Contains(string(b), `"nodeId"`) {
		t.Fatalf("échange : %d %s", st, b)
	}
	if st, _ := g.Serve(ctx, "y", []byte("cle"), "/interne/attestation", []byte(`{"jwt":"a.b.c","nodeId":"AAAAAAAAAAAAAAAAAAAAAA"}`)); st != 403 {
		t.Fatalf("jeton refusé : %d", st)
	}
	if st, _ := g.Serve(ctx, "y", []byte("cle"), "/grappe/rejoindre", []byte(`{}`)); st != 403 {
		t.Fatalf("après refus : %d", st)
	}
}

func TestArretDuLeader(t *testing.T) { // « Consensus », « Une VM perdue », PEV-01 (changement de leader), API.md §2.13
	e, h, x := pinCluster(t)
	old := e.leader()
	ok(t, e.run(func() jcs.Object { return e.open(old).evaluer(h, x) }, false))
	e.f.Net.Remove("sesame-test-" + old) // leader arrêté
	e.ticks(60)
	nl := e.leader()
	if nl == old {
		t.Fatal("pas de nouvelle élection")
	}
	for _, p := range []string{"a", "b", "c"} { // la VM restante sert par relais
		if p != old {
			ok(t, e.run(func() jcs.Object { return e.open(p).evaluer(h, x) }, true))
		}
	}
	if r, _ := e.g(nl).Record(h.uid); r.Evals != 3 { // aucune écriture perdue
		t.Fatalf("après l'élection : %+v", r)
	}
}
