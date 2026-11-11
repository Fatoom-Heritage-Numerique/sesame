package registre

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

type lastEpoch struct {
	mu sync.Mutex
	v  int64
}

func (l *lastEpoch) raise(e int64) {
	l.mu.Lock()
	l.v = max(l.v, e)
	l.mu.Unlock()
}

func (l *lastEpoch) get() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.v
}

// Reprise est la constante reprise compilée dans une version (§6.4) :
// époque et empreinte de la sauvegarde restaurée, ou SansSauvegarde.
type Reprise struct {
	Epoque         int64
	Empreinte      string
	SansSauvegarde bool
}

// Tailles de la borne (choix de ce dépôt : la spécification ne borne que par
// le nombre de ressources distinctes) et période de dépôt (§8.1).
const (
	MaxEvents   = 10_000
	BornePeriod = 10 * time.Minute
	maxListed   = 2_000
)

// Grappe rend l'identité de la grappe publiée dans la borne : grappeId (nil
// sans grappe) et nombre de votants.
type Grappe func() (grappeID any, votants int)

// Borne tient la borne du registre d'une instance (§8.1) et le contrôle
// anti-retour (§6.3) : événements vus depuis la borne précédente, dépôt
// signé par session-sign en création seule, relecture des bornes des 3
// dernières heures.
type Borne struct {
	R                                   *Registre
	KMS                                 ports.KMS
	Stockage                            ports.Stockage
	Horloge                             ports.Horloge
	Kid, InstanceID, ImageDigest, GCEID string
	Grappe                              Grappe
	Reprise                             *Reprise
	mu                                  sync.Mutex
	events                              map[[2]string]string
	listed                              []Deposited
	keys                                map[string][]byte
	ownSeen, unreadable                 bool
}

// Signal note un événement (type, ressource) à sa première occurrence de la
// période (vuLe).
func (b *Borne) Signal(kind, resource string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.events == nil {
		b.events = map[[2]string]string{}
	}
	k := [2]string{kind, resource}
	if _, ok := b.events[k]; !ok && len(b.events) < MaxEvents {
		b.events[k] = b.Horloge.Now().UTC().Format(primitives.TimeLayout)
	}
}

// Load lit les clés de session-sign (toute version activée) et les bornes des
// 3 dernières heures, au démarrage.
func (b *Borne) Load(ctx context.Context) error {
	keys, err := b.KMS.PublicKeys(ctx, ports.SessionSign)
	if err != nil {
		return ErrUnavailable
	}
	b.mu.Lock()
	b.keys = keys
	b.mu.Unlock()
	return b.relist(ctx)
}

func (b *Borne) relist(ctx context.Context) error {
	now := b.Horloge.Now().UTC()
	var listed []Deposited
	for h := 3; h >= 0; h-- {
		objs, err := b.Stockage.List(ctx, ports.BucketAudit, now.Add(-time.Duration(h)*time.Hour).Format("bornes/2006/01/02/15-"), maxListed)
		if err != nil {
			b.mu.Lock()
			b.unreadable = true
			b.mu.Unlock()
			return ErrUnavailable
		}
		for _, o := range objs {
			v, err := jcs.Parse(o.Content)
			if err == nil {
				listed = append(listed, Deposited{o.Name, v, o.Created})
			}
		}
	}
	b.mu.Lock()
	b.listed, b.unreadable = listed, false
	b.mu.Unlock()
	return nil
}

// UpToDate fait le contrôle anti-retour à l'heure du contrôle, sur la
// dernière lecture du bucket : le registre lu n'est plus ancien ni que « la
// dernière borne », ni que la dernière époque écrite par l'instance. En échec
// (dernière lecture du bucket ou registre illisible compris), l'événement
// registre_ancien est noté.
func (b *Borne) UpToDate(ctx context.Context) bool {
	read, _, err := b.R.HeadEpoch(ctx)
	b.mu.Lock()
	bornes := b.listed
	if b.Reprise != nil {
		own := bornes[:0:0]
		for _, d := range bornes {
			f, _ := d.File.(jcs.Object)
			if bo, _ := f["borne"].(jcs.Object); bo != nil && bo["imageDigest"] == b.ImageDigest {
				own = append(own, d)
			}
		}
		bornes = own
	}
	ref, ok := Reference(bornes, b.keys, b.Horloge.Now())
	if b.Reprise != nil && !ok && !b.ownSeen && len(bornes) == 0 {
		ref, ok = b.Reprise.Epoque, true
	}
	b.ownSeen = b.ownSeen || (b.Reprise != nil && len(bornes) > 0)
	up := err == nil && b.keys != nil && !b.unreadable && UpToDate(read, ref, ok, b.R.last.get())
	b.mu.Unlock()
	if !up {
		b.Signal("registre_ancien", Head)
	}
	return up
}

// Deposit dépose la borne de la période (création seule, §8.1), puis relit les
// bornes. La borne porte registre_ancien tant que le contrôle échoue.
func (b *Borne) Deposit(ctx context.Context) error {
	b.UpToDate(ctx)
	epoch, head, err := b.R.HeadEpoch(ctx)
	if err != nil {
		return err
	}
	sum, _ := head["empreinte"].(string)
	if head == nil {
		sum = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	gid, votants := b.Grappe()
	b.mu.Lock()
	events := jcs.Array{}
	for k, vu := range b.events {
		events = append(events, jcs.Object{"type": k[0], "ressource": k[1], "vuLe": vu})
	}
	b.events = nil
	b.mu.Unlock()
	borne := jcs.Object{"v": 1, "instanceId": b.InstanceID, "imageDigest": b.ImageDigest, "gceInstanceId": b.GCEID,
		"borneLe": b.Horloge.Now().UTC().Format(primitives.TimeLayout), "grappe": jcs.Object{"grappeId": gid, "votants": votants},
		"registre": jcs.Object{"epoque": epoch, "empreinte": sum}, "evenements": events}
	j, _ := jcs.Marshal(borne)
	v, _ := jcs.Parse(j)
	msg, err := BorneMessage(v)
	if err != nil {
		return err
	}
	name, _ := BorneName(v)
	der, err := b.KMS.Sign(ctx, ports.SessionSign, msg)
	if err != nil {
		return ErrUnavailable
	}
	raw, err := jetons.DERToRaw(der)
	if err != nil {
		return err
	}
	file, _ := jcs.Marshal(jcs.Object{"borne": borne, "signature": base64.RawURLEncoding.EncodeToString(raw), "kid": b.Kid})
	if err := b.Stockage.Create(ctx, ports.BucketAudit, name, file); err != nil {
		return ErrUnavailable
	}
	return b.relist(ctx)
}
