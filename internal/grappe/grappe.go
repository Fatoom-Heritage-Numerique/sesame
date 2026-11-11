// Paquet grappe : la grappe de trois VM du protocole de PIN (ARCHITECTURE.md
// §9.3 ; API.md §3 et §4), sur go.etcd.io/raft/v3 en mémoire seulement
// (décisions 0017, 0018, 0071). Transport entre VM attesté (échange des
// jetons de paire, contrôles entre VM sans la liste des versions), relais
// des routes du PIN et décomptes au leader, admission d'une VM recréée,
// amorçage commandé d'une grappe neuve, liste inscrite des places. La
// connexion (TLS mutuel, DNS zonal) est à l'adaptateur (ports.Reseau) ;
// tout ce qui accepte ou refuse est ici.
package grappe

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"sync"
	"time"

	"go.etcd.io/raft/v3"
	pb "go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/pin"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Durées de la grappe (protocole-pin.md ; API.md §4, §7) : votant absent
// après 2 minutes (en ticks : horloge monotone), demande d'entrée toutes les
// 30 secondes, réservation d'une configuration 2 minutes ; Container est la
// mémoire du conteneur (1,38 Gio), seuils d'alerte et de refus des nouveaux
// dépôts à 70 % et 90 %.
const (
	AbsentTicks = 1200
	JoinTicks   = 300
	Reservation = 2 * time.Minute
	Container   = 1_481_763_717
	maxConns    = 64
	maxBatch    = 4096
)

// ErrPause : pas de majorité, ou pas de leader joignable.
var ErrPause = errors.New("pin_indisponible_temporaire")

// Config est ce que l'instance donne à sa grappe. Relay exécute au leader
// une route relayée (API.md §1.3, réponse {statut, …}) ; Count un décompte
// (nil pour une fin de session) ; Limit une limite par nodeId ; Journal un
// événement sans donnée de titulaire ; Memoire la part utilisée de la
// mémoire du conteneur (nil : mesurée) ; Attente l'attente d'une validation.
type Config struct {
	Horloge  ports.Horloge
	Alea     ports.Alea
	Lanceur  ports.Lanceur
	Reseau   ports.Reseau
	Racine   []byte
	HWModels []string
	Journal  func(event, resultat string)
	Limit    func(key string, n int, per time.Duration) (bool, time.Time)
	Relay    func(ctx context.Context, req jcs.Object) jcs.Object
	Count    func(req jcs.Object) jcs.Object
	Memoire  func() float64
	Attente  time.Duration
}

type conn struct {
	nonce, cle []byte
	peer       *Membre
	jwt        string
}

type link struct {
	l    ports.Lien
	peer Membre
	jwt  string // jeton de paire présenté sur cette connexion
}

type resa struct {
	id, from string
	membres  []Membre
	until    time.Time
}

// Grappe est la part d'une VM dans la grappe : son identifiant de nœud neuf
// et la clé de son certificat entre VM (tirés au démarrage, en mémoire), son
// nœud Raft (nil sans grappe), ses connexions.
type Grappe struct {
	cfg        Config
	NodeID     string
	Key        *ecdsa.PrivateKey
	id         uint64
	cle        []byte
	mu         sync.Mutex
	n          *noeud
	own        *attestation.Claims
	resa       *resa
	conns      map[string]*conn
	links      map[string]*link
	waits      map[uint64]chan result
	seq, ticks uint64
	out        []*pb.Message
	signal     chan struct{}
	heard      map[uint64]uint64
	vms        map[uint64]string // VM des pairs entendus : un nœud qui rejoint n'a pas encore de configuration
	absent     map[uint64]bool
	refused    time.Time
	pending    bool
	had        bool
	joining    bool
	mem        int
	onLead     chan struct{}
}

// New tire l'identifiant de nœud et la clé du certificat entre VM.
func New(cfg Config) (*Grappe, error) {
	raw, err := cfg.Alea.Read(16)
	key, err2 := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err = errors.Join(err, err2); err != nil {
		return nil, err
	}
	cle, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if cfg.Attente == 0 {
		cfg.Attente = 10 * time.Second
	}
	return &Grappe{cfg: cfg, NodeID: b64url.EncodeToString(raw), Key: key, id: raftID(raw), cle: cle, conns: map[string]*conn{},
		links: map[string]*link{}, waits: map[uint64]chan result{}, signal: make(chan struct{}, 1), heard: map[uint64]uint64{}, vms: map[uint64]string{}, absent: map[uint64]bool{}}, err
}

var b64url = base64.RawURLEncoding

func raftID(nodeID []byte) uint64 {
	return max(binary.BigEndian.Uint64(append(nodeID, make([]byte, 8)...)[:8]), 1)
}

func idOf(nodeID string) uint64 { b, _ := b64url.DecodeString(nodeID); return raftID(b) }

// Cle rend la clé publique du certificat entre VM (SubjectPublicKeyInfo DER).
func (g *Grappe) Cle() []byte { return g.cle }

// ID rend le grappeId de la grappe en service, "" sans grappe.
func (g *Grappe) ID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.n == nil {
		return ""
	}
	return g.n.s.id
}

func (g *Grappe) lead() uint64 {
	if g.n == nil {
		return 0
	}
	return g.n.rn.BasicStatus().Lead
}

// Leader dit si cette VM est le leader ; Paused, si elle est dans une grappe
// sans leader connu (pause) ; Runs, si elle fait la tâche horaire : leader,
// ou aucun leader (pause, VM sans grappe).
func (g *Grappe) Leader() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.n != nil && g.lead() == g.id
}
func (g *Grappe) Paused() bool { g.mu.Lock(); defer g.mu.Unlock(); return g.n != nil && g.lead() == 0 }
func (g *Grappe) Runs() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lead() == 0 || g.lead() == g.id
}

// Borne rend la grappe publiée dans la borne : grappeId (nil sans grappe) et
// nombre de votants.
func (g *Grappe) Borne() (any, int) {
	if id := g.ID(); id != "" {
		g.mu.Lock()
		defer g.mu.Unlock()
		return id, len(g.n.cs.GetVoters())
	}
	return nil, 0
}

// Record rend une copie de l'enregistrement du PIN de uid, tel que cette VM
// l'a appliqué.
func (g *Grappe) Record(uid string) (pin.Record, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.n == nil || g.n.s.records[uid] == nil {
		return pin.Record{}, false
	}
	return *g.n.s.records[uid], true
}

// Full dit si la mémoire du processus a passé 90 % : nouveaux dépôts refusés.
func (g *Grappe) Full() bool { return g.memory() >= 0.9 }

func (g *Grappe) memory() float64 {
	if g.cfg.Memoire != nil {
		return g.cfg.Memoire()
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Sys-m.HeapReleased) / Container
}

func (g *Grappe) log(event, res string) {
	if g.cfg.Journal != nil {
		g.cfg.Journal(event, res)
	}
}

// token rend le jeton de paire de cette VM sur nonce et garde ses claims.
func (g *Grappe) token(ctx context.Context, nonce []byte) (string, error) {
	nonces, err := attestation.PairNonces(nonce, g.NodeID, g.cle)
	if err != nil {
		return "", err
	}
	jwt, err := g.cfg.Lanceur.Token(ctx, nonces)
	if err != nil {
		return "", err
	}
	c, err := attestation.Verify(jwt, g.cfg.Racine, g.cfg.Horloge.Now(), nonces)
	if err == nil {
		g.mu.Lock()
		g.own = c
		g.mu.Unlock()
	}
	return jwt, err
}

// self rend les claims de cette VM. Sans grappe (demande d'entrée, amorçage,
// réservation), elle oublie aussi ses connexions : un pair recréé depuis est
// joint de nouveau, jamais sous son ancienne identité.
func (g *Grappe) self(ctx context.Context) *attestation.Claims {
	g.mu.Lock()
	if g.n == nil {
		g.links = map[string]*link{}
	}
	own := g.own
	g.mu.Unlock()
	if nonce, err := g.cfg.Alea.Read(32); own == nil && err == nil {
		_, _ = g.token(ctx, nonce)
		g.mu.Lock()
		own = g.own
		g.mu.Unlock()
	}
	return own
}

// peer vérifie le jeton de paire d'un pair par les contrôles entre VM
// (API.md §3), sans la liste des versions : nonces de cette connexion, même
// image, même env_override, même projet et même dépôt d'images que cette
// VM, hwmodel de la constante de l'image.
func (g *Grappe) peer(jwt, nodeID string, nonce, cle []byte) *Membre {
	nonces, err := attestation.PairNonces(nonce, nodeID, cle)
	c, err2 := attestation.Verify(jwt, g.cfg.Racine, g.cfg.Horloge.Now(), nonces)
	g.mu.Lock()
	own := g.own
	g.mu.Unlock()
	if err != nil || err2 != nil || own == nil || c.ImageDigest != own.ImageDigest || c.ImageReference != own.ImageReference ||
		!reflect.DeepEqual(c.ProjectNumber, own.ProjectNumber) || !reflect.DeepEqual(c.EnvOverride, own.EnvOverride) ||
		!slices.Contains(g.cfg.HWModels, c.HWModel) {
		return nil
	}
	return &Membre{nodeID, c.Name, c.Zone, base64.StdEncoding.EncodeToString(cle)}
}

// peers rend les noms des deux autres places : même radical, autre lettre.
func peers(name string) []string {
	var out []string
	for _, p := range "abc" {
		if name != "" && rune(name[len(name)-1]) != p {
			out = append(out, name[:len(name)-1]+string(p))
		}
	}
	return out
}

func call(ctx context.Context, l ports.Lien, path string, body jcs.Object) (int, jcs.Object, error) {
	b, err := jcs.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	st, resp, err := l.Call(ctx, path, b)
	v, _ := jcs.Parse(resp)
	o, _ := v.(jcs.Object)
	return st, o, err
}

func str(o jcs.Object, k string) string { s, _ := o[k].(string); return s }

// connect ouvre une connexion attestée vers vm : jetons de paire échangés
// dans les deux sens (POST /interne/attestation) avant toute autre requête.
func (g *Grappe) connect(ctx context.Context, vm string) (*link, error) {
	g.mu.Lock()
	k := g.links[vm]
	g.mu.Unlock()
	if k != nil {
		return k, nil
	}
	l, err := g.cfg.Reseau.Dial(ctx, vm)
	na, err2 := g.cfg.Alea.Read(32)
	if err = errors.Join(err, err2); err != nil {
		return nil, err
	}
	st, r, err := call(ctx, l, "/interne/attestation", jcs.Object{"noncePaire": b64url.EncodeToString(na)})
	nb, err2 := primitives.B64URL(str(r, "noncePaire"))
	if err = errors.Join(err, err2); err != nil || st != 200 || len(nb) != 32 {
		return nil, ErrPause
	}
	tok, err := g.token(ctx, nb)
	m := g.peer(str(r, "jwt"), str(r, "nodeId"), na, l.Cle())
	if err != nil || m == nil || m.VM != vm {
		return nil, ErrPause
	}
	if st, _, err = call(ctx, l, "/interne/attestation", jcs.Object{"jwt": tok, "nodeId": g.NodeID}); err != nil || st != 204 {
		return nil, ErrPause
	}
	k = &link{l, *m, tok}
	g.mu.Lock()
	g.links[vm] = k
	g.mu.Unlock()
	return k, nil
}

// send envoie une requête à vm ; une connexion en échec est oubliée.
func (g *Grappe) send(ctx context.Context, vm, path string, body []byte) (int, []byte, error) {
	k, err := g.connect(ctx, vm)
	if err != nil {
		return 0, nil, err
	}
	st, resp, err := k.l.Call(ctx, path, body)
	if err != nil || st == 403 {
		g.mu.Lock()
		delete(g.links, vm)
		g.mu.Unlock()
	}
	return st, resp, err
}

func refusal(st int, code string) (int, []byte) {
	b, _ := jcs.Marshal(jcs.Object{"erreur": code, "message": "Requête refusée : " + code + "."})
	return st, b
}

func answer(o jcs.Object) (int, []byte) { b, _ := jcs.Marshal(o); return 200, b }

// Serve sert une requête entre VM reçue sur la connexion connID, dont le
// pair présente la clé cle (API.md §4 ; protocole-pin.md, « Transport »).
func (g *Grappe) Serve(ctx context.Context, connID string, cle []byte, path string, body []byte) (int, []byte) {
	v, err := jcs.Parse(body)
	req, _ := v.(jcs.Object)
	if path == "/interne/attestation" {
		return g.handshake(ctx, connID, cle, req)
	}
	g.mu.Lock()
	c := g.conns[connID]
	g.mu.Unlock()
	switch {
	case c == nil || c.peer == nil || !bytes.Equal(c.cle, cle):
		return refusal(403, "pair_refuse")
	case path == "/interne/raft":
		return g.raftIn(c, body)
	case err != nil || req == nil:
		return refusal(400, "message_refuse")
	case path == "/interne/relais" && str(req, "route") == "grappe/rejoindre" && g.Leader():
		st, b := g.admit(ctx, c, sub(req, "corps"), true)
		v, _ := jcs.Parse(b)
		return answer(jcs.Object{"statut": st, "corps": v})
	case path == "/interne/relais" || path == "/interne/decompte":
		if !g.Leader() {
			return refusal(409, "pas_leader")
		}
		if path == "/interne/relais" {
			return answer(g.cfg.Relay(ctx, req))
		}
		if o := g.cfg.Count(req); o != nil {
			return answer(o)
		}
		return 204, nil
	}
	n := map[string]int{"/grappe/rejoindre": 4, "/grappe/preparer": 2, "/grappe/demarrer": 2}[path]
	if n == 0 {
		return refusal(404, "message_refuse")
	}
	if ok, retry := g.cfg.Limit(path+"|"+c.peer.NodeID, n, time.Minute); !ok {
		b, _ := jcs.Marshal(jcs.Object{"erreur": "trop_d_appels", "message": "Trop d'appels.", "details": jcs.Object{"reessayerLe": retry.UTC().Format(primitives.TimeLayout)}})
		return 429, b
	}
	switch path {
	case "/grappe/rejoindre":
		return g.admit(ctx, c, req, false)
	case "/grappe/preparer":
		return g.prepare(ctx, c, sub(req, "configuration"))
	}
	return g.demarrer(c, str(req, "grappeId"))
}

func sub(o jcs.Object, k string) jcs.Object { m, _ := o[k].(jcs.Object); return m }

// handshake tient l'échange des jetons de paire de la connexion connID.
func (g *Grappe) handshake(ctx context.Context, connID string, cle []byte, req jcs.Object) (int, []byte) {
	if na, err := primitives.B64URL(str(req, "noncePaire")); len(req) == 1 && err == nil && len(na) == 32 {
		nb, err := g.cfg.Alea.Read(32)
		tok, err2 := g.token(ctx, na)
		if err = errors.Join(err, err2); err != nil {
			return refusal(503, "pin_indisponible_temporaire")
		}
		g.mu.Lock()
		for k := range g.conns {
			if len(g.conns) < maxConns {
				break
			}
			delete(g.conns, k)
		}
		g.conns[connID] = &conn{nonce: nb, cle: cle}
		g.mu.Unlock()
		return answer(jcs.Object{"jwt": tok, "nodeId": g.NodeID, "noncePaire": b64url.EncodeToString(nb)})
	}
	g.mu.Lock()
	c := g.conns[connID]
	g.mu.Unlock()
	if len(req) != 2 || c == nil || c.peer != nil || !bytes.Equal(c.cle, cle) {
		return refusal(403, "pair_refuse")
	}
	m := g.peer(str(req, "jwt"), str(req, "nodeId"), c.nonce, cle)
	g.mu.Lock()
	defer g.mu.Unlock()
	if m == nil {
		delete(g.conns, connID)
		return refusal(403, "pair_refuse")
	}
	c.peer, c.jwt = m, str(req, "jwt")
	return 204, nil
}

// raftIn reçoit un lot de messages Raft d'un pair attesté : chacun doit venir
// de l'identifiant que lie son attestation, non retiré, vers cette VM.
func (g *Grappe) raftIn(c *conn, body []byte) (int, []byte) {
	var msgs []*pb.Message
	for len(body) > 0 && len(msgs) < maxBatch {
		n, k := binary.Uvarint(body)
		m := &pb.Message{}
		if k <= 0 || n > uint64(len(body)-k) || proto.Unmarshal(body[k:k+int(n)], m) != nil {
			return refusal(400, "message_refuse")
		}
		msgs, body = append(msgs, m), body[k+int(n):]
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	from := idOf(c.peer.NodeID)
	for _, m := range msgs {
		if g.n == nil || g.n.s.removed[c.peer.NodeID] || len(body) > 0 || m.GetFrom() != from || m.GetTo() != g.id ||
			raft.IsLocalMsg(m.GetType()) || m.GetType() == pb.MsgProp {
			return refusal(403, "pair_refuse")
		}
	}
	g.heard[from], g.absent[from], g.vms[from] = g.ticks, false, c.peer.VM
	for _, m := range msgs {
		g.step(m)
	}
	g.process()
	return 204, nil
}

func (g *Grappe) step(m *pb.Message) {
	defer func() {
		if recover() != nil {
			g.log("grappe_message_rejete", "")
		}
	}()
	_ = g.n.rn.Step(m)
}

// process traite le travail de Raft, sous le verrou.
func (g *Grappe) process() {
	g.n.ready(g.cfg.Horloge.Now(), &g.out, g.done, func(e string) { g.log(e, "") })
	if len(g.out) > 0 {
		select {
		case g.signal <- struct{}{}:
		default:
		}
	}
	if g.lead() != g.id {
		for id, w := range g.waits {
			w <- result{code: "pas_leader"}
			delete(g.waits, id)
		}
	} else if g.onLead != nil {
		close(g.onLead)
		g.onLead = nil
	}
}

func (g *Grappe) done(id uint64, r result) {
	if r.code == "conf" {
		g.pending, r.code = false, ""
	}
	if w, ok := g.waits[id]; ok {
		w <- r
		delete(g.waits, id)
	}
}

// Flush envoie les messages Raft en attente, un lot par pair ; Deliver le
// fait à chaque signal jusqu'à la fin de ctx.
func (g *Grappe) Flush(ctx context.Context) {
	g.mu.Lock()
	out, nodes := g.out, map[uint64]Membre{}
	g.out = nil
	for k, vm := range g.vms {
		nodes[k] = Membre{VM: vm}
	}
	if g.n != nil {
		for k, v := range g.n.s.nodes {
			nodes[k] = v
		}
	}
	g.mu.Unlock()
	by := map[uint64][]byte{}
	snaps := map[uint64]bool{}
	for _, m := range out {
		b, _ := proto.Marshal(m)
		by[m.GetTo()] = append(binary.AppendUvarint(by[m.GetTo()], uint64(len(b))), b...)
		snaps[m.GetTo()] = snaps[m.GetTo()] || m.GetType() == pb.MsgSnap
	}
	for to, b := range by {
		st, _, err := g.send(ctx, nodes[to].VM, "/interne/raft", b)
		g.mu.Lock()
		if g.n != nil && (err != nil || st != 204) {
			g.n.rn.ReportUnreachable(to)
		}
		if g.n != nil && snaps[to] {
			g.n.rn.ReportSnapshot(to, map[bool]raft.SnapshotStatus{true: raft.SnapshotFinish, false: raft.SnapshotFailure}[err == nil && st == 204])
		}
		g.mu.Unlock()
	}
}

func (g *Grappe) Deliver(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-g.signal:
			g.Flush(ctx)
		}
	}
}

// Tick avance la grappe d'un tick de Raft (100 ms) : élection et battements,
// votants absents, promotion d'une apprenante, réservation échue, mémoire,
// et pour une VM sans grappe une demande d'entrée toutes les 30 secondes.
func (g *Grappe) Tick(ctx context.Context) {
	g.mu.Lock()
	g.ticks++
	now := g.cfg.Horloge.Now()
	if g.resa != nil && !now.Before(g.resa.until) {
		g.resa = nil
	}
	if g.n != nil {
		g.n.rn.Tick()
		g.process()
		g.maintain()
	}
	join := g.n == nil && g.resa == nil && !g.joining && g.ticks%JoinTicks == 0
	g.joining = g.joining || join
	check := g.ticks%10 == 0
	g.mu.Unlock()
	if check {
		m := g.memory()
		lvl := map[bool]int{true: 1}[m >= 0.7] + map[bool]int{true: 1}[m >= 0.9]
		if g.mu.Lock(); lvl > g.mem {
			g.log("grappe_memoire", []string{"", "70", "90"}[lvl])
		}
		g.mem = lvl
		g.mu.Unlock()
	}
	if join {
		go g.join(ctx)
	}
}

// maintain : alertes de votant absent et de perte de majorité ; au leader,
// une modification de membres à la fois pour la remplaçante apprenante
// rattrapée : retrait de l'ancien membre de sa place, puis promotion.
func (g *Grappe) maintain() {
	st := g.n.rn.Status()
	if g.had && st.Lead == 0 {
		g.log("grappe_sans_majorite", "")
	}
	g.had = st.Lead != 0
	if st.Lead != g.id {
		return
	}
	for _, v := range g.n.cs.GetVoters() {
		if _, ok := g.heard[v]; !ok || v == g.id {
			g.heard[v] = g.ticks
		}
		if g.ticks-g.heard[v] >= AbsentTicks && !g.absent[v] {
			g.absent[v] = true
			g.log("grappe_votant_absent", "")
		}
	}
	for _, l := range g.n.cs.GetLearners() {
		if g.pending || st.Progress[l].Match < g.n.applied {
			return
		}
		t, id := pb.ConfChangeAddNode, l
		for _, v := range g.n.cs.GetVoters() {
			if g.n.s.nodes[v].VM == g.n.s.nodes[l].VM {
				t, id = pb.ConfChangeRemoveNode, v
			}
		}
		g.conf(t, id, g.n.s.nodes[id], 0)
	}
}

func (g *Grappe) conf(t pb.ConfChangeType, id uint64, m Membre, wait uint64) {
	c, _ := json.Marshal(struct {
		ID, De uint64
		Membre Membre
	}{wait, g.id, m})
	if g.n.rn.ProposeConfChange(&pb.ConfChange{Type: t.Enum(), NodeId: &id, Context: c}) == nil {
		g.pending = true
	}
	g.process()
}

// propose fait valider e par la majorité ; seul le leader propose.
func (g *Grappe) propose(ctx context.Context, e entry) (result, error) {
	g.mu.Lock()
	if g.n == nil || g.lead() != g.id {
		g.mu.Unlock()
		return result{}, ErrPause
	}
	g.seq++
	e.ID, e.From = g.seq, g.id
	w := g.wait(e.ID)
	data, _ := json.Marshal(e)
	err := g.n.rn.Propose(data)
	g.process()
	g.mu.Unlock()
	if err != nil {
		return result{}, ErrPause
	}
	return g.await(ctx, e.ID, w)
}

func (g *Grappe) wait(id uint64) chan result {
	w := make(chan result, 1)
	g.waits[id] = w
	return w
}

func (g *Grappe) await(ctx context.Context, id uint64, w chan result) (result, error) {
	t := time.NewTimer(g.cfg.Attente)
	defer t.Stop()
	select {
	case r := <-w:
		if r.code != "pas_leader" {
			return r, nil
		}
	case <-t.C:
	case <-ctx.Done():
	}
	g.mu.Lock()
	delete(g.waits, id)
	g.mu.Unlock()
	return result{}, ErrPause
}

// Propose fait valider op par la majorité et rend le code de son application
// ("" si admise) et l'enregistrement qui en résulte.
func (g *Grappe) Propose(ctx context.Context, op pin.Op) (string, pin.Record, error) {
	r, err := g.propose(ctx, entry{Op: &op})
	return r.code, r.rec, err
}

// Relay envoie req au leader (path /interne/relais ou /interne/decompte) et
// rend sa réponse ; faux sans leader joignable. Après pas_leader, un seul
// nouvel envoi, au nouveau leader.
func (g *Grappe) Relay(ctx context.Context, path string, req jcs.Object) (jcs.Object, bool) {
	b, err := jcs.Marshal(req)
	for range 2 {
		g.mu.Lock()
		vm, lead := "", g.lead()
		if g.n != nil && lead != 0 && lead != g.id {
			vm = g.n.s.nodes[lead].VM
		}
		g.mu.Unlock()
		if vm == "" || err != nil {
			return nil, false
		}
		st, resp, err := g.send(ctx, vm, path, b)
		v, _ := jcs.Parse(resp)
		if o, ok := v.(jcs.Object); err == nil && (st == 200 && ok || st == 204) {
			return o, true
		}
		if st != 409 {
			return nil, false
		}
	}
	return nil, false
}

// admit décide d'une demande grappe/rejoindre (API.md §4.1), au leader ; un
// autre membre la relaie au leader avec l'identité attestée du demandeur.
func (g *Grappe) admit(ctx context.Context, c *conn, req jcs.Object, relayed bool) (int, []byte) {
	m := *c.peer
	if relayed {
		m = Membre{str(req, "nodeId"), str(req, "vm"), str(req, "zone"), str(req, "cle")}
	} else if len(req) != 2 || str(req, "jwt") != c.jwt || str(req, "nodeId") != m.NodeID {
		return g.refuse()
	}
	g.mu.Lock()
	lead := g.lead()
	switch {
	case g.n == nil:
		g.mu.Unlock()
		return g.refuse()
	case lead == 0:
		g.mu.Unlock()
		return refusal(503, "pin_indisponible_temporaire")
	case lead != g.id:
		g.mu.Unlock()
		r, ok := g.Relay(ctx, "/interne/relais", jcs.Object{"route": "grappe/rejoindre", "corps": jcs.Object{"nodeId": m.NodeID, "vm": m.VM, "zone": m.Zone, "cle": m.Cle}})
		st, _ := r["statut"].(float64)
		b, _ := jcs.Marshal(r["corps"])
		if !ok || st == 0 {
			return refusal(503, "pin_indisponible_temporaire")
		}
		return int(st), b
	}
	var old uint64
	for _, v := range g.n.cs.GetVoters() {
		if g.n.s.nodes[v].VM == m.VM {
			old = v
		}
	}
	_, known := g.n.s.nodes[idOf(m.NodeID)]
	if old == 0 || !g.absent[old] || g.n.s.places[m.VM] != m.Zone || g.pending || len(g.n.cs.GetLearners()) > 0 || known || g.n.s.removed[m.NodeID] {
		g.mu.Unlock()
		return g.refuse()
	}
	g.seq++
	id, w := g.seq, g.wait(g.seq)
	g.conf(pb.ConfChangeAddLearnerNode, idOf(m.NodeID), m, id)
	g.mu.Unlock()
	if _, err := g.await(ctx, id, w); err != nil {
		return refusal(503, "pin_indisponible_temporaire")
	}
	return answer(jcs.Object{"admise": true, "role": "apprenant"})
}

// refuse rend refusee et journalise grappe_admission_refusee au plus une
// fois par heure.
func (g *Grappe) refuse() (int, []byte) {
	g.mu.Lock()
	now := g.cfg.Horloge.Now()
	if g.refused.IsZero() || now.Sub(g.refused) >= time.Hour {
		g.refused = now
		g.log("grappe_admission_refusee", "")
	}
	g.mu.Unlock()
	return refusal(409, "refusee")
}

// join demande l'entrée dans la grappe en service à chaque autre place.
func (g *Grappe) join(ctx context.Context) {
	defer func() { g.mu.Lock(); g.joining = false; g.mu.Unlock() }()
	own := g.self(ctx)
	if own == nil {
		return
	}
	for _, vm := range peers(own.Name) {
		k, err := g.connect(ctx, vm)
		if err != nil {
			continue
		}
		if st, _, err := call(ctx, k.l, "/grappe/rejoindre", jcs.Object{"jwt": k.jwt, "nodeId": g.NodeID}); err == nil && st == 200 {
			g.mu.Lock()
			if g.n == nil && g.resa == nil {
				g.n, _ = newNoeud(g.id, nil, g.cfg.Horloge.Now())
			}
			g.mu.Unlock()
			return
		}
	}
}

// configuration rend la configuration initiale en JSON canonique (API.md
// §4.3) et son grappeId.
func configuration(digest string, ms []Membre) (jcs.Object, string) {
	a := jcs.Array{}
	for _, m := range ms {
		a = append(a, jcs.Object{"nodeId": m.NodeID, "vm": m.VM, "zone": m.Zone, "cle": m.Cle})
	}
	o := jcs.Object{"v": 1, "imageDigest": digest, "membres": a}
	b, _ := jcs.Marshal(o)
	return o, hex.EncodeToString(primitives.SHA256(b))
}

// Amorcer amorce une grappe neuve (grappe/amorcer, API.md §4.3), le jeton
// d'exploitation déjà vérifié : rend la réponse, ou le code du refus.
func (g *Grappe) Amorcer(ctx context.Context, email string) (jcs.Object, string) {
	own := g.self(ctx)
	g.mu.Lock()
	if g.n != nil || g.resa != nil || own == nil {
		g.mu.Unlock()
		return nil, "amorcage_refuse"
	}
	g.resa = &resa{from: g.NodeID, until: g.cfg.Horloge.Now().Add(Reservation)}
	g.mu.Unlock()
	ms := []Membre{{g.NodeID, own.Name, own.Zone, base64.StdEncoding.EncodeToString(g.cle)}}
	for _, vm := range peers(own.Name) {
		k, err := g.connect(ctx, vm)
		if err != nil {
			return g.unreserve()
		}
		ms = append(ms, k.peer)
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].VM < ms[j].VM })
	conf, id := configuration(own.ImageDigest, ms)
	for _, path := range []string{"/grappe/preparer", "/grappe/demarrer"} {
		for _, vm := range peers(own.Name) {
			k, err := g.connect(ctx, vm)
			if err == nil {
				var st int
				var r jcs.Object
				st, r, err = call(ctx, k.l, path, map[bool]jcs.Object{true: {"configuration": conf}, false: {"grappeId": id}}[path == "/grappe/preparer"])
				if st != 200 || path == "/grappe/preparer" && str(r, "grappeId") != id {
					err = ErrPause
				}
			}
			if err != nil {
				return g.unreserve()
			}
		}
	}
	lead := make(chan struct{})
	g.mu.Lock()
	if g.start(id, ms); g.n == nil {
		g.mu.Unlock()
		return g.unreserve()
	}
	g.onLead = lead
	_ = g.n.rn.Campaign()
	g.process()
	g.mu.Unlock()
	t := time.NewTimer(Reservation)
	defer t.Stop()
	select {
	case <-lead:
	case <-t.C:
		return nil, "pin_indisponible_temporaire"
	}
	if _, err := g.propose(ctx, entry{Init: id, Places: g.places(ms)}); err != nil {
		return nil, "pin_indisponible_temporaire"
	}
	g.log("grappe_amorcee", email)
	return jcs.Object{"grappeId": id, "votants": 3}, ""
}

func (g *Grappe) unreserve() (jcs.Object, string) {
	g.mu.Lock()
	g.resa = nil
	g.mu.Unlock()
	return nil, "amorcage_refuse"
}

func (g *Grappe) places(ms []Membre) map[string]string {
	p := map[string]string{}
	for _, m := range ms {
		p[m.VM] = m.Zone
	}
	return p
}

// start démarre Raft en état new, les trois membres votants (sous le verrou).
func (g *Grappe) start(id string, ms []Membre) {
	var ps []raft.Peer
	for _, m := range ms {
		c, _ := json.Marshal(struct{ Membre Membre }{m})
		ps = append(ps, raft.Peer{ID: idOf(m.NodeID), Context: c})
	}
	n, err := newNoeud(g.id, ps, g.cfg.Horloge.Now())
	if err != nil {
		return
	}
	n.s.id, n.s.places, g.n, g.resa = id, g.places(ms), n, nil
	g.process()
}

// prepare décide de grappe/preparer : la configuration décrit les VM que
// voit cette VM, jointe à chaque autre membre par ses propres connexions.
func (g *Grappe) prepare(ctx context.Context, c *conn, conf jcs.Object) (int, []byte) {
	own := g.self(ctx)
	g.mu.Lock()
	busy := g.n != nil || g.resa != nil
	g.mu.Unlock()
	var ms []Membre
	a, _ := conf["membres"].(jcs.Array)
	for _, e := range a {
		o, _ := e.(jcs.Object)
		ms = append(ms, Membre{str(o, "nodeId"), str(o, "vm"), str(o, "zone"), str(o, "cle")})
	}
	_, id := configuration(str(conf, "imageDigest"), ms)
	if conf["v"] != 1.0 || len(conf) != 3 || len(ms) != 3 || own == nil {
		return refusal(400, "message_refuse")
	}
	ok := !busy && str(conf, "imageDigest") == own.ImageDigest && sort.SliceIsSorted(ms, func(i, j int) bool { return ms[i].VM < ms[j].VM })
	for _, m := range ms {
		switch m.VM {
		case own.Name:
			ok = ok && m == Membre{g.NodeID, own.Name, own.Zone, base64.StdEncoding.EncodeToString(g.cle)}
		case c.peer.VM:
			ok = ok && m == *c.peer
		default:
			ok = ok && slices.Contains(peers(own.Name), m.VM)
			if ok {
				k, err := g.connect(ctx, m.VM)
				ok = err == nil && k.peer == m
			}
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !ok || g.n != nil || g.resa != nil {
		return refusal(409, "amorcage_refuse")
	}
	g.resa = &resa{id: id, from: c.peer.NodeID, membres: ms, until: g.cfg.Horloge.Now().Add(Reservation)}
	return answer(jcs.Object{"reservee": true, "grappeId": id})
}

// demarrer décide de grappe/demarrer : de la VM qui a fait réserver, pour la
// configuration réservée, dans le délai.
func (g *Grappe) demarrer(c *conn, id string) (int, []byte) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.resa
	if r == nil || r.id != id || r.from != c.peer.NodeID || g.n != nil || !g.cfg.Horloge.Now().Before(r.until) {
		return refusal(409, "amorcage_refuse")
	}
	g.start(id, r.membres)
	return answer(jcs.Object{"demarree": true})
}

// Membres inscrit une nouvelle liste des places (grappe/membres, API.md
// §4.2), au leader : même grappe, zone d'un seul membre changée, ce membre
// injoignable depuis 2 minutes, aucune autre modification en cours.
func (g *Grappe) Membres(ctx context.Context, grappeID string, places map[string]string) string {
	g.mu.Lock()
	if g.n == nil || g.lead() != g.id {
		g.mu.Unlock()
		return map[bool]string{true: "membres_refuses", false: "pin_indisponible_temporaire"}[g.n == nil]
	}
	changed := 0
	for vm, z := range places {
		if old, ok := g.n.s.places[vm]; !ok {
			changed = 2
		} else if old != z {
			changed++
			for _, v := range g.n.cs.GetVoters() {
				if g.n.s.nodes[v].VM == vm && !g.absent[v] {
					changed = 2
				}
			}
		}
	}
	ok := grappeID == g.n.s.id && changed == 1 && len(places) == len(g.n.s.places) && !g.pending && len(g.n.cs.GetLearners()) == 0
	g.mu.Unlock()
	if !ok {
		return "membres_refuses"
	}
	if _, err := g.propose(ctx, entry{Places: places}); err != nil {
		return "pin_indisponible_temporaire"
	}
	return ""
}
