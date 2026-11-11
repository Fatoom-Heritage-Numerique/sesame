package canal

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Durées et tailles du canal (API.md §1.2, §1.5, §7) ; MaxChannels et
// MaxChallenges sont des choix de ce dépôt : au-delà de MaxChannels, le canal
// resté le plus longtemps sans requête est fermé comme après 1 h ;
// MaxChallenges couvre deux fenêtres de la limite de defi (30 / h par canal)
// pendant la vie d'un défi.
const (
	Idle          = time.Hour
	ChallengeLife = 5 * time.Minute
	BloomBytes    = 16 << 20
	MaxChannels   = 100_000
	MaxChallenges = 60
)

// ErrChannel est le refus en clair 400 canal_invalide.
const ErrChannel = primitives.Error("canal_invalide")

// Channel est l'état d'un canal ouvert (instanceId, cliPub) : ses clés,
// l'empreinte de sa forme (kemCt, ivPrefixe), le dernier n accepté, ses
// défis. Key vaut cliPub.
type Channel struct {
	Key        string
	KReq, KRes []byte
	Prefix     []byte
	shape      []byte
	lastN      int64
	seen       time.Time
	challenges map[string]challenge
	fresh      bool
}

type challenge struct {
	uid    string
	expiry time.Time
}

// Table tient les canaux d'une instance, au plus limit, et le filtre de Bloom
// de taille fixe des canaux qu'elle a fermés, gardé jusqu'à son arrêt.
// OnClose est appelé, hors verrou, pour chaque canal fermé.
type Table struct {
	mu       sync.Mutex
	channels map[string]*Channel
	bloom    []uint64
	salt     []byte
	max      int
	OnClose  func(c *Channel)
}

// NewTable rend une table vide ; bloomBytes est la taille du filtre, salt de
// l'aléa propre à l'instance.
func NewTable(bloomBytes, limit int, salt []byte) *Table {
	return &Table{channels: map[string]*Channel{}, bloom: make([]uint64, max(bloomBytes/8, 1)), salt: salt, max: limit}
}

func (t *Table) bits(key string) [4]uint64 {
	h := primitives.SHA256(t.salt, []byte(key))
	m := uint64(len(t.bloom)) * 64
	var b [4]uint64
	for i := range b {
		b[i] = binary.BigEndian.Uint64(h[8*i:]) % m
	}
	return b
}

func (t *Table) closed(key string) bool {
	for _, b := range t.bits(key) {
		if t.bloom[b/64]&(1<<(b%64)) == 0 {
			return false
		}
	}
	return true
}

// Prepare rend le canal de l'enveloppe e. Connu, il doit porter les mêmes
// kemCt et ivPrefixe et un n plus grand que le dernier accepté ; neuf, ses
// clés sont dérivées par in, et il n'entre dans la table qu'à Accept, une
// fois sa première requête déchiffrée. Tout refus est ErrChannel.
func (t *Table) Prepare(in *Instance, e *Envelope) (*Channel, error) {
	key, shape := string(e.CliPub), primitives.SHA256(e.KemCt, e.IVPrefix)
	t.mu.Lock()
	c, known := t.channels[key]
	ok := known && primitives.Equal(c.shape, shape) && e.N > c.lastN
	closed := !known && t.closed(key)
	t.mu.Unlock()
	switch {
	case known && !ok, closed:
		return nil, ErrChannel
	case known:
		return c, nil
	}
	kReq, kRes, err := in.Keys(e.CliPub, e.KemCt)
	if err != nil {
		return nil, ErrChannel
	}
	return &Channel{Key: key, KReq: kReq, KRes: kRes, Prefix: e.IVPrefix, shape: shape, challenges: map[string]challenge{}, fresh: true}, nil
}

// Accept avance le dernier n du canal une fois sa requête n déchiffrée, et
// inscrit un canal neuf ; un n qui n'est plus croissant (requête concurrente)
// est ErrChannel.
func (t *Table) Accept(c *Channel, n int64, now time.Time) error {
	var evicted *Channel
	t.mu.Lock()
	switch {
	case c.fresh:
		if _, dup := t.channels[c.Key]; dup || t.closed(c.Key) {
			t.mu.Unlock()
			return ErrChannel
		}
		if len(t.channels) >= t.max {
			for _, o := range t.channels {
				if evicted == nil || o.seen.Before(evicted.seen) {
					evicted = o
				}
			}
			t.close(evicted)
		}
		c.fresh = false
		t.channels[c.Key] = c
	case t.channels[c.Key] != c || n <= c.lastN:
		t.mu.Unlock()
		return ErrChannel
	}
	c.lastN, c.seen = n, now
	t.mu.Unlock()
	t.notify(evicted)
	return nil
}

func (t *Table) close(c *Channel) {
	delete(t.channels, c.Key)
	for _, b := range t.bits(c.Key) {
		t.bloom[b/64] |= 1 << (b % 64)
	}
	clear(c.KReq)
	clear(c.KRes)
	c.challenges = nil
}

func (t *Table) notify(cs ...*Channel) {
	for _, c := range cs {
		if c != nil && t.OnClose != nil {
			t.OnClose(c)
		}
	}
}

// CloseIdle ferme les canaux sans requête depuis 1 h.
func (t *Table) CloseIdle(now time.Time) {
	var closed []*Channel
	t.mu.Lock()
	for _, c := range t.channels {
		if now.Sub(c.seen) >= Idle {
			t.close(c)
			closed = append(closed, c)
		}
	}
	t.mu.Unlock()
	t.notify(closed...)
}

// DrawChallenge lie un défi au canal et à l'uid pour 5 minutes (API.md
// §1.5) ; au-delà de MaxChallenges défis vivants, il n'est pas gardé.
func (t *Table) DrawChallenge(c *Channel, value, uid string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if c.challenges == nil {
		return
	}
	for k, d := range c.challenges {
		if !now.Before(d.expiry) {
			delete(c.challenges, k)
		}
	}
	if len(c.challenges) < MaxChallenges {
		c.challenges[value] = challenge{uid, now.Add(ChallengeLife)}
	}
}

// ConsumeChallenge efface le défi présenté sur ce canal et dit s'il était
// valable : tiré sur ce canal, pour cet uid, depuis moins de 5 minutes.
func (t *Table) ConsumeChallenge(c *Channel, value, uid string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	d, ok := c.challenges[value]
	delete(c.challenges, value)
	return ok && d.uid == uid && now.Before(d.expiry)
}

// Len rend le nombre de canaux ouverts.
func (t *Table) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.channels)
}
