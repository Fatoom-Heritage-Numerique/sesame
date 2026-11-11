package factice_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/cles"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
)

// Autres cas `session/*` et `lettre/secours` (API.md §2.10, §2.14 à §2.21).

func TestOuvrirEcheance(t *testing.T) { // SOU-09, API.md §2.14 ; R-PARTAGE-08
	e := newEnv(t)
	k := newCoffre(t, e, "u-echeance")
	p := k.add("p", "proche", "+33600000001", nil)
	s := ok(t, e.signal(k.h.uid, "acte"))
	if s["transmissibleLe"] != "2026-11-18T22:59:59.000Z" {
		t.Fatalf("échéance : %v", s)
	}
	e.f.Clock.Set(time.Date(2026, 11, 18, 22, 58, 0, 0, time.UTC)) // 23 h 58, heure de Paris
	tick(e)
	expect(t, e.open("b").openCode(p.code, p.lookup), 403, "code_refuse")
	e.f.Clock.Set(time.Date(2026, 11, 18, 23, 0, 0, 0, time.UTC))
	tick(e)
	ok(t, e.open("b").openCode(p.code, p.lookup))
}

func TestOuvrirAntiRetour(t *testing.T) { // SOU-15, SVE-07, LSC-03, API.md §2.14, §2.10
	e := newEnv(t)
	k := newCoffre(t, e, "u-anti")
	p := k.add("p", "gardien", "+33600000001", nil)
	c := e.open("b")
	id := str(ok(t, c.openCode(p.code, p.lookup)), "sessionId")
	e.f.Outbox.Set("Sms", true)
	expect(t, c.sid("session/verifier", id, "otp", "123456"), 503, "fournisseur_indisponible")
	e.f.Outbox.Set("Sms", false)
	e.f.Stockage.Set("Read:audit", true)
	tick(e)
	expect(t, e.open("b").openCode(p.code, hex64()), 503, "reessayer_plus_tard")
	expect(t, c.sid("session/verifier", id, "otp", "123456"), 503, "reessayer_plus_tard")
	expect(t, e.secours(k.h.uid, p.contact), 503, "reessayer_plus_tard")
	if d := e.f.Base.Doc("titulaires/" + k.h.uid + "/essais/" + p.contact).Data; d != nil {
		t.Fatalf("essai décompté : %v", d)
	}
}

func hex64() string { return strings.Repeat("ab", 32) }

func TestDernierCodeBloque(t *testing.T) { // SVE-03, API.md §2.15, §7
	e := newEnv(t)
	k := newCoffre(t, e, "u-otp")
	p := k.add("p", "gardien", "+33600000001", nil)
	c := e.open("b")
	id := str(ok(t, c.openCode(p.code, p.lookup)), "sessionId")
	t0 := e.now()
	for i := range 5 {
		res := c.sid("session/verifier", id, "otp", "000000")
		if expect(t, res, 403, "otp_refuse"); sub(res, "details")["essaisRestants"] != float64(4-i) {
			t.Fatalf("essai %d : %v", i+1, res)
		}
		if i < 4 {
			e.f.Clock.Advance(14 * time.Minute / 4)
		}
	}
	e.f.Clock.Set(t0.Add(20 * time.Minute))
	res := c.sid("session/verifier", id, "otp", "123456")
	if expect(t, res, 429, "trop_d_essais"); sub(res, "details")["reessayerLe"] != stampOf(t0.Add(29*time.Minute)) {
		t.Fatalf("blocage : %v", res)
	}
	// Le blocage du dernier code ne bloque pas le code d'accès, et tient dans une nouvelle session.
	c2 := e.open("b")
	id2 := str(ok(t, c2.openCode(p.code, p.lookup)), "sessionId")
	expect(t, c2.sid("session/verifier", id2, "otp", "123456"), 429, "trop_d_essais")
	e.f.Clock.Set(t0.Add(29 * time.Minute))
	ok(t, c2.sid("session/verifier", id2, "otp", "123456"))
}

func TestBaseApresInactivite(t *testing.T) { // SVE-02, SOU-06, API.md §2.15 ; R-VIE-07
	e := newEnv(t)
	k := newCoffre(t, e, "u-base")
	p := k.add("p", "proche", "+33600000001", nil)
	l := k.add("l", "proche", nil, nil)
	k.transmit(t, "inactivite")
	e.advance(time.Hour)
	code := e.printed(0)
	lookup := cles.IndexAcces(code)
	expire := e.f.Base.Doc("invitations/" + lookup).Data["expireLe"]
	k.transmit(t, "acte")
	if _, _, v := k.session(t, p); v["base"] != "inactivite" {
		t.Fatalf("base : %v", v)
	}
	e.advance(time.Hour)
	inv := e.f.Base.Doc("invitations/" + lookup).Data
	if len(e.f.Post.Sent()) != 1 || inv["expireLe"] == expire || inv["expireLe"] != stampOf(e.now().Add(-time.Hour-time.Second).In(parisZone()).AddDate(1, 0, 0)) {
		t.Fatalf("prolongation : %v (avant %v)", inv, expire)
	}
	inv["expireLe"] = stampOf(e.now().Add(-time.Minute))
	e.put("invitations/"+lookup, inv)
	expect(t, e.open("b").openCode(code, lookup), 403, "code_refuse")
	_ = l
}

func parisZone() *time.Location { z, _ := time.LoadLocation("Europe/Paris"); return z }

func TestSessionLimites(t *testing.T) { // SOC-09, SOC-11, SOC-12, SID-05, SRR-04, ENV-03, API.md §2.16 à §2.21, §7
	e := newEnv(t)
	k := newCoffre(t, e, "u-limites")
	g := k.add("g", "gardien", "+33600000001", nil)
	p := k.add("p", "proche", "+33600000002", nil)
	k.transmit(t, "acte")
	c, id, _ := k.session(t, g)
	big := jcs.Array{}
	for range 400 {
		big = append(big, jcs.Object{"type": "fichier", "id": strings.Repeat("A", 22), "contactId": g.contact, "enveloppe": b64(random(48)), "iv": b64(random(12))})
	}
	if r := ok(t, c.sid("session/ouvrir-cles", id, "enveloppes", big)); len(r["refusees"].(jcs.Array)) != 400 { // corps de plus de 64 Kio admis
		t.Fatalf("ouvrir-cles : %d", len(r["refusees"].(jcs.Array)))
	}
	for range 59 {
		ok(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{}))
	}
	expect(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{}), 429, "trop_d_appels")
	clearErr(t, e.open("b").clearCall("session/ouvrir-cles", jcs.Object{"sessionId": id, "enveloppes": jcs.Array{}}), 400, "canal_invalide")
	// Ligne rejetée depuis l'ouverture : session_expiree.
	c, id, _ = k.session(t, g)
	e.reject("titulaires/" + k.h.uid)
	expect(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{}), 410, "session_expiree")
	k2 := newCoffre(t, e, "u-limites-2")
	q := k2.add("q", "proche", "+33600000003", nil)
	k2.transmit(t, "acte")
	c = e.open("b")
	id = str(ok(t, c.openCode(q.code, q.lookup)), "sessionId")
	for range 5 {
		ok(t, c.sid("session/identite/bascule", id))
	}
	expect(t, c.sid("session/identite/bascule", id), 429, "trop_d_appels")
	c, id, _ = k2.session(t, q)
	for range 5 {
		expect(t, c.sid("session/refuser-role", id), 403, "role_insuffisant")
	}
	expect(t, c.sid("session/refuser-role", id), 429, "trop_d_appels")
	_ = p
}
