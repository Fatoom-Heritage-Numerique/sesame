// Paquet passerelle : les requêtes de Sésame à la passerelle du client
// (API.md §5) : six en-têtes, signature session-sign sur le message de
// jetons.GatewayMessage, et notifications réessayées pendant 24 h avec un
// délai croissant (1 min, puis le double, plafonné à 1 h), en mémoire de la
// VM qui a commencé l'envoi, bornée.
package passerelle

import (
	"context"
	"encoding/hex"
	"sync"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// MaxPending borne les essais en attente (choix de ce dépôt) ; au-delà,
// l'envoi est tenu pour perdu.
const (
	MaxPending = 10_000
	retryFor   = 24 * time.Hour
	firstDelay = time.Minute
	maxDelay   = time.Hour
)

// Retrier refait, en mémoire de la VM, les envois refusés : délai de 1 min,
// puis le double du précédent, plafonné à 1 h, jusqu'à 24 h après le premier
// essai (API.md §5.5) ; puis lost. Il sert aux notifications et aux alertes
// au titulaire.
type Retrier struct {
	Horloge ports.Horloge
	mu      sync.Mutex
	jobs    []*job
}

type job struct {
	try         func(ctx context.Context) bool
	lost        func()
	first, next time.Time
	delay       time.Duration
}

// Do essaie try ; un échec le met en attente d'un nouvel essai.
func (r *Retrier) Do(ctx context.Context, try func(ctx context.Context) bool, lost func()) {
	now := r.Horloge.Now()
	if try(ctx) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.jobs) >= MaxPending {
		lost()
		return
	}
	r.jobs = append(r.jobs, &job{try, lost, now, now.Add(firstDelay), firstDelay})
}

// Retry refait les essais dus.
func (r *Retrier) Retry(ctx context.Context) {
	now := r.Horloge.Now()
	r.mu.Lock()
	var due, keep []*job
	for _, j := range r.jobs {
		if now.Before(j.next) {
			keep = append(keep, j)
		} else {
			due = append(due, j)
		}
	}
	r.jobs = keep
	r.mu.Unlock()
	for _, j := range due {
		if j.try(ctx) {
			continue
		}
		j.delay = min(2*j.delay, maxDelay)
		if j.next = now.Add(j.delay); j.next.Sub(j.first) > retryFor {
			j.lost()
			continue
		}
		r.mu.Lock()
		r.jobs = append(r.jobs, j)
		r.mu.Unlock()
	}
}

// Pending rend le nombre d'envois en attente.
func (r *Retrier) Pending() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.jobs)
}

// Client envoie à la passerelle au nom d'une instance.
type Client struct {
	*Retrier
	KMS                ports.KMS
	Alea               ports.Alea
	Horloge            ports.Horloge
	Passerelle         ports.Passerelle
	Journal            ports.Journal
	Kid                string
	SessionPub, KemPub []byte
	Token              func() string
}

func (c *Client) uuid() (string, error) {
	b, err := c.Alea.Read(16)
	if err != nil {
		return "", err
	}
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}

// Send signe et envoie une requête ; requestID vide en tire un neuf.
func (c *Client) Send(ctx context.Context, method, pathQuery, requestID string, body []byte) (int, []byte, error) {
	if requestID == "" {
		var err error
		if requestID, err = c.uuid(); err != nil {
			return 0, nil, err
		}
	}
	date := c.Horloge.Now().UTC().Format(primitives.TimeLayout)
	msg, err := jetons.GatewayMessage(method, pathQuery, date, requestID, body)
	if err != nil {
		return 0, nil, err
	}
	der, err := c.KMS.Sign(ctx, ports.SessionSign, msg)
	if err != nil {
		return 0, nil, err
	}
	sig, err := jetons.DERToRaw(der)
	if err != nil {
		return 0, nil, err
	}
	return c.Passerelle.Send(ctx, method, pathQuery, map[string]string{
		"X-Sesame-Attestation": c.Token(), "X-Sesame-Session-Pub": primitives.B64Encode(c.SessionPub),
		"X-Sesame-Kem-Pub": primitives.B64Encode(c.KemPub), "X-Sesame-Date": date, "X-Sesame-Request-Id": requestID,
		"X-Sesame-Signature": primitives.B64Encode(sig), "Content-Type": "application/json",
	}, body)
}

func accepted(status int, resp []byte, err error) bool {
	return err == nil && status == 200 && string(resp) == "{}"
}

// Notify envoie la notification name (API.md §5.5) ; une réponse autre que
// 200 {} la fait réessayer, même X-Sesame-Request-Id, signée de nouveau.
func (c *Client) Notify(ctx context.Context, name string, body jcs.Object) {
	b, err := jcs.Marshal(body)
	id, err2 := c.uuid()
	lost := func() { c.Journal.Write(ports.Event{Event: "notification_perdue", Resultat: name}) }
	if err != nil || err2 != nil {
		lost()
		return
	}
	c.Do(ctx, func(ctx context.Context) bool { return accepted(c.Send(ctx, "POST", "/"+name, id, b)) }, lost)
}
