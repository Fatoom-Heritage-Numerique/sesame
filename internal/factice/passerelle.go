package factice

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Notifications admises par la passerelle (API.md §5.5) ; toute autre,
// echeanceModifiee comprise, reçoit 400.
var Notifications = []string{"ouvertureNotifiee", "sessionTerminee", "signalementAnnule", "roleRefuse", "vieApresTransmission",
	"lettreEnvoyee", "statutLettre", "lettreEchouee", "identiteEchouee"}

// Check est ce que la passerelle de test vérifie d'une requête de Sésame,
// avec les règles d'un client (API.md §5.1) : jeton d'attestation sous la
// racine de test et la liste des versions de test, eat_nonce contre
// X-Sesame-Session-Pub et X-Sesame-Kem-Pub, signature contre chaque clé de
// clesSession, horodatage à moins de 5 min, identifiant d'un GET /bundle non
// rejoué dans les 10 min, notification déjà traitée ⇒ 200 {} sans nouveau
// traitement, 300 appels par minute et par instance.
type Check struct {
	Config  *Config
	Clock   *Clock
	mu      sync.Mutex
	seen    map[string]time.Time // X-Sesame-Request-Id vus, et leur heure
	minute  map[string][]time.Time
	Refused []string // motif de chaque requête refusée (401, 400, 429)
}

// verdict rend 0 pour une requête à traiter, 200 pour une notification déjà
// traitée, ou le statut d'un refus.
func (c *Check) verdict(r Request) int {
	h, now := r.Headers, c.Clock.Now()
	sp, err1 := primitives.B64(h["X-Sesame-Session-Pub"])
	kp, err2 := primitives.B64(h["X-Sesame-Kem-Pub"])
	claims, err := attestation.Verify(h["X-Sesame-Attestation"], c.Config.RootSum(), now, attestation.ChannelNonces(sp, kp))
	if err == nil {
		_, err = attestation.CheckList(claims, c.Config.VersionList(), c.Config.Projet, c.Config.RegistreImages)
	}
	msg, err3 := jetons.GatewayMessage(r.Method, r.PathQuery, h["X-Sesame-Date"], h["X-Sesame-Request-Id"], r.Body)
	sig, _ := primitives.B64(h["X-Sesame-Signature"])
	signed := false
	for _, k := range c.Config.VersionList().SessionKeys {
		ok, _ := primitives.P256Verify(k, msg, sig)
		signed = signed || ok
	}
	date, _ := primitives.ReadTime(h["X-Sesame-Date"])
	id := h["X-Sesame-Request-Id"]
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen == nil {
		c.seen, c.minute = map[string]time.Time{}, map[string][]time.Time{}
	}
	refuse := func(status int, motif string) int { c.Refused = append(c.Refused, motif); return status }
	switch first, dup := c.seen[id]; {
	case err1 != nil || err2 != nil || len(kp) != 1184 || err != nil:
		return refuse(401, "attestation")
	case err3 != nil || !signed:
		return refuse(401, "signature")
	case now.Sub(date).Abs() > 5*time.Minute:
		return refuse(401, "horodatage")
	case dup && r.Method == "POST":
		return 200
	case dup && now.Sub(first) < 10*time.Minute:
		return refuse(401, "rejeu")
	case r.Method == "POST" && !slices.Contains(Notifications, strings.TrimPrefix(r.PathQuery, "/")):
		return refuse(400, "notification inconnue")
	}
	calls := slices.DeleteFunc(c.minute[h["X-Sesame-Session-Pub"]], func(t time.Time) bool { return now.Sub(t) >= time.Minute })
	if c.minute[h["X-Sesame-Session-Pub"]] = append(calls, now); len(calls) >= 300 {
		return refuse(429, "limite")
	}
	if len(c.seen) >= MaxRequests {
		clear(c.seen)
	}
	c.seen[id] = now
	return 0
}

// forget oublie une notification que la passerelle n'a pas traitée (réponse
// autre que 200) : son nouvel essai sera traité.
func (c *Check) forget(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.seen, id)
}
