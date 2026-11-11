// Paquet jetons : ce que Sésame signe par session-sign (ARCHITECTURE.md §4 ;
// API.md §1.6 et §5.1) : JWS compacts ES256 des reçus, de la réponse de etat
// et du jeton de session, et requêtes à la passerelle. KMS rend une signature
// DER, convertie en R ‖ S de 64 octets ; toute autre forme est refusée. En-tête
// et charge en JSON canonique, base64url sans remplissage. Aucune E/S : la
// signature elle-même est faite par l'appelant (KMS ou double).
package jetons

import (
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

var (
	errInvalid = primitives.ErrInvalid
	b64url     = base64.RawURLEncoding
	// order est l'ordre n du groupe de P-256.
	order, _ = new(big.Int).SetString("ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551", 16)
)

// SessionDuration est la durée d'un jeton de session (API.md §1.6).
const SessionDuration = 3600

// DERToRaw convertit une signature ECDSA P-256 DER (SEQUENCE de deux INTEGER,
// forme minimale) en R ‖ S sur 64 octets.
func DERToRaw(der []byte) ([]byte, error) {
	var sig struct{ R, S *big.Int }
	rest, err := asn1.Unmarshal(der, &sig)
	if err != nil || len(rest) != 0 {
		return nil, errInvalid
	}
	if again, _ := asn1.Marshal(sig); string(again) != string(der) {
		return nil, errInvalid
	}
	raw := make([]byte, 64)
	for i, x := range []*big.Int{sig.R, sig.S} {
		if x.Sign() <= 0 || x.Cmp(order) >= 0 {
			return nil, errInvalid
		}
		x.FillBytes(raw[32*i : 32*i+32])
	}
	return raw, nil
}

// SigningInput rend base64url(en-tête) "." base64url(charge), chacun en JSON
// canonique : l'octet signé d'un JWS. jwt ajoute "typ":"JWT" à l'en-tête.
func SigningInput(kid string, jwt bool, payload jcs.Object) (string, error) {
	header := jcs.Object{"alg": "ES256", "kid": kid}
	if jwt {
		header["typ"] = "JWT"
	}
	h, err := jcs.Marshal(header)
	if err != nil {
		return "", err
	}
	p, err := jcs.Marshal(payload)
	if err != nil {
		return "", err
	}
	return b64url.EncodeToString(h) + "." + b64url.EncodeToString(p), nil
}

// Compact rend le JWS compact d'un octet signé et de sa signature DER.
func Compact(input string, der []byte) (string, error) {
	raw, err := DERToRaw(der)
	if err != nil {
		return "", err
	}
	return input + "." + b64url.EncodeToString(raw), nil
}

func canonicalSegment(s string) (jcs.Object, error) {
	b, err := primitives.B64URL(s)
	if err != nil {
		return nil, err
	}
	v, err := jcs.ReadCanonical(b)
	o, _ := v.(jcs.Object)
	if err != nil || o == nil {
		return nil, errInvalid
	}
	return o, nil
}

// Verify vérifie un JWS ES256 de Sésame par la clé session-sign de son kid
// (keys : points non compressés par kid) et rend sa charge : en-tête
// canonique {alg, kid} (reçu, etat) ou {alg, kid, typ: "JWT"} (jeton de
// session), signature R ‖ S, charge canonique d'émetteur "sesame".
func Verify(jws string, keys map[string][]byte, jwt bool) (jcs.Object, error) {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return nil, errInvalid
	}
	h, err := canonicalSegment(parts[0])
	if err != nil {
		return nil, err
	}
	want := 2
	if jwt {
		want = 3
	}
	kid, _ := h["kid"].(string)
	key, ok := keys[kid]
	sig, err := primitives.B64URL(parts[2])
	if len(h) != want || h["alg"] != "ES256" || (jwt && h["typ"] != "JWT") || !ok || err != nil || len(sig) != 64 {
		return nil, errInvalid
	}
	if valid, err := primitives.P256Verify(key, []byte(parts[0]+"."+parts[1]), sig); err != nil || !valid {
		return nil, errInvalid
	}
	c, err := canonicalSegment(parts[1])
	if err != nil || c["iss"] != "sesame" {
		return nil, errInvalid
	}
	return c, nil
}

// Session sont les claims d'un jeton de session (API.md §1.6).
type Session struct {
	Sub, UID, Base, Role string
	Iat, Exp             int64
}

// ReadSession vérifie un jeton de session non expiré à now : champs exacts,
// scope "destine", base et role dans leurs listes, exp = iat + 3600.
func ReadSession(jws string, keys map[string][]byte, now time.Time) (*Session, error) {
	c, err := Verify(jws, keys, true)
	if err != nil {
		return nil, err
	}
	s := &Session{}
	s.Sub, _ = c["sub"].(string)
	s.UID, _ = c["uid"].(string)
	s.Base, _ = c["base"].(string)
	s.Role, _ = c["role"].(string)
	iat, _ := c["iat"].(float64)
	exp, _ := c["exp"].(float64)
	s.Iat, s.Exp = int64(iat), int64(exp)
	switch {
	case len(c) != 8 || c["scope"] != "destine" || float64(s.Iat) != iat || float64(s.Exp) != exp,
		s.Base != "vivant" && s.Base != "deces" && s.Base != "inactivite",
		s.Role != "proche" && s.Role != "gardien",
		!primitives.ID(s.Sub) || !primitives.ID(s.UID), s.Exp != s.Iat+SessionDuration, s.Exp*1000 <= now.UnixMilli():
		return nil, errInvalid
	}
	return s, nil
}

var (
	reUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	rePath = regexp.MustCompile(`^/[^|\s]*$`)
)

// GatewayMessage rend le message signé d'une requête à la passerelle
// (API.md §5.1) : méthode|chemin et query|X-Sesame-Date|X-Sesame-Request-Id|
// hex(SHA-256(corps)).
func GatewayMessage(method, pathQuery, date, requestID string, body []byte) ([]byte, error) {
	if _, err := primitives.ReadTime(date); err != nil || (method != "GET" && method != "POST") ||
		!rePath.MatchString(pathQuery) || !reUUID.MatchString(requestID) {
		return nil, errInvalid
	}
	return []byte(method + "|" + pathQuery + "|" + date + "|" + requestID + "|" + hex.EncodeToString(primitives.SHA256(body))), nil
}
