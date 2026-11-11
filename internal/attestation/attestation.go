// Paquet attestation : vérification d'un jeton PKI de Confidential Space
// (ARCHITECTURE.md §3 ; API.md §3) et lecture de la liste des versions
// (RELEASE.md §2). Verify contrôle la forme, la chaîne x5c jusqu'à la racine
// épinglée, la signature RS256 et les claims communs aux clients et aux VM ;
// CheckList, les claims qui portent sur la liste des versions, l'image et
// env_override, comme les clients et le script public. Le refus nomme le
// premier claim faux, dans l'ordre des clients ; il ne porte aucune valeur
// reçue. L'heure est un paramètre. Aucune E/S.
package attestation

import (
	"crypto/x509"
	"encoding/hex"
	"slices"
	"strings"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Refusal est le claim refusé : forme, signature, iss, aud, exp, iat,
// eat_nonce, swname, support_attributes, dbgstat, secboot, image_digest,
// retiree, hwmodel, project_number, image_reference ou env_override.
type Refusal string

func (r Refusal) Error() string { return string(r) }

// Issuer et Audience sont l'émetteur et l'audience exigés (ARCHITECTURE.md §3).
const (
	Issuer   = "https://confidentialcomputing.googleapis.com"
	Audience = "Fatoom-Heritage-Numerique/sesame"
	// iatAhead est l'avance admise de iat sur l'horloge qui vérifie.
	iatAhead = 5 * time.Minute
)

// Claims sont les claims d'un jeton accepté par Verify, que CheckList ou le
// contrôle entre VM comparent ensuite.
type Claims struct {
	ImageDigest, ImageReference, HWModel string
	ProjectNumber                        any
	EnvOverride                          any
	Exp                                  int64
}

func obj(v any) jcs.Object {
	o, _ := v.(jcs.Object)
	return o
}

func segment(s string) (jcs.Object, error) {
	b, err := primitives.B64URL(s)
	if err != nil {
		return nil, err
	}
	v, err := jcs.Parse(b)
	if o := obj(v); err == nil && o != nil {
		return o, nil
	}
	return nil, primitives.ErrInvalid
}

// integer lit un entier JSON sûr (|n| ≤ 2⁵³).
func integer(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f != float64(int64(f)) || f > 1<<53 || f < -(1<<53) {
		return 0, false
	}
	return int64(f), true
}

// Verify vérifie le jeton jwt sous la racine d'empreinte SHA-256 root, à
// l'heure now ; nonces sont les valeurs exactes qu'eat_nonce doit porter
// (ChannelNonces pour un client, PairNonces entre VM).
func Verify(jwt string, root []byte, now time.Time, nonces []string) (*Claims, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, Refusal("forme")
	}
	header, err1 := segment(parts[0])
	c, err2 := segment(parts[1])
	sig, err3 := primitives.B64URL(parts[2])
	x5c, ok := header["x5c"].(jcs.Array)
	if err1 != nil || err2 != nil || err3 != nil || !ok {
		return nil, Refusal("forme")
	}
	chain := make([][]byte, len(x5c))
	for i, e := range x5c {
		s, _ := e.(string)
		if chain[i], err1 = primitives.B64(s); err1 != nil || s == "" {
			return nil, Refusal("forme")
		}
	}
	if header["alg"] != "RS256" {
		return nil, Refusal("signature")
	}
	leaf := verifyChain(chain, root, now)
	if leaf == nil {
		return nil, Refusal("signature")
	}
	if ok, err := primitives.RSAVerify(leaf, []byte(parts[0]+"."+parts[1]), sig); err != nil || !ok {
		return nil, Refusal("signature")
	}
	secs := float64(now.UnixMilli()) / 1000
	exp, okExp := integer(c["exp"])
	iat, okIat := integer(c["iat"])
	submods := obj(c["submods"])
	attrs, _ := obj(submods["confidential_space"])["support_attributes"].(jcs.Array)
	container := obj(submods["container"])
	for _, check := range []struct {
		motif string
		ok    bool
	}{
		{"iss", c["iss"] == Issuer},
		{"aud", c["aud"] == Audience},
		{"exp", okExp && float64(exp) > secs},
		{"iat", okIat && float64(iat) <= secs+iatAhead.Seconds()},
		{"eat_nonce", NoncesAccepted(c["eat_nonce"], nonces)},
		{"swname", c["swname"] == "CONFIDENTIAL_SPACE"},
		{"support_attributes", contains(attrs, "STABLE")},
		{"dbgstat", c["dbgstat"] == "disabled-since-boot"},
		{"secboot", c["secboot"] == true},
	} {
		if !check.ok {
			return nil, Refusal(check.motif)
		}
	}
	digest, _ := container["image_digest"].(string)
	ref, _ := container["image_reference"].(string)
	hw, _ := c["hwmodel"].(string)
	return &Claims{ImageDigest: digest, ImageReference: ref, HWModel: hw,
		ProjectNumber: obj(submods["gce"])["project_number"], EnvOverride: container["env_override"], Exp: exp}, nil
}

func contains(a jcs.Array, s string) bool {
	for _, e := range a {
		if e == s {
			return true
		}
	}
	return false
}

// verifyChain vérifie x5c (feuille, intermédiaire, racine) : trois
// certificats X.509 v3 en sha256WithRSAEncryption, racine d'empreinte root,
// chacun signé par le suivant et d'émetteur égal à son sujet, émetteurs cA,
// tous valides à now. Rend la clé publique de la feuille, ou nil.
func verifyChain(chain [][]byte, root []byte, now time.Time) []byte {
	if len(chain) != 3 || len(root) != 32 || !primitives.Equal(primitives.SHA256(chain[2]), root) {
		return nil
	}
	certs := make([]*x509.Certificate, 3)
	for i, der := range chain {
		c, err := x509.ParseCertificate(der)
		if err != nil || c.Version != 3 || c.SignatureAlgorithm != x509.SHA256WithRSA || now.Before(c.NotBefore) || now.After(c.NotAfter) {
			return nil
		}
		certs[i] = c
	}
	for i := range 2 {
		child, parent := certs[i], certs[i+1]
		if !parent.BasicConstraintsValid || !parent.IsCA || !primitives.Equal(child.RawIssuer, parent.RawSubject) {
			return nil
		}
		if ok, err := primitives.RSAVerify(parent.RawSubjectPublicKeyInfo, child.RawTBSCertificate, child.Signature); err != nil || !ok {
			return nil
		}
	}
	return certs[0].RawSubjectPublicKeyInfo
}

// CheckList contrôle, après Verify, les claims qui portent sur la liste des
// versions (ARCHITECTURE.md §3 ; RELEASE.md §2) : image en service, hwmodel de
// sa ligne, numéro de projet en chaîne, chemin du dépôt d'images (prefix,
// terminé par « / ») et empreinte d'env_override égale à envDigest.
func CheckList(c *Claims, l *Versions, project, prefix string) (*Version, error) {
	v := l.ByDigest(c.ImageDigest)
	switch {
	case v == nil:
		return nil, Refusal("image_digest")
	case v.Status != "en_service":
		return nil, Refusal("retiree")
	case !slices.Contains(v.HWModels, c.HWModel):
		return nil, Refusal("hwmodel")
	case c.ProjectNumber != project:
		return nil, Refusal("project_number")
	case !strings.HasSuffix(prefix, "/") || !strings.HasPrefix(c.ImageReference, prefix):
		return nil, Refusal("image_reference")
	}
	if d, err := EnvDigest(c.EnvOverride); err != nil || d != v.EnvDigest {
		return nil, Refusal("env_override")
	}
	return v, nil
}

// EnvDigest rend "sha256:" ‖ hex(SHA-256(JSON canonique)) d'env_override,
// dictionnaire de chaînes (ARCHITECTURE.md §2.3).
func EnvDigest(env any) (string, error) {
	o := obj(env)
	if o == nil {
		return "", primitives.ErrInvalid
	}
	for _, v := range o {
		if _, ok := v.(string); !ok {
			return "", primitives.ErrInvalid
		}
	}
	b, err := jcs.Marshal(o)
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(primitives.SHA256(b)), nil
}
