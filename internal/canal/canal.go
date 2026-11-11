// Paquet canal : le canal chiffré hybride X25519 + ML-KEM-768 côté enclave
// (ARCHITECTURE.md §5 ; API.md §1.2 et §1.3 ; décision 0019) et la forme de
// l'instanceId. Dérivation des deux clés par sens depuis la transcription,
// IV et aad de chaque message, ouverture d'une requête sous K_req, réponse
// chiffrée sous K_res. L'état du canal (n strictement croissant, canaux
// fermés) est tenu par l'appelant. Aucune E/S.
package canal

import (
	"encoding/base64"
	"encoding/binary"
	"regexp"
	"strconv"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Longueurs du canal (§5) et plus grand n (API.md §1.2).
const (
	KeySize    = 32
	KemPubSize = 1184
	KemCtSize  = 1088
	PrefixSize = 4
	MaxN       = 1<<32 - 1
)

var (
	errInvalid = primitives.ErrInvalid
	b64        = base64.StdEncoding
)

// Instance porte les clés de session d'une instance, tirées au démarrage.
type Instance struct {
	SessionPriv, SessionPub []byte
	KemSeed, KemPub         []byte
}

// NewInstance reconstitue les clés d'une instance depuis sessionPriv (32
// octets) et la graine ML-KEM-768 d ‖ z (64 octets).
func NewInstance(sessionPriv, kemSeed []byte) (*Instance, error) {
	pub, err := primitives.X25519Public(sessionPriv)
	if err != nil {
		return nil, err
	}
	kemPub, err := primitives.MLKEMPublic(kemSeed)
	if err != nil {
		return nil, err
	}
	return &Instance{SessionPriv: sessionPriv, SessionPub: pub, KemSeed: kemSeed, KemPub: kemPub}, nil
}

// Transcript rend sessionPub ‖ cliPub ‖ kemPub ‖ kemCt (2 336 octets), sel des
// deux dérivations.
func Transcript(sessionPub, cliPub, kemPub, kemCt []byte) ([]byte, error) {
	if len(sessionPub) != KeySize || len(cliPub) != KeySize || len(kemPub) != KemPubSize {
		return nil, primitives.ErrKey
	}
	if len(kemCt) != KemCtSize {
		return nil, errInvalid
	}
	return append(append(append(append([]byte{}, sessionPub...), cliPub...), kemPub...), kemCt...), nil
}

// DeriveKeys rend K_req et K_res : HKDF-SHA256(ssKem ‖ shared, transcription,
// "fatoom-sesame-canal-req" | "fatoom-sesame-canal-res", 32).
func DeriveKeys(ssKem, shared, transcript []byte) (kReq, kRes []byte, err error) {
	if len(ssKem) != 32 || len(shared) != 32 {
		return nil, nil, errInvalid
	}
	ikm := append(append([]byte{}, ssKem...), shared...)
	defer clear(ikm)
	if kReq, err = primitives.HKDF(ikm, transcript, []byte("fatoom-sesame-canal-req"), 32); err != nil {
		return nil, nil, err
	}
	kRes, err = primitives.HKDF(ikm, transcript, []byte("fatoom-sesame-canal-res"), 32)
	return kReq, kRes, err
}

// Keys dérive les clés du canal (cliPub, kemCt) d'un client : secret X25519
// (refusé s'il est nul), décapsulation de kemCt, transcription.
func (in *Instance) Keys(cliPub, kemCt []byte) (kReq, kRes []byte, err error) {
	transcript, err := Transcript(in.SessionPub, cliPub, in.KemPub, kemCt)
	if err != nil {
		return nil, nil, err
	}
	shared, err := primitives.X25519(in.SessionPriv, cliPub)
	if err != nil {
		return nil, nil, err
	}
	defer clear(shared)
	ssKem, err := primitives.MLKEMDecapsulate(in.KemSeed, kemCt)
	if err != nil {
		return nil, nil, err
	}
	defer clear(ssKem)
	return DeriveKeys(ssKem, shared, transcript)
}

func checkN(n int64) error {
	if n < 1 || n > MaxN {
		return errInvalid
	}
	return nil
}

// IV rend IV_n = ivPrefixe ‖ n sur 8 octets gros-boutiste.
func IV(prefix []byte, n int64) ([]byte, error) {
	if len(prefix) != PrefixSize || checkN(n) != nil {
		return nil, errInvalid
	}
	return binary.BigEndian.AppendUint64(append([]byte{}, prefix...), uint64(n)), nil
}

// AAD rend "fatoom-sesame-req|<n>" (requête) ou "fatoom-sesame-res|<n>".
func AAD(request bool, n int64) ([]byte, error) {
	if err := checkN(n); err != nil {
		return nil, err
	}
	dir := "res"
	if request {
		dir = "req"
	}
	return []byte("fatoom-sesame-" + dir + "|" + strconv.FormatInt(n, 10)), nil
}

// Envelope est le corps de POST /n/{a|b|c}/canal (API.md §1.2).
type Envelope struct {
	InstanceID              string
	CliPub, KemCt, IVPrefix []byte
	N                       int64
	Body                    []byte
}

// ReadEnvelope lit l'enveloppe d'une requête : exactement ses six champs,
// octets en base64 standard, longueurs du canal, n de 1 à 2³² − 1.
func ReadEnvelope(text []byte) (*Envelope, error) {
	v, err := jcs.Parse(text)
	o, _ := v.(jcs.Object)
	if err != nil || len(o) != 6 {
		return nil, errInvalid
	}
	e := &Envelope{}
	e.InstanceID, _ = o["instanceId"].(string)
	n, _ := o["n"].(float64)
	e.N = int64(n)
	for _, f := range []struct {
		key  string
		dst  *[]byte
		size int
	}{{"cliPub", &e.CliPub, KeySize}, {"kemCt", &e.KemCt, KemCtSize}, {"ivPrefixe", &e.IVPrefix, PrefixSize}, {"body", &e.Body, -1}} {
		s, _ := o[f.key].(string)
		if *f.dst, err = primitives.B64(s); err != nil || (f.size >= 0 && len(*f.dst) != f.size) {
			return nil, errInvalid
		}
	}
	if _, err := ParseInstanceID(e.InstanceID); err != nil || float64(e.N) != n || checkN(e.N) != nil {
		return nil, errInvalid
	}
	return e, nil
}

var reRoute = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+)*$`)

// OpenRequest ouvre body sous K_req avec IV_n et l'aad de la requête : le
// clair est exactement le JSON canonique {"corps": {…}, "route": "<route>"}.
func OpenRequest(kReq []byte, e *Envelope) (route string, corps jcs.Object, err error) {
	iv, err := IV(e.IVPrefix, e.N)
	if err != nil {
		return "", nil, err
	}
	aad, _ := AAD(true, e.N)
	plain, err := primitives.Open(kReq, iv, e.Body, aad)
	if err != nil {
		return "", nil, err
	}
	defer clear(plain)
	v, err := jcs.ReadCanonical(plain)
	o, _ := v.(jcs.Object)
	route, _ = o["route"].(string)
	corps, _ = o["corps"].(jcs.Object)
	if err != nil || len(o) != 2 || corps == nil || !reRoute.MatchString(route) {
		return "", nil, errInvalid
	}
	return route, corps, nil
}

// SealResponse chiffre la réponse resp ({statut: 200, corps} ou {statut,
// erreur, message, details?}, API.md §1.3) sous K_res avec IV_n et rend
// l'enveloppe {"body", "n"} en JSON canonique.
func SealResponse(kRes, prefix []byte, n int64, resp jcs.Object) ([]byte, error) {
	iv, err := IV(prefix, n)
	if err != nil {
		return nil, err
	}
	aad, _ := AAD(false, n)
	plain, err := jcs.Marshal(resp)
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	body, err := primitives.Seal(kRes, iv, plain, aad)
	if err != nil {
		return nil, err
	}
	return jcs.Marshal(jcs.Object{"body": b64.EncodeToString(body), "n": n})
}

var reInstance = regexp.MustCompile(`^[abc]-[A-Za-z0-9_-]{22}$`)

// InstanceID rend l'instanceId d'une VM de préfixe a, b ou c depuis ses 16
// octets d'aléa : préfixe, tiret, 22 caractères base64url (API.md §1.2).
func InstanceID(prefix string, random []byte) (string, error) {
	if (prefix != "a" && prefix != "b" && prefix != "c") || len(random) != 16 {
		return "", errInvalid
	}
	return prefix + "-" + base64.RawURLEncoding.EncodeToString(random), nil
}

// ParseInstanceID contrôle la forme d'un instanceId et rend son préfixe.
func ParseInstanceID(id string) (string, error) {
	if !reInstance.MatchString(id) {
		return "", errInvalid
	}
	if _, err := primitives.B64URL(id[2:]); err != nil {
		return "", err
	}
	return id[:1], nil
}
