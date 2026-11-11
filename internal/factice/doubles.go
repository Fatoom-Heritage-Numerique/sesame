package factice

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
)

// errDouble est la panne simulée d'un double.
var errDouble = errors.New("panne_simulee")

// Clock est l'horloge : réelle, ou fixée par Set et avancée par Advance.
type Clock struct {
	mu    sync.Mutex
	fixed time.Time
}

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fixed.IsZero() {
		return time.Now().UTC().Truncate(time.Millisecond)
	}
	return c.fixed
}

// Set fixe l'heure ; Advance l'avance.
func (c *Clock) Set(t time.Time) { c.mu.Lock(); c.fixed = t.UTC(); c.mu.Unlock() }
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.fixed = c.fixed.Add(d)
	c.mu.Unlock()
}

// Rand est la primitive d'aléa (crypto/rand).
type Rand struct{}

func (Rand) Read(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

// Faults est un jeu de pannes simulées, par nom d'opération.
type Faults struct {
	mu sync.Mutex
	on map[string]bool
}

// Set active ou retire la panne de l'opération op.
func (f *Faults) Set(op string, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.on == nil {
		f.on = map[string]bool{}
	}
	f.on[op] = on
}

func (f *Faults) fail(op string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.on[op]
}

// KMS tient en mémoire une clé AES-256-GCM par clé symétrique, la clé
// session-sign de la configuration et une clé registre-sign tirée au
// démarrage ; une version par clé. Pannes : "Encrypt", "Decrypt", "Sign".
type KMS struct {
	Faults
	mu   sync.Mutex
	sym  map[string]cipher.AEAD
	sign map[string]*ecdsa.PrivateKey
	kids map[string]string
}

// NewKMS rend le double de KMS ; sessionKey et sessionKid viennent de la
// configuration.
func NewKMS(sessionKey *ecdsa.PrivateKey, sessionKid string) *KMS {
	k := &KMS{sym: map[string]cipher.AEAD{}, sign: map[string]*ecdsa.PrivateKey{ports.SessionSign: sessionKey},
		kids: map[string]string{ports.SessionSign: sessionKid, ports.RegistreSign: "registre-sign/1"}}
	reg, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	k.sign[ports.RegistreSign] = reg
	for _, n := range []string{ports.KtsSeal, ports.AlerteSeal, ports.InvitationSeal} {
		key := make([]byte, 32)
		_, _ = rand.Read(key)
		block, _ := aes.NewCipher(key)
		k.sym[n], _ = cipher.NewGCM(block)
	}
	return k
}

func (k *KMS) Encrypt(_ context.Context, key string, plain, aad []byte) ([]byte, error) {
	a := k.sym[key]
	if a == nil || k.fail("Encrypt") {
		return nil, errDouble
	}
	nonce := make([]byte, a.NonceSize())
	_, _ = rand.Read(nonce)
	return a.Seal(nonce, nonce, plain, aad), nil
}

func (k *KMS) Decrypt(_ context.Context, key string, sealed, aad []byte) ([]byte, error) {
	a := k.sym[key]
	if a == nil || k.fail("Decrypt") || len(sealed) < a.NonceSize() {
		return nil, errDouble
	}
	return a.Open(nil, sealed[:a.NonceSize()], sealed[a.NonceSize():], aad)
}

func (k *KMS) Sign(_ context.Context, key string, msg []byte) ([]byte, error) {
	p := k.sign[key]
	if p == nil || k.fail("Sign") {
		return nil, errDouble
	}
	h := sha256.Sum256(msg)
	return ecdsa.SignASN1(rand.Reader, p, h[:])
}

func (k *KMS) PublicKeys(_ context.Context, key string) (map[string][]byte, error) {
	p := k.sign[key]
	if p == nil || k.fail("PublicKeys") {
		return nil, errDouble
	}
	pub, _ := p.PublicKey.ECDH()
	return map[string][]byte{k.kids[key]: pub.Bytes()}, nil
}

// Base est le double de Firestore : documents en JSON canonique, update_time
// posé à la validation, concurrence optimiste, au plus MaxDocs documents.
// Pannes : "Get", "Commit".
type Base struct {
	Faults
	Clock   ports.Horloge
	MaxDocs int
	mu      sync.Mutex
	docs    map[string]stored
}

type stored struct {
	data    []byte
	update  time.Time
	version uint64
}

// MaxDocs par défaut du registre en mémoire (choix de ce dépôt).
const MaxDocs = 200_000

type tx struct {
	b      *Base
	reads  map[string]uint64
	writes map[string][]byte
	order  []string
}

func (b *Base) Transaction(ctx context.Context, f func(ports.Tx) error) error {
	for range 5 {
		t := &tx{b: b, reads: map[string]uint64{}, writes: map[string][]byte{}}
		if err := f(t); err != nil {
			return err
		}
		if len(t.writes) == 0 {
			return nil
		}
		ok, err := b.commit(t)
		if err != nil || ok {
			return err
		}
	}
	return ports.ErrConflict
}

func (b *Base) commit(t *tx) (bool, error) {
	if b.fail("Commit") {
		return false, errDouble
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.docs == nil {
		b.docs = map[string]stored{}
	}
	for p, v := range t.reads {
		if b.docs[p].version != v {
			return false, nil
		}
	}
	limit, added := MaxDocs, 0
	if b.MaxDocs > 0 {
		limit = b.MaxDocs
	}
	for p := range t.writes {
		if _, ok := b.docs[p]; !ok {
			added++
		}
	}
	if len(b.docs)+added > limit {
		return false, errDouble
	}
	now := b.Clock.Now()
	for _, p := range t.order {
		b.docs[p] = stored{t.writes[p], now, b.docs[p].version + 1}
	}
	return true, nil
}

func (b *Base) doc(p string, s stored) ports.Doc {
	d := ports.Doc{Path: p, UpdateTime: s.update}
	if s.data != nil {
		v, _ := jcs.Parse(s.data)
		d.Data, _ = v.(jcs.Object)
	}
	return d
}

func (t *tx) Get(p string) (ports.Doc, error) {
	if t.b.fail("Get") {
		return ports.Doc{}, errDouble
	}
	t.b.mu.Lock()
	defer t.b.mu.Unlock()
	s := t.b.docs[p]
	t.reads[p] = s.version
	return t.b.doc(p, s), nil
}

func (t *tx) List(coll, field, after string, limit int) ([]ports.Doc, error) {
	if t.b.fail("Get") {
		return nil, errDouble
	}
	t.b.mu.Lock()
	defer t.b.mu.Unlock()
	var out []ports.Doc
	for p, s := range t.b.docs {
		rest, ok := strings.CutPrefix(p, coll+"/")
		if !ok || strings.Contains(rest, "/") || s.data == nil {
			continue
		}
		d := t.b.doc(p, s)
		if v, _ := d.Data[field].(string); field != "" && v <= after {
			continue
		}
		if out = append(out, d); len(out) > limit {
			return nil, errDouble
		}
		t.reads[p] = s.version
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (t *tx) Set(p string, data map[string]any) {
	b, err := jcs.Marshal(jcs.Object(data))
	if err != nil {
		b = []byte("null")
	}
	if _, dup := t.writes[p]; !dup {
		t.order = append(t.order, p)
	}
	t.writes[p] = b
}

// Doc lit un document tel que la base le tient (pour les tests).
func (b *Base) Doc(p string) ports.Doc {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.doc(p, b.docs[p])
}

// Tamper écrit un document hors de l'enclave (recopie, modification) avec
// son update_time ; data nil l'efface.
func (b *Base) Tamper(p string, data map[string]any, update time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.docs == nil {
		b.docs = map[string]stored{}
	}
	if data == nil {
		delete(b.docs, p)
		return
	}
	j, _ := jcs.Marshal(jcs.Object(data))
	b.docs[p] = stored{j, update, b.docs[p].version + 1}
}

// Len rend le nombre de documents.
func (b *Base) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.docs)
}

// Stockage est le double de Cloud Storage : création seule, timeCreated posé
// par le double, au plus MaxObjects objets. Pannes : "Read:<bucket>",
// "Create:<bucket>".
type Stockage struct {
	Faults
	Clock ports.Horloge
	mu    sync.Mutex
	objs  map[string]ports.Object
}

// MaxObjects borne les objets du double (choix de ce dépôt).
const MaxObjects = 200_000

func (s *Stockage) Create(_ context.Context, bucket, name string, content []byte) error {
	if s.fail("Create:" + bucket) {
		return errDouble
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objs == nil {
		s.objs = map[string]ports.Object{}
	}
	k := bucket + "/" + name
	if _, ok := s.objs[k]; ok {
		return ports.ErrExists
	}
	if len(s.objs) >= MaxObjects {
		return errDouble
	}
	s.objs[k] = ports.Object{Name: name, Created: s.Clock.Now(), Content: append([]byte{}, content...)}
	return nil
}

func (s *Stockage) Read(_ context.Context, bucket, name string) ([]byte, error) {
	if s.fail("Read:" + bucket) {
		return nil, errDouble
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.objs[bucket+"/"+name]; ok {
		return o.Content, nil
	}
	return nil, nil
}

func (s *Stockage) List(_ context.Context, bucket, prefix string, limit int) ([]ports.Object, error) {
	if s.fail("Read:" + bucket) {
		return nil, errDouble
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ports.Object
	for k, o := range s.objs {
		if strings.HasPrefix(k, bucket+"/"+prefix) {
			if out = append(out, o); len(out) > limit {
				return nil, errDouble
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Put dépose un objet à un timeCreated choisi (pour les tests : borne
// ancienne, objet recopié).
func (s *Stockage) Put(bucket, name string, content []byte, created time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objs == nil {
		s.objs = map[string]ports.Object{}
	}
	s.objs[bucket+"/"+name] = ports.Object{Name: name, Created: created, Content: content}
}

// Secrets est le double de Secret Manager.
type Secrets map[string][]byte

func (s Secrets) Secret(_ context.Context, name string) ([]byte, error) {
	if b, ok := s[name]; ok {
		return b, nil
	}
	return nil, errDouble
}

// Journal est le double de Cloud Logging : les MaxEvents derniers événements.
type Journal struct {
	mu     sync.Mutex
	events []ports.Event
}

// MaxEvents borne le journal en mémoire.
const MaxEvents = 10_000

func (j *Journal) Write(e ports.Event) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.events) >= MaxEvents {
		j.events = j.events[1:]
	}
	j.events = append(j.events, e)
}

// Events rend une copie des événements journalisés.
func (j *Journal) Events() []ports.Event {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]ports.Event{}, j.events...)
}

// Emetteurs rend les clés publiques de test de la configuration.
type Emetteurs map[string]map[string][]byte

func (e Emetteurs) Keys(_ context.Context, issuer string) (map[string][]byte, error) {
	if k, ok := e[issuer]; ok {
		return k, nil
	}
	return nil, errDouble
}

// Request est une requête reçue par la passerelle de test.
type Request struct {
	Method, PathQuery string
	Headers           map[string]string
	Body              []byte
}

// Gateway est la passerelle de test : elle garde les MaxRequests dernières
// requêtes et répond par Answer (par défaut 200 {}).
type Gateway struct {
	mu       sync.Mutex
	requests []Request
	Answer   func(Request) (int, []byte, error)
	Check    *Check // vérification des requêtes de Sésame (passerelle.go), nil : aucune
}

// MaxRequests borne les requêtes gardées par la passerelle de test.
const MaxRequests = 10_000

func (g *Gateway) Send(_ context.Context, method, pq string, h map[string]string, body []byte) (int, []byte, error) {
	r := Request{method, pq, h, append([]byte{}, body...)}
	g.mu.Lock()
	if len(g.requests) >= MaxRequests {
		g.requests = g.requests[1:]
	}
	g.requests = append(g.requests, r)
	answer := g.Answer
	g.mu.Unlock()
	if g.Check != nil {
		if st := g.Check.verdict(r); st != 0 {
			return st, []byte("{}"), nil
		}
	}
	st, body, err := 200, []byte("{}"), error(nil)
	if answer != nil {
		st, body, err = answer(r)
	}
	if g.Check != nil && method == "POST" && (err != nil || st != 200) {
		g.Check.forget(h["X-Sesame-Request-Id"])
	}
	return st, body, err
}

// Requests rend une copie des requêtes reçues.
func (g *Gateway) Requests() []Request {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]Request{}, g.requests...)
}

// Message est un SMS, un email ou un code Verify envoyé à un double.
type Message struct{ Kind, From, To, Subject, Text string }

// Outbox est le double de Twilio (Verify et Messaging) et de SendGrid : il
// garde les MaxRequests derniers envois ; Verify accepte le code "123456".
// Pannes : "Sms", "Email".
type Outbox struct {
	Faults
	mu   sync.Mutex
	sent []Message
}

func (o *Outbox) add(m Message, op string) error {
	if o.fail(op) {
		return errDouble
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.sent) >= MaxRequests {
		o.sent = o.sent[1:]
	}
	o.sent = append(o.sent, m)
	return nil
}

func (o *Outbox) SendCode(_ context.Context, service, phone string) error {
	return o.add(Message{Kind: "verify", From: service, To: phone}, "Sms")
}

func (o *Outbox) CheckCode(_ context.Context, service, phone, code string) (bool, error) {
	if o.fail("Sms") {
		return false, errDouble
	}
	return code == "123456", nil
}

func (o *Outbox) SendText(_ context.Context, from, phone, text string) error {
	return o.add(Message{Kind: "sms", From: from, To: phone, Text: text}, "Sms")
}

func (o *Outbox) Send(_ context.Context, from, to, subject, text string) error {
	return o.add(Message{Kind: "email", From: from, To: to, Subject: subject, Text: text}, "Email")
}

// Sent rend une copie des envois.
func (o *Outbox) Sent() []Message {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]Message{}, o.sent...)
}

// Identity est le double de Stripe Identity : chaque session créée est
// "processing" jusqu'à ce que le test lui donne un verdict (Verdict) ; il
// garde les demandes de suppression. Pannes : "Identite", "Redact".
type Identity struct {
	Faults
	mu       sync.Mutex
	sessions map[string]ports.Verification
	Redacted []string
}

func (s *Identity) Create(_ context.Context) (string, string, error) {
	if s.fail("Identite") {
		return "", "", errDouble
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = map[string]ports.Verification{}
	}
	if len(s.sessions) >= MaxRequests {
		return "", "", errDouble
	}
	id := fmt.Sprintf("vs_test_%d", len(s.sessions)+1)
	s.sessions[id] = ports.Verification{Status: "processing"}
	return id, id + "_secret_test", nil
}

func (s *Identity) Read(_ context.Context, id string) (ports.Verification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[id]
	if s.fail("Identite") || !ok {
		return v, errDouble
	}
	return v, nil
}

func (s *Identity) Redact(_ context.Context, id string) error {
	if s.fail("Redact") {
		return errDouble
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Redacted = append(s.Redacted, id)
	return nil
}

// Verdict donne son verdict à la dernière session créée.
func (s *Identity) Verdict(v ports.Verification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[fmt.Sprintf("vs_test_%d", len(s.sessions))] = v
}

// Letter est une lettre reçue par le double de Pingen.
type Letter struct {
	ID, Organisation string
	PDF              []byte
}

// Post est le double de Pingen : il garde les lettres reçues et rend l'état
// que le test leur donne (Statuses, "commandee" par défaut). Panne : "Courrier".
type Post struct {
	Faults
	mu       sync.Mutex
	Letters  []Letter
	Statuses map[string]string
}

func (p *Post) Send(_ context.Context, org string, pdf []byte) (string, error) {
	if p.fail("Courrier") {
		return "", errDouble
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.Letters) >= MaxRequests {
		return "", errDouble
	}
	id := fmt.Sprintf("lettre-test-%d", len(p.Letters)+1)
	p.Letters = append(p.Letters, Letter{id, org, append([]byte{}, pdf...)})
	return id, nil
}

func (p *Post) Status(_ context.Context, _ string, id string) (string, error) {
	if p.fail("Courrier") {
		return "", errDouble
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if st := p.Statuses[id]; st != "" {
		return st, nil
	}
	return "commandee", nil
}

// Sent rend une copie des lettres reçues.
func (p *Post) Sent() []Letter {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Letter{}, p.Letters...)
}

// SetStatus donne un état à la lettre id.
func (p *Post) SetStatus(id, st string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Statuses == nil {
		p.Statuses = map[string]string{}
	}
	p.Statuses[id] = st
}
