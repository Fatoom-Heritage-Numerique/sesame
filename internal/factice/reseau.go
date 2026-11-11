package factice

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/routes"
)

// Reseau est le réseau entre VM simulé en mémoire de sesame-fake (grappe
// locale et tests) : chaque connexion porte la clé du certificat entre VM de
// la VM qui l'ouvre, comme le TLS mutuel de l'image. Les tests y coupent des
// liens (Cut, Isolate), perdent des lots Raft (Loss, tirage déterministe) ou
// les retiennent (Hold, puis Release : latence).
type Reseau struct {
	mu    sync.Mutex
	vms   map[string]*routes.Instance
	cut   map[[2]string]bool
	Loss  float64
	Hold  bool
	held  []func()
	rnd   *rand.Rand
	conns atomic.Uint64
}

var errCut = errors.New("pair injoignable (réseau simulé)")

// NewReseau rend un réseau vide, de pertes tirées par la graine seed.
func NewReseau(seed uint64) *Reseau {
	return &Reseau{vms: map[string]*routes.Instance{}, cut: map[[2]string]bool{}, rnd: rand.New(rand.NewPCG(seed, seed))}
}

// Add branche (ou remplace) la VM vm ; Remove la débranche (VM arrêtée).
func (n *Reseau) Add(vm string, in *routes.Instance) { n.mu.Lock(); n.vms[vm] = in; n.mu.Unlock() }
func (n *Reseau) Remove(vm string)                   { n.mu.Lock(); delete(n.vms, vm); n.mu.Unlock() }

// Cut coupe ou rétablit le lien entre a et b ; Isolate, entre a et toute VM.
func (n *Reseau) Cut(a, b string, cut bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.cut[[2]string{a, b}], n.cut[[2]string{b, a}] = cut, cut
}

func (n *Reseau) Isolate(a string, cut bool) {
	for vm := range n.snapshot() {
		if vm != a {
			n.Cut(a, vm, cut)
		}
	}
}

func (n *Reseau) snapshot() map[string]*routes.Instance {
	n.mu.Lock()
	defer n.mu.Unlock()
	m := map[string]*routes.Instance{}
	for k, v := range n.vms {
		m[k] = v
	}
	return m
}

// Release délivre les lots retenus.
func (n *Reseau) Release() {
	n.mu.Lock()
	held := n.held
	n.held = nil
	n.mu.Unlock()
	for _, f := range held {
		f()
	}
}

// From rend le réseau tel que le voit la VM from.
func (n *Reseau) From(from string) ports.Reseau { return dialer{n, from} }

type dialer struct {
	n    *Reseau
	from string
}

func (d dialer) Dial(_ context.Context, vm string) (ports.Lien, error) {
	d.n.mu.Lock()
	defer d.n.mu.Unlock()
	if d.n.vms[vm] == nil || d.n.cut[[2]string{d.from, vm}] {
		return nil, errCut
	}
	return &memLink{d, vm, d.n.vms[vm], strconv.FormatUint(d.n.conns.Add(1), 10)}, nil
}

type memLink struct {
	d    dialer
	to   string
	in   *routes.Instance
	conn string
}

func (l *memLink) Cle() []byte { return l.in.Grappe().Cle() }

func (l *memLink) Call(ctx context.Context, path string, body []byte) (int, []byte, error) {
	n := l.d.n
	n.mu.Lock()
	from, cur := n.vms[l.d.from], n.vms[l.to]
	lost := path == "/interne/raft" && n.Loss > 0 && n.rnd.Float64() < n.Loss
	if from == nil || cur != l.in || n.cut[[2]string{l.d.from, l.to}] || lost {
		n.mu.Unlock()
		return 0, nil, errCut
	}
	if path == "/interne/raft" && n.Hold {
		n.held = append(n.held, func() { l.in.Grappe().Serve(ctx, l.conn, from.Grappe().Cle(), path, body) })
		n.mu.Unlock()
		return 204, nil, nil
	}
	n.mu.Unlock()
	st, b := l.in.Grappe().Serve(ctx, l.conn, from.Grappe().Cle(), path, body)
	return st, b, nil
}

// En-têtes du transport HTTP entre conteneurs de sesame-fake : identifiant de
// connexion et clé du certificat entre VM, que l'image tient de son TLS
// mutuel (lot 9).
const (
	headerConn = "X-Sesame-Fake-Connexion"
	headerCle  = "X-Sesame-Fake-Cle"
)

// HTTPReseau joint les autres conteneurs aux adresses de la configuration
// (pairs : nom de VM → adresse http), en mode instance.
type HTTPReseau struct {
	Pairs  map[string]string
	Self   func() []byte
	Client *http.Client
}

func (h HTTPReseau) Dial(_ context.Context, vm string) (ports.Lien, error) {
	if h.Pairs[vm] == "" {
		return nil, errCut
	}
	id, _ := Rand{}.Read(16)
	return &httpLink{h: h, base: h.Pairs[vm], conn: base64.RawURLEncoding.EncodeToString(id)}, nil
}

type httpLink struct {
	h    HTTPReseau
	base string
	conn string
	cle  []byte
}

func (l *httpLink) Cle() []byte { return l.cle }

func (l *httpLink) Call(ctx context.Context, path string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.base+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set(headerConn, l.conn)
	req.Header.Set(headerCle, base64.StdEncoding.EncodeToString(l.h.Self()))
	c := l.h.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	cle, _ := base64.StdEncoding.DecodeString(resp.Header.Get(headerCle))
	if l.cle == nil {
		l.cle = cle
	} else if !bytes.Equal(l.cle, cle) {
		return 0, nil, errCut
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	return resp.StatusCode, b, err
}

// GrappeHandler sert les routes entre VM d'une instance (/interne/*,
// grappe/rejoindre, preparer, demarrer) sur l'écoute entre conteneurs.
func GrappeHandler(in *routes.Instance) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<20))
		cle, err2 := base64.StdEncoding.DecodeString(r.Header.Get(headerCle))
		w.Header().Set(headerCle, base64.StdEncoding.EncodeToString(in.Grappe().Cle()))
		if err != nil || err2 != nil || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		st, b := in.Grappe().Serve(r.Context(), r.Header.Get(headerConn), cle, r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(st)
		_, _ = w.Write(b)
	})
}
