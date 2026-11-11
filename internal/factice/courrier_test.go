package factice_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/cles"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// Cas des lettres, de `lettre/secours`, des alertes de Sésame et de la tâche
// horaire (routes-etat.md ; API.md §2.10, §2.14, §5.5 ; tests.md de Fatoom,
// « Lettres »).

var rePrinted = regexp.MustCompile(`\(([0-9A-Z]{4}-[0-9A-Z]{4}-[0-9A-Z]{4})\) Tj`)

// printed rend le code imprimé sur la lettre i reçue par le double de Pingen.
func (e *env) printed(i int) string {
	m := rePrinted.FindSubmatch(e.f.Post.Sent()[i].PDF)
	if m == nil {
		e.t.Fatalf("lettre %d sans code", i)
	}
	code, _ := cles.NormalizeCode(string(m[1]))
	return code
}

// fiche publie la fiche d'alerte du titulaire (alerte/fiche).
func (k *coffre) fiche(t *testing.T, phone *string) {
	f := vie.Alert{UID: k.h.uid, Generation: 1, Email: "titulaire@exemple.fr", Telephone: phone}
	msg, err := f.Message()
	b := jcs.Object{"uid": k.h.uid, "generation": 1, "telephone": nil, "email": f.Email, "signature": k.h.sign(msg, err)}
	if phone != nil {
		b["telephone"] = *phone
	}
	ok(t, k.e.open("a").call("alerte/fiche", b))
}

func (e *env) secours(uid, contact string) jcs.Object {
	return e.open("c").call("lettre/secours", jcs.Object{"jetonFatoom": e.oidc(e.cfg.EnvOverride["COMPTE_DECES"]), "uid": uid, "contactId": contact})
}

func (e *env) sent(kind string) []factice.Message {
	var out []factice.Message
	for _, m := range e.f.Outbox.Sent() {
		if m.Kind == kind {
			out = append(out, m)
		}
	}
	return out
}

func TestLettreEtInvitation(t *testing.T) { // SOU-03, SOU-04, SOU-05, SOU-08, SOU-10, PAS-01, API.md §2.14, §5.3 ; « Lettres »
	e := newEnv(t)
	k := newCoffre(t, e, "u-lettre")
	l := k.add("p-l", "proche", nil, nil)
	k.add("p-m", "proche", nil, "m@exemple.fr")
	k.transmit(t, "insee")
	e.advance(time.Hour) // tâche horaire des trois instances : une seule lettre
	if n := len(e.f.Post.Sent()); n != 1 || len(e.notes("lettreEnvoyee")) != 1 {
		t.Fatalf("lettres : %d", n)
	}
	code := e.printed(0)
	letter := e.f.Base.Doc("titulaires/" + k.h.uid + "/lettres/" + l.contact).Data
	lookup := cles.IndexAcces(code)
	inv := e.f.Base.Doc("invitations/" + lookup).Data
	if letter["invitationLookup"] != lookup || letter["statut"] != "commandee" || letter["reserveeLe"] != nil || inv["contactId"] != l.contact ||
		len(inv) != 5 || !strings.Contains(string(e.f.Post.Sent()[0].PDF), "Texte de test lettre_deces. L\xe9a Jean Dupont") {
		t.Fatalf("lettre : %v %v", letter, inv)
	}
	for _, n := range e.notes("lettreEnvoyee") {
		if strings.Contains(jcsString(n), code) {
			t.Fatal("code dans une notification")
		}
	}
	c := e.open("b")
	o := ok(t, c.openCode(cles.DisplayCode(code), lookup))
	if o["canal"] != "identite" || o["masque"] != nil {
		t.Fatalf("invitation : %v", o)
	}
	used := e.f.Base.Doc("titulaires/" + k.h.uid + "/lettres/" + l.contact).Data["invitationUtiliseeLe"]
	if used == nil {
		t.Fatal("invitationUtiliseeLe")
	}
	id := str(o, "sessionId")
	ok(t, c.sid("session/identite/demarrer", id))
	e.f.Identity.Verdict(ports.Verification{Status: "verified", FirstName: "Léa", LastName: "Martin", BirthDate: "1990-05-04"})
	ok(t, c.sid("session/identite/verifier", id))
	if e.f.Base.Doc("titulaires/" + k.h.uid + "/lettres/" + l.contact).Data["identiteVerifieeLe"] == nil {
		t.Fatal("identiteVerifieeLe")
	}
	// Identité déjà vérifiée : canal aucun et jeton aussitôt.
	o = ok(t, e.open("b").openCode(code, lookup))
	if o["canal"] != "aucun" || o["base"] != "deces" || str(o, "jeton") == "" || len(e.notes("ouvertureNotifiee")) != 2 {
		t.Fatalf("aucun : %v", o)
	}
	if e.f.Base.Doc("titulaires/" + k.h.uid + "/lettres/" + l.contact).Data["invitationUtiliseeLe"] != used {
		t.Fatal("invitationUtiliseeLe réécrit")
	}
	// Autre code avec cet index : code faux du proche.
	other, _ := cles.DrawCode(random(12))
	expect(t, e.open("b").openCode(other, lookup), 403, "code_refuse")
	if e.f.Base.Doc("titulaires/" + k.h.uid + "/essais/" + l.contact).Data["essaisCode"] != 1.0 {
		t.Fatal("code faux d'invitation non compté")
	}
	// Charte d'une génération antérieure à celle du registre.
	k.publish(t, l, 2)
	l.gen = 1
	expect(t, e.open("b").openCode(code, lookup), 403, "code_refuse")
}

func TestSuiviDesLettres(t *testing.T) { // « Lettres » (renvois, erreurs, lettreEchouee), API.md §5.5
	e := newEnv(t)
	k := newCoffre(t, e, "u-suivi")
	l := k.add("p-l", "proche", nil, nil)
	k.transmit(t, "acte")
	e.advance(time.Hour)
	code := e.printed(0)
	path := "titulaires/" + k.h.uid + "/lettres/" + l.contact
	id := str(e.f.Base.Doc(path).Data, "pingenDocumentId")
	e.f.Post.SetStatus(id, "imprimee")
	e.advance(time.Hour)
	if n := e.notes("statutLettre"); len(n) != 1 || n[0]["statut"] != "imprimee" {
		t.Fatalf("statutLettre : %v", n)
	}
	for i := 1; i <= 3; i++ {
		id = str(e.f.Base.Doc(path).Data, "pingenDocumentId")
		e.f.Post.SetStatus(id, "erreur")
		e.advance(time.Hour)
		if d := e.f.Base.Doc(path).Data; d["erreurs"] != float64(i) {
			t.Fatalf("erreur %d : %v", i, d)
		}
	}
	if n := len(e.f.Post.Sent()); n != 3 || e.printed(2) != code || len(e.notes("lettreEchouee")) != 1 {
		t.Fatalf("nouveaux envois : %d", n)
	}
	// Renvoi 8 jours après le premier envoi, même code.
	k2 := newCoffre(t, e, "u-renvoi")
	l2 := k2.add("p-l", "proche", nil, nil)
	k2.transmit(t, "acte")
	e.advance(time.Hour)
	p2 := "titulaires/" + k2.h.uid + "/lettres/" + l2.contact
	d := e.f.Base.Doc(p2).Data
	before := len(e.f.Post.Sent())
	d["envoyeeLe"] = stampOf(e.now().Add(-8 * 24 * time.Hour))
	e.put(p2, d)
	e.advance(time.Hour)
	if d := e.f.Base.Doc(p2).Data; len(d["renvoyeeLe"].([]any)) != 1 || len(e.f.Post.Sent()) != before+1 || e.printed(before) != e.printed(before-1) {
		t.Fatalf("renvoi : %v", d)
	}
}

func TestLettreDeSecours(t *testing.T) { // LSC-01, LSC-02, LSC-04, LSC-05, LSC-06, API.md §2.10
	e := newEnv(t)
	k := newCoffre(t, e, "u-secours")
	p := k.add("p", "proche", "+33600000001", "p@exemple.fr")
	q := k.add("q", "proche", nil, "q@exemple.fr")
	q.charte["nom"] = nil
	if r := ok(t, e.secours(k.h.uid, p.contact)); r["commandee"] != false {
		t.Fatalf("avant la transmission : %v", r)
	}
	k.transmit(t, "acte")
	if r := ok(t, e.secours(k.h.uid, p.contact)); r["commandee"] != true || len(e.f.Post.Sent()) != 1 {
		t.Fatalf("secours : %v", r)
	}
	if r := ok(t, e.secours(k.h.uid, p.contact)); r["commandee"] != false {
		t.Fatalf("déjà commandée : %v", r)
	}
	if r := ok(t, e.secours(k.h.uid, q.contact)); r["commandee"] != false {
		t.Fatalf("sans nom : %v", r)
	}
	code := e.printed(0)
	if o := ok(t, e.open("b").openCode(code, cles.IndexAcces(code))); o["canal"] != "identite" {
		t.Fatalf("code de la lettre de secours : %v", o)
	}
	k3 := newCoffre(t, e, "u-secours-3")
	r3 := k3.add("r", "proche", nil, "r@exemple.fr")
	k3.transmit(t, "acte")
	e.f.Post.Set("Courrier", true)
	expect(t, e.secours(k3.h.uid, r3.contact), 503, "fournisseur_indisponible")
	e.f.Post.Set("Courrier", false)
	e.advance(time.Hour) // reprise de l'envoi réservé, même code
	if n := len(e.f.Post.Sent()); n != 2 || e.f.Base.Doc("titulaires/" + k3.h.uid + "/lettres/" + r3.contact).Data["reserveeLe"] != nil {
		t.Fatalf("reprise : %d", n)
	}
	r4 := k3.add("r4", "proche", nil, "r4@exemple.fr")
	e.f.Gateway.Answer = func(factice.Request) (int, []byte, error) { return 200, []byte("pas du JSON"), nil }
	expect(t, e.secours(k3.h.uid, r4.contact), 503, "passerelle_indisponible")
	e.f.Gateway.Answer = k3.serve
	c := e.open("c")
	expect(t, c.call("lettre/secours", jcs.Object{"jetonFatoom": e.oidc("autre@exemple.test"), "uid": k3.h.uid, "contactId": r3.contact}), 401, "jeton_invalide")
	expect(t, e.secours("inconnu", r3.contact), 409, "titulaire_indisponible")
	for range 100 {
		e.open("c").call("lettre/secours", jcs.Object{"jetonFatoom": e.oidc(e.cfg.EnvOverride["COMPTE_DECES"]), "uid": "inconnu", "contactId": "x"})
	}
	expect(t, e.secours("inconnu", "x"), 429, "trop_d_appels")
}

func TestAlertes(t *testing.T) { // DSG-01, DSG-04, DSG-08, SVE-01, PAS-09, API.md §2.8 ; versions-et-alertes.md
	e := newEnv(t)
	k := newCoffre(t, e, "u-alertes")
	phone := "+33611111111"
	k.fiche(t, &phone)
	s := ok(t, e.signal(k.h.uid, "insee"))
	if m := e.sent("email"); len(m) != 1 || m[0].To != "titulaire@exemple.fr" || !strings.Contains(m[0].Text, "deces-2026-m10.txt") || len(e.sent("sms")) != 1 {
		t.Fatalf("J0 : %v", e.f.Outbox.Sent())
	}
	ok(t, e.signal(k.h.uid, "acte")) // signalement renvoyé : pas de nouvelle alerte
	path := "titulaires/" + k.h.uid + "/signalements/" + str(s, "signalementId")
	for i, h := range []time.Duration{73 * time.Hour, 145 * time.Hour} {
		d := e.f.Base.Doc(path).Data
		d["signaleLe"] = stampOf(e.now().Add(-h))
		e.put(path, d)
		e.advance(time.Hour)
		if n := len(e.sent("email")); n != i+2 || !strings.HasPrefix(e.sent("email")[i+1].Subject, "Texte de test alerte_signalement_insee_rappel_email.") {
			t.Fatalf("rappel %d : %v", i+1, e.sent("email"))
		}
	}
	// Visite du contact de confiance de son vivant.
	k2 := newCoffre(t, e, "u-alertes-2")
	k2.fiche(t, nil)
	g := k2.add("g", "gardien", "+33600000001", nil)
	before := len(e.sent("email"))
	k2.session(t, g)
	if m := e.sent("email"); len(m) != before+1 || !strings.HasPrefix(m[before].Text, "Texte de test alerte_confiance_email.") {
		t.Fatalf("alerte de confiance : %v", m)
	}
	// Ligne rejetée : alerte au passage suivant, au plus une par jour.
	e.reject("titulaires/" + k2.h.uid)
	expect(t, e.open("a").call("etat", jcs.Object{"uid": k2.h.uid, "idToken": e.idToken(k2.h.uid), "repere": jcs.Object{"ouvertures": nil, "evaluations": 0}}), 409, "titulaire_indisponible")
	tick(e)
	e.open("a").call("etat", jcs.Object{"uid": k2.h.uid, "idToken": e.idToken(k2.h.uid), "repere": jcs.Object{"ouvertures": nil, "evaluations": 0}})
	tick(e)
	rejected := 0
	for _, m := range e.sent("email") {
		if strings.HasPrefix(m.Text, "Texte de test alerte_ligne_rejetee_email.") {
			rejected++
		}
	}
	if rejected != 1 {
		t.Fatalf("alertes de ligne rejetée : %d", rejected)
	}
	// Alerte refusée par le fournisseur : réessais 24 h, puis alerte_refusee.
	k3 := newCoffre(t, e, "u-alertes-3")
	k3.fiche(t, nil)
	e.f.Outbox.Set("Email", true)
	ok(t, e.signal(k3.h.uid, "acte"))
	for range 26 {
		e.f.Clock.Advance(time.Hour)
		for _, in := range e.f.Server.Instances {
			in.Tick(t.Context(), false)
		}
	}
	found := false
	for _, ev := range e.f.Journal.Events() {
		found = found || ev.Event == "alerte_refusee" && ev.UIDMasque != "" && !strings.Contains(ev.UIDMasque, k3.h.uid)
	}
	if !found {
		t.Fatal("alerte_refusee non journalisée")
	}
}

func TestSessionsPerdues(t *testing.T) { // SFE-03, SOU-19, API.md §5.5 ; registre.md « Autres précisions »
	e := newEnv(t)
	k := newCoffre(t, e, "u-perdue")
	g := k.add("g", "gardien", "+33600000001", nil)
	_, _, v := k.session(t, g)
	if v["base"] != "vivant" {
		t.Fatalf("session : %v", v)
	}
	if err := e.f.Restart(t.Context(), "b"); err != nil {
		t.Fatal(err)
	}
	e.advance(time.Hour)
	n := e.notes("sessionTerminee")
	if len(n) != 1 || n[0]["ouvertLe"] == nil || n[0]["contactId"] != g.contact {
		t.Fatalf("sessionTerminee : %v", n)
	}
	e.advance(time.Hour)
	if len(e.notes("sessionTerminee")) != 1 {
		t.Fatal("seconde notification")
	}
}

func jcsString(o jcs.Object) string { b, _ := jcs.Marshal(o); return string(b) }

func TestPasserelleDeTest(t *testing.T) { // PAS-02, PAS-03, PAS-04, PAS-10, API.md §5.1, §5.5
	e := newEnv(t)
	k := newCoffre(t, e, "u-passerelle")
	g := k.add("g", "gardien", "+33600000001", nil)
	k.session(t, g)
	if r := e.f.Gateway.Check.Refused; len(r) != 0 {
		t.Fatalf("requêtes de Sésame refusées : %v", r)
	}
	var get, post factice.Request
	for _, r := range e.f.Gateway.Requests() {
		if r.Method == "GET" {
			get = r
		} else {
			post = r
		}
	}
	send := func(r factice.Request, edit func(h map[string]string)) int {
		h := map[string]string{}
		for k, v := range r.Headers {
			h[k] = v
		}
		edit(h)
		st, _, _ := e.f.Gateway.Send(t.Context(), r.Method, r.PathQuery, h, r.Body)
		return st
	}
	same := func(map[string]string) {}
	if send(get, same) != 401 || send(post, same) != 200 {
		t.Fatal("rejeu d'un GET /bundle ou notification déjà traitée")
	}
	for name, edit := range map[string]func(h map[string]string){
		"kem absent":       func(h map[string]string) { delete(h, "X-Sesame-Kem-Pub") },
		"autre kem":        func(h map[string]string) { h["X-Sesame-Kem-Pub"] = b64(random(1184)) },
		"signature":        func(h map[string]string) { h["X-Sesame-Signature"] = b64(random(64)) },
		"attestation":      func(h map[string]string) { h["X-Sesame-Attestation"] = "x.y.z" },
		"identifiant":      func(h map[string]string) { h["X-Sesame-Request-Id"] = "00000000-0000-4000-8000-000000000000" },
		"echeanceModifiee": func(h map[string]string) {},
	} {
		r := post
		if name == "echeanceModifiee" {
			r.PathQuery = "/echeanceModifiee"
		}
		if st := send(r, edit); st != 401 && st != 400 {
			t.Fatalf("%s : %d", name, st)
		}
	}
	e.f.Clock.Advance(6 * time.Minute)
	if send(post, func(h map[string]string) { h["X-Sesame-Request-Id"] = "11111111-1111-4111-8111-111111111111" }) != 401 {
		t.Fatal("horodatage de plus de 5 min accepté")
	}
}
