// Paquet routes : le serveur de Sésame (API.md §1 et §2) : GET /attestation,
// OPTIONS et POST /n/{a|b|c}/canal, enveloppes et erreurs en clair, puis une
// décision par route du canal, dans l'ordre des contrôles du §6 et sous les
// limites du §7. Il ne voit que les interfaces d'internal/ports (décision
// 0063).
package routes

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/canal"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/passerelle"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
)

// Tailles du canal (API.md §7).
const (
	MaxEnvelope  = 400 << 10
	MaxBody      = 64 << 10
	MaxBodyCles  = 256 << 10
	tokenPeriod  = 10 * time.Minute
	vuLePeriod   = time.Minute
	idlePeriod   = time.Minute
	attestLimitN = 60
)

// Ports regroupe les adaptateurs que branche le binaire.
type Ports struct {
	Horloge    ports.Horloge
	Alea       ports.Alea
	KMS        ports.KMS
	Base       ports.Base
	Stockage   ports.Stockage
	Secrets    ports.Secrets
	Journal    ports.Journal
	Emetteurs  ports.Emetteurs
	Lanceur    ports.Lanceur
	Passerelle ports.Passerelle
	Sms        ports.Sms
	Email      ports.Email
	Identite   ports.Identite
	Courrier   ports.Courrier
}

// Config est ce que le binaire assemble : ports, variables attestées
// (env_override), empreinte de l'image, version de session-sign fixée par
// l'image, constante reprise (nil hors reprise publiée) et taille du filtre
// de Bloom.
type Config struct {
	Ports
	Env         map[string]string
	ImageDigest string
	SessionKid  string
	Reprise     *registre.Reprise
	BloomBytes  int
	reglages    Reglages
}

// Instance est une VM de Sésame : son instanceId, ses clés de canal, ses
// canaux, son registre (époque écrite), sa borne, ses limites.
type Instance struct {
	cfg     *Config
	ID      string
	gceID   string
	keys    *canal.Instance
	table   *canal.Table
	reg     *registre.Registre
	borne   *registre.Borne
	notif   *passerelle.Client
	local   *limiter
	token   atomic.Pointer[string]
	refusal string
	started time.Time
}

// NewInstance démarre une instance de préfixe a, b ou c : tirage de
// l'instanceId et des clés de canal, lecture des clés KMS et des bornes,
// contrôle des variables attestées et du document des textes, inscription
// d'instances/{instanceId}. Un refus ne fait pas sortir : l'instance répond
// attestation_indisponible à toute route (décision 0058).
func NewInstance(ctx context.Context, cfg *Config, prefix, gceID string) (*Instance, error) {
	rnd, err := cfg.Alea.Read(16 + 32 + 64 + 16)
	if err != nil {
		return nil, err
	}
	id, err := canal.InstanceID(prefix, rnd[:16])
	if err != nil {
		return nil, err
	}
	keys, err := canal.NewInstance(rnd[16:48], rnd[48:112])
	if err != nil {
		return nil, err
	}
	in := &Instance{cfg: cfg, ID: id, gceID: gceID, keys: keys, local: newLimiter(), started: cfg.Horloge.Now()}
	in.table = canal.NewTable(cmp(cfg.BloomBytes, canal.BloomBytes), canal.MaxChannels, rnd[112:])
	in.reg = &registre.Registre{Base: cfg.Base, KMS: cfg.KMS, Stockage: cfg.Stockage, Horloge: cfg.Horloge}
	in.borne = &registre.Borne{R: in.reg, KMS: cfg.KMS, Stockage: cfg.Stockage, Horloge: cfg.Horloge, Kid: cfg.SessionKid,
		InstanceID: id, ImageDigest: cfg.ImageDigest, GCEID: gceID, Reprise: cfg.Reprise, Grappe: func() (any, int) { return nil, 0 }}
	in.reg.Event = in.borne.Signal
	in.notif = &passerelle.Client{KMS: cfg.KMS, Alea: cfg.Alea, Horloge: cfg.Horloge, Passerelle: cfg.Passerelle, Journal: cfg.Journal,
		Kid: cfg.SessionKid, SessionPub: keys.SessionPub, KemPub: keys.KemPub, Token: in.currentToken}
	in.refusal = in.start(ctx)
	if in.refusal != "" {
		in.log("demarrage_refuse", "", in.refusal)
	}
	return in, nil
}

func cmp(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func (in *Instance) start(ctx context.Context) string {
	r, err := readReglages(ctx, in.cfg)
	if err != nil {
		return "configuration_refusee"
	}
	in.cfg.reglages = r
	if in.reg.Load(ctx) != nil || in.borne.Load(ctx) != nil {
		return "version_importee"
	}
	if in.writeInstance(ctx) != nil {
		return "registre_indisponible"
	}
	in.borne.UpToDate(ctx)
	in.refreshToken(ctx)
	return ""
}

func (in *Instance) currentToken() string {
	if t := in.token.Load(); t != nil {
		return *t
	}
	return ""
}

func (in *Instance) refreshToken(ctx context.Context) {
	if t, err := in.cfg.Lanceur.Token(ctx, attestation.ChannelNonces(in.keys.SessionPub, in.keys.KemPub)); err == nil {
		in.token.Store(&t)
	}
}

func (in *Instance) writeInstance(ctx context.Context) error {
	return in.reg.Write(ctx, func(w *registre.Writer) error {
		now := w.Now.UTC().Format(primitives.TimeLayout)
		w.Put("instances/"+in.ID, jcs.Object{"imageDigest": in.cfg.ImageDigest, "demarreLe": in.started.UTC().Format(primitives.TimeLayout), "vuLe": now})
		return nil
	})
}

// Run tient la boucle de fond de l'instance jusqu'à la fin de ctx : jeton
// du lanceur toutes les 10 min, vuLe toutes les 60 s, borne toutes les
// 10 min, fermeture des canaux inactifs, réessais des notifications.
func (in *Instance) Run(ctx context.Context) {
	if in.refusal != "" {
		return
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var lastTok, lastVu, lastBorne, lastIdle time.Time
	lastTok, lastVu = in.cfg.Horloge.Now(), in.cfg.Horloge.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		in.Tick(ctx, &lastTok, &lastVu, &lastBorne, &lastIdle)
	}
}

// Tick fait ce qui est dû à l'heure de l'horloge de l'instance (les tests
// l'appellent avec une horloge fixe).
func (in *Instance) Tick(ctx context.Context, lastTok, lastVu, lastBorne, lastIdle *time.Time) {
	now := in.cfg.Horloge.Now()
	due := func(last *time.Time, p time.Duration) bool {
		if last.IsZero() || now.Sub(*last) >= p {
			*last = now
			return true
		}
		return false
	}
	if due(lastTok, tokenPeriod) {
		in.refreshToken(ctx)
	}
	if due(lastVu, vuLePeriod) {
		_ = in.writeInstance(ctx)
	}
	if due(lastBorne, registre.BornePeriod) {
		if err := in.borne.Deposit(ctx); err != nil {
			in.log("borne_non_deposee", "", "registre_indisponible")
		}
	}
	if due(lastIdle, idlePeriod) {
		in.table.CloseIdle(now)
	}
	in.notif.Retry(ctx)
}

func (in *Instance) log(event, uid, code string) {
	e := ports.Event{Event: event, InstanceID: in.ID, ImageDigest: in.cfg.ImageDigest, CodeErreur: code}
	if uid != "" {
		e.UIDMasque = registre.Masque(uid)
	}
	in.cfg.Journal.Write(e)
}

// Server sert les instances : en mode instance une seule, en grappe locale
// les trois préfixes (décision 0063). GET /attestation est servie par l'une
// d'elles, à tour de rôle.
type Server struct {
	Instances []*Instance
	turn      atomic.Uint32
	sem       chan struct{}
	once      sync.Once
}

// MaxRequests borne les requêtes traitées ensemble.
const MaxRequests = 256

func clearError(w http.ResponseWriter, status int, code string, details jcs.Object) {
	o := jcs.Object{"erreur": code, "message": message(code)}
	if details != nil {
		o["details"] = details
	}
	b, _ := jcs.Marshal(o)
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.once.Do(func() { s.sem = make(chan struct{}, MaxRequests) })
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-r.Context().Done():
		return
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Type", "application/json; charset=utf-8")
	var in *Instance
	if r.URL.Path == "/attestation" && len(s.Instances) > 0 {
		in = s.Instances[int(s.turn.Add(1))%len(s.Instances)]
	} else if p, ok := strings.CutPrefix(r.URL.Path, "/n/"); ok && strings.HasSuffix(p, "/canal") && len(p) == 7 {
		for _, i := range s.Instances {
			if i.ID[:1] == p[:1] {
				in = i
			}
		}
	}
	switch {
	case in == nil || r.URL.RawQuery != "":
		w.WriteHeader(http.StatusNotFound)
	case r.Method == http.MethodOptions:
		h.Set("Access-Control-Allow-Methods", "GET, POST")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		h.Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusNoContent)
	case in.refusal != "" || in.currentToken() == "":
		clearError(w, http.StatusServiceUnavailable, "attestation_indisponible", nil)
	case r.URL.Path == "/attestation" && r.Method == http.MethodGet:
		in.attestation(w, r)
	case r.URL.Path != "/attestation" && r.Method == http.MethodPost:
		in.channel(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ClientIP est l'avant-dernière valeur de X-Forwarded-For, celle qu'ajoute
// le répartiteur (API.md §1.1) ; sans elle, l'adresse de la connexion.
func ClientIP(r *http.Request) string {
	if v := strings.Split(r.Header.Get("X-Forwarded-For"), ","); len(v) >= 2 {
		return strings.TrimSpace(v[len(v)-2])
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func (in *Instance) attestation(w http.ResponseWriter, r *http.Request) {
	now := in.cfg.Horloge.Now()
	if ok, retry := in.local.hit("attestation|ip|"+ClientIP(r), limit{attestLimitN, time.Minute}, now); !ok {
		clearError(w, http.StatusTooManyRequests, "trop_d_appels", jcs.Object{"reessayerLe": stamp(retry)})
		return
	}
	b, _ := jcs.Marshal(jcs.Object{"jwt": in.currentToken(), "sessionPub": primitives.B64Encode(in.keys.SessionPub),
		"kemPub": primitives.B64Encode(in.keys.KemPub), "instanceId": in.ID})
	_, _ = w.Write(b)
}

func stamp(t time.Time) string { return t.UTC().Format(primitives.TimeLayout) }

func (in *Instance) channel(w http.ResponseWriter, r *http.Request) {
	text, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxEnvelope))
	if err != nil {
		clearError(w, http.StatusBadRequest, "canal_invalide", nil)
		return
	}
	e, err := canal.ReadEnvelope(text)
	switch {
	case err != nil:
		clearError(w, http.StatusBadRequest, "canal_invalide", nil)
		return
	case e.InstanceID != in.ID:
		clearError(w, http.StatusGone, "instance_inconnue", nil)
		return
	}
	ch, err := in.table.Prepare(in.keys, e)
	if err != nil {
		clearError(w, http.StatusBadRequest, "canal_invalide", nil)
		return
	}
	route, corps, err := canal.OpenRequest(ch.KReq, e)
	if errors.Is(err, primitives.ErrDecrypt) || in.table.Accept(ch, e.N, in.cfg.Horloge.Now()) != nil {
		clearError(w, http.StatusBadRequest, "canal_invalide", nil)
		return
	}
	resp := in.dispatch(r, ch, route, corps, err == nil, len(e.Body)-16)
	out, err := canal.SealResponse(ch.KRes, ch.Prefix, e.N, resp)
	if err != nil {
		clearError(w, http.StatusBadRequest, "canal_invalide", nil)
		return
	}
	_, _ = w.Write(out)
}

// Channels rend le nombre de canaux ouverts de l'instance.
func (in *Instance) Channels() int { return in.table.Len() }
