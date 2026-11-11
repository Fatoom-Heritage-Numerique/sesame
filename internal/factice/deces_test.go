package factice_test

import (
	"context"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/routes"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// Cas `charte/generation`, `alerte/fiche`, `deces/signaler`, `deces/annuler`,
// `titulaire/supprimer` de routes-etat.md (API.md §2.6 à §2.9, §2.11).

func TestCharteGeneration(t *testing.T) { // CHG-01, CHG-02, CHG-04, CHG-06, CHG-08, CHG-09, API.md §2.6
	e := newEnv(t)
	h, c := enrolled(t, e, "u-chg")
	o := ok(t, c.call("charte/generation", e.generation(c, h, "p1", 2)))
	p := e.payload(str(o, "recu"), false)
	if o["generation"] != 2.0 || p["typ"] != "generation" || p["contactId"] != "p1" || p["generation"] != 2.0 || p["uid"] != h.uid {
		t.Fatalf("reçu : %v", p)
	}
	expect(t, c.call("charte/generation", e.generation(c, h, "p1", 2)), 409, "generation_non_croissante")
	ok(t, c.call("charte/generation", e.generation(c, h, "p2", 7)))
	ok(t, c.call("charte/generation", e.generation(c, h, "p3", -1)))
	ok(t, c.call("charte/generation", e.generation(c, h, "p1", -1)))
	for _, g := range []int64{3, 100, -1} {
		expect(t, c.call("charte/generation", e.generation(c, h, "p1", g)), 409, "generation_non_croissante")
	}
	if ch := sub(e.line(h.uid), "chartes"); ch["p1"] != -1.0 || ch["p2"] != 7.0 || ch["p3"] != -1.0 {
		t.Fatalf("chartes : %v", ch)
	}
	b := e.generation(c, newTitulaire(h.uid), "p4", 1)
	expect(t, c.call("charte/generation", b), 400, "message_refuse")
	expect(t, c.call("charte/generation", e.generation(c, newTitulaire("jamais"), "p1", 1)), 409, "titulaire_indisponible")
	r, c2 := enrolled(t, e, "u-chg-rej")
	e.reject("titulaires/" + r.uid)
	expect(t, c2.call("charte/generation", e.generation(c2, r, "p1", 1)), 409, "titulaire_indisponible")
	k, c3 := enrolled(t, e, "u-chg-lim")
	for i := range 30 {
		ok(t, c3.call("charte/generation", e.generation(c3, k, "p"+itoa(int64(i)), 1)))
	}
	expect(t, c3.call("charte/generation", e.generation(c3, k, "q", 1)), 429, "trop_d_appels")
}

func (e *env) fiche(c *ch, h *titulaire, gen int64, email string, phone any, signer *titulaire) jcs.Object {
	a := vie.Alert{UID: h.uid, Generation: gen, Email: email}
	if p, ok := phone.(string); ok {
		a.Telephone = &p
	}
	msg, err := a.Message()
	if err != nil {
		aad, _ := vie.AlertAAD(h.uid, gen)
		msg, err = append(aad, []byte(email)...), nil
	}
	return jcs.Object{"uid": h.uid, "generation": gen, "telephone": phone, "email": email, "signature": signer.sign(msg, err)}
}

func TestAlerteFiche(t *testing.T) { // ALF-01, ALF-02, ALF-03, ALF-04, ALF-05, ALF-06, ALF-07, API.md §2.7
	e := newEnv(t)
	h, c := enrolled(t, e, "u-alf")
	gen := e.now().UnixMilli()
	o := ok(t, c.call("alerte/fiche", e.fiche(c, h, gen, "lea.martin@exemple.fr", "+33612345647", h)))
	l := e.line(h.uid)
	sealed, _ := primitives.B64(str(l, "ficheScellee"))
	aad, _ := vie.AlertAAD(h.uid, gen)
	plain, err := e.f.KMS.Decrypt(context.Background(), "alerte-seal", sealed, aad)
	if o["ficheGeneration"] != float64(gen) || err != nil || string(plain) != `{"email":"lea.martin@exemple.fr","generation":`+itoa(gen)+`,"telephone":"+33612345647","uid":"u-alf","v":1}` {
		t.Fatalf("fiche : %v %s", o, plain)
	}
	ok(t, c.call("alerte/fiche", e.fiche(c, h, gen+1, "lea.martin@exemple.fr", nil, h)))
	expect(t, c.call("alerte/fiche", e.fiche(c, h, gen+1, "lea.martin@exemple.fr", nil, h)), 409, "generation_non_croissante")
	expect(t, c.call("alerte/fiche", e.fiche(c, h, gen, "lea.martin@exemple.fr", nil, h)), 409, "generation_non_croissante")
	c.n++
	nfd := `{"corps":{"email":"le` + "\u0301" + `a@exemple.fr","generation":` + itoa(gen+2) + `,"signature":"` + b64(random(64)) + `","telephone":null,"uid":"u-alf"},"route":"alerte/fiche"}`
	expect(t, c.openResponse(c.n, c.post(c.sealed(c.n, []byte(nfd)))), 400, "message_refuse")
	expect(t, c.call("alerte/fiche", e.fiche(c, h, gen+2, "Lea@exemple.fr", nil, h)), 400, "message_refuse")
	expect(t, c.raw("alerte/fiche", e.fiche(c, h, gen+2, "lea@exemple.fr", "0612345647", h)), 400, "message_refuse")
	expect(t, c.call("alerte/fiche", e.fiche(c, h, gen+2, "lea@exemple.fr", nil, newTitulaire(h.uid))), 400, "message_refuse")
	b := e.fiche(c, h, gen+3, "lea@exemple.fr", nil, h)
	b["generation"] = gen + 4
	expect(t, c.call("alerte/fiche", b), 400, "message_refuse")
	expect(t, c.call("alerte/fiche", e.fiche(c, newTitulaire("jamais"), 1, "a@b.fr", nil, h)), 409, "titulaire_indisponible")
	for i := range 6 {
		ok(t, c.call("alerte/fiche", e.fiche(c, h, gen+10+int64(i), "lea@exemple.fr", nil, h)))
	}
	expect(t, c.call("alerte/fiche", e.fiche(c, h, gen+20, "lea@exemple.fr", nil, h)), 429, "trop_d_appels") // 11ᵉ en un jour
}

func TestEcheance(t *testing.T) { // DSG-01 (heure de Paris), DSG-06, API.md §2.8
	paris, _ := time.LoadLocation("Europe/Paris")
	for _, c := range []struct{ signaled, want string }{
		{"2026-11-11T10:00:00Z", "2026-11-18T22:59:59Z"},
		{"2026-10-20T10:00:00Z", "2026-10-27T22:59:59Z"}, // passage à l'heure d'hiver le 25 octobre
		{"2026-03-25T10:00:00Z", "2026-04-01T21:59:59Z"}, // passage à l'heure d'été le 29 mars
		{"2026-10-09T22:30:00Z", "2026-10-17T21:59:59Z"}, // déjà le 10 octobre à Paris
	} {
		s, _ := time.Parse(time.RFC3339, c.signaled)
		if got := routes.Transmissible(s).Format(time.RFC3339); got != c.want {
			t.Errorf("transmissibleLe de %s : %s, attendu %s", c.signaled, got, c.want)
		}
	}
	for _, c := range []struct{ alive, want string }{
		{"2025-10-15T10:00:00", "2026-02-14T10:00:00"},
		{"2025-10-30T10:00:00", "2026-03-01T10:00:00"},
	} {
		a, _ := time.ParseInLocation("2006-01-02T15:04:05", c.alive, paris)
		if got := routes.InactivityFloor(a).In(paris).Format("2006-01-02T15:04:05"); got != c.want {
			t.Errorf("plancher de %s : %s, attendu %s", c.alive, got, c.want)
		}
	}
}

func TestSignaler(t *testing.T) { // DSG-01, DSG-02, DSG-06, DSG-09, API.md §2.8
	e := newEnv(t)
	h, _ := enrolled(t, e, "u-sig")
	s := ok(t, e.signal(h.uid, "insee"))
	p := e.payload(str(s, "recu"), false)
	if str(s, "transmissibleLe") != "2026-11-18T22:59:59.000Z" || p["typ"] != "signalement" || p["source"] != "insee" || p["signalementId"] != s["signalementId"] {
		t.Fatalf("signalement : %v %v", s, p)
	}
	doc := e.f.Base.Doc("titulaires/" + h.uid + "/signalements/" + str(s, "signalementId")).Data
	if doc["preuveNom"] != "deces-2026-m10.txt" || len(str(doc, "empreintePreuve")) != 64 || doc["deces"] != true || doc["dateDeces"] != "2026-11-01" {
		t.Fatalf("registre : %v", doc)
	}
	e.advance(time.Hour)
	a := ok(t, e.signal(h.uid, "acte"))
	if a["signalementId"] != s["signalementId"] || a["signaleLe"] != s["signaleLe"] || a["transmissibleLe"] != s["transmissibleLe"] {
		t.Fatalf("même nature : %v", a)
	}
	if doc2 := e.f.Base.Doc("titulaires/" + h.uid + "/signalements/" + str(s, "signalementId")).Data; doc2["empreintePreuve"] != doc["empreintePreuve"] {
		t.Fatal("pièce de l'acte inscrite")
	}
	e.advance(months)
	expect(t, e.signal(h.uid, "inactivite"), 400, "message_refuse") // décès non annulé, même échu
	g, c := enrolled(t, e, "u-sig2")
	d := ok(t, e.signal(g.uid, "acte"))
	expect(t, e.signal(g.uid, "inactivite"), 400, "message_refuse")
	ok(t, c.cancel(g, str(d, "signalementId")))
	expect(t, e.signal(g.uid, "inactivite"), 409, "inactivite_trop_recente")
	e.advance(months)
	ok(t, e.signal(g.uid, "inactivite"))
}

func TestSignalerRefus(t *testing.T) { // DSG-07, DSG-10, DSG-11, DSG-12, API.md §2.8
	e := newEnv(t)
	h, c := enrolled(t, e, "u-sref")
	tok := e.oidc(e.cfg.EnvOverride["COMPTE_DECES"])
	proof := hexOf(random(32))
	for _, b := range []jcs.Object{
		{"source": "admin"},
		{"source": "insee", "dateDeces": "2026-11-01", "preuveNom": "f.txt"},
		{"source": "acte", "dateDeces": "2026-11-01"},
		{"source": "insee", "dateDeces": "2026-11-01", "preuve": proof},
		{"source": "acte", "dateDeces": "2026-11-01", "preuve": proof, "preuveNom": "f.txt"},
		{"source": "inactivite", "preuveNom": "f.txt"},
		{"source": "insee", "dateDeces": "2026-11-01", "preuve": proof, "preuveNom": "f/x"},
		{"source": "insee", "dateDeces": "2026-11-01", "preuve": proof, "preuveNom": string(make([]byte, 0)) + "x" + string(random(0)) + hexOf(random(64))},
		{"source": "acte", "preuve": proof},
	} {
		b["jetonFatoom"], b["uid"] = tok, h.uid
		expect(t, c.raw("deces/signaler", b), 400, "message_refuse")
	}
	sa := e.cfg.EnvOverride["SESAME_AUDIENCE"]
	for _, claims := range []jcs.Object{
		{"iss": "https://accounts.google.com", "aud": "https://autre.exemple.test", "email": e.cfg.EnvOverride["COMPTE_DECES"], "email_verified": true},
		{"iss": "https://accounts.google.com", "aud": sa, "email": "autre@comptes.exemple.test", "email_verified": true},
		{"iss": "https://autre.exemple.test", "aud": sa, "email": e.cfg.EnvOverride["COMPTE_DECES"], "email_verified": true},
		{"iss": "https://accounts.google.com", "aud": sa, "email": e.cfg.EnvOverride["COMPTE_DECES"]},
		{"iss": "https://accounts.google.com", "aud": sa, "email": e.cfg.EnvOverride["COMPTE_EXPLOITATION"], "email_verified": true},
	} {
		claims["exp"] = e.now().Unix() + 60
		b := jcs.Object{"jetonFatoom": e.jwt("google", claims), "uid": h.uid, "source": "inactivite"}
		expect(t, c.call("deces/signaler", b), 401, "jeton_invalide")
	}
	expired := e.jwt("google", jcs.Object{"iss": "https://accounts.google.com", "aud": sa, "email": e.cfg.EnvOverride["COMPTE_DECES"], "email_verified": true, "exp": e.now().Unix()})
	expect(t, c.call("deces/signaler", jcs.Object{"jetonFatoom": expired, "uid": h.uid, "source": "inactivite"}), 401, "jeton_invalide")
	expect(t, c.raw("deces/signaler", jcs.Object{"jetonFatoom": "", "uid": h.uid, "source": "inactivite"}), 400, "message_refuse")
	expect(t, e.signal("jamais", "inactivite"), 409, "titulaire_indisponible")
	r, _ := enrolled(t, e, "u-srej")
	e.reject("titulaires/" + r.uid)
	expect(t, e.signal(r.uid, "insee"), 409, "titulaire_indisponible")
	s, c2 := enrolled(t, e, "u-ssup")
	ok(t, c2.call("titulaire/supprimer", jcs.Object{"jetonFatoom": tok, "uid": s.uid}))
	expect(t, e.signal(s.uid, "insee"), 409, "titulaire_indisponible")
	n := 0
	for range 100 {
		if r := e.signal(h.uid, "insee"); r["statut"] == 200.0 || str(r, "erreur") != "trop_d_appels" {
			n++
		}
	}
	expect(t, e.signal(h.uid, "insee"), 429, "trop_d_appels") // compte de service : 100 / min
	if n > 100 {
		t.Fatal(n)
	}
}

func TestAnnuler(t *testing.T) { // DAN-01 à DAN-09, API.md §2.9
	e := newEnv(t)
	h, c := enrolled(t, e, "u-dan")
	s := ok(t, e.signal(h.uid, "acte"))
	id := str(s, "signalementId")
	expect(t, c.cancel(h, "AAAAAAAAAAAAAAAAAAAAAA"), 400, "message_refuse") // autre que le courant
	d := c.defi(h.uid)
	expect(t, c.call("deces/annuler", jcs.Object{"uid": h.uid, "signalementId": id, "defi": d, "signature": newTitulaire(h.uid).sign(vie.CancelMessage(h.uid, id, d))}), 400, "message_refuse")
	expect(t, c.call("deces/annuler", jcs.Object{"uid": h.uid, "signalementId": id, "defi": d, "signature": h.sign(vie.CancelMessage(h.uid, id, d))}), 409, "defi_invalide")
	// DAN-08 : anti-retour en échec (bucket d'audit illisible) : annulation servie.
	e.f.Stockage.Set("Read:audit", true)
	o := ok(t, c.cancel(h, id))
	e.f.Stockage.Set("Read:audit", false)
	p := e.payload(str(o, "recu"), false)
	if sub(o, "signalement")["annuleLe"] != "2026-11-11T10:00:00.000Z" || p["typ"] != "annulation" || p["annuleLe"] != sub(o, "signalement")["annuleLe"] {
		t.Fatalf("annulation : %v", o)
	}
	if n := e.notes("signalementAnnule"); len(n) != 1 || n[0]["annulePar"] != "annulation" {
		t.Fatalf("signalementAnnule : %v", n)
	}
	e.advance(8 * 24 * time.Hour)
	c = e.open("a")
	if o2 := ok(t, c.cancel(h, id)); sub(o2, "signalement")["annuleLe"] != sub(o, "signalement")["annuleLe"] || len(e.notes("signalementAnnule")) != 1 {
		t.Fatalf("seconde annulation : %v", o2)
	}
	g, c := enrolled(t, e, "u-dan2")
	gid := str(ok(t, e.signal(g.uid, "insee")), "signalementId")
	e.advance(8 * 24 * time.Hour)
	expect(t, e.open("a").cancel(g, gid), 409, "deja_transmis")
	k, c := enrolled(t, e, "u-dan3")
	e.advance(months)
	kid := str(ok(t, e.signal(k.uid, "inactivite")), "signalementId")
	c = e.open("a")
	expect(t, c.cancel(k, kid), 400, "message_refuse")
	if e.f.Base.Doc("titulaires/" + k.uid + "/signalements/" + kid).Data["annuleLe"] != nil {
		t.Fatal("inactivité annulée par deces/annuler")
	}
	r, c := enrolled(t, e, "u-dan4")
	rid := str(ok(t, e.signal(r.uid, "insee")), "signalementId")
	e.reject("titulaires/" + r.uid)
	ok(t, c.cancel(r, rid)) // DAN-06
	m, c := enrolled(t, e, "u-dan5")
	mid := str(ok(t, e.signal(m.uid, "insee")), "signalementId")
	e.f.Base.Tamper("titulaires/"+m.uid, nil, time.Time{})
	expect(t, c.cancel(m, mid), 409, "titulaire_indisponible")
	if e.f.Base.Doc("titulaires/" + m.uid + "/signalements/" + mid).Data["annuleLe"] != nil {
		t.Fatal("annulation écrite sans ligne")
	}
	expect(t, c.cancel(newTitulaire("jamais"), mid), 409, "titulaire_indisponible")
	l, c := enrolled(t, e, "u-dan6")
	for range 10 {
		c.cancel(l, mid)
	}
	expect(t, c.cancel(l, mid), 429, "trop_d_appels")
}

func TestSupprimer(t *testing.T) { // TSP-01, TSP-02, TSP-03, TSP-04, API.md §2.11
	e := newEnv(t)
	tok := e.oidc(e.cfg.EnvOverride["COMPTE_DECES"])
	h, c := enrolled(t, e, "u-tsp")
	for range 2 {
		if o := ok(t, c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": tok, "uid": h.uid})); o["etat"] != "supprime" {
			t.Fatalf("supprimer : %v", o)
		}
	}
	if e.line(h.uid)["etat"] != "supprime" {
		t.Fatal("ligne non supprime")
	}
	expect(t, c.call("enroler", c.enrolBody(h, 2, h.pub, h)), 409, "titulaire_indisponible")
	n := e.f.Base.Len()
	if o := ok(t, c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": tok, "uid": "jamais"})); o["etat"] != "absent" || e.f.Base.Len() != n {
		t.Fatalf("absent : %v", o)
	}
	r, _ := enrolled(t, e, "u-tsp2")
	e.reject("titulaires/" + r.uid)
	expect(t, c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": tok, "uid": r.uid}), 409, "titulaire_indisponible")
	m, _ := enrolled(t, e, "u-tsp3")
	e.f.Base.Tamper("titulaires/"+m.uid, nil, time.Time{})
	expect(t, c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": tok, "uid": m.uid}), 409, "titulaire_indisponible")
	expect(t, c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": e.oidc("autre@comptes.exemple.test"), "uid": h.uid}), 401, "jeton_invalide")
	for range 95 {
		c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": tok, "uid": "x"})
	}
	expect(t, c.call("titulaire/supprimer", jcs.Object{"jetonFatoom": tok, "uid": "x"}), 429, "trop_d_appels") // 101ᵉ en une minute
}
