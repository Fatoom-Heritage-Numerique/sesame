package factice_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/grappe"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Pannes de l'amorçage (GAM-05 à GAM-10), admission après grappe/membres et
// retour de la place (GMB-02, GMB-06), dérive d'horloge des VM (« Grappe du
// PIN » de tests.md) : réseau entre VM simulé et horloge pilotée.

func vm(p string) string { return "sesame-test-" + p }

// launcher rend le lanceur factice de l'instance de préfixe p.
func (e *env) launcher(p string) *factice.Launcher {
	for i, in := range e.f.Server.Instances {
		if in.ID[:1] == p {
			return e.f.Launchers[i]
		}
	}
	e.t.Fatalf("instance %s absente", p)
	return nil
}

// pair ouvre, au nom de la VM from (son identifiant de nœud, sa clé, son
// lanceur), une connexion attestée vers to : jetons de paire échangés
// (POST /interne/attestation). Rend le lien et les claims du jeton de to.
func (e *env) pair(from, to string) (ports.Lien, jcs.Object) {
	e.t.Helper()
	ctx, g := context.Background(), e.g(from)
	l, err := e.f.Net.From(vm(from)).Dial(ctx, vm(to))
	if err != nil {
		e.t.Fatal(err)
	}
	st, b := e.inter(l, "/interne/attestation", jcs.Object{"noncePaire": base64.RawURLEncoding.EncodeToString(random(32))})
	nb, _ := primitives.B64URL(str(b, "noncePaire"))
	nonces, err := attestation.PairNonces(nb, g.NodeID, g.Cle())
	if err != nil || st != 200 {
		e.t.Fatalf("échange : %d %v", st, err)
	}
	jwt, _ := e.launcher(from).Token(ctx, nonces)
	if st, _ := e.inter(l, "/interne/attestation", jcs.Object{"jwt": jwt, "nodeId": g.NodeID}); st != 204 {
		e.t.Fatalf("jeton refusé : %d", st)
	}
	parts := strings.Split(str(b, "jwt"), ".")
	claims, _ := base64.RawURLEncoding.DecodeString(parts[1])
	return l, decode(e.t, claims)
}

// inter envoie body sur un lien entre VM et rend le vrai statut et le corps.
func (e *env) inter(l ports.Lien, path string, body jcs.Object) (int, jcs.Object) {
	e.t.Helper()
	b, _ := jcs.Marshal(body)
	st, resp, err := l.Call(context.Background(), path, b)
	if err != nil {
		e.t.Fatal(err)
	}
	if len(resp) == 0 {
		return st, nil
	}
	return st, decode(e.t, resp)
}

// member rend le membre de la configuration initiale pour la VM p.
func (e *env) member(p string) jcs.Object {
	g := e.g(p)
	return jcs.Object{"nodeId": g.NodeID, "vm": vm(p), "zone": "europe-west9-" + p, "cle": base64.StdEncoding.EncodeToString(g.Cle())}
}

func refused(t *testing.T, st int, b jcs.Object, want int, code string) {
	t.Helper()
	if st != want || str(b, "erreur") != code {
		t.Fatalf("attendu %d %s, obtenu %d %v", want, code, st, b)
	}
}

func TestAmorcageRefuse(t *testing.T) { // GAM-05, GAM-09, API.md §4.3 ; écart E-22
	for _, cas := range []string{"injoignable", "empreinte", "hwmodel"} {
		e := newEnv(t, manual)
		switch cas {
		case "injoignable":
			e.f.Net.Cut(vm("a"), vm("c"), true)
		case "empreinte":
			e.launcher("c").Claims.Edit = func(c jcs.Object) {
				c["submods"].(jcs.Object)["container"].(jcs.Object)["image_digest"] = "sha256:" + strings.Repeat("0", 64)
			}
		case "hwmodel":
			e.launcher("c").Claims.Edit = func(c jcs.Object) { c["hwmodel"] = "GCP_AUTRE_MODELE" }
		}
		expect(t, e.amorcer("a"), 409, "amorcage_refuse")
		e.f.Net.Cut(vm("a"), vm("c"), false)
		e.launcher("c").Claims.Edit = nil
		ok(t, e.amorcer("a")) // aucune VM n'a gardé de réservation : amorçage admis aussitôt
	}
	// Premier pair (b) déjà dans une grappe : refus avant toute réservation.
	e := newEnv(t, manual)
	ok(t, e.amorcer("a"))
	for _, p := range []string{"a", "c"} {
		if err := e.f.Restart(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	expect(t, e.amorcer("a"), 409, "amorcage_refuse")
	if err := e.f.Restart(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	ok(t, e.amorcer("a"))
	// Second pair (c) déjà dans une grappe : b a déjà réservé (E-22) ; sa
	// réservation n'est oubliée qu'après 2 minutes (GAM-09), faute d'un
	// message du contrat qui l'annule.
	for _, p := range []string{"a", "b"} {
		if err := e.f.Restart(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	expect(t, e.amorcer("a"), 409, "amorcage_refuse")
	if err := e.f.Restart(context.Background(), "c"); err != nil {
		t.Fatal(err)
	}
	expect(t, e.amorcer("a"), 409, "amorcage_refuse") // réservation de b en cours
	e.f.Clock.Advance(grappe.Reservation)
	e.ticks(1)
	ok(t, e.amorcer("a"))
}

func TestPreparerDemarrer(t *testing.T) { // GAM-07, GAM-08, GAM-09, GAM-10, GAM-12, API.md §4.3, §7
	e := newEnv(t, manual)
	ab, peer := e.pair("a", "b")
	digest := str(sub(sub(peer, "submods"), "container"), "image_digest")
	conf := func(edit func(m []jcs.Object)) jcs.Object {
		m := []jcs.Object{e.member("a"), e.member("b"), e.member("c")}
		if edit != nil {
			edit(m)
		}
		return jcs.Object{"configuration": jcs.Object{"v": 1, "imageDigest": digest, "membres": jcs.Array{m[0], m[1], m[2]}}}
	}
	wrong := []func(m []jcs.Object){
		func(m []jcs.Object) { m[2]["nodeId"] = e.g("a").NodeID },
		func(m []jcs.Object) { m[2]["zone"] = "europe-west9-a" },
		func(m []jcs.Object) { m[2]["cle"] = m[0]["cle"] },
		nil, // imageDigest différent, ci-dessous
	}
	for i, w := range wrong {
		if i == 2 { // 2 / min par nodeId
			st, b := e.inter(ab, "/grappe/preparer", conf(w))
			refused(t, st, b, 429, "trop_d_appels")
			e.f.Clock.Advance(time.Minute)
		}
		c := conf(w)
		if w == nil {
			c["configuration"].(jcs.Object)["imageDigest"] = "sha256:" + strings.Repeat("1", 64)
		}
		st, b := e.inter(ab, "/grappe/preparer", c)
		refused(t, st, b, 409, "amorcage_refuse") // ne décrit pas les VM que voit b (GAM-07)
	}
	e.f.Clock.Advance(time.Minute)
	st, b := e.inter(ab, "/grappe/preparer", conf(nil))
	id := str(b, "grappeId")
	if st != 200 || b["reservee"] != true || len(id) != 64 {
		t.Fatalf("réservation : %d %v", st, b)
	}
	cb, _ := e.pair("c", "b")
	st, b = e.inter(cb, "/grappe/preparer", conf(nil))
	refused(t, st, b, 409, "amorcage_refuse") // seconde configuration pendant la réservation
	st, b = e.inter(cb, "/grappe/demarrer", jcs.Object{"grappeId": id})
	refused(t, st, b, 409, "amorcage_refuse") // autre VM que celle qui a fait réserver (GAM-08)
	st, b = e.inter(ab, "/grappe/demarrer", jcs.Object{"grappeId": strings.Repeat("ab", 32)})
	refused(t, st, b, 409, "amorcage_refuse") // autre grappeId
	ac, _ := e.pair("a", "c")
	st, b = e.inter(ac, "/grappe/demarrer", jcs.Object{"grappeId": id})
	refused(t, st, b, 409, "amorcage_refuse") // sans réservation
	e.f.Clock.Advance(grappe.Reservation)
	st, b = e.inter(ab, "/grappe/demarrer", jcs.Object{"grappeId": id})
	refused(t, st, b, 409, "amorcage_refuse") // après les 2 minutes
	e.ticks(1)
	// GAM-10 : la VM qui commande (a) tombe après grappe/demarrer vers b
	// seule : b démarrée reste en pause, c oublie sa réservation.
	for _, l := range []ports.Lien{ab, ac} {
		if st, b := e.inter(l, "/grappe/preparer", conf(nil)); st != 200 || str(b, "grappeId") != id {
			t.Fatalf("réservation : %d %v", st, b)
		}
	}
	if st, b := e.inter(ab, "/grappe/demarrer", jcs.Object{"grappeId": id}); st != 200 || b["demarree"] != true {
		t.Fatalf("démarrage : %d %v", st, b)
	}
	e.ticks(100) // a, qui commande, ne démarre pas : panne

	if e.g("b").ID() != id || !e.g("b").Paused() {
		t.Fatal("b démarrée sans majorité : en pause attendue")
	}
	h, _ := enrolled(t, e, "u-gam10")
	expect(t, e.open("b").evaluer(h, element(t)), 503, "pin_indisponible_temporaire")
	e.f.Clock.Advance(grappe.Reservation)
	e.ticks(1)
	if e.g("c").ID() != "" {
		t.Fatal("c dans une grappe")
	}
	// c redevient sans grappe et accepte une nouvelle configuration (GAM-09).
	if st, b := e.inter(ac, "/grappe/preparer", conf(nil)); st != 200 || b["reservee"] != true {
		t.Fatalf("après l'oubli : %d %v", st, b)
	}
}
