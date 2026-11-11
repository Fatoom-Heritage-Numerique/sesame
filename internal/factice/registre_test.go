package factice_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
)

// Registre en mémoire, borne et contrôle anti-retour (tests.md, « Registre,
// clés et état de l'enclave » et « CI du dépôt sesame », borne ;
// ARCHITECTURE.md §6, §8.1).

func (e *env) bornes() []jcs.Object {
	objs, err := e.f.Stockage.List(context.Background(), "audit", "bornes/", 1000)
	if err != nil {
		e.t.Fatal(err)
	}
	var out []jcs.Object
	for _, o := range objs {
		f := decode(e.t, o.Content)
		if !registre.VerifyBorne(o.Name, f, e.cfg.VersionList().SessionKeys) {
			e.t.Fatalf("borne invalide : %s", o.Name)
		}
		out = append(out, sub(f, "borne"))
	}
	return out
}

func TestRegistreChaineEtSig(t *testing.T) {
	e := newEnv(t)
	h, c := enrolled(t, e, "u-reg")
	head := e.f.Base.Doc(registre.Head).Data
	l := e.line(h.uid)
	if head["epoque"] != 4.0 || len(str(head, "empreinte")) != 64 || len(str(l, "sig")) != 86 || str(l, "ecritLe") != "2026-11-11T10:00:00.000Z" {
		t.Fatalf("tête %v, ligne %v", head, l) // époques : trois instances, puis la ligne
	}
	// sig fausse, ecritLe futur : ligne rejetée, jamais réécrite.
	l["vivantLe"] = "2030-01-01T00:00:00.000Z"
	e.f.Base.Tamper("titulaires/"+h.uid, l, e.now())
	expect(t, c.call("charte/generation", e.generation(c, h, "p1", 1)), 409, "titulaire_indisponible")
	g, c := enrolled(t, e, "u-reg2")
	l = e.line(g.uid)
	e.put("titulaires/"+g.uid, l)
	e.f.Base.Tamper("titulaires/"+g.uid, e.line(g.uid), e.now().Add(-3*time.Minute))
	expect(t, c.call("charte/generation", e.generation(c, g, "p1", 1)), 409, "titulaire_indisponible")
	// KMS indisponible : rien d'inscrit.
	k, c := enrolled(t, e, "u-reg3")
	e.f.KMS.Set("Sign", true)
	expect(t, c.call("charte/generation", e.generation(c, k, "p1", 1)), 503, "registre_indisponible")
	e.f.KMS.Set("Sign", false)
	if sub(e.line(k.uid), "chartes")["p1"] != nil {
		t.Fatal("écrit sans signature")
	}
}

func TestBorne(t *testing.T) {
	e := newEnv(t)
	h, c := enrolled(t, e, "u-borne")
	e.reject("titulaires/" + h.uid)
	for range 10 {
		c.etat(h, nil, 0)
	}
	e.advance(10 * time.Minute)
	bs := e.bornes()
	if len(bs) != 3 {
		t.Fatalf("bornes : %d", len(bs))
	}
	events := 0
	for _, b := range bs {
		for _, ev := range b["evenements"].(jcs.Array) {
			o := ev.(jcs.Object)
			if o["type"] == "ligne_rejetee" {
				events++
				if strings.Contains(str(o, "ressource"), h.uid) || str(o, "vuLe") != "2026-11-11T10:00:00.000Z" {
					t.Fatalf("événement : %v", o)
				}
			}
		}
		if sub(b, "registre")["epoque"] == nil || sub(b, "grappe")["grappeId"] != nil {
			t.Fatalf("borne : %v", b)
		}
	}
	if events != 1 {
		t.Fatalf("ligne_rejetee : %d entrées, attendu 1 (instance a)", events)
	}
	// Lue encore après le dépôt : de nouveau dans la borne suivante.
	c = e.open("a")
	c.etat(h, nil, 0)
	e.advance(10 * time.Minute)
	if n := strings.Count(strings.Join(eventsOf(e.bornes()), ","), "ligne_rejetee"); n != 2 {
		t.Fatalf("ligne_rejetee : %d", n)
	}
	// Création seule : la même borne ne se redépose pas.
	if err := e.f.Stockage.Create(context.Background(), "audit", "bornes/x", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.f.Stockage.Create(context.Background(), "audit", "bornes/x", nil); err == nil {
		t.Fatal("objet écrasé")
	}
}

func eventsOf(bs []jcs.Object) []string {
	var out []string
	for _, b := range bs {
		for _, ev := range b["evenements"].(jcs.Array) {
			out = append(out, str(ev.(jcs.Object), "type"))
		}
	}
	return out
}

func TestAntiRetour(t *testing.T) {
	// Toute première version ({sansSauvegarde: true}), bucket vide, époque 0 : réussi.
	e := newEnv(t)
	in := e.f.Server.Instances[0]
	if !in.UpToDate(context.Background()) {
		t.Fatal("première version en échec")
	}
	e.advance(10 * time.Minute)
	if !in.UpToDate(context.Background()) {
		t.Fatal("en échec après ses propres bornes")
	}
	// Bucket illisible : échec fermé, registre_ancien publié.
	e.f.Stockage.Set("Read:audit", true)
	e.advance(10 * time.Minute)
	if in.UpToDate(context.Background()) {
		t.Fatal("bucket illisible accepté")
	}
	e.f.Stockage.Set("Read:audit", false)
	e.advance(10 * time.Minute)
	// Registre remis en arrière : tête recopiée d'une époque plus ancienne.
	head := e.f.Base.Doc(registre.Head).Data
	head["epoque"] = 1
	e.put(registre.Head, head)
	if in.UpToDate(context.Background()) {
		t.Fatal("registre ancien accepté")
	}
	e.advance(10 * time.Minute)
	if !strings.Contains(strings.Join(eventsOf(e.bornes()), ","), "registre_ancien") {
		t.Fatal("registre_ancien absent des bornes")
	}
	// Sans constante reprise et sans borne : échec fermé, qui dure.
	e2 := newEnv(t, withReprise(nil))
	in2 := e2.f.Server.Instances[0]
	for range 3 {
		e2.advance(10 * time.Minute)
		if in2.UpToDate(context.Background()) {
			t.Fatal("version sans reprise acceptée sans borne")
		}
	}
	// Plus de 2 heures sans borne comptée : échec.
	e.f.Stockage.Set("Create:audit", true)
	e.advance(3 * time.Hour)
	if in.UpToDate(context.Background()) {
		t.Fatal("bornes de plus de 2 heures acceptées")
	}
}
