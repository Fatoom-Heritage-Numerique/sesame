package factice_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Cas partiels de routes-etat.md qui touchent la sécurité : rotation et refus
// du rôle vus d'une autre VM, suppressions chez Stripe abandonnées après
// 7 jours, Stripe en échec, limite de la passerelle de test.

func TestFermeturesAutreVM(t *testing.T) { // ENR-02, SOC-08, SRR-01, API.md §2.3, §2.19, §2.21 ; R-PARTAGE-13
	e := newEnv(t)
	k := newCoffre(t, e, "u-autre-vm")
	g := k.add("p-g", "gardien", "+33600000001", nil)
	c, id, _ := k.session(t, g) // tenue par b
	a := e.open("a")
	ok(t, a.call("enroler", a.enrolBody(k.h, 2, k.h.pub, k.h))) // rotation reçue par a
	before := len(e.notes("sessionTerminee"))
	expect(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{}), 410, "session_expiree")
	expect(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{}), 410, "session_expiree")
	if n := len(e.notes("sessionTerminee")); n != before+1 {
		t.Fatalf("sessionTerminee après une rotation sur une autre VM : %d", n-before)
	}
	// Rôle refusé par une session tenue par c : la session de b se ferme.
	k2 := newCoffre(t, e, "u-autre-vm-2")
	g2 := k2.add("p-g2", "gardien", "+33600000002", nil)
	c1, id1, _ := k2.session(t, g2) // tenue par b
	c2 := e.open("c")
	id2 := str(ok(t, c2.openCode(g2.code, g2.lookup)), "sessionId")
	ok(t, c2.sid("session/verifier", id2, "otp", "123456"))
	before = len(e.notes("sessionTerminee"))
	ok(t, c2.sid("session/refuser-role", id2))
	expect(t, c1.sid("session/ouvrir-cles", id1, "enveloppes", jcs.Array{}), 410, "session_expiree")
	if n := len(e.notes("sessionTerminee")); n != before+2 || len(e.notes("roleRefuse")) != 1 {
		t.Fatalf("sessionTerminee après un refus du rôle sur une autre VM : %d", n-before)
	}
}

// verified mène une vérification d'identité conforme pour p, suppression
// chez Stripe refusée.
func verified(t *testing.T, e *env, p *proche) {
	c := e.open("b")
	id := str(ok(t, c.openCode(p.code, p.lookup)), "sessionId")
	ok(t, c.sid("session/identite/bascule", id))
	ok(t, c.sid("session/identite/demarrer", id))
	e.f.Identity.Verdict(ports.Verification{Status: "verified", FirstName: "LÉA", LastName: "MARTIN", BirthDate: "1990-05-04"})
	e.f.Identity.Set("Redact", true)
	ok(t, c.sid("session/identite/verifier", id))
}

func TestRedactionAbandonnee(t *testing.T) { // SID-08, API.md §2.17 ; R-COMPTE-20
	e := newEnv(t)
	k := newCoffre(t, e, "u-redact")
	p := k.add("p", "proche", "+33600000001", nil)
	k.transmit(t, "insee")
	verified(t, e, p)
	path := "titulaires/" + k.h.uid + "/identites/" + p.contact
	pending := func() int { l, _ := e.f.Base.Doc(path).Data["redactionsEnAttente"].([]any); return len(l) }
	start := e.now()
	e.walkTo(start.Add(6 * 24 * time.Hour)) // représentée à chaque passage, refusée
	if pending() != 1 || e.events("redaction_identite_abandonnee") != 0 {
		t.Fatalf("avant 7 jours : %d", pending())
	}
	e.walkTo(start.Add(7*24*time.Hour + 2*time.Hour))
	if pending() != 0 || e.events("redaction_identite_abandonnee") != 1 || len(e.f.Identity.Redacted) != 0 {
		t.Fatalf("après 7 jours : %d, %d journalisés", pending(), e.events("redaction_identite_abandonnee"))
	}
}

func TestIdentiteFournisseur(t *testing.T) { // SID-05, API.md §2.16, §2.17
	e := newEnv(t)
	k := newCoffre(t, e, "u-stripe")
	p := k.add("p", "proche", "+33600000001", nil)
	k.transmit(t, "insee")
	c := e.open("b")
	id := str(ok(t, c.openCode(p.code, p.lookup)), "sessionId")
	ok(t, c.sid("session/identite/bascule", id))
	e.f.Identity.Set("Identite", true)
	expect(t, c.sid("session/identite/demarrer", id), 503, "fournisseur_indisponible")
	e.f.Identity.Set("Identite", false)
	ok(t, c.sid("session/identite/demarrer", id))
	e.f.Identity.Set("Identite", true)
	expect(t, c.sid("session/identite/verifier", id), 503, "fournisseur_indisponible")
	if d := e.f.Base.Doc("titulaires/" + k.h.uid + "/identites/" + p.contact).Data; d["essaisIdentite"] != nil && d["essaisIdentite"] != 0.0 {
		t.Fatalf("échec décompté : %v", d)
	}
}

func TestPasserelleLimite(t *testing.T) { // PAS-03, API.md §5.1, §7
	e := newEnv(t)
	k := newCoffre(t, e, "u-limite")
	g := k.add("g", "gardien", "+33600000001", nil)
	k.session(t, g)
	var post factice.Request
	for _, r := range e.f.Gateway.Requests() {
		if r.Method == "POST" {
			post = r
		}
	}
	e.f.Clock.Advance(time.Minute) // fenêtre neuve
	send := func(i int) int {
		h := map[string]string{}
		for k, v := range post.Headers {
			h[k] = v
		}
		h["X-Sesame-Date"] = e.now().Format(primitives.TimeLayout)
		h["X-Sesame-Request-Id"] = fmt.Sprintf("%08x-cccc-4ccc-8ccc-cccccccccccc", i)
		msg, _ := jetons.GatewayMessage(post.Method, post.PathQuery, h["X-Sesame-Date"], h["X-Sesame-Request-Id"], post.Body)
		der, _ := e.f.KMS.Sign(context.Background(), ports.SessionSign, msg)
		sig, _ := jetons.DERToRaw(der)
		h["X-Sesame-Signature"] = primitives.B64Encode(sig)
		st, _, _ := e.f.Gateway.Send(t.Context(), post.Method, post.PathQuery, h, post.Body)
		return st
	}
	for i := range 300 {
		if st := send(i); st != 200 {
			t.Fatalf("appel %d : %d", i+1, st)
		}
	}
	if st := send(300); st != 429 { // 301ᵉ appel en une minute d'une instance
		t.Fatalf("301ᵉ appel : %d", st)
	}
	e.f.Clock.Advance(time.Minute)
	if st := send(301); st != 200 {
		t.Fatalf("minute suivante : %d", st)
	}
}
