package factice_test

import (
	"encoding/hex"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/cles"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/routes"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// clearCall envoie une requête dont la réponse attendue est en clair.
func (c *ch) clearCall(route string, corps jcs.Object) *httptest.ResponseRecorder {
	c.n++
	return c.post(c.envelope(c.n, route, corps))
}

// Cas `session/*` et `lettre/secours` de routes-etat.md (API.md §2.10,
// §2.14 à §2.21) : un coffre de test, ses chartes servies par la passerelle
// de test.

// coffre est un titulaire enrôlé dont le test connaît la KTS, et les
// chartes que la passerelle de test sert (API.md §5.2 à §5.4).
type coffre struct {
	e         *env
	h         *titulaire
	kts       []byte
	ktsGen    int64
	sealedKts string
	proches   map[string]*proche
	refused   map[string]bool // bundle refusé (404)
}

// proche est un proche du coffre : sa charte en clair, son code d'accès et
// l'index que calculerait le navigateur (un index de test : Sésame ne le
// recalcule jamais, §2.14).
type proche struct {
	contact, code, lookup string
	gen                   int64
	charte                jcs.Object
	unlocked              bool
}

func newCoffre(t *testing.T, e *env, uid string) *coffre {
	k := &coffre{e: e, h: newTitulaire(uid), kts: random(32), ktsGen: 1, proches: map[string]*proche{}, refused: map[string]bool{}}
	c := e.open("a")
	o := ok(t, c.call("enroler", jcs.Object{"uid": uid, "idToken": e.idToken(uid), "ktsGeneration": 1, "kts": b64(k.kts), "cleViePub": b64(k.h.pub)}))
	k.sealedKts = str(o, "sealedKts")
	e.f.Gateway.Answer = k.serve
	return k
}

// add ajoute un proche : role, téléphone, email (nil ⇒ charte « courrier »).
func (k *coffre) add(contact, role string, phone, email any) *proche {
	code, _ := cles.DrawCode(random(12))
	p := &proche{contact: contact, code: code, lookup: hex.EncodeToString(random(32)), gen: 1, unlocked: true}
	p.charte = jcs.Object{"adresse": "Léa Martin\n1 rue de l'Exemple\n75001 Paris", "codeVerifier": cles.CodeVerifier(code), "contactId": contact,
		"courrier": phone == nil && email == nil, "creeLe": "2026-01-01T00:00:00.000Z", "dateNaissance": "1990-05-04", "email": email,
		"generation": 1, "nom": "Martin", "prenom": "Léa", "role": role, "telephone": phone, "titulaireAffiche": "Jean Dupont", "uid": k.h.uid, "v": 1}
	k.proches[contact] = p
	return p
}

// sealed scelle la charte de p sous KTS_auth à sa génération.
func (k *coffre) sealed(p *proche) (string, string) {
	p.charte["generation"] = p.gen
	plain, _ := jcs.Marshal(p.charte)
	auth, _ := cles.KTSAuth(k.kts, k.h.uid)
	aad, _ := cles.CharterAAD(k.h.uid, p.contact, p.gen, k.ktsGen)
	iv := random(12)
	ct, _ := primitives.Seal(auth, iv, plain, aad)
	return b64(ct), b64(iv)
}

func (k *coffre) bundle(p *proche) jcs.Object {
	ct, iv := k.sealed(p)
	return jcs.Object{"uid": k.h.uid, "contactId": p.contact, "charte": ct, "charteIv": iv, "charteGeneration": p.gen,
		"sealedKts": k.sealedKts, "ktsGeneration": k.ktsGen, "isUnlocked": p.unlocked, "titulaireAffiche": "Jean Dupont"}
}

// serve est la passerelle de test : GET /bundle aux trois formes, 200 {}
// aux notifications.
func (k *coffre) serve(r factice.Request) (int, []byte, error) {
	if !strings.HasPrefix(r.PathQuery, "/bundle?") {
		return 200, []byte("{}"), nil
	}
	q, _ := url.ParseQuery(r.PathQuery[len("/bundle?"):])
	var out jcs.Object
	for _, p := range k.proches {
		switch {
		case k.refused[p.contact]:
		case q.Get("lookup") == p.lookup, q.Get("contactId") == p.contact:
			out = k.bundle(p)
		}
	}
	if q.Get("courrier") == "1" {
		out = jcs.Object{"uid": k.h.uid, "sealedKts": k.sealedKts, "ktsGeneration": k.ktsGen, "titulaireAffiche": "Jean Dupont", "chartes": jcs.Array{}}
		for _, p := range k.proches {
			if p.charte["telephone"] == nil && p.charte["email"] == nil {
				b := k.bundle(p)
				out["chartes"] = append(out["chartes"].(jcs.Array), jcs.Object{"contactId": p.contact, "charte": b["charte"], "charteIv": b["charteIv"], "charteGeneration": p.gen})
			}
		}
	}
	if out == nil {
		return 404, []byte("{}"), nil
	}
	b, _ := jcs.Marshal(out)
	return 200, b, nil
}

// publish publie une nouvelle génération de la charte de p (charte/generation).
func (k *coffre) publish(t *testing.T, p *proche, gen int64) {
	c := k.e.open("a")
	ok(t, c.call("charte/generation", k.e.generation(c, k.h, p.contact, gen)))
	p.gen = gen
}

// transmit inscrit un signalement de source et le fait arriver à terme en
// récrivant ses dates (signé comme Sésame l'écrirait) plutôt qu'en avançant
// l'horloge : un saut de plusieurs heures sans borne déposée met, comme il se
// doit, le contrôle anti-retour en échec (ARCHITECTURE.md §6.3).
func (k *coffre) transmit(t *testing.T, source string) jcs.Object {
	if source == "inactivite" {
		line := k.e.line(k.h.uid)
		line["vivantLe"] = stampOf(k.e.now().Add(-months))
		k.e.put("titulaires/"+k.h.uid, line)
	}
	s := ok(t, k.e.signal(k.h.uid, source))
	path := "titulaires/" + k.h.uid + "/signalements/" + str(s, "signalementId")
	d := k.e.f.Base.Doc(path).Data
	d["signaleLe"], d["transmissibleLe"] = stampOf(k.e.now().Add(-8*24*time.Hour)), stampOf(k.e.now().Add(-time.Second))
	k.e.put(path, d)
	return s
}

func (c *ch) openCode(code, lookup string) jcs.Object {
	return c.call("session/ouvrir", jcs.Object{"code": code, "lookup": lookup})
}

func (c *ch) sid(route, id string, more ...any) jcs.Object {
	b := jcs.Object{"sessionId": id}
	for i := 0; i < len(more); i += 2 {
		b[more[i].(string)] = more[i+1]
	}
	return c.call(route, b)
}

// session ouvre une session pour p et la vérifie par le dernier code.
func (k *coffre) session(t *testing.T, p *proche) (*ch, string, jcs.Object) {
	c := k.e.open("b")
	o := ok(t, c.openCode(p.code, p.lookup))
	id := str(o, "sessionId")
	return c, id, ok(t, c.sid("session/verifier", id, "otp", k.otp(o)))
}

// otp rend le dernier code envoyé : 123456 par le double de Twilio Verify,
// le code du dernier email sinon.
func (k *coffre) otp(o jcs.Object) string {
	if o["canal"] == "sms" {
		return "123456"
	}
	sent := k.e.f.Outbox.Sent()
	text := sent[len(sent)-1].Text
	return text[len(text)-6:]
}

func TestOuvrirGardien(t *testing.T) { // SOU-01, SOU-11, SOU-17, SVE-01, SVE-04, SID-04, PAS-06, API.md §2.14, §2.15
	e := newEnv(t)
	k := newCoffre(t, e, "u-gardien")
	g := k.add("p-gardien", "gardien", "+33612345647", "lea@exemple.fr")
	m := k.add("p-mail", "proche", nil, "lea.martin@exemple.fr")
	c := e.open("b")
	o := ok(t, c.openCode(strings.ToLower(cles.DisplayCode(g.code)[:5])+" "+cles.DisplayCode(g.code)[5:], g.lookup))
	if o["canal"] != "sms" || o["masque"] != "+33 6 •• •• •• 47" || len(str(o, "sessionId")) != 43 || len(o) != 3 {
		t.Fatalf("session/ouvrir : %v", o)
	}
	if s := e.f.Outbox.Sent(); s[len(s)-1].Kind != "verify" || s[len(s)-1].To != "+33612345647" {
		t.Fatalf("dernier code : %v", s)
	}
	expect(t, c.sid("session/verifier", str(o, "sessionId"), "otp", "000000"), 403, "otp_refuse")
	v := ok(t, c.sid("session/verifier", str(o, "sessionId"), "otp", "123456"))
	tok, err := jetons.ReadSession(str(v, "jeton"), e.cfg.VersionList().SessionKeys, e.now())
	if err != nil || tok.Base != "vivant" || tok.Role != "gardien" || tok.Sub != g.contact || v["prenom"] != "Léa" || v["titulaireAffiche"] != "Jean Dupont" ||
		str(v, "expireLe") != "2026-11-11T11:00:00.000Z" {
		t.Fatalf("jeton : %v %v", v, err)
	}
	n := e.notes("ouvertureNotifiee")
	if len(n) != 1 || n[0]["role"] != "gardien" || n[0]["navigateur"] != nil || len(n[0]) != 5 {
		t.Fatalf("ouvertureNotifiee : %v", n)
	}
	expect(t, c.sid("session/identite/bascule", str(o, "sessionId")), 403, "bascule_impossible")
	// Proche à email seul avant l'échéance : refusé ; après : code par email.
	expect(t, e.open("b").openCode(m.code, m.lookup), 403, "code_refuse")
	k.transmit(t, "acte")
	c = e.open("b")
	o = ok(t, c.openCode(m.code, m.lookup))
	if o["canal"] != "email" || o["masque"] != "l•••••@exemple.fr" {
		t.Fatalf("email : %v", o)
	}
	code := k.otp(o)
	e.advance(5 * time.Minute)
	ok(t, c.sid("session/ouvrir", str(o, "sessionId")))
	if k.otp(o) != code {
		t.Fatalf("renvoi dans les 10 minutes : autre code")
	}
	e.advance(6 * time.Minute)
	expect(t, c.sid("session/verifier", str(o, "sessionId"), "otp", code), 403, "otp_refuse")
	ok(t, c.sid("session/ouvrir", str(o, "sessionId")))
	if v := ok(t, c.sid("session/verifier", str(o, "sessionId"), "otp", k.otp(o))); v["base"] != "deces" || v["role"] != "proche" {
		t.Fatalf("proche : %v", v)
	}
}

func TestOuvrirMasques(t *testing.T) { // SOU-01, API.md §2.14
	e := newEnv(t)
	k := newCoffre(t, e, "u-masque")
	p := k.add("p-be", "gardien", "+3225551234", nil) // hors +33 : 4 premiers caractères et 2 derniers chiffres
	if o := ok(t, e.open("b").openCode(p.code, p.lookup)); o["masque"] != "+322•••••34" {
		t.Fatalf("masque : %v", o["masque"])
	}
}

func TestOuvrirRefus(t *testing.T) { // SOU-07, SOU-08, SOU-10, SOU-12, SOU-13, SOU-14, SOU-16, SOU-18, API.md §2.14
	e := newEnv(t)
	k := newCoffre(t, e, "u-refus")
	g := k.add("p-g", "gardien", "+33600000001", nil)
	r := k.add("p-r", "proche", "+33600000002", nil)
	l := k.add("p-l", "proche", nil, nil)
	other := k.add("p-o", "gardien", "+33600000003", nil)
	cases := []struct {
		name          string
		code, lookup  string
		left          float64
		before, after func()
	}{
		{"code inconnu", g.code, hex.EncodeToString(random(32)), 4, nil, nil},
		{"lookup d'un autre code", other.code, g.lookup, 4, nil, nil},
		{"proche avant l'échéance", r.code, r.lookup, 4, nil, nil},
		{"charte courrier par lookup", l.code, l.lookup, 4, nil, nil},
		{"isUnlocked faux", g.code, g.lookup, 3, func() { g.unlocked = false }, func() { g.unlocked = true }},
		{"bundle refusé", g.code, g.lookup, 4, func() { k.refused[g.contact] = true }, func() { k.refused[g.contact] = false }},
	}
	for _, tc := range cases {
		if tc.before != nil {
			tc.before()
		}
		res := e.open("b").openCode(tc.code, tc.lookup)
		expect(t, res, 403, "code_refuse")
		if sub(res, "details")["essaisRestants"] != tc.left {
			t.Fatalf("%s : %v", tc.name, res)
		}
		if tc.after != nil {
			tc.after()
		}
	}
	if d := e.line(k.h.uid); d == nil || e.f.Base.Doc("titulaires/" + k.h.uid + "/essais/" + r.contact).Data["essaisCode"] != 1.0 {
		t.Fatalf("essais du proche : %v", e.f.Base.Doc("titulaires/"+k.h.uid+"/essais/"+r.contact).Data)
	}
	for _, m := range e.f.Outbox.Sent() {
		if m.To == "+33600000002" {
			t.Fatalf("dernier code envoyé pour un code refusé")
		}
	}
	// Passerelle muette ⇒ passerelle_indisponible ; Twilio en échec.
	e.f.Gateway.Answer = func(factice.Request) (int, []byte, error) { return 500, nil, nil }
	expect(t, e.open("b").openCode(g.code, g.lookup), 503, "passerelle_indisponible")
	e.f.Gateway.Answer = k.serve
	e.f.Outbox.Set("Sms", true)
	expect(t, e.open("b").openCode(other.code, other.lookup), 503, "fournisseur_indisponible")
	e.f.Outbox.Set("Sms", false)
	// Écriture d'un code faux refusée ⇒ registre_indisponible, jamais code_refuse.
	e.f.Base.Set("Commit", true)
	expect(t, e.open("b").openCode(other.code, g.lookup), 503, "registre_indisponible")
	e.f.Base.Set("Commit", false)
}

func TestOuvrirEssais(t *testing.T) { // SOU-12, SOU-13, SOU-15, API.md §2.14, §7
	e := newEnv(t)
	k := newCoffre(t, e, "u-essais")
	g := k.add("p-g", "gardien", "+33600000001", nil)
	t0 := e.now()
	for i := range 5 {
		res := e.open("b").openCode(k.add("x", "gardien", "+33600000009", nil).code, g.lookup)
		expect(t, res, 403, "code_refuse")
		if sub(res, "details")["essaisRestants"] != float64(4-i) {
			t.Fatalf("essai %d : %v", i+1, res)
		}
		delete(k.proches, "x")
		if i < 4 {
			e.f.Clock.Advance(14 * time.Minute / 4)
		}
	}
	doc := e.f.Base.Doc("titulaires/" + k.h.uid + "/essais/" + g.contact).Data
	if doc["codeBloqueJusquau"] != stampOf(t0.Add(29*time.Minute)) {
		t.Fatalf("blocage : %v", doc)
	}
	e.f.Clock.Set(t0.Add(20 * time.Minute))
	res := e.open("b").openCode(g.code, g.lookup)
	expect(t, res, 429, "trop_d_essais")
	if sub(res, "details")["reessayerLe"] != stampOf(t0.Add(29*time.Minute)) {
		t.Fatalf("reessayerLe : %v", res)
	}
	e.f.Clock.Set(t0.Add(29 * time.Minute))
	ok(t, e.open("b").openCode(g.code, g.lookup))
	e.f.Clock.Set(t0.Add(30 * time.Minute))
	if res := e.open("b").openCode(k.add("y", "gardien", "+33600000009", nil).code, g.lookup); sub(res, "details")["essaisRestants"] != 4.0 {
		t.Fatalf("nouvelle fenêtre : %v", res)
	}
	delete(k.proches, "y")
	// Par IP : 5 codes faux d'une même IP, puis blocage de 15 min.
	for range 5 {
		c := e.open("b")
		c.ip = "10.7.7.7"
		expect(t, c.openCode(g.code, hex.EncodeToString(random(32))), 403, "code_refuse")
	}
	c := e.open("b")
	c.ip = "10.7.7.7"
	expect(t, c.openCode(g.code, g.lookup), 429, "trop_d_essais")
	// Document d'essais recopié (ecritLe ancien) : refusé sans vérification.
	e.reject("titulaires/" + k.h.uid + "/essais/" + g.contact)
	expect(t, e.open("b").openCode(g.code, g.lookup), 403, "code_refuse")
}

func stampOf(t time.Time) string { return t.UTC().Format(primitives.TimeLayout) }

func TestSessionsSimultanees(t *testing.T) { // SOU-12, SOU-02, SOU-19, SID-06, API.md §2.14, §7
	e := newEnv(t)
	k := newCoffre(t, e, "u-simul")
	g := k.add("p-g", "gardien", "+33600000001", nil)
	for range 3 {
		ok(t, e.open("b").openCode(g.code, g.lookup))
	}
	expect(t, e.open("b").openCode(g.code, g.lookup), 429, "trop_d_appels")
	// VM redémarrée : la session n'existe plus.
	c := e.open("b")
	if err := e.f.Restart(t.Context(), "b"); err != nil {
		t.Fatal(err)
	}
	clearErr(t, c.post(c.envelope(1, "session/verifier", jcs.Object{"sessionId": strings.Repeat("A", 43), "otp": "123456"})), 410, "instance_inconnue")
	expect(t, e.open("b").sid("session/verifier", strings.Repeat("A", 43), "otp", "123456"), 410, "session_expiree")
	ok(t, e.open("b").openCode(g.code, g.lookup))
}

func TestSessionLieeAuCanal(t *testing.T) { // SOU-20, SVE-05, SVE-06, SVE-08, SVE-09, SFE-01, SFE-02, SFE-04, API.md §2.15, §2.20
	e := newEnv(t)
	k := newCoffre(t, e, "u-canal")
	g := k.add("p-g", "gardien", "+33600000001", nil)
	c := e.open("b")
	id := str(ok(t, c.openCode(g.code, g.lookup)), "sessionId")
	clearErr(t, e.open("b").clearCall("session/verifier", jcs.Object{"sessionId": id, "otp": "123456"}), 400, "canal_invalide")
	for range 3 {
		ok(t, c.sid("session/ouvrir", id))
	}
	res := c.sid("session/ouvrir", id)
	expect(t, res, 429, "trop_d_essais")
	if sub(res, "details")["reessayerLe"] != stampOf(e.now().Add(time.Hour)) {
		t.Fatalf("4ᵉ renvoi : %v", res)
	}
	for range 20 {
		c.sid("session/verifier", id, "otp", "000000")
	}
	expect(t, c.sid("session/verifier", id, "otp", "000000"), 429, "trop_d_appels")
	ok(t, c.sid("session/fermer", id))
	n := e.notes("sessionTerminee")
	if len(n) != 1 || n[0]["ouvertLe"] != nil || n[0]["contactId"] != g.contact {
		t.Fatalf("sessionTerminee : %v", n)
	}
	ok(t, c.sid("session/fermer", id))
	if len(e.notes("sessionTerminee")) != 1 {
		t.Fatalf("seconde notification")
	}
	expect(t, c.sid("session/verifier", id, "otp", "123456"), 410, "session_expiree")
	// Expiration après 1 h sans appel.
	id = str(ok(t, c.openCode(g.code, g.lookup)), "sessionId")
	e.f.Clock.Advance(time.Hour)
	expect(t, c.sid("session/verifier", id, "otp", "123456"), 410, "session_expiree")
	if len(e.notes("sessionTerminee")) != 2 {
		t.Fatalf("expiration non notifiée")
	}
	for range 10 {
		c.sid("session/fermer", id)
	}
	expect(t, c.sid("session/fermer", id), 429, "trop_d_appels")
}

func TestOuvrirCles(t *testing.T) { // SOC-01, SOC-02, SOC-03, SOC-04, SOC-05, SOC-06, SOC-08, SOC-10, SFE-01, API.md §2.19
	e := newEnv(t)
	k := newCoffre(t, e, "u-cles")
	g := k.add("p-g", "gardien", "+33600000001", nil)
	o := k.add("p-o", "proche", "+33600000002", nil)
	c, id, _ := k.session(t, g)
	env := func(typ, content, contact string, kts []byte) jcs.Object {
		key, _ := cles.KTSEnv(kts, k.h.uid)
		d := cles.Envelope{UID: k.h.uid, Type: typ, ContentID: content, ContactID: contact, KTSGeneration: 1}
		aad, _ := d.AAD()
		iv := random(12)
		ct, _ := primitives.Seal(key, iv, []byte(strings.Repeat("k", 32)), aad)
		return jcs.Object{"type": typ, "id": content, "contactId": contact, "enveloppe": b64(ct), "iv": b64(iv)}
	}
	fid := strings.Repeat("A", 22)
	r := ok(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{env("fichier", fid, g.contact, k.kts), env("volontes", "courant", g.contact, k.kts),
		env("mot", o.contact, o.contact, k.kts), env("motdepasse", fid, g.contact, random(32))}))
	if len(r["cles"].(jcs.Array)) != 2 || len(r["refusees"].(jcs.Array)) != 2 || str(r, "jeton") == "" {
		t.Fatalf("ouvrir-cles : %v", r)
	}
	e.f.Clock.Advance(30 * time.Minute)
	if r := ok(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{})); str(r, "expireLe") != stampOf(e.now().Add(time.Hour)) {
		t.Fatalf("jeton renouvelé : %v", r)
	}
	big := jcs.Array{}
	for range 501 {
		big = append(big, env("fichier", fid, g.contact, k.kts))
	}
	expect(t, c.raw("session/ouvrir-cles", jcs.Object{"sessionId": id, "enveloppes": big}), 400, "message_refuse")
	if d := e.f.Base.Doc("titulaires/" + k.h.uid); len(d.Data) == 0 {
		t.Fatal("ligne absente")
	}
	// Nouvelle génération de la charte sur une autre VM : session_expiree.
	k.publish(t, g, 2)
	expect(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{}), 410, "session_expiree")
	if n := e.notes("sessionTerminee"); len(n) != 1 || n[0]["ouvertLe"] == nil {
		t.Fatalf("sessionTerminee : %v", n)
	}
}

func TestFermeturesParLeTitulaire(t *testing.T) { // ENR-02, VIE-04, CHG-03, CHG-07, SOC-07, SOC-08, SOC-09, API.md §2.3, §2.4, §2.6, §2.19
	e := newEnv(t)
	k := newCoffre(t, e, "u-ferme")
	g := k.add("p-g", "gardien", "+33600000001", nil)
	// Génération reçue par la VM qui tient la session : fermée aussitôt.
	_, _, _ = k.session(t, g)
	c := e.open("b")
	ok(t, c.call("charte/generation", e.generation(c, k.h, g.contact, 2)))
	g.gen = 2
	if n := e.notes("sessionTerminee"); len(n) != 1 {
		t.Fatalf("génération : %v", n)
	}
	g.gen = 1 // passerelle qui sert une charte d'une génération antérieure
	expect(t, e.open("b").openCode(g.code, g.lookup), 403, "code_refuse")
	g.gen = 2
	// Rotation de la KTS : sessions du titulaire fermées.
	_, _, _ = k.session(t, g)
	c = e.open("b")
	ok(t, c.call("enroler", c.enrolBody(k.h, 2, k.h.pub, k.h)))
	if len(e.notes("sessionTerminee")) != 2 {
		t.Fatalf("rotation non notifiée")
	}
	// Ligne supprimée : session/verifier ⇒ session_expiree, rien d'inscrit.
	k2 := newCoffre(t, e, "u-ferme-2")
	p := k2.add("p", "gardien", "+33600000001", nil)
	c = e.open("b")
	id := str(ok(t, c.openCode(p.code, p.lookup)), "sessionId")
	ok(t, e.open("a").call("titulaire/supprimer", jcs.Object{"jetonFatoom": e.oidc(e.cfg.EnvOverride["COMPTE_DECES"]), "uid": k2.h.uid}))
	expect(t, c.sid("session/verifier", id, "otp", "123456"), 410, "session_expiree")
	expect(t, e.open("b").openCode(p.code, p.lookup), 403, "code_refuse")
}

func TestRetourDuTitulaire(t *testing.T) { // VIE-04, SOC-08, SOU-05, API.md §2.4, §2.19
	e := newEnv(t)
	k := newCoffre(t, e, "u-retour")
	p := k.add("p", "proche", "+33600000001", nil)
	k.transmit(t, "inactivite")
	c, id, v := k.session(t, p)
	if v["base"] != "inactivite" {
		t.Fatalf("base : %v", v)
	}
	a := e.open("a")
	d := a.defi(k.h.uid)
	ok(t, a.call("vie", jcs.Object{"uid": k.h.uid, "defi": d, "signature": k.h.sign(vie.LifeMessage(k.h.uid, d))}))
	if n := e.notes("vieApresTransmission"); len(n) != 1 {
		t.Fatalf("vieApresTransmission : %v", n)
	}
	expect(t, c.sid("session/ouvrir-cles", id, "enveloppes", jcs.Array{}), 410, "session_expiree")
	expect(t, e.open("b").openCode(p.code, p.lookup), 403, "code_refuse")
}

func TestRefuserRole(t *testing.T) { // SRR-01, SRR-02, SRR-03, SRR-04, SOU-18, CHG-05, API.md §2.21
	e := newEnv(t)
	k := newCoffre(t, e, "u-role")
	g := k.add("p-g", "gardien", "+33600000001", nil)
	p := k.add("p-p", "proche", "+33600000002", nil)
	c, id, _ := k.session(t, g)
	if r := ok(t, c.sid("session/refuser-role", id)); r["refuse"] != true {
		t.Fatalf("refuser-role : %v", r)
	}
	if rr := sub(sub(e.line(k.h.uid), "rolesRefuses"), g.contact); rr["generation"] != 1.0 || len(e.notes("roleRefuse")) != 1 || len(e.notes("sessionTerminee")) != 1 {
		t.Fatalf("rolesRefuses : %v", e.line(k.h.uid))
	}
	expect(t, e.open("b").openCode(g.code, g.lookup), 403, "code_refuse")
	k.transmit(t, "acte")
	_, _, v := k.session(t, g)
	if v["role"] != "proche" || v["base"] != "deces" {
		t.Fatalf("après le refus : %v", v)
	}
	c, id, _ = k.session(t, p)
	expect(t, c.sid("session/refuser-role", id), 403, "role_insuffisant")
	k.publish(t, g, 2)
	if _, _, v := k.session(t, g); v["role"] != "gardien" {
		t.Fatalf("nouvelle génération : %v", v)
	}
}

func TestIdentite(t *testing.T) { // SID-01, SID-03, SID-04, SID-05, SID-06, SID-07, SID-08, API.md §2.16 à §2.18
	e := newEnv(t)
	k := newCoffre(t, e, "u-identite")
	p := k.add("p", "proche", "+33600000001", nil)
	k.transmit(t, "insee")
	for i := range 3 { // trois échecs répartis sur trois sessions
		c := e.open("b")
		id := str(ok(t, c.openCode(p.code, p.lookup)), "sessionId")
		if i == 0 {
			expect(t, c.sid("session/identite/demarrer", id), 400, "message_refuse")
		}
		if r := ok(t, c.sid("session/identite/bascule", id)); r["canal"] != "identite" {
			t.Fatalf("bascule : %v", r)
		}
		if r := ok(t, c.sid("session/identite/demarrer", id)); !strings.Contains(str(r, "clientSecret"), "_secret_") {
			t.Fatalf("demarrer : %v", r)
		}
		if i == 0 {
			expect(t, c.sid("session/identite/verifier", id), 409, "identite_en_attente")
			clearErr(t, e.open("b").clearCall("session/identite/demarrer", jcs.Object{"sessionId": id}), 400, "canal_invalide")
		}
		e.f.Identity.Verdict(ports.Verification{Status: "verified", FirstName: "Paul", LastName: "Martin", BirthDate: "1990-05-04"})
		if i < 2 {
			expect(t, c.sid("session/identite/verifier", id), 403, "identite_non_confirmee")
		} else {
			res := c.sid("session/identite/verifier", id)
			expect(t, res, 423, "identite_bloquee")
			if sub(res, "details")["reessayerLe"] != stampOf(e.now().Add(24*time.Hour)) {
				t.Fatalf("blocage : %v", res)
			}
		}
		ok(t, c.sid("session/fermer", id))
	}
	if n := e.notes("identiteEchouee"); len(n) != 3 || n[2]["essaisIdentite"] != 3.0 || len(e.f.Identity.Redacted) != 3 {
		t.Fatalf("identiteEchouee : %v, redact %v", n, e.f.Identity.Redacted)
	}
	c := e.open("b")
	id := str(ok(t, c.openCode(p.code, p.lookup)), "sessionId")
	ok(t, c.sid("session/identite/bascule", id))
	expect(t, c.sid("session/identite/demarrer", id), 423, "identite_bloquee")
	ok(t, c.sid("session/fermer", id))
	e.advance(24 * time.Hour)
	c = e.open("b")
	id = str(ok(t, c.openCode(p.code, p.lookup)), "sessionId")
	ok(t, c.sid("session/identite/bascule", id))
	ok(t, c.sid("session/identite/demarrer", id))
	e.f.Identity.Verdict(ports.Verification{Status: "verified", FirstName: "LÉA MARIE", LastName: "MARTIN", BirthDate: "1990-05-04"})
	e.f.Identity.Set("Redact", true)
	if v := ok(t, c.sid("session/identite/verifier", id)); v["base"] != "deces" || v["role"] != "proche" {
		t.Fatalf("identité vérifiée : %v", v)
	}
	d := e.f.Base.Doc("titulaires/" + k.h.uid + "/identites/" + p.contact).Data
	if d["essaisIdentite"] != 0.0 || len(d["redactionsEnAttente"].([]any)) != 1 {
		t.Fatalf("identites : %v", d)
	}
	// Une nouvelle session par le code d'accès revient au dernier code (LIM-21).
	if o := ok(t, e.open("b").openCode(p.code, p.lookup)); o["canal"] != "sms" {
		t.Fatalf("après une bascule : %v", o)
	}
}

func TestNamesMatch(t *testing.T) { // SID-02, docs/technique/sesame/protocoles.md « Ouverture »
	v := func(first, last string) bool {
		return routes.NamesMatch("Jean-Marie", "Œuvray-D'Aubigné", "1990-05-04", ports.Verification{Status: "verified", FirstName: first, LastName: last, BirthDate: "1990-05-04"})
	}
	if !v("JEAN MARIE PAUL", "OEUVRAY D AUBIGNE") || v("JEAN PAUL MARIE", "OEUVRAY D AUBIGNE") || !routes.NamesMatch("Marie Paul", "Müller", "1990-05-04",
		ports.Verification{Status: "verified", FirstName: "JEAN MARIE PAUL", LastName: "MULLER", BirthDate: "1990-05-04"}) || routes.NamesMatch("Jean Paul", "Müller", "1990-05-04",
		ports.Verification{Status: "verified", FirstName: "JEAN MARIE PAUL", LastName: "MULLER", BirthDate: "1990-05-04"}) {
		t.Fatal("normalisation des noms")
	}
	if !cles.EmailNormalized("lea.martin@exemple.fr") || cles.EmailNormalized("Lea.Martin@Exemple.FR") {
		t.Fatal("email normalisé")
	}
}
