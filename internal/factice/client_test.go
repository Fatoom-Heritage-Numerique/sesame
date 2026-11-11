package factice_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/canal"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// t0 est l'heure fixe des tests.
var t0 = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

var b64 = base64.StdEncoding.EncodeToString

// env est un sesame-fake de test, sur la configuration de testdata/factice.
type env struct {
	ips     int
	t       *testing.T
	f       *factice.Fake
	cfg     *factice.Config
	issuers map[string]*rsa.PrivateKey
	remote  map[string]string // conteneurs de sesame-fake joints en HTTP, par préfixe (conteneurs_test.go)
	target  string
}

type option func(*factice.Options)

func withReprise(r *registre.Reprise) option { return func(o *factice.Options) { o.Reprise = r } }

// instanceMode : sesame-fake en mode instance, préfixe p.
var instanceMode string

func newEnv(t *testing.T, opts ...option) *env {
	t.Helper()
	t.Setenv(factice.LauncherEnv, "1")
	cfg, err := factice.ReadConfig("../../testdata/factice/configuration.json")
	if err != nil {
		t.Fatal(err)
	}
	if instanceMode != "" {
		cfg.Mode, cfg.Prefixe = "instance", instanceMode
	}
	clock := &factice.Clock{}
	clock.Set(t0)
	o := factice.Options{Clock: clock, BloomBytes: 1 << 12, Reprise: &registre.Reprise{SansSauvegarde: true}}
	for _, opt := range opts {
		opt(&o)
	}
	f, err := factice.New(context.Background(), cfg, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Close)
	e := &env{t: t, f: f, cfg: cfg, issuers: map[string]*rsa.PrivateKey{}}
	var priv struct{ Emetteurs map[string]map[string]string }
	b, err := os.ReadFile("../../testdata/factice/emetteurs-prives.json")
	if err == nil {
		err = json.Unmarshal(b, &priv)
	}
	if err != nil {
		t.Fatal(err)
	}
	for iss, keys := range priv.Emetteurs {
		for _, s := range keys {
			der, _ := base64.StdEncoding.DecodeString(s)
			k, err := x509.ParsePKCS8PrivateKey(der)
			if err != nil {
				t.Fatal(err)
			}
			e.issuers[iss] = k.(*rsa.PrivateKey)
		}
	}
	return e
}

func (e *env) now() time.Time { return e.f.Clock.Now() }

// do envoie une requête HTTP au serveur, depuis l'IP ip (X-Forwarded-For).
func (e *env) do(method, path, ip string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("X-Forwarded-For", ip+", 10.0.0.1")
	w := httptest.NewRecorder()
	if e.remote == nil {
		e.f.Server.ServeHTTP(w, r)
		return w
	}
	base := e.remote[e.target]
	if p, ok := strings.CutPrefix(path, "/n/"); ok {
		base = e.remote[p[:1]]
	}
	req, _ := http.NewRequest(method, base+path, bytes.NewReader(body))
	req.Header = r.Header
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	return w
}

func decode(t *testing.T, b []byte) jcs.Object {
	t.Helper()
	v, err := jcs.Parse(b)
	o, _ := v.(jcs.Object)
	if err != nil || o == nil {
		t.Fatalf("réponse illisible : %q", b)
	}
	return o
}

// jwt signe des claims RS256 par la clé de test de l'émetteur iss.
func (e *env) jwt(iss string, claims jcs.Object) string {
	h, _ := jcs.Marshal(jcs.Object{"alg": "RS256", "kid": iss + "-test-1", "typ": "JWT"})
	p, _ := jcs.Marshal(claims)
	in := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(p)
	sum := sha256.Sum256([]byte(in))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, e.issuers[iss], crypto.SHA256, sum[:])
	return in + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (e *env) idToken(uid string) string {
	p := e.cfg.EnvOverride["IDTOKEN_PROJET"]
	return e.jwt("idToken", jcs.Object{"iss": "https://securetoken.google.com/" + p, "aud": p, "sub": uid, "exp": e.now().Unix() + 3600})
}

func (e *env) oidc(email string) string {
	return e.jwt("google", jcs.Object{"iss": "https://accounts.google.com", "aud": e.cfg.EnvOverride["SESAME_AUDIENCE"],
		"email": email, "email_verified": true, "exp": e.now().Unix() + 3600})
}

// ch est un canal ouvert par le client de test vers une instance.
type ch struct {
	e                   *env
	id                  string
	sessionPub, kemPub  []byte
	cliPub, kemCt, pref []byte
	kReq, kRes          []byte
	n                   int64
	ip                  string
}

// attest appelle GET /attestation jusqu'à joindre l'instance de préfixe p
// (vide : la première) et vérifie le jeton comme un client.
func (e *env) attest(p string) jcs.Object {
	e.t.Helper()
	if e.target = p; p == "" {
		e.target = "a"
	}
	for range 6 {
		e.ips++
		w := e.do("GET", "/attestation", "10.9."+itoa(int64(e.ips/250))+"."+itoa(int64(e.ips%250)), nil)
		if w.Code != 200 {
			e.t.Fatalf("GET /attestation : %d %s", w.Code, w.Body)
		}
		o := decode(e.t, w.Body.Bytes())
		if p != "" && str(o, "instanceId")[:1] != p {
			continue
		}
		sp, _ := primitives.B64(str(o, "sessionPub"))
		kp, _ := primitives.B64(str(o, "kemPub"))
		c, err := attestation.Verify(str(o, "jwt"), e.cfg.RootSum(), e.now(), attestation.ChannelNonces(sp, kp))
		if err == nil {
			_, err = attestation.CheckList(c, e.cfg.VersionList(), e.cfg.Projet, e.cfg.RegistreImages)
		}
		if err != nil {
			e.t.Fatalf("jeton refusé par le client : %v", err)
		}
		return o
	}
	e.t.Fatalf("instance %s introuvable", p)
	return nil
}

// open ouvre un canal neuf vers l'instance de préfixe p.
func (e *env) open(p string) *ch {
	e.t.Helper()
	o := e.attest(p)
	c := &ch{e: e, id: str(o, "instanceId"), ip: "10.8." + itoa(int64(e.ips/250)) + "." + itoa(int64(e.ips%250))}
	c.sessionPub, _ = primitives.B64(str(o, "sessionPub"))
	c.kemPub, _ = primitives.B64(str(o, "kemPub"))
	priv := make([]byte, 32)
	_, _ = rand.Read(priv)
	c.cliPub, _ = primitives.X25519Public(priv)
	shared, err := primitives.X25519(priv, c.sessionPub)
	if err != nil {
		e.t.Fatal(err)
	}
	ek, err := mlkem.NewEncapsulationKey768(c.kemPub)
	if err != nil {
		e.t.Fatal(err)
	}
	ss, ct := ek.Encapsulate()
	c.kemCt = ct
	tr, _ := canal.Transcript(c.sessionPub, c.cliPub, c.kemPub, c.kemCt)
	c.kReq, c.kRes, _ = canal.DeriveKeys(ss, shared, tr)
	c.pref = make([]byte, 4)
	_, _ = rand.Read(c.pref)
	return c
}

// envelope chiffre {route, corps} pour la requête n.
func (c *ch) envelope(n int64, route string, corps jcs.Object) []byte {
	plain, err := jcs.Marshal(jcs.Object{"route": route, "corps": corps})
	if err != nil {
		c.e.t.Fatal(err)
	}
	return c.sealed(n, plain)
}

func (c *ch) sealed(n int64, plain []byte) []byte {
	iv, _ := canal.IV(c.pref, n)
	aad, _ := canal.AAD(true, n)
	body, _ := primitives.Seal(c.kReq, iv, plain, aad)
	b, _ := jcs.Marshal(jcs.Object{"instanceId": c.id, "cliPub": b64(c.cliPub), "kemCt": b64(c.kemCt), "ivPrefixe": b64(c.pref), "n": n, "body": b64(body)})
	return b
}

func (c *ch) post(env []byte) *httptest.ResponseRecorder {
	return c.e.do("POST", "/n/"+c.id[:1]+"/canal", c.ip, env)
}

// open ouvre une réponse chiffrée du canal pour la requête n.
func (c *ch) openResponse(n int64, w *httptest.ResponseRecorder) jcs.Object {
	c.e.t.Helper()
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		c.e.t.Fatalf("réponse du canal : %d %s", w.Code, w.Body)
	}
	o := decode(c.e.t, w.Body.Bytes())
	body, err := primitives.B64(str(o, "body"))
	if len(o) != 2 || o["n"] != float64(n) || err != nil {
		c.e.t.Fatalf("enveloppe de réponse hors forme : %s", w.Body)
	}
	iv, _ := canal.IV(c.pref, n)
	aad, _ := canal.AAD(false, n)
	plain, err := primitives.Open(c.kRes, iv, body, aad)
	if err != nil {
		c.e.t.Fatalf("réponse indéchiffrable sous K_res")
	}
	v, err := jcs.ReadCanonical(plain)
	r, _ := v.(jcs.Object)
	if err != nil || r == nil {
		c.e.t.Fatalf("réponse hors JSON canonique")
	}
	checkStatus(c.e.t, r)
	return r
}

// raw appelle une route sans valider la requête contre l'OpenAPI (corps
// hors contrat).
func (c *ch) raw(route string, corps jcs.Object) jcs.Object {
	c.e.t.Helper()
	c.n++
	return c.openResponse(c.n, c.post(c.envelope(c.n, route, corps)))
}

// call appelle une route : la requête et la réponse sont validées contre
// l'OpenAPI (openapi_test.go).
func (c *ch) call(route string, corps jcs.Object) jcs.Object {
	c.e.t.Helper()
	c.n++
	validateRequest(c.e.t, route, corps)
	r := c.openResponse(c.n, c.post(c.envelope(c.n, route, corps)))
	validateResponse(c.e.t, route, r)
	return r
}

// checkStatus vérifie la forme du statut chiffré (API.md §1.3).
func checkStatus(t *testing.T, r jcs.Object) {
	t.Helper()
	st, _ := r["statut"].(float64)
	_, hasCorps := r["corps"]
	_, hasErr := r["erreur"]
	_, hasMsg := r["message"]
	_, hasDet := r["details"]
	n := 1 + map[bool]int{true: 1}[hasCorps] + map[bool]int{true: 1}[hasErr] + map[bool]int{true: 1}[hasMsg] + map[bool]int{true: 1}[hasDet]
	if st == 200 && (!hasCorps || hasErr || hasDet || n != 2) || st != 200 && (hasCorps || !hasErr || !hasMsg || n != len(r)) || len(r) != n {
		t.Fatalf("statut chiffré hors forme : %v", r)
	}
}

func str(o jcs.Object, k string) string { s, _ := o[k].(string); return s }

func sub(o jcs.Object, k string) jcs.Object { m, _ := o[k].(jcs.Object); return m }

// expect vérifie le statut et le code d'une réponse chiffrée.
func expect(t *testing.T, r jcs.Object, statut int, code string) {
	t.Helper()
	if r["statut"] != float64(statut) || str(r, "erreur") != code {
		t.Fatalf("attendu %d %s, obtenu %v", statut, code, r)
	}
}

func ok(t *testing.T, r jcs.Object) jcs.Object {
	t.Helper()
	expect(t, r, 200, "")
	return sub(r, "corps")
}

// clearErr vérifie une erreur en clair.
func clearErr(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status || !strings.Contains(w.Body.String(), `"erreur":"`+code+`"`) || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("attendu %d %s en clair, obtenu %d %s", status, code, w.Code, w.Body)
	}
}

// titulaire est un titulaire de test : son uid et sa clé de vie.
type titulaire struct {
	uid  string
	seed []byte
	pub  []byte
}

func newTitulaire(uid string) *titulaire {
	seed := make([]byte, 32)
	_, _ = rand.Read(seed)
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	return &titulaire{uid, seed, pub}
}

func (h *titulaire) sign(msg []byte, err error) string {
	if err != nil {
		panic(err)
	}
	return b64(ed25519.Sign(ed25519.NewKeyFromSeed(h.seed), msg))
}

func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 2*len(b))
	for i, c := range b {
		out[2*i], out[2*i+1] = digits[c>>4], digits[c&15]
	}
	return string(out)
}

// signal inscrit un signalement par deces/signaler sur un canal neuf.
func (e *env) signal(uid, source string) jcs.Object {
	c := e.open("b")
	b := jcs.Object{"jetonFatoom": e.oidc(e.cfg.EnvOverride["COMPTE_DECES"]), "uid": uid, "source": source}
	if source != "inactivite" {
		b["dateDeces"], b["preuve"] = "2026-11-01", hexOf(random(32))
	}
	if source == "insee" {
		b["preuveNom"] = "deces-2026-m10.txt"
	}
	return c.call("deces/signaler", b)
}

// generation compose un corps de charte/generation signé par h.
func (e *env) generation(c *ch, h *titulaire, contact string, gen int64) jcs.Object {
	return jcs.Object{"uid": h.uid, "contactId": contact, "generation": gen, "signature": h.sign(vie.GenerationMessage(h.uid, contact, gen))}
}

// advance avance l'horloge et fait passer les tâches de fond (jeton du
// lanceur renouvelé, canaux inactifs fermés, borne déposée).
func (e *env) advance(d time.Duration) {
	e.f.Clock.Advance(d)
	tick(e)
}

// put écrit un document signé comme Sésame l'écrirait, hors de la chaîne
// (pour préparer un état : ouverture, plafond du PIN).
func (e *env) put(path string, doc jcs.Object) {
	doc["ecritLe"] = e.now().Format(primitives.TimeLayout)
	delete(doc, "sig")
	msg, _ := registre.LineMessage(doc)
	der, _ := e.f.KMS.Sign(context.Background(), "registre-sign", msg)
	raw, _ := jetons.DERToRaw(der)
	doc["sig"] = base64.RawURLEncoding.EncodeToString(raw)
	e.f.Base.Tamper(path, doc, e.now())
}

// notes rend les corps des notifications name reçues par la passerelle de
// test, après avoir vérifié leurs en-têtes et leur signature (API.md §5.1).
func (e *env) notes(name string) []jcs.Object {
	e.t.Helper()
	var out []jcs.Object
	for _, r := range e.f.Gateway.Requests() {
		if r.PathQuery != "/"+name {
			continue
		}
		h := r.Headers
		msg, err := jetons.GatewayMessage(r.Method, r.PathQuery, h["X-Sesame-Date"], h["X-Sesame-Request-Id"], r.Body)
		sig, _ := primitives.B64(h["X-Sesame-Signature"])
		key := e.cfg.VersionList().SessionKeys[e.cfg.SessionSign.Kid]
		if valid, _ := primitives.P256Verify(key, msg, sig); err != nil || !valid || h["X-Sesame-Attestation"] == "" || h["X-Sesame-Session-Pub"] == "" || h["X-Sesame-Kem-Pub"] == "" {
			e.t.Fatalf("requête à la passerelle hors contrat : %v", h)
		}
		out = append(out, decode(e.t, r.Body))
	}
	return out
}
