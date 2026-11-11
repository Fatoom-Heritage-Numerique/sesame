package routes

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Reglages sont les variables attestées d'env_override (ARCHITECTURE.md §2.3).
type Reglages struct {
	VerificationService, CourrierOrganisation, EmailExpediteur, PasserelleURL, SesameAudience string
	SmsExpediteur, IDTokenProjet, CompteDeces, CompteExploitation, SesameTextes               string
	Textes                                                                                    map[string]string
}

// Textes attendus du document des textes (ARCHITECTURE.md §2.3).
var Textes = []string{"lettre_deces", "lettre_inactivite", "dernier_code_email", "alerte_pin_email", "alerte_pin_sms",
	"alerte_confiance_email", "alerte_confiance_sms", "alerte_ligne_rejetee_email", "alerte_ligne_rejetee_sms"}

// SecretTextes est le nom du secret qui porte le document des textes.
const SecretTextes = "sesame-textes"

var errConfig = errors.New("configuration_refusee")

// readReglages exige chaque variable, non vide et aucune autre, puis le
// document des textes d'empreinte SESAME_TEXTES qui porte chaque texte attendu.
func readReglages(ctx context.Context, cfg *Config) (Reglages, error) {
	v := []string{"VERIFICATION_SERVICE", "COURRIER_ORGANISATION", "EMAIL_EXPEDITEUR", "PASSERELLE_URL", "SESAME_AUDIENCE",
		"SMS_EXPEDITEUR", "IDTOKEN_PROJET", "COMPTE_DECES", "COMPTE_EXPLOITATION", "SESAME_TEXTES"}
	got := make([]string, len(v))
	for i, k := range v {
		if got[i] = cfg.Env[k]; got[i] == "" {
			return Reglages{}, errConfig
		}
	}
	r := Reglages{got[0], got[1], got[2], got[3], got[4], got[5], got[6], got[7], got[8], got[9], nil}
	doc, err := cfg.Secrets.Secret(ctx, SecretTextes)
	val, err2 := jcs.ReadCanonical(doc)
	o, _ := val.(jcs.Object)
	t, _ := o["textes"].(jcs.Object)
	if len(cfg.Env) != len(v) || err != nil || err2 != nil || len(o) != 2 || o["v"] != 1.0 || t == nil ||
		r.SesameTextes != "sha256:"+hex.EncodeToString(primitives.SHA256(doc)) {
		return Reglages{}, errConfig
	}
	r.Textes = map[string]string{}
	for k, s := range t {
		r.Textes[k], _ = s.(string)
	}
	for _, n := range append(Textes, textesSignalement()...) {
		if r.Textes[n] == "" {
			return Reglages{}, errConfig
		}
	}
	return r, nil
}

func textesSignalement() []string {
	var n []string
	for _, s := range []string{"insee", "acte", "inactivite"} {
		n = append(n, "alerte_signalement_"+s+"_email", "alerte_signalement_"+s+"_rappel_email", "alerte_signalement_"+s+"_sms")
	}
	return n
}

// verifyJWT vérifie un JWT RS256 par la clé de son kid chez l'émetteur et
// rend ses claims non expirés à l'heure de l'appel.
func (a *call) verifyJWT(token, issuer string) jcs.Object {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	seg := func(s string) jcs.Object {
		b, err := primitives.B64URL(s)
		v, err2 := jcs.Parse(b)
		o, _ := v.(jcs.Object)
		if err != nil || err2 != nil {
			return nil
		}
		return o
	}
	h, c := seg(parts[0]), seg(parts[1])
	sig, err := primitives.B64URL(parts[2])
	keys, err2 := a.in.cfg.Emetteurs.Keys(a.ctx, issuer)
	key, ok := keys[str(h, "kid")]
	if h == nil || c == nil || err != nil || err2 != nil || !ok || h["alg"] != "RS256" {
		return nil
	}
	exp, _ := c["exp"].(float64)
	if v, err := primitives.RSAVerify(key, []byte(parts[0]+"."+parts[1]), sig); err != nil || !v || exp*1000 <= float64(a.now.UnixMilli()) {
		return nil
	}
	return c
}

// idToken accepte le jeton d'identité du titulaire uid (API.md §1.4).
func (a *call) idToken(uid string) error {
	p := a.in.cfg.reglages.IDTokenProjet
	c := a.verifyJWT(str(a.body, "idToken"), ports.IssuerIDToken)
	if c == nil || c["iss"] != "https://securetoken.google.com/"+p || c["aud"] != p || c["sub"] != uid {
		return fail("jeton_invalide")
	}
	return nil
}

// oidc accepte le jeton OIDC Google du compte de service email, lu dans le
// champ name du corps (API.md §1.4).
func (a *call) oidc(name, email string) error {
	c := a.verifyJWT(str(a.body, name), ports.IssuerGoogle)
	if c == nil || c["iss"] != "https://accounts.google.com" || c["aud"] != a.in.cfg.reglages.SesameAudience ||
		c["email"] != email || c["email_verified"] != true {
		return fail("jeton_invalide")
	}
	return nil
}

// signJWS signe une pièce par session-sign (API.md §1.6) ; jwt ajoute
// "typ": "JWT" à l'en-tête (jeton de session).
func (in *Instance) signJWS(ctx context.Context, jwt bool, payload jcs.Object) (string, error) {
	input, err := jetons.SigningInput(in.cfg.SessionKid, jwt, payload)
	if err != nil {
		return "", err
	}
	der, err := in.cfg.KMS.Sign(ctx, ports.SessionSign, []byte(input))
	if err != nil {
		return "", err
	}
	return jetons.Compact(input, der)
}

// receipt signe un reçu de type typ (API.md §1.6).
func (a *call) receipt(typ string, claims jcs.Object) (string, error) {
	claims["iss"], claims["typ"], claims["iat"] = "sesame", typ, a.now.Unix()
	return a.in.signJWS(a.ctx, false, claims)
}
