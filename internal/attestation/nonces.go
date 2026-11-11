package attestation

import (
	"encoding/base64"
	"slices"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// maxNonces borne eat_nonce : au plus 6 valeurs de 10 à 74 octets ASCII
// imprimables (ARCHITECTURE.md §3).
const maxNonces = 6

var b64url = base64.RawURLEncoding

// ChannelNonces rend les deux valeurs d'eat_nonce du jeton servi aux clients :
// base64url(SHA-256(sessionPub)) et base64url(SHA-256(kemPub)) (§3, §5).
func ChannelNonces(sessionPub, kemPub []byte) []string {
	return []string{b64url.EncodeToString(primitives.SHA256(sessionPub)), b64url.EncodeToString(primitives.SHA256(kemPub))}
}

// PairNonces rend les trois valeurs d'un jeton de paire entre VM (API.md §3) :
// base64url(noncePaire) (32 octets), base64url(SHA-256(nodeId)) (nodeId :
// 16 octets en base64url) et base64url(SHA-256(clé)) (SubjectPublicKeyInfo
// DER du certificat TLS entre VM).
func PairNonces(noncePaire []byte, nodeID string, key []byte) ([]string, error) {
	id, err := primitives.B64URL(nodeID)
	if len(noncePaire) != 32 || err != nil || len(id) != 16 || len(key) == 0 {
		return nil, primitives.ErrInvalid
	}
	return []string{b64url.EncodeToString(noncePaire), b64url.EncodeToString(primitives.SHA256(id)), b64url.EncodeToString(primitives.SHA256(key))}, nil
}

// NoncesAccepted dit si eat_nonce (chaîne ou tableau) porte exactement les
// valeurs want, dans n'importe quel ordre : un jeton de client (deux valeurs)
// n'est jamais accepté pour un jeton de paire (trois), ni l'inverse.
func NoncesAccepted(eatNonce any, want []string) bool {
	var got []string
	switch v := eatNonce.(type) {
	case string:
		got = []string{v}
	case jcs.Array:
		for _, e := range v {
			s, ok := e.(string)
			if !ok {
				return false
			}
			got = append(got, s)
		}
	}
	if len(got) == 0 || len(got) > maxNonces || len(got) != len(want) {
		return false
	}
	rest := slices.Clone(want)
	for _, s := range got {
		if len(s) < 10 || len(s) > 74 {
			return false
		}
		for _, c := range []byte(s) {
			if c < 0x20 || c > 0x7e {
				return false
			}
		}
		i := slices.Index(rest, s)
		if i < 0 {
			return false
		}
		rest = slices.Delete(rest, i, i+1)
	}
	return true
}
