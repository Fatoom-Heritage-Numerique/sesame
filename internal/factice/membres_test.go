package factice_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/grappe"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Admission de la VM déplacée par grappe/membres et retour de la place
// (GMB-02, GMB-06), option A (GAM-06), dérive d'horloge des VM (« Grappe du
// PIN » de tests.md, « Dérive d'horloge »).

// recreate recrée la VM de la place p, vide, dans zone.
func (e *env) recreate(p, zone string) {
	e.t.Helper()
	if err := e.f.Restart(context.Background(), p); err != nil {
		e.t.Fatal(err)
	}
	e.launcher(p).Claims.Zone = zone
}

// admitted fait passer demandes d'entrée et ticks jusqu'à la promotion de la
// VM p (au plus 20 minutes d'horloge et de ticks).
func (e *env) admitted(p string) bool {
	for range 40 {
		e.f.Clock.Advance(grappe.JoinTicks * grappe.Tick)
		e.ticks(grappe.JoinTicks)
		e.run(func() jcs.Object { e.ticks(1); return nil }, false)
		if e.g(p).ID() != "" && !e.g(p).Paused() {
			e.ticks(200) // rattrapage, retrait de l'ancien, promotion
			_, n := e.g(e.leader()).Borne()
			return n == 3
		}
	}
	return false
}

func TestPlaceDeplacee(t *testing.T) { // GMB-02, GMB-06, GRJ-02, « Zone indisponible longtemps », API.md §4.1, §4.2
	e, h, x := pinCluster(t)
	id, lead := e.g("a").ID(), e.leader()
	victim, other := map[string][2]string{"a": {"c", "b"}, "b": {"c", "a"}, "c": {"b", "a"}}[lead][0], map[string][2]string{"a": {"c", "b"}, "b": {"c", "a"}, "c": {"b", "a"}}[lead][1]
	home, away := "europe-west9-"+victim, "europe-west9-"+other
	command := func(zone string) jcs.Object {
		l := jcs.Array{}
		for _, p := range []string{"a", "b", "c"} {
			z := "europe-west9-" + p
			if p == victim {
				z = zone
			}
			l = append(l, jcs.Object{"vm": vm(p), "zone": z})
		}
		c := e.open(other)
		b := jcs.Object{"jetonExploitation": e.oidc(e.cfg.EnvOverride["COMPTE_EXPLOITATION"]), "grappeId": id, "membres": l}
		return e.run(func() jcs.Object { return c.call("grappe/membres", b) }, true)
	}
	e.f.Net.Remove(vm(victim)) // zone indisponible
	e.ticks(grappe.AbsentTicks + 10)
	e.recreate(victim, away) // sans la commande : refusée (zone autre que celle inscrite)
	e.ticks(grappe.JoinTicks + 1)
	e.run(func() jcs.Object { e.ticks(1); return nil }, false)
	if e.g(victim).ID() != "" || e.events("grappe_admission_refusee") == 0 {
		t.Fatal("VM d'une autre zone admise sans grappe/membres")
	}
	e.f.Net.Remove(vm(victim))
	ok(t, command(away))
	e.recreate(victim, away) // GMB-02 : admise, ancien membre retiré, promue
	if !e.admitted(victim) || e.g(victim).ID() != id {
		t.Fatal("VM déplacée non admise")
	}
	ok(t, e.run(func() jcs.Object { return e.open(victim).evaluer(h, x) }, true))
	if r, _ := e.g(victim).Record(h.uid); r.Evals != 1 {
		t.Fatalf("copie de la VM déplacée : %+v", r)
	}
	// GMB-06 : retour de la place, VM déplacée arrêtée, liste d'origine.
	e.f.Net.Remove(vm(victim))
	e.ticks(grappe.AbsentTicks + 10)
	ok(t, command(home))
	e.recreate(victim, home)
	if !e.admitted(victim) || e.g(victim).ID() != id {
		t.Fatal("place non revenue dans sa zone d'origine")
	}
	if r, _ := e.g(victim).Record(h.uid); r.Evals != 1 || r.Left != 9 {
		t.Fatalf("copie après le retour : %+v", r)
	}
}

func TestOptionA(t *testing.T) { // GAM-06, GAM-02, API.md §4.3
	e := newEnv(t, manual)
	e.launcher("b").Claims.Zone = "europe-west9-a" // deux VM dans la même zone
	_, peer := e.pair("a", "c")
	digest := str(sub(sub(peer, "submods"), "container"), "image_digest")
	id := str(ok(t, e.amorcer("c")), "grappeId")
	ms := jcs.Array{}
	for _, p := range []string{"a", "b", "c"} { // triés par vm
		m := e.member(p)
		if p == "b" {
			m["zone"] = "europe-west9-a"
		}
		ms = append(ms, m)
	}
	b, _ := jcs.Marshal(jcs.Object{"v": 1, "imageDigest": digest, "membres": ms})
	if id != hex.EncodeToString(primitives.SHA256(b)) || e.g("b").ID() != id {
		t.Fatalf("configuration de l'option A : %s", id)
	}
}

func TestDeriveHorloge(t *testing.T) { // « Dérive d'horloge » (tests.md), API.md §2.13 ; R-PIN-15
	drift := func(o *factice.Options) {
		o.Decalage = map[string]time.Duration{"a": 2 * time.Minute, "b": -2 * time.Minute}
	}
	e := newEnv(t, manual, drift)
	ok(t, e.amorcer("a"))
	if e.leader() != "a" {
		t.Fatal("la VM qui commande n'est pas leader")
	}
	h, c := enrolled(t, e, "u-derive")
	x := element(t)
	ok(t, e.run(func() jcs.Object { return c.depot(h, "nouveau_pin", x) }, false))
	for range 3 { // essais 1 à 3, sans délai ; le 4ᵉ attend 1 min à l'heure du leader a (+2 min)
		ok(t, e.run(func() jcs.Object { return c.evaluer(h, x) }, false))
	}
	next := e.now().Add(3 * time.Minute)
	e.f.Net.Remove(vm("a")) // changement de leader
	e.ticks(60)
	nl := e.leader()
	cl := e.open(nl)
	try := func() jcs.Object { return e.run(func() jcs.Object { return cl.evaluer(h, x) }, true) }
	r := try()
	expect(t, r, 429, "trop_d_essais")
	if at := str(sub(r, "details"), "reessayerLe"); at != next.Format(primitives.TimeLayout) {
		t.Fatalf("reessayerLe : %s, attendu %s", at, next.Format(primitives.TimeLayout))
	}
	shift := map[string]time.Duration{"b": -2 * time.Minute, "c": 0}[nl]
	e.f.Clock.Set(next.Add(-shift - time.Second)) // heure du nouveau leader juste avant prochainEssaiLe
	expect(t, try(), 429, "trop_d_essais")
	e.f.Clock.Advance(time.Second)
	ok(t, try())
	if e.now().Before(next.Add(-2 * time.Minute)) {
		t.Fatal("essai admis plus de 2 minutes avant l'heure renvoyée")
	}
	// Absence d'un votant comptée en ticks : un saut de l'heure murale
	// n'admet pas la VM recréée avant 2 minutes.
	e.recreate("a", "europe-west9-a")
	e.f.Clock.Advance(10 * time.Minute)
	e.ticks(grappe.JoinTicks + 1)
	e.run(func() jcs.Object { e.ticks(1); return nil }, false)
	if e.g("a").ID() != "" {
		t.Fatal("admise sur un saut de l'heure murale")
	}
	// Décalage au-delà de 2 minutes : ce qu'écrit cette VM est rejeté par le
	// contrôle de fraîcheur, jamais accepté. Elle écrit dès son démarrage
	// (instances/{id}) : chaine/tete rejetée, aucune VM n'écrit plus au
	// registre (échec fermé, registre_indisponible).
	f := newEnv(t, manual, func(o *factice.Options) { o.Decalage = map[string]time.Duration{"c": 3 * time.Minute} })
	k, ca := newTitulaire("u-derive3"), f.open("a")
	expect(t, ca.call("enroler", ca.enrolBody(k, 1, k.pub, nil)), 503, "registre_indisponible")
	if le := str(f.f.Base.Doc("chaine/tete").Data, "ecritLe"); le != t0.Add(3*time.Minute).Format(primitives.TimeLayout) {
		t.Fatalf("chaine/tete : %s", le)
	}
}
