package grappe

import (
	"encoding/json"
	"math/rand/v2"
	"strconv"
	"testing"
	"time"

	"go.etcd.io/raft/v3"
	pb "go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/pin"
)

// Nœuds Raft reliés à la main, sans transport : « Journal Raft borné » et
// « Entrées malformées » (tests.md du client, « Grappe du PIN »).

var t0 = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

type banc struct {
	t     *testing.T
	nodes map[uint64]*noeud
	out   []*pb.Message
	lost  int
}

func (b *banc) ready(n *noeud) {
	n.ready(t0, &b.out, func(uint64, result) {}, func(string) { b.lost++ })
}

// pump délivre les messages jusqu'au calme.
func (b *banc) pump() {
	for range 1000 {
		if len(b.out) == 0 {
			return
		}
		out := b.out
		b.out = nil
		for _, m := range out {
			if n := b.nodes[m.GetTo()]; n != nil {
				_ = n.rn.Step(m)
				b.ready(n)
			}
		}
	}
}

func membre(id uint64) []byte {
	c, _ := json.Marshal(struct{ Membre Membre }{Membre{NodeID: strconv.FormatUint(id, 10), VM: "vm-" + strconv.FormatUint(id, 10)}})
	return c
}

// seul rend un banc d'un votant élu.
func seul(t *testing.T) (*banc, *noeud) {
	n, err := newNoeud(1, []raft.Peer{{ID: 1, Context: membre(1)}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	b := &banc{t: t, nodes: map[uint64]*noeud{1: n}}
	b.ready(n)
	if err := n.rn.Campaign(); err != nil {
		t.Fatal(err)
	}
	b.ready(n)
	return b, n
}

func (b *banc) propose(n *noeud, e entry) {
	data, _ := json.Marshal(e)
	if err := n.rn.Propose(data); err != nil {
		b.t.Fatal(err)
	}
	b.ready(n)
	b.pump()
}

func TestJournalBorne(t *testing.T) { // « Journal Raft borné » : instantané au seuil, journal compacté, apprenant rattrapé depuis l'instantané
	b, n := seul(t)
	key, _ := pin.NewKey(make([]byte, 32))
	for i := range 3 * SnapEvery {
		b.propose(n, entry{Op: &pin.Op{UID: "u" + strconv.Itoa(i%100), Motif: "nouveau_pin", Key: key, At: t0}})
	}
	first, _ := n.st.FirstIndex()
	last, _ := n.st.LastIndex()
	if last-first > SnapEvery+KeepEntries || n.snap == 0 || len(n.s.records) != 100 {
		t.Fatalf("journal de %d entrées, instantané %d", last-first, n.snap)
	}
	// Un apprenant ajouté après la compaction reçoit l'instantané.
	l, _ := newNoeud(2, nil, t0)
	b.nodes[2] = l
	cc := &pb.ConfChange{Type: pb.ConfChangeAddLearnerNode.Enum(), NodeId: new(uint64(2)), Context: membre(2)}
	if err := n.rn.ProposeConfChange(cc); err != nil {
		t.Fatal(err)
	}
	b.ready(n)
	b.pump()
	for range 20 {
		n.rn.Tick()
		b.ready(n)
		b.pump()
	}
	if len(l.s.records) != 100 || l.s.nodes[1].VM != "vm-1" || l.applied < n.snap {
		t.Fatalf("apprenant : %d enregistrements, appliqué %d", len(l.s.records), l.applied)
	}
}

func TestEntreesFautives(t *testing.T) { // « Entrées malformées » : appliquées sans panique, rejetées, journalisées
	b, n := seul(t)
	for _, data := range [][]byte{[]byte("{"), []byte(`{"op":{"uid":"u","motif":"inconnu"}}`), []byte(`{"places":{"x":"y"}}`), []byte(`{"op":{"uid":"u"}}`)} {
		if err := n.rn.Propose(data); err != nil {
			t.Fatal(err)
		}
		b.ready(n)
	}
	if b.lost != 3 || len(n.s.records) != 0 || len(n.s.places) != 0 { // l'essai sur un enregistrement absent est refusé par pin.Apply
		t.Fatalf("entrées fautives : %d rejetées, %v", b.lost, n.s)
	}
	cc, _ := proto.Marshal(&pb.ConfChange{Type: pb.ConfChangeAddNode.Enum(), NodeId: new(uint64(9)), Context: []byte("{")})
	n.apply(&pb.Entry{Type: pb.EntryConfChange.Enum(), Index: new(n.applied + 1), Data: cc}, func(uint64, result) {}, func(string) { b.lost++ })
	if b.lost != 4 || len(n.s.nodes) != 1 {
		t.Fatal("modification de membres fautive appliquée")
	}
}

func TestInstantaneSansPanique(t *testing.T) { // instantané : relu à l'identique ; forme fausse rejetée, sans panique
	s := newEtat()
	s.id, s.places["vm-a"] = "id", "z"
	s.nodes[7] = Membre{"n", "vm-a", "z", "c"}
	s.removed["vieux"] = true
	s.records["u"] = &pin.Record{Key: make([]byte, 32), Left: 3, Next: t0, Created: t0, Evals: 7}
	s.records["v"] = &pin.Record{Left: 0, Created: t0, Evals: 10}
	b := s.encode()
	got, ok := decode(b)
	if !ok || got.id != "id" || got.nodes[7] != s.nodes[7] || !got.removed["vieux"] || got.records["u"].Evals != 7 ||
		!got.records["u"].Next.Equal(t0) || got.records["v"].Key != nil || !got.records["v"].Next.IsZero() {
		t.Fatalf("instantané relu : %+v", got)
	}
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 2000 {
		c := append([]byte{}, b...)
		switch i % 3 {
		case 0:
			c = c[:r.IntN(len(c))]
		case 1:
			c[r.IntN(len(c))] ^= byte(1 + r.IntN(255))
		default:
			c = make([]byte, r.IntN(64))
			for k := range c {
				c[k] = byte(r.Uint32())
			}
		}
		decode(c) // ne panique pas
	}
	if _, ok := decode(b[:len(b)-1]); ok {
		t.Fatal("instantané tronqué accepté")
	}
}
