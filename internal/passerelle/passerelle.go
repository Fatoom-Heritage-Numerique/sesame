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

// MaxPending borne les notifications en attente d'un nouvel essai (choix de
// ce dépôt) ; au-delà, la notification est journalisée comme perdue.
const (
	MaxPending = 10_000
	retryFor   = 24 * time.Hour
	firstDelay = time.Minute
	maxDelay   = time.Hour
)

// Client envoie à la passerelle au nom d'une instance.
type Client struct {
	KMS                ports.KMS
	Alea               ports.Alea
	Horloge            ports.Horloge
	Passerelle         ports.Passerelle
	Journal            ports.Journal
	Kid                string
	SessionPub, KemPub []byte
	Token              func() string
	mu                 sync.Mutex
	pending            []*pending
}

type pending struct {
	name, requestID string
	body            []byte
	first, next     time.Time
	delay           time.Duration
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
// 200 {} la met en attente d'un nouvel essai, même X-Sesame-Request-Id.
func (c *Client) Notify(ctx context.Context, name string, body jcs.Object) {
	b, err := jcs.Marshal(body)
	id, err2 := c.uuid()
	if err != nil || err2 != nil {
		c.Journal.Write(ports.Event{Event: "notification_perdue", Resultat: name})
		return
	}
	now := c.Horloge.Now()
	if accepted(c.Send(ctx, "POST", "/"+name, id, b)) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pending) >= MaxPending {
		c.Journal.Write(ports.Event{Event: "notification_perdue", Resultat: name})
		return
	}
	c.pending = append(c.pending, &pending{name, id, b, now, now.Add(firstDelay), firstDelay})
}

// Retry refait les essais dus ; au-delà de 24 h après le premier, la
// notification est journalisée comme perdue.
func (c *Client) Retry(ctx context.Context) {
	now := c.Horloge.Now()
	c.mu.Lock()
	var due, keep []*pending
	for _, p := range c.pending {
		if now.Before(p.next) {
			keep = append(keep, p)
		} else {
			due = append(due, p)
		}
	}
	c.pending = keep
	c.mu.Unlock()
	for _, p := range due {
		if accepted(c.Send(ctx, "POST", "/"+p.name, p.requestID, p.body)) {
			continue
		}
		p.delay = min(2*p.delay, maxDelay)
		if p.next = now.Add(p.delay); p.next.Sub(p.first) > retryFor {
			c.Journal.Write(ports.Event{Event: "notification_perdue", Resultat: p.name})
			continue
		}
		c.mu.Lock()
		c.pending = append(c.pending, p)
		c.mu.Unlock()
	}
}

// Pending rend le nombre de notifications en attente.
func (c *Client) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}
