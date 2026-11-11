package grappe

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"time"

	"go.etcd.io/raft/v3"
	pb "go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/pin"
)

// Réglages de Raft (protocole-pin.md, « Noms et pairs ») : tick de 100 ms,
// élection à 10 ticks, battement à chaque tick ; instantané toutes les
// 10 000 entrées appliquées et au moins une fois par heure, journal compacté
// aux 5 000 dernières. MaxRemoved borne les identifiants retirés gardés.
const (
	Tick        = 100 * time.Millisecond
	SnapEvery   = 10_000
	KeepEntries = 5_000
	MaxRemoved  = 1_000
)

// Membre est un membre de la configuration initiale (API.md §4.3) ; il
// accompagne chaque modification de membres.
type Membre struct {
	NodeID string `json:"nodeId"`
	VM     string `json:"vm"`
	Zone   string `json:"zone"`
	Cle    string `json:"cle"`
}

// etat est l'état répliqué : identité de la grappe, liste inscrite des places
// (vm → zone), nœuds de la configuration Raft, identifiants retirés et
// enregistrements du PIN. Il n'est jamais écrit hors de la mémoire.
type etat struct {
	id      string
	places  map[string]string
	nodes   map[uint64]Membre
	removed map[string]bool
	records map[string]*pin.Record
}

// entry est une entrée normale du journal : une opération du PIN, la
// configuration initiale (Init, puis places) ou une nouvelle liste inscrite.
type entry struct {
	ID     uint64            `json:"id"`
	From   uint64            `json:"de"`
	Op     *pin.Op           `json:"op,omitempty"`
	Init   string            `json:"init,omitempty"`
	Places map[string]string `json:"places,omitempty"`
}

type result struct {
	code string
	rec  pin.Record
}

// noeud est un nœud Raft en mémoire seulement (MemoryStorage).
type noeud struct {
	id      uint64
	rn      *raft.RawNode
	st      *raft.MemoryStorage
	s       etat
	cs      *pb.ConfState
	applied uint64
	snap    uint64
	snapLe  time.Time
}

func newEtat() etat {
	return etat{places: map[string]string{}, nodes: map[uint64]Membre{}, removed: map[string]bool{}, records: map[string]*pin.Record{}}
}

func newNoeud(id uint64, peers []raft.Peer, now time.Time) (*noeud, error) {
	st := raft.NewMemoryStorage()
	rn, err := raft.NewRawNode(&raft.Config{ID: id, ElectionTick: 10, HeartbeatTick: 1, Storage: st, MaxSizePerMsg: 1 << 20,
		MaxInflightMsgs: 256, CheckQuorum: true, PreVote: true, DisableProposalForwarding: true, StepDownOnRemoval: true,
		Logger: &raft.DefaultLogger{Logger: log.New(io.Discard, "", 0)}})
	if err == nil && peers != nil {
		err = rn.Bootstrap(peers)
	}
	return &noeud{id: id, rn: rn, st: st, s: newEtat(), cs: &pb.ConfState{}, snapLe: now}, err
}

// ready traite le travail de Raft : instantané reçu, entrées à garder,
// messages à envoyer (out), entrées validées à appliquer (apply rend le
// résultat d'une proposition de ce nœud).
func (n *noeud) ready(now time.Time, out *[]*pb.Message, done func(uint64, result), lost func(string)) {
	for n.rn.HasReady() {
		rd := n.rn.Ready()
		if !raft.IsEmptySnap(rd.Snapshot) && n.st.ApplySnapshot(rd.Snapshot) == nil {
			if s, ok := decode(rd.Snapshot.GetData()); ok {
				n.s, n.cs, n.applied = s, rd.Snapshot.GetMetadata().GetConfState(), rd.Snapshot.GetMetadata().GetIndex()
			} else {
				lost("instantane_rejete")
			}
		}
		_ = n.st.Append(rd.Entries)
		if !raft.IsEmptyHardState(rd.HardState) {
			_ = n.st.SetHardState(rd.HardState)
		}
		*out = append(*out, rd.Messages...)
		for _, e := range rd.CommittedEntries {
			n.apply(e, done, lost)
		}
		n.rn.Advance(rd)
	}
	if n.applied-n.snap >= SnapEvery || n.applied > n.snap && now.Sub(n.snapLe) >= time.Hour {
		if _, err := n.st.CreateSnapshot(n.applied, n.cs, n.s.encode()); err == nil {
			n.snap, n.snapLe = n.applied, now
			if n.applied > KeepEntries {
				_ = n.st.Compact(n.applied - KeepEntries)
			}
		}
	}
}

// apply applique une entrée validée sans panique : une entrée fautive est
// rejetée et journalisée (« Mémoire et robustesse »).
func (n *noeud) apply(e *pb.Entry, done func(uint64, result), lost func(string)) {
	n.applied = e.GetIndex()
	if e.GetType() == pb.EntryConfChange {
		var cc pb.ConfChange
		var c struct {
			ID, De uint64
			Membre Membre
		}
		if proto.Unmarshal(e.GetData(), &cc) != nil || json.Unmarshal(cc.GetContext(), &c) != nil {
			lost("entree_rejetee")
			return
		}
		n.cs = n.rn.ApplyConfChange(&cc)
		if id := cc.GetNodeId(); cc.GetType() == pb.ConfChangeRemoveNode {
			if m, ok := n.s.nodes[id]; ok && len(n.s.removed) < MaxRemoved {
				n.s.removed[m.NodeID] = true
			}
			delete(n.s.nodes, id)
		} else {
			n.s.nodes[id] = c.Membre
		}
		if c.De == n.id {
			done(c.ID, result{code: "conf"})
		}
		return
	}
	var en entry
	if len(e.GetData()) == 0 || e.GetType() != pb.EntryNormal {
		return
	}
	r := result{code: "entree_rejetee"}
	switch {
	case json.Unmarshal(e.GetData(), &en) != nil:
	case en.Op != nil:
		var rec *pin.Record
		if r.code, rec = pin.Apply(n.s.records, *en.Op); rec != nil {
			r.rec = *rec
		}
	case en.Init != "" && (n.s.id == "" || n.s.id == en.Init) && len(en.Places) == 3:
		n.s.id, n.s.places, r.code = en.Init, en.Places, ""
	case en.Places != nil && len(en.Places) == 3:
		n.s.places, r.code = en.Places, ""
	}
	if r.code == "entree_rejetee" {
		lost(r.code)
	}
	if en.From == n.id {
		done(en.ID, r)
	}
}

// encode écrit l'état en binaire pour un instantané : chaînes et octets
// préfixés de leur longueur, entiers en varint.
func (s etat) encode() []byte {
	b := binary.AppendUvarint(nil, uint64(len(s.places)))
	str := func(v string) { b = binary.AppendUvarint(b, uint64(len(v))); b = append(b, v...) }
	str(s.id)
	for vm, z := range s.places {
		str(vm)
		str(z)
	}
	b = binary.AppendUvarint(b, uint64(len(s.nodes)))
	for id, m := range s.nodes {
		b = binary.AppendUvarint(b, id)
		str(m.NodeID)
		str(m.VM)
		str(m.Zone)
		str(m.Cle)
	}
	b = binary.AppendUvarint(b, uint64(len(s.removed)))
	for id := range s.removed {
		str(id)
	}
	b = binary.AppendUvarint(b, uint64(len(s.records)))
	for uid, r := range s.records {
		str(uid)
		str(string(r.Key))
		next := r.Next.UnixMilli()
		if r.Next.IsZero() {
			next = 0
		}
		for _, v := range []int64{int64(r.Left), next, r.Created.UnixMilli(), r.Evals} {
			b = binary.AppendVarint(b, v)
		}
	}
	return b
}

// decode lit un instantané ; une forme fausse le fait rejeter, sans panique.
func decode(b []byte) (etat, bool) {
	s, bad := newEtat(), false
	u := func() uint64 {
		v, k := binary.Uvarint(b)
		bad = bad || k <= 0
		b = b[max(k, 0):]
		return v
	}
	i := func() int64 {
		v, k := binary.Varint(b)
		bad = bad || k <= 0
		b = b[max(k, 0):]
		return v
	}
	str := func() string {
		n := u()
		if bad || n > uint64(len(b)) {
			bad = true
			return ""
		}
		v := string(b[:n])
		b = b[n:]
		return v
	}
	places := u()
	s.id = str()
	for k := uint64(0); k < places && !bad; k++ {
		s.places[str()] = str()
	}
	for k, nodes := uint64(0), u(); k < nodes && !bad; k++ {
		s.nodes[u()] = Membre{str(), str(), str(), str()}
	}
	for k, removed := uint64(0), u(); k < removed && !bad; k++ {
		s.removed[str()] = true
	}
	for k, records := uint64(0), u(); k < records && !bad; k++ {
		uid, key := str(), []byte(str())
		left, next, created, evals := i(), i(), i(), i()
		r := &pin.Record{Key: key, Left: int(left), Next: time.UnixMilli(next).UTC(), Created: time.UnixMilli(created).UTC(), Evals: evals}
		if next == 0 {
			r.Next = time.Time{}
		}
		if len(key) == 0 {
			r.Key = nil
		}
		s.records[uid] = r
	}
	return s, !bad && len(b) == 0
}
