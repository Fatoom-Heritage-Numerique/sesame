package factice_test

import (
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// Cas `vie` et `etat` de routes-etat.md (API.md §2.4, §2.5).

const months = 130 * 24 * time.Hour // au-delà de 3 mois et 30 jours

// enrolled rend un titulaire enrôlé et un canal vers l'instance a.
func enrolled(t *testing.T, e *env, uid string) (*titulaire, *ch) {
	h, c := newTitulaire(uid), e.open("a")
	c.enrol(h)
	return h, c
}

func TestVieNominale(t *testing.T) { // VIE-01, API.md §2.4
	e := newEnv(t)
	h, c := enrolled(t, e, "u-vie")
	e.advance(time.Hour)
	c = e.open("a")
	o := ok(t, c.life(h))
	if str(o, "vivantLe") != "2026-11-11T11:00:00.000Z" || o["signalement"] != nil || o["transmis"] != false || o["pinPresent"] != true ||
		o["pinBloque"] != false || len(o["ouvertures"].(jcs.Array)) != 0 || e.line(h.uid)["vivantLe"] != "2026-11-11T11:00:00.000Z" {
		t.Fatalf("vie : %v", o)
	}
	e.put("titulaires/"+h.uid+"/ouvertures/AAAAAAAAAAAAAAAAAAAAAA", jcs.Object{"contactId": "p1", "role": "proche", "ouvertLe": "2026-11-11T11:30:00.000Z", "instanceId": c.id})
	e.advance(time.Hour)
	o = ok(t, e.open("a").life(h))
	if op := o["ouvertures"].(jcs.Array); len(op) != 1 || op[0].(jcs.Object)["contactId"] != "p1" || len(op[0].(jcs.Object)) != 3 {
		t.Fatalf("ouvertures : %v", o)
	}
}

func TestVieInactivite(t *testing.T) { // VIE-03, VIE-04, DSG-05, API.md §2.4, §2.8
	e := newEnv(t)
	h, _ := enrolled(t, e, "u-inact")
	e.advance(months)
	s1 := ok(t, e.signal(h.uid, "inactivite"))
	o := ok(t, e.open("a").life(h))
	if sub(o, "signalement")["annuleLe"] != str(o, "vivantLe") || o["transmis"] != false {
		t.Fatalf("inactivité annulée : %v", o)
	}
	n := e.notes("signalementAnnule")
	if len(n) != 1 || n[0]["annulePar"] != "preuve_de_vie" || n[0]["signalementId"] != s1["signalementId"] {
		t.Fatalf("signalementAnnule : %v", n)
	}
	e.advance(months)
	s2 := ok(t, e.signal(h.uid, "inactivite"))
	e.advance(8 * 24 * time.Hour)
	o = ok(t, e.open("a").life(h))
	if sub(o, "signalement")["closLe"] != str(o, "vivantLe") || o["transmis"] != true || sub(o, "signalement")["source"] != "inactivite" {
		t.Fatalf("inactivité close : %v", o)
	}
	if o = ok(t, e.open("a").life(h)); o["transmis"] != false {
		t.Fatalf("après la clôture : %v", o)
	}
	if n := e.notes("vieApresTransmission"); len(n) != 1 || n[0]["signalementId"] != s2["signalementId"] {
		t.Fatalf("vieApresTransmission : %v", n)
	}
	e.advance(months)
	if s3 := ok(t, e.signal(h.uid, "inactivite")); s3["signalementId"] == s2["signalementId"] {
		t.Fatal("signalement clos renvoyé")
	}
}

func TestVieDeces(t *testing.T) { // VIE-05, VIE-06, VIE-10, DSG-03, API.md §2.4
	e := newEnv(t)
	h, _ := enrolled(t, e, "u-deces")
	s := ok(t, e.signal(h.uid, "insee"))
	o := ok(t, e.open("a").life(h))
	if sub(o, "signalement")["annuleLe"] != nil || o["transmis"] != false || sub(o, "signalement")["signalementId"] != s["signalementId"] {
		t.Fatalf("vie pendant un décès : %v", o)
	}
	e.advance(8 * 24 * time.Hour)
	if o = ok(t, e.open("a").life(h)); o["transmis"] != true || sub(o, "signalement")["source"] != "insee" {
		t.Fatalf("après l'échéance : %v", o)
	}
	// Inactivité transmise, non close, puis décès dans son délai.
	g, _ := enrolled(t, e, "u-r-vie-07")
	e.advance(months)
	ok(t, e.signal(g.uid, "inactivite"))
	e.advance(8 * 24 * time.Hour)
	d := ok(t, e.signal(g.uid, "acte"))
	o = ok(t, e.open("a").life(g))
	if o["transmis"] != true || sub(o, "signalement")["signalementId"] != d["signalementId"] || sub(o, "signalement")["annuleLe"] != nil {
		t.Fatalf("R-VIE-07 : %v", o)
	}
	// Décès annulé : le dernier inscrit redevient courant.
	k, c := enrolled(t, e, "u-annul")
	ds := ok(t, e.signal(k.uid, "insee"))
	ok(t, c.cancel(k, str(ds, "signalementId")))
	if o = ok(t, c.life(k)); sub(o, "signalement")["signalementId"] != ds["signalementId"] || sub(o, "signalement")["annuleLe"] == nil {
		t.Fatalf("décès annulé : %v", o)
	}
}

func (c *ch) cancel(h *titulaire, id string) jcs.Object {
	d := c.defi(h.uid)
	return c.call("deces/annuler", jcs.Object{"uid": h.uid, "signalementId": id, "defi": d, "signature": h.sign(vie.CancelMessage(h.uid, id, d))})
}

func TestVieRefus(t *testing.T) { // VIE-07, VIE-08, VIE-09, API.md §2.4
	e := newEnv(t)
	h, c := enrolled(t, e, "u-vref")
	e.reject("titulaires/" + h.uid)
	before := e.f.Base.Doc("titulaires/" + h.uid)
	e.f.Clock.Advance(time.Minute)
	if o := ok(t, c.life(h)); str(o, "vivantLe") != "2026-11-11T10:00:00.000Z" || e.f.Base.Doc("titulaires/"+h.uid).UpdateTime != before.UpdateTime {
		t.Fatalf("ligne rejetée : %v", o)
	}
	g, c := enrolled(t, e, "u-vref2")
	d := c.defi(g.uid)
	other := newTitulaire(g.uid)
	expect(t, c.call("vie", jcs.Object{"uid": g.uid, "defi": d, "signature": other.sign(vie.LifeMessage(g.uid, d))}), 400, "message_refuse")
	expect(t, c.life(newTitulaire("jamais")), 409, "titulaire_indisponible")
	ok(t, c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": e.oidc(e.cfg.EnvOverride["COMPTE_DECES"]), "uid": g.uid}))
	expect(t, c.life(g), 409, "titulaire_indisponible")
	// Marqueur sans ligne : sans détail hors reprise, details.reprise pendant une reprise.
	for _, rep := range []*registre.Reprise{nil, {SansSauvegarde: true}} {
		e := newEnv(t, withReprise(rep))
		m, c := enrolled(t, e, "u-marq")
		e.f.Base.Tamper("titulaires/"+m.uid, nil, time.Time{})
		r := c.life(m)
		expect(t, r, 409, "titulaire_indisponible")
		if (sub(r, "details")["reprise"] == true) != (rep != nil) || e.line(m.uid) != nil {
			t.Fatalf("reprise %v : %v", rep, r)
		}
	}
}

func TestVieLimite(t *testing.T) { // VIE-11, API.md §7
	e := newEnv(t)
	h, c := enrolled(t, e, "u-vlim")
	for range 10 {
		ok(t, c.life(h))
	}
	expect(t, c.life(h), 429, "trop_d_appels")
}

func TestEtat(t *testing.T) { // ETA-01, ETA-02, ETA-06, API.md §2.5
	e := newEnv(t)
	h, c := enrolled(t, e, "u-etat")
	e.put("titulaires/"+h.uid+"/ouvertures/AAAAAAAAAAAAAAAAAAAAAA", jcs.Object{"contactId": "p1", "role": "gardien", "ouvertLe": "2026-11-11T09:00:00.000Z", "instanceId": c.id})
	e.put("titulaires/"+h.uid+"/ouvertures/BBBBBBBBBBBBBBBBBBBBBB", jcs.Object{"contactId": "p2", "role": "proche", "ouvertLe": "2026-11-11T09:30:00.000Z", "instanceId": c.id})
	l := e.line(h.uid)
	l["pinBloqueLe"], l["pinEvaluations"] = "2026-11-10T00:00:00.000Z", 9
	e.put("titulaires/"+h.uid, l)
	n := e.f.Base.Len()
	o := ok(t, c.etat(h, "2026-11-11T09:00:00.000Z", 5))
	p := e.payload(str(o, "etat"), false)
	op := p["ouvertures"].(jcs.Array)
	if p["typ"] != "etat" || p["uid"] != h.uid || len(op) != 1 || op[0].(jcs.Object)["contactId"] != "p2" || p["pinPresent"] != true ||
		p["pinBloque"] != false || p["evaluationsDepuis"] != 0.0 || sub(p, "repere")["ouvertures"] != "2026-11-11T09:30:00.000Z" || e.f.Base.Len() != n {
		t.Fatalf("etat : %v", p)
	}
	if p := e.payload(str(ok(t, c.etat(h, nil, 0)), "etat"), false); len(p["ouvertures"].(jcs.Array)) != 2 {
		t.Fatalf("etat sans repère : %v", p)
	}
}

func TestEtatRefus(t *testing.T) { // ETA-07, ETA-08, API.md §2.5
	e := newEnv(t, withReprise(&registre.Reprise{SansSauvegarde: true}))
	h, c := enrolled(t, e, "u-eref")
	b := jcs.Object{"uid": h.uid, "idToken": e.idToken("autre"), "repere": jcs.Object{"ouvertures": nil, "evaluations": 0}}
	expect(t, c.call("etat", b), 401, "jeton_invalide")
	expect(t, c.etat(newTitulaire("jamais"), nil, 0), 409, "titulaire_indisponible")
	e.reject("titulaires/" + h.uid)
	expect(t, c.etat(h, nil, 0), 409, "titulaire_indisponible")
	g, c := enrolled(t, e, "u-eref2")
	e.f.Base.Tamper("titulaires/"+g.uid, nil, time.Time{})
	if r := c.etat(g, nil, 0); sub(r, "details")["reprise"] != true {
		t.Fatalf("reprise : %v", r)
	}
	k, c := enrolled(t, e, "u-elim")
	for range 60 {
		c.etat(k, nil, 0)
	}
	expect(t, c.etat(k, nil, 0), 429, "trop_d_appels") // 61ᵉ par IP
	for i := range 40 {
		c := e.open("a")
		if r := c.etat(k, nil, 0); r["statut"] != 200.0 {
			t.Fatalf("appel %d : %v", 61+i, r)
		}
	}
	expect(t, e.open("a").etat(k, nil, 0), 429, "trop_d_appels") // 101ᵉ pour l'uid
}
