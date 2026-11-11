package factice_test

import (
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// Cas `defi`, `enroler`, `vie`, `etat` de routes-etat.md (API.md §2.2 à §2.5).

func (c *ch) defi(uid string) string {
	c.e.t.Helper()
	return str(ok(c.e.t, c.call("defi", jcs.Object{"uid": uid})), "defi")
}

func random(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

// enrolBody compose un corps d'enroler ; proofBy signe la preuve s'il est
// donné.
func (c *ch) enrolBody(h *titulaire, gen int64, pub []byte, proofBy *titulaire) jcs.Object {
	b := jcs.Object{"uid": h.uid, "idToken": c.e.idToken(h.uid), "ktsGeneration": gen, "kts": b64(random(32)), "cleViePub": b64(pub)}
	if proofBy != nil {
		d := c.defi(h.uid)
		b["preuve"] = jcs.Object{"defi": d, "signature": proofBy.sign(vie.EnrolMessage(h.uid, d, gen, pub))}
	}
	return b
}

func (c *ch) enrol(h *titulaire) jcs.Object {
	c.e.t.Helper()
	return ok(c.e.t, c.call("enroler", c.enrolBody(h, 1, h.pub, nil)))
}

func (c *ch) life(h *titulaire) jcs.Object {
	d := c.defi(h.uid)
	return c.call("vie", jcs.Object{"uid": h.uid, "defi": d, "signature": h.sign(vie.LifeMessage(h.uid, d))})
}

func (c *ch) etat(h *titulaire, since any, evals int64) jcs.Object {
	return c.call("etat", jcs.Object{"uid": h.uid, "idToken": c.e.idToken(h.uid), "repere": jcs.Object{"ouvertures": since, "evaluations": evals}})
}

func (e *env) line(uid string) jcs.Object { return e.f.Base.Doc("titulaires/" + uid).Data }

// reject recopie la ligne telle quelle, avec un update_time 10 minutes plus
// tard : ecritLe ancien, ligne rejetée par le contrôle de fraîcheur.
func (e *env) reject(path string) {
	d := e.f.Base.Doc(path)
	e.f.Base.Tamper(path, d.Data, d.UpdateTime.Add(10*time.Minute))
}

func (e *env) payload(jws string, jwt bool) jcs.Object {
	e.t.Helper()
	p, err := jetons.Verify(jws, e.cfg.VersionList().SessionKeys, jwt)
	if err != nil {
		e.t.Fatalf("JWS refusé : %v", err)
	}
	return p
}

func TestDefi(t *testing.T) { // DEF-01, DEF-02, DEF-03, API.md §2.2
	e := newEnv(t)
	c := e.open("a")
	before := e.f.Base.Len()
	o := ok(t, c.call("defi", jcs.Object{"uid": "inconnu"}))
	d, _ := primitives.B64(str(o, "defi"))
	if len(d) != 32 || str(o, "expireLe") != "2026-11-11T10:05:00.000Z" || o["grappeId"] != e.f.Server.Instances[0].Grappe().ID() || e.f.Base.Len() != before {
		t.Fatalf("défi : %v", o)
	}
	tick(e)
	objs, _ := e.f.Stockage.List(t.Context(), "audit", "bornes/", 100)
	if len(objs) != 3 || !strings.Contains(string(objs[0].Content), `"grappe":{"grappeId":"`+e.f.Server.Instances[0].Grappe().ID()+`","votants":3}`) {
		t.Fatalf("bornes : %d", len(objs))
	}
}

func TestDefiInvalide(t *testing.T) { // DEF-04, ENR-06, VIE-08, API.md §1.5
	e := newEnv(t)
	c, other := e.open("a"), e.open("a")
	h := newTitulaire("u-defi")
	c.enrol(h)
	try := func(on *ch, d, uid string) jcs.Object {
		return on.call("vie", jcs.Object{"uid": uid, "defi": d, "signature": h.sign(vie.LifeMessage(uid, d))})
	}
	expect(t, try(other, c.defi(h.uid), h.uid), 409, "defi_invalide")
	expect(t, try(c, c.defi("autre"), h.uid), 409, "defi_invalide")
	d := c.defi(h.uid)
	ok(t, try(c, d, h.uid))
	expect(t, try(c, d, h.uid), 409, "defi_invalide")
	d = c.defi(h.uid)
	e.f.Clock.Advance(5 * time.Minute)
	expect(t, try(c, d, h.uid), 409, "defi_invalide")
	d = c.defi(h.uid)
	body := c.enrolBody(h, 2, h.pub, nil)
	body["preuve"] = jcs.Object{"defi": d, "signature": h.sign(vie.EnrolMessage(h.uid, d, 2, h.pub))}
	e.f.Clock.Advance(6 * time.Minute)
	expect(t, c.call("enroler", body), 409, "defi_invalide")
}

func TestDefiLimites(t *testing.T) { // DEF-05, API.md §7
	e := newEnv(t)
	c := e.open("b")
	for range 30 {
		c.defi("u1")
	}
	expect(t, c.call("defi", jcs.Object{"uid": "u1"}), 429, "trop_d_appels")
	for range 4 {
		c2 := e.open("b")
		c2.ip = "192.0.2.50"
		for range 30 {
			c2.defi("u" + itoa(int64(c2.n)))
		}
	}
	c3 := e.open("b")
	c3.ip = "192.0.2.50"
	expect(t, c3.call("defi", jcs.Object{"uid": "u1"}), 429, "trop_d_appels")
	ok(t, e.open("b").call("defi", jcs.Object{"uid": "u1"})) // aucune limite par uid
}

func TestPremierEnrolement(t *testing.T) { // ENR-01, ENR-15, API.md §2.3
	e := newEnv(t)
	c := e.open("a")
	h := newTitulaire("u-premier")
	o := c.enrol(h)
	if str(o, "cleViePub") != b64(h.pub) || str(o, "imageDigest") != e.cfg.ImageDigest() || str(o, "enroleLe") != "2026-11-11T10:00:00.000Z" || o["ktsGeneration"] != 1.0 {
		t.Fatalf("enroler : %v", o)
	}
	l := e.line(h.uid)
	if l["vivantLe"] != l["enroleLe"] || l["etat"] != "actif" || l["cleViePub"] != b64(h.pub) {
		t.Fatalf("ligne : %v", l)
	}
	m, _ := e.f.Stockage.Read(t.Context(), "inscrits", markerPath(h.uid))
	if string(m) != `{"cleViePub":"`+b64(h.pub)+`","v":1}` {
		t.Fatalf("marqueur : %s", m)
	}
	// R-PARTAGE-10 : pas d'inactivité avant 3 mois et 30 jours.
	expect(t, e.signal(h.uid, "inactivite"), 409, "inactivite_trop_recente")
}

func markerPath(uid string) string {
	return "inscrits/" + hexOf(primitives.SHA256([]byte(uid)))
}

func TestRotationEtRescellement(t *testing.T) { // ENR-02 (partiel : sessions), ENR-03, ENR-12, API.md §2.3
	e := newEnv(t)
	c := e.open("a")
	h := newTitulaire("u-rot")
	enrole := str(c.enrol(h), "enroleLe")
	ok(t, c.call("charte/generation", e.generation(c, h, "p1", 3)))
	e.advance(time.Hour)
	c = e.open("a")
	h2 := newTitulaire(h.uid)
	o := ok(t, c.call("enroler", c.enrolBody(h, 2, h2.pub, h)))
	l := e.line(h.uid)
	if str(o, "enroleLe") != enrole || l["cleViePub"] != b64(h2.pub) || l["ktsGeneration"] != 2.0 || l["vivantLe"] != enrole {
		t.Fatalf("rotation : %v / %v", o, l)
	}
	ok(t, c.call("enroler", c.enrolBody(h2, 2, h2.pub, h2)))
	l = e.line(h.uid)
	if sub(l, "chartes")["p1"] != 3.0 || l["vivantLe"] != enrole || l["enroleLe"] != enrole {
		t.Fatalf("rescellement : %v", l)
	}
	expect(t, c.call("enroler", c.enrolBody(h, 2, h.pub, h2)), 400, "message_refuse")
}

func TestEnrolementRefus(t *testing.T) { // ENR-04, ENR-05, ENR-07, ENR-08, ENR-09, ENR-13, ENR-14, API.md §2.3
	e := newEnv(t)
	c := e.open("a")
	h := newTitulaire("u-ref")
	c.enrol(h)
	ok(t, c.call("enroler", c.enrolBody(h, 3, h.pub, h)))
	expect(t, c.call("enroler", c.enrolBody(h, 3, h.pub, nil)), 403, "preuve_requise")
	expect(t, c.call("enroler", c.enrolBody(h, 3, h.pub, newTitulaire(h.uid))), 403, "preuve_requise")
	expect(t, c.call("enroler", c.enrolBody(h, 2, h.pub, h)), 409, "generation_non_croissante")
	for _, tok := range []string{
		e.idToken("autre"),
		e.jwt("idToken", jcs.Object{"iss": "https://securetoken.google.com/x", "aud": "x", "sub": h.uid, "exp": e.now().Unix() + 60}),
		e.jwt("idToken", jcs.Object{"iss": "https://securetoken.google.com/projet-de-test", "aud": "projet-de-test", "sub": h.uid, "exp": e.now().Unix()}),
		e.jwt("google", jcs.Object{"iss": "https://securetoken.google.com/projet-de-test", "aud": "projet-de-test", "sub": h.uid, "exp": e.now().Unix() + 60}),
	} {
		b := c.enrolBody(h, 4, h.pub, newTitulaire(h.uid)) // ENR-09 : jeton refusé et preuve fausse
		b["idToken"] = tok
		expect(t, c.call("enroler", b), 401, "jeton_invalide")
	}
	b := c.enrolBody(h, 4, h.pub, nil)
	delete(b, "cleViePub")
	expect(t, c.raw("enroler", b), 400, "message_refuse")
	// Hors d'une reprise publiée.
	e2 := newEnv(t, withReprise(nil))
	c2 := e2.open("b")
	b = c2.enrolBody(newTitulaire("u-x"), 1, random(32), nil)
	b["cleViePubAttendue"] = b64(random(32))
	expect(t, c2.call("enroler", b), 409, "reprise_absente")
	// Pendant une reprise, sans marqueur.
	b["idToken"] = e.idToken("u-x")
	expect(t, c.call("enroler", b), 400, "message_refuse")
	// ENR-13 : supprime, rejetée, absente avec marqueur hors reprise.
	s := newTitulaire("u-supp")
	c.enrol(s)
	ok(t, c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": e.oidc(e.cfg.EnvOverride["COMPTE_DECES"]), "uid": s.uid}))
	expect(t, c.call("enroler", c.enrolBody(s, 2, s.pub, s)), 409, "titulaire_indisponible")
	r := newTitulaire("u-rej")
	c.enrol(r)
	e.reject("titulaires/" + r.uid)
	expect(t, c.call("enroler", c.enrolBody(r, 2, r.pub, r)), 409, "titulaire_indisponible")
	a := newTitulaire("u-abs")
	c2.enrol(a)
	e2.f.Base.Tamper("titulaires/"+a.uid, nil, time.Time{})
	expect(t, c2.call("enroler", c2.enrolBody(a, 1, a.pub, nil)), 409, "titulaire_indisponible")
	// ENR-14 : bucket des marqueurs illisible.
	e.f.Stockage.Set("Read:inscrits", true)
	n := newTitulaire("u-neuf")
	expect(t, c.call("enroler", c.enrolBody(n, 1, n.pub, nil)), 503, "registre_indisponible")
	if e.line(n.uid) != nil {
		t.Fatal("ligne écrite sans marqueur")
	}
}

func TestEnrolementReprise(t *testing.T) { // ENR-10, ENR-11, API.md §2.3, ARCHITECTURE.md §6.4
	for _, rep := range []*registre.Reprise{{SansSauvegarde: true}, {Epoque: 0, Empreinte: strings.Repeat("0", 64)}} {
		e := newEnv(t, withReprise(rep))
		c := e.open("c")
		h := newTitulaire("u-rep")
		c.enrol(h)
		e.f.Base.Tamper("titulaires/"+h.uid, nil, time.Time{})
		e.advance(time.Hour)
		c = e.open("c")
		h2 := newTitulaire(h.uid)
		b := c.enrolBody(h, 1, h2.pub, h)
		b["cleViePubAttendue"] = b64(h.pub)
		o := ok(t, c.call("enroler", b))
		l := e.line(h.uid)
		if str(o, "enroleLe") != "2026-11-11T11:00:00.000Z" || l["vivantLe"] != l["enroleLe"] || l["cleViePub"] != b64(h2.pub) {
			t.Fatalf("reprise : %v", l)
		}
		b = c.enrolBody(h2, 1, h2.pub, h2)
		b["cleViePubAttendue"] = b64(h2.pub)
		expect(t, c.call("enroler", b), 400, "message_refuse")
		e.f.Base.Tamper("titulaires/"+h.uid, nil, time.Time{})
		b = c.enrolBody(h, 1, h2.pub, newTitulaire("x"))
		b["cleViePubAttendue"] = b64(h.pub)
		expect(t, c.call("enroler", b), 403, "preuve_requise")
	}
}

func TestRejeuPremierEnrolement(t *testing.T) { // ENR-16, API.md §2.3
	e := newEnv(t)
	c := e.open("a")
	h := newTitulaire("u-rejeu")
	ok(t, c.call("enroler", c.enrolBody(h, 1, h.pub, h))) // ni ligne ni marqueur
	ok(t, c.call("enroler", c.enrolBody(h, 1, h.pub, h))) // ligne de même cleViePub : rescellement
	e.f.Base.Tamper("titulaires/"+h.uid, nil, time.Time{})
	e.f.Clock.Advance(time.Minute)
	o := ok(t, c.call("enroler", c.enrolBody(h, 1, h.pub, h))) // marqueur sans ligne
	if str(o, "enroleLe") != "2026-11-11T10:01:00.000Z" || e.line(h.uid)["vivantLe"] != "2026-11-11T10:01:00.000Z" {
		t.Fatalf("ligne recréée : %v", o)
	}
	e.f.Base.Tamper("titulaires/"+h.uid, nil, time.Time{})
	other := newTitulaire(h.uid)
	expect(t, c.call("enroler", c.enrolBody(h, 1, other.pub, other)), 409, "titulaire_indisponible")
	expect(t, c.call("enroler", c.enrolBody(h, 1, h.pub, other)), 409, "titulaire_indisponible")
	expect(t, c.call("enroler", c.enrolBody(h, 1, h.pub, nil)), 409, "titulaire_indisponible")
	if e.line(h.uid) != nil {
		t.Fatal("ligne recréée sans preuve du marqueur")
	}
	g := newTitulaire("u-autre")
	c.enrol(g)
	expect(t, c.call("enroler", c.enrolBody(g, 1, other.pub, other)), 403, "preuve_requise")
}

func TestEnrolementLimites(t *testing.T) { // ENR-17, ENV-12, API.md §2.3, §7
	e := newEnv(t)
	c := e.open("a")
	c.ip = "192.0.2.80"
	h := newTitulaire("u-lim")
	c.enrol(h)
	for range 3 {
		expect(t, c.call("enroler", c.enrolBody(h, 2, h.pub, nil)), 403, "preuve_requise") // ne compte pas pour l'uid
	}
	for range 4 {
		ok(t, c.call("enroler", c.enrolBody(h, 2, h.pub, h)))
	}
	expect(t, c.call("enroler", c.enrolBody(h, 2, h.pub, h)), 429, "trop_d_appels") // 6ᵉ pour l'uid
	for i := range 11 {
		b := c.enrolBody(newTitulaire("v"+itoa(int64(i))), 1, random(32), nil)
		if i < 3 {
			b["idToken"] = "x.y.z" // compte pour l'IP seule
			expect(t, c.call("enroler", b), 401, "jeton_invalide")
			continue
		}
		ok(t, c.call("enroler", b))
	}
	expect(t, c.call("enroler", c.enrolBody(newTitulaire("w"), 1, random(32), nil)), 429, "trop_d_appels") // 21ᵉ pour l'IP
}
