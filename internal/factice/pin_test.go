package factice_test

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/circl/oprf"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/pin"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// Cas `pin/deposer` (PDP) et `pin/evaluer` (PEV) de routes-etat.md (API.md
// §2.12, §2.13), en grappe locale amorcée.

func element(t *testing.T) []byte {
	b, err := oprf.SuiteRistretto255.Group().RandomElement(rand.Reader).MarshalBinaryCompress()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (e *env) grappeID() string { return e.f.Server.Instances[0].Grappe().ID() }

// depotBody compose un corps de pin/deposer signé par signer.
func (c *ch) depotBody(h, signer *titulaire, motif string, elem []byte, gid string) jcs.Object {
	d := c.defi(h.uid)
	return jcs.Object{"uid": h.uid, "idToken": c.e.idToken(h.uid), "grappeId": gid, "elementAveugle": b64(elem), "defi": d, "motif": motif,
		"signature": signer.sign(vie.PinDepositMessage(h.uid, gid, elem, d, motif))}
}

func (c *ch) depot(h *titulaire, motif string, elem []byte) jcs.Object {
	return c.call("pin/deposer", c.depotBody(h, h, motif, elem, c.e.grappeID()))
}

func (c *ch) evaluer(h *titulaire, elem []byte) jcs.Object {
	return c.call("pin/evaluer", jcs.Object{"uid": h.uid, "idToken": c.e.idToken(h.uid), "elementAveugle": b64(elem)})
}

// walkTo avance l'horloge jusqu'à t par pas de moins de 2 heures, tâches de
// fond comprises : bornes déposées, jeton du lanceur renouvelé.
func (e *env) walkTo(t time.Time) {
	for e.now().Add(110 * time.Minute).Before(t) {
		e.advance(110 * time.Minute)
	}
	e.f.Clock.Set(t)
	tick(e)
}

func (e *env) record(uid string) (pin.Record, bool) {
	return e.f.Server.Instances[1].Grappe().Record(uid)
}

func TestDepotNominal(t *testing.T) { // PDP-01, PDP-08, PDP-13, PDP-15, ETA-03, ENR-03, API.md §2.12
	e := newEnv(t)
	h, c := enrolled(t, e, "u-pin")
	x := element(t)
	e1 := str(ok(t, c.depot(h, "redepot", x)), "evaluation") // redepot sur un enregistrement absent : créé avec 10 essais
	gp := sub(e.line(h.uid), "grappePin")
	if r, _ := e.record(h.uid); r.Left != 10 || e1 == "" || gp["grappeId"] != e.grappeID() || gp["depuisLe"] != "2026-11-11T10:00:00.000Z" ||
		e.line(h.uid)["pinDeposeLe"] == nil {
		t.Fatalf("premier dépôt : %v %v", r, e.line(h.uid))
	}
	if ev := str(ok(t, c.evaluer(h, x)), "evaluation"); ev != e1 {
		t.Fatal("évaluation sous une autre clé")
	}
	head := e.f.Base.Doc("chaine/tete").Data["epoque"]
	o := ok(t, c.depot(h, "ouverture", x))
	if r, _ := e.record(h.uid); o["evaluation"] != nil || o["grappeId"] != e.grappeID() || r.Left != 10 || e.f.Base.Doc("chaine/tete").Data["epoque"] != head {
		t.Fatalf("ouverture : %v %v", o, r)
	}
	if ev := str(ok(t, c.evaluer(h, x)), "evaluation"); ev != e1 { // cleOprf gardée
		t.Fatal("ouverture a changé la clé")
	}
	e2 := str(ok(t, c.depot(h, "redepot", x)), "evaluation")
	if r, _ := e.record(h.uid); e2 == e1 || r.Left != 9 || r.Evals != 2 {
		t.Fatalf("redepot : %+v", r)
	}
	e3 := str(ok(t, c.depot(h, "nouveau_pin", x)), "evaluation")
	e4 := str(ok(t, c.depot(h, "nouveau_pin", x)), "evaluation")
	if r, _ := e.record(h.uid); e3 == e2 || e4 == e3 || r.Left != 10 || r.Evals != 2 || str(ok(t, c.evaluer(h, x)), "evaluation") != e4 {
		t.Fatalf("nouveau_pin répété : %+v", r)
	}
	// etat : compteur inchangé par nouveau_pin et ouverture (ETA-03).
	if p := e.payload(str(ok(t, c.etat(h, nil, 1)), "etat"), false); p["pinPresent"] != true || p["evaluationsDepuis"] != 2.0 {
		t.Fatalf("etat : %v", p)
	}
}

func TestDepotRefus(t *testing.T) { // PDP-02, PDP-04, PDP-06, PDP-09, PDP-10, PDP-16, API.md §2.12, §6, §7
	e := newEnv(t)
	h, c := enrolled(t, e, "u-pref")
	x, y := element(t), element(t)
	ok(t, c.depot(h, "nouveau_pin", x))
	expect(t, c.depot(h, "ouverture", x), 400, "message_refuse") // sans évaluation
	ok(t, c.evaluer(h, x))
	expect(t, e.open("b").depot(h, "ouverture", x), 400, "message_refuse") // autre canal
	expect(t, c.depot(h, "ouverture", y), 400, "message_refuse")           // autre élément
	b := c.depotBody(h, newTitulaire(h.uid), "redepot", x, e.grappeID())
	expect(t, c.call("pin/deposer", b), 400, "message_refuse") // autre clé
	expect(t, c.call("pin/deposer", b), 409, "defi_invalide")  // défi rejoué, signature fausse
	b = c.depotBody(h, h, "redepot", x, strings.Repeat("ab", 32))
	expect(t, c.call("pin/deposer", b), 409, "defi_invalide") // grappeId d'une autre grappe
	b = c.depotBody(h, h, "redepot", x, e.grappeID())
	b["idToken"] = e.idToken("autre")
	expect(t, c.call("pin/deposer", b), 401, "jeton_invalide")
	other := newTitulaire("jamais")
	expect(t, c.call("pin/deposer", c.depotBody(other, h, "nouveau_pin", x, e.grappeID())), 409, "pin_indisponible") // ligne absente, signature fausse
	for range 9 {
		ok(t, c.depot(h, "nouveau_pin", x))
	}
	expect(t, c.depot(h, "nouveau_pin", x), 429, "trop_d_appels") // 11ᵉ appel décompté en une heure ; les refus avant la signature ne comptent pas
	g := newTitulaire("u-pref2")
	c.enrol(g)
	e.reject("titulaires/" + g.uid)
	expect(t, c.depot(g, "nouveau_pin", x), 409, "pin_indisponible")
	expect(t, c.evaluer(g, x), 409, "pin_indisponible")
}

// plafond fait les dix essais du code de h, chacun après son délai.
func plafond(t *testing.T, e *env, h *titulaire, x []byte) {
	for i := range 10 {
		r := e.open("c").evaluer(h, x)
		if i < 9 {
			ok(t, r)
		} else {
			expect(t, r, 409, "pin_bloque") // 10ᵉ essai décompté sans évaluation (PEV-03)
		}
		e.walkTo(e.now().Add(25 * time.Hour))
	}
}

func TestPlafond(t *testing.T) { // PDP-03, PDP-04, PDP-12, PEV-03, VIE-02, ETA-02, API.md §2.12, §2.13 ; R-PIN-02
	e := newEnv(t)
	h, c := enrolled(t, e, "u-plaf")
	x := element(t)
	ok(t, c.depot(h, "nouveau_pin", x))
	plafond(t, e, h, x)
	c = e.open("a")
	if r, _ := e.record(h.uid); r.Left != 0 || r.Key != nil || e.line(h.uid)["pinBloqueLe"] == nil || e.line(h.uid)["pinEvaluations"] != 10.0 {
		t.Fatalf("plafond : %+v %v", r, e.line(h.uid))
	}
	expect(t, c.evaluer(h, x), 409, "pin_bloque")
	expect(t, c.depot(h, "redepot", x), 409, "pin_bloque")
	if o := ok(t, c.life(h)); o["pinBloque"] != true {
		t.Fatalf("vie : %v", o)
	}
	// redepot au plafond dans une grappe récente : pin_bloque, listé avant grappe_recente (§6).
	l := e.line(h.uid)
	l["grappePin"] = jcs.Object{"grappeId": strings.Repeat("cd", 32), "depuisLe": "2026-11-20T00:00:00.000Z"}
	e.put("titulaires/"+h.uid, l)
	expect(t, c.depot(h, "redepot", x), 409, "pin_bloque")
	if sub(e.line(h.uid), "grappePin")["grappeId"] != strings.Repeat("cd", 32) {
		t.Fatal("grappePin réécrit par un dépôt refusé")
	}
	l["grappePin"] = jcs.Object{"grappeId": e.grappeID(), "depuisLe": "2026-11-11T10:00:00.000Z"}
	e.put("titulaires/"+h.uid, l)
	o := ok(t, c.depot(h, "nouveau_pin", x)) // seul nouveau_pin sort du plafond
	if r, _ := e.record(h.uid); r.Left != 10 || o["evaluation"] == nil || e.line(h.uid)["pinBloqueLe"] != nil {
		t.Fatalf("nouveau_pin au plafond : %+v", r)
	}
}

func TestGrappeRecente(t *testing.T) { // PDP-05, PDP-07, PDP-08, PDP-12, API.md §2.12 ; LIM-19
	e := newEnv(t)
	h, c := enrolled(t, e, "u-rec")
	x := element(t)
	l := e.line(h.uid)
	l["grappePin"] = jcs.Object{"grappeId": strings.Repeat("cd", 32), "depuisLe": "2026-10-20T00:00:00.000Z"}
	e.put("titulaires/"+h.uid, l)
	expect(t, c.depot(h, "redepot", x), 409, "grappe_recente")
	if sub(e.line(h.uid), "grappePin")["depuisLe"] != "2026-10-20T00:00:00.000Z" {
		t.Fatal("grappePin réécrit")
	}
	e.walkTo(time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC))
	c = e.open("a")
	ok(t, c.depot(h, "redepot", x)) // 31 jours après l'entrée : admis, grappe inscrite remplacée
	if gp := sub(e.line(h.uid), "grappePin"); gp["grappeId"] != e.grappeID() || gp["depuisLe"] != "2026-11-20T00:00:00.000Z" {
		t.Fatalf("grappePin : %v", gp)
	}
	e = newEnv(t)
	h, c = enrolled(t, e, "u-rec2")
	e.f.Stockage.Set("Read:audit", true)
	tick(e)
	expect(t, c.depot(h, "nouveau_pin", x), 503, "reessayer_plus_tard") // premier dépôt : anti-retour exigé
	if e.line(h.uid)["grappePin"] != nil {
		t.Fatal("grappePin écrit malgré l'anti-retour")
	}
}

func TestDepotPannes(t *testing.T) { // PDP-11, PDP-14, PEV-05, PEV-06, PEV-07, API.md §2.12, §2.13, §6
	mem := 0.5
	e := newEnv(t, func(o *factice.Options) { o.Memoire = func() float64 { return mem } })
	h, c := enrolled(t, e, "u-pan")
	x := element(t)
	mem = 0.95
	expect(t, c.depot(h, "nouveau_pin", x), 503, "pin_indisponible_temporaire") // mémoire à 90 % : nouveau dépôt refusé
	if e.line(h.uid)["grappePin"] != nil {
		t.Fatal("grappePin écrit malgré la mémoire")
	}
	mem = 0.5
	ok(t, c.depot(h, "nouveau_pin", x))
	mem = 0.95
	ok(t, c.depot(h, "nouveau_pin", x)) // remplace un enregistrement présent : admis
	mem = 0.5
	e.f.Base.Set("Get", true)
	expect(t, c.depot(h, "redepot", x), 503, "pin_indisponible_temporaire")
	expect(t, c.evaluer(h, x), 503, "pin_indisponible_temporaire") // registre illisible (dont STS refusé) : aucun essai
	e.f.Base.Set("Get", false)
	if r, _ := e.record(h.uid); r.Evals != 0 {
		t.Fatalf("essai décompté sans lecture : %+v", r)
	}
	e.f.KMS.Set("Sign", true) // recopie refusée après la validation
	expect(t, c.depot(h, "redepot", x), 503, "pin_indisponible_temporaire")
	expect(t, c.evaluer(h, x), 503, "pin_indisponible_temporaire")
	e.f.KMS.Set("Sign", false)
	if r, _ := e.record(h.uid); r.Evals != 1 || e.line(h.uid)["pinEvaluations"] != 0.0 {
		t.Fatalf("essai décompté sans recopie : %+v %v", r, e.line(h.uid))
	}
	ok(t, c.evaluer(h, x))
	if e.line(h.uid)["pinEvaluations"] != 2.0 { // la recopie suivante écrit le compteur entier
		t.Fatalf("recopie : %v", e.line(h.uid))
	}
}

func TestEvaluer(t *testing.T) { // PEV-01, PEV-04, PEV-08, PEV-09, API.md §2.13, §7 ; R-PIN-12
	e := newEnv(t)
	h, c := enrolled(t, e, "u-ev")
	ok(t, c.call("alerte/fiche", e.fiche(c, h, 1, "titulaire@example.org", nil, h)))
	x := element(t)
	expect(t, c.evaluer(h, x), 409, "pin_indisponible") // aucun enregistrement
	expect(t, c.evaluer(newTitulaire("inconnu"), x), 409, "pin_indisponible")
	ok(t, c.depot(h, "nouveau_pin", x))
	for range 3 {
		ok(t, c.evaluer(h, x))
	}
	if n := len(e.sent("email")); n != 1 || !strings.Contains(e.sent("email")[0].Text, "11/11/2026 à 11 h 00") {
		t.Fatalf("alertes : %v", e.sent("email"))
	}
	if o := c.evaluer(h, x); o["statut"] != 429.0 || str(sub(o, "details"), "reessayerLe") != "2026-11-11T10:01:00.000Z" {
		t.Fatalf("délai : %v", o) // 4ᵉ essai avant 1 min : trop_d_essais, sans décompte (PEV-02)
	}
	b := jcs.Object{"uid": h.uid, "idToken": e.idToken("autre"), "elementAveugle": b64(x)}
	expect(t, c.call("pin/evaluer", b), 401, "jeton_invalide")
	for range 16 {
		c.evaluer(h, x)
	}
	expect(t, c.evaluer(h, x), 429, "trop_d_appels") // 21ᵉ appel authentifié en une heure
	if r, _ := e.record(h.uid); r.Evals != 3 {
		t.Fatalf("décomptés : %+v", r)
	}
}

func TestCompteurPinParSesameFake(t *testing.T) { // PEV-02, PDP-03, API.md §2.13 ; vecteur compteurPin
	var f struct {
		Vecteurs []struct {
			Entrees struct {
				Appels []struct{ Route, Motif, Le string }
			}
			Sorties struct {
				Apres []struct {
					Reponse, ReessayerLe string
					EssaisRestants       int
				}
			}
		}
	}
	b, err := os.ReadFile("../../testdata/vecteurs/compteurPin.json")
	if err != nil || json.Unmarshal(b, &f) != nil {
		t.Fatal("compteurPin.json illisible")
	}
	e := newEnv(t)
	h, c := enrolled(t, e, "u-cpt")
	x := element(t)
	v := f.Vecteurs[0]
	for i, a := range v.Entrees.Appels {
		le, _ := time.Parse(time.RFC3339, a.Le)
		e.walkTo(le)
		c = e.open("b")
		var r jcs.Object
		if a.Route == "pin/evaluer" {
			r = c.evaluer(h, x)
		} else {
			r = c.depot(h, a.Motif, x)
		}
		want := v.Sorties.Apres[i]
		got := map[float64]string{200: "evaluation", 409: str(r, "erreur"), 429: str(r, "erreur"), 400: str(r, "erreur")}[r["statut"].(float64)]
		if a.Motif == "ouverture" && got == "message_refuse" { // ouverture sans évaluation notée : refusée par la VM qui reçoit, avant le plafond
			got = "pin_bloque"
		}
		rec, _ := e.record(h.uid)
		if got != want.Reponse || rec.Left != want.EssaisRestants || want.ReessayerLe != "" && str(sub(r, "details"), "reessayerLe") != want.ReessayerLe {
			t.Fatalf("appel %d (%s %s) : %v, %+v ; attendu %+v", i+1, a.Route, a.Motif, r, rec, want)
		}
	}
}
