package routes

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/canal"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/sessions"
)

// statuses et messages : codes d'erreur du corps chiffré et des erreurs en
// clair (API.md §6). Le message n'est pas destiné à l'affichage.
var statuses = map[string]int{
	"canal_invalide": 400, "instance_inconnue": 410, "attestation_indisponible": 503, "message_refuse": 400,
	"jeton_invalide": 401, "code_refuse": 403, "otp_refuse": 403, "role_insuffisant": 403, "pair_refuse": 403,
	"preuve_requise": 403, "identite_non_confirmee": 403, "bascule_impossible": 403, "deja_transmis": 409,
	"generation_non_croissante": 409, "reprise_absente": 409, "defi_invalide": 409, "inactivite_trop_recente": 409,
	"grappe_recente": 409, "pin_bloque": 409, "pin_indisponible": 409, "identite_en_attente": 409,
	"titulaire_indisponible": 409, "refusee": 409, "amorcage_refuse": 409, "membres_refuses": 409, "pas_leader": 409,
	"session_expiree": 410, "identite_bloquee": 423, "trop_d_essais": 429, "trop_d_appels": 429,
	"pin_indisponible_temporaire": 503, "reessayer_plus_tard": 503, "registre_indisponible": 503,
	"passerelle_indisponible": 503, "fournisseur_indisponible": 503,
}

var messages = map[string]string{
	"canal_invalide": "Requête refusée par le canal.", "instance_inconnue": "Instance inconnue : reprendre à l'attestation.",
	"attestation_indisponible": "Attestation pas encore disponible.", "trop_d_appels": "Trop d'appels.",
}

func message(code string) string {
	if m, ok := messages[code]; ok {
		return m
	}
	return "Requête refusée : " + code + "."
}

// refusal est une erreur du contrat : son code et ses details.
type refusal struct {
	code    string
	details jcs.Object
}

func (r *refusal) Error() string { return r.code }

func fail(code string) error { return &refusal{code: code} }

func failAt(code, key string, t time.Time) error {
	return &refusal{code, jcs.Object{key: stamp(t)}}
}

// limit est une limite de débit : n appels par fenêtre (API.md §7).
type limit struct {
	n   int
	per time.Duration
}

// route est une route du canal : forme de son corps, taille, limites par IP
// et par canal (décomptées à l'entrée, refusées à l'étape 3 du §6), par
// session (décomptée après l'authentification de la session), limite par uid
// ou par compte, et sa décision.
type route struct {
	fields             []field
	max                int
	ip, perCh, perSess limit
	uid                limit
	run                func(a *call) (jcs.Object, error)
}

// call est une requête du canal en cours de décision.
type call struct {
	ctx      context.Context
	in       *Instance
	ch       *canal.Channel
	name     string
	r        *route
	body     jcs.Object
	now      time.Time
	ip       string
	ua       any               // User-Agent tronqué à 256 caractères, nil s'il manque (API.md §5.5)
	held     *sessions.Session // session verrouillée par l'appel
	overflow time.Time         // fin de fenêtre d'une limite IP ou canal dépassée
}

// limits rend trop_d_appels si une limite par IP ou par canal est dépassée
// (étape 3 du §6).
func (a *call) limits() error {
	if !a.overflow.IsZero() {
		return failAt("trop_d_appels", "reessayerLe", a.overflow)
	}
	return nil
}

// limitKey décompte un appel authentifié pour key (uid, compte de service) ;
// sans leader joignable, la VM qui reçoit décompte elle-même (§7).
func (a *call) limitKey(key string) error {
	if a.r.uid.n == 0 {
		return nil
	}
	if ok, retry := a.in.local.hit(a.name+"|uid|"+key, a.r.uid, a.now); !ok {
		return failAt("trop_d_appels", "reessayerLe", retry)
	}
	return nil
}

func (in *Instance) dispatch(r *http.Request, ch *canal.Channel, name string, corps jcs.Object, formOK bool, size int) jcs.Object {
	a := &call{ctx: r.Context(), in: in, ch: ch, name: name, body: corps, now: in.cfg.Horloge.Now(), ip: ClientIP(r)}
	if ua := []rune(r.Header.Get("User-Agent")); len(r.Header.Values("User-Agent")) > 0 {
		a.ua = string(ua[:min(len(ua), 256)])
	}
	rt, known := table[name]
	if !formOK || !known || size > rt.max || !valid(corps, rt.fields) {
		return response(nil, fail("message_refuse"))
	}
	a.r = &rt
	for _, l := range []struct {
		key string
		lim limit
	}{{"|ip|" + a.ip, rt.ip}, {"|canal|" + ch.Key, rt.perCh}} {
		if ok, retry := in.local.hit(name+l.key, l.lim, a.now); l.lim.n > 0 && !ok && a.overflow.IsZero() {
			a.overflow = retry
		}
	}
	out := response(rt.run(a))
	if a.held != nil {
		a.held.Unlock()
	}
	return out
}

func response(corps jcs.Object, err error) jcs.Object {
	var ref *refusal
	switch {
	case err == nil:
		return jcs.Object{"statut": 200, "corps": corps}
	case errors.As(err, &ref):
	case errors.Is(err, registre.ErrUnavailable):
		ref = &refusal{code: "registre_indisponible"}
	default:
		ref = &refusal{code: "registre_indisponible"}
	}
	o := jcs.Object{"statut": statuses[ref.code], "erreur": ref.code, "message": message(ref.code)}
	if ref.details != nil {
		o["details"] = ref.details
	}
	return o
}
