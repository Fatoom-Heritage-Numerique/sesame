package factice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/routes"
)

// Options règle un Fake : horloge (réelle si nil), constante reprise de la
// version factice, taille du filtre de Bloom, passerelle (celle de test en
// mémoire si nil).
type Options struct {
	Clock      *Clock
	Reprise    *registre.Reprise
	BloomBytes int
	Passerelle ports.Passerelle
}

// Fake est sesame-fake assemblé : ses doubles, partagés par ses instances
// comme le registre et les buckets le sont par les VM, et son serveur.
type Fake struct {
	Config    *Config
	Clock     *Clock
	KMS       *KMS
	Base      *Base
	Stockage  *Stockage
	Journal   *Journal
	Gateway   *Gateway
	Outbox    *Outbox
	Identity  *Identity
	Post      *Post
	Launchers []*Launcher
	Server    *routes.Server
	rc        *routes.Config
	mu        sync.Mutex
	// LauncherDown fait échouer le lanceur des instances démarrées ensuite.
	LauncherDown bool
}

// New assemble sesame-fake : trois instances a, b, c en grappe locale, une
// seule en mode instance.
func New(ctx context.Context, cfg *Config, opt Options) (*Fake, error) {
	if opt.Clock == nil {
		opt.Clock = &Clock{}
	}
	f := &Fake{Config: cfg, Clock: opt.Clock, KMS: NewKMS(cfg.sessionKey, cfg.SessionSign.Kid), Journal: &Journal{},
		Gateway: &Gateway{Check: &Check{Config: cfg, Clock: opt.Clock}}, Outbox: &Outbox{}, Identity: &Identity{}, Post: &Post{}}
	f.Base = &Base{Clock: f.Clock}
	f.Stockage = &Stockage{Clock: f.Clock}
	gw := opt.Passerelle
	if gw == nil {
		gw = f.Gateway
	}
	env := map[string]string{}
	for k, v := range cfg.EnvOverride {
		env[k] = v
	}
	f.rc = &routes.Config{Ports: routes.Ports{Horloge: f.Clock, Alea: Rand{}, KMS: f.KMS, Base: f.Base, Stockage: f.Stockage,
		Secrets: Secrets{routes.SecretTextes: cfg.textes}, Journal: f.Journal, Emetteurs: &Remote{Static: cfg.issuers, URLs: cfg.EmetteursDistants, Client: &http.Client{Timeout: 10 * time.Second}, Clock: f.Clock}, Passerelle: gw,
		Sms: f.Outbox, Email: f.Outbox, Identite: f.Identity, Courrier: f.Post}, Env: env, ImageDigest: cfg.imageDigest, SessionKid: cfg.SessionSign.Kid,
		Reprise: opt.Reprise, BloomBytes: opt.BloomBytes}
	prefixes := []string{"a", "b", "c"}
	if cfg.Mode == "instance" {
		prefixes = []string{cfg.Prefixe}
	}
	f.Server = &routes.Server{}
	for _, p := range prefixes {
		in, l, err := f.instance(ctx, p)
		if err != nil {
			return nil, err
		}
		f.Server.Instances = append(f.Server.Instances, in)
		f.Launchers = append(f.Launchers, l)
	}
	return f, nil
}

func (f *Fake) instance(ctx context.Context, prefix string) (*routes.Instance, *Launcher, error) {
	n, _ := Rand{}.Read(8)
	gce := new(big.Int).SetBytes(n).String()
	l, err := NewLauncher(f.Config, f.Clock, Claims{InstanceName: f.Config.NomVM + "-" + prefix, Zone: "europe-west9-" + prefix, InstanceID: gce})
	if err != nil {
		return nil, nil, err
	}
	l.Fail = f.LauncherDown
	rc := *f.rc
	rc.Lanceur = l
	in, err := routes.NewInstance(ctx, &rc, prefix, gce)
	return in, l, err
}

// Restart remplace l'instance de ce préfixe par une neuve (VM redémarrée) :
// nouvel instanceId, canaux, défis et sessions perdus, registre gardé.
func (f *Fake) Restart(ctx context.Context, prefix string) error {
	in, l, err := f.instance(ctx, prefix)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, o := range f.Server.Instances {
		if o.ID[:1] == prefix {
			f.Server.Instances[i], f.Launchers[i] = in, l
			return nil
		}
	}
	return fmt.Errorf("préfixe %s absent", prefix)
}

// Run tient la boucle de fond de chaque instance jusqu'à la fin de ctx.
func (f *Fake) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, in := range f.Server.Instances {
		wg.Add(1)
		go func() { defer wg.Done(); in.Run(ctx) }()
	}
	wg.Wait()
}

// HTTPGateway envoie à une passerelle réelle (émulateur du client), en HTTP.
type HTTPGateway struct {
	Base   string
	Client *http.Client
}

// MaxGatewayBody borne une réponse de la passerelle (choix de ce dépôt).
const MaxGatewayBody = 1 << 20

func (g HTTPGateway) Send(ctx context.Context, method, pq string, h map[string]string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, g.Base+pq, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	for k, v := range h {
		req.Header.Set(k, v)
	}
	c := g.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxGatewayBody))
	return resp.StatusCode, b, err
}

// LimitListener borne les connexions acceptées ensemble à n.
func LimitListener(l net.Listener, n int) net.Listener {
	return &limitListener{Listener: l, sem: make(chan struct{}, n)}
}

type limitListener struct {
	net.Listener
	sem chan struct{}
}

func (l *limitListener) Accept() (net.Conn, error) {
	l.sem <- struct{}{}
	c, err := l.Listener.Accept()
	if err != nil {
		<-l.sem
		return nil, err
	}
	return &limitConn{Conn: c, release: func() { <-l.sem }}, nil
}

type limitConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *limitConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.release)
	return err
}
