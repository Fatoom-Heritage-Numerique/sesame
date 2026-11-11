// Paquet vie : messages signés par la clé de vie du titulaire, que Sésame
// vérifie avec cleViePub (ARCHITECTURE.md §10.5 et §9.2 ; API.md §2.7) :
// preuve de vie, annulation, publication de génération, preuve d'enrôlement,
// dépôt du PIN et fiche d'alerte. Message = chaîne de domaine ‖ JSON
// canonique ; chaque domaine est distinct, si bien qu'une signature ne vaut
// jamais pour un autre message. Aucune E/S.
package vie

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strconv"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/cles"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

var (
	errInvalid = primitives.ErrInvalid
	b64        = base64.StdEncoding
	reHex64    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	rePhone    = regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`)
)

// GenerationDeleted est la génération d'un proche supprimé (§10.5).
const GenerationDeleted = -1

func signed(domain string, ids []string, body jcs.Object) ([]byte, error) {
	for _, id := range ids {
		if !primitives.ID(id) {
			return nil, errInvalid
		}
	}
	j, err := jcs.Marshal(body)
	if err != nil {
		return nil, err
	}
	return append([]byte(domain), j...), nil
}

// checkDefi exige un défi de 32 octets en base64 standard.
func checkDefi(defi string) error {
	if b, err := primitives.B64(defi); err != nil || len(b) != 32 {
		return errInvalid
	}
	return nil
}

// LifeMessage rend le message d'une preuve de vie :
// "fatoom-sesame-vie|1|" ‖ uid ‖ {"defi","uid","v":1}.
func LifeMessage(uid, defi string) ([]byte, error) {
	if err := checkDefi(defi); err != nil {
		return nil, err
	}
	return signed("fatoom-sesame-vie|1|"+uid, []string{uid}, jcs.Object{"v": 1, "uid": uid, "defi": defi})
}

// CancelMessage rend le message d'une annulation : "fatoom-sesame-annulation|1|"
// ‖ uid ‖ "|" ‖ signalementId ‖ {"defi","signalementId","uid","v":1}.
func CancelMessage(uid, reportID, defi string) ([]byte, error) {
	if err := checkDefi(defi); err != nil {
		return nil, err
	}
	return signed("fatoom-sesame-annulation|1|"+uid+"|"+reportID, []string{uid, reportID},
		jcs.Object{"v": 1, "uid": uid, "signalementId": reportID, "defi": defi})
}

// GenerationMessage rend le message d'une publication de génération de
// charte (≥ 1, ou -1 pour un proche supprimé) : "fatoom-sesame-gen|1|" ‖ uid ‖
// "|" ‖ contactId ‖ {"contactId","generation","uid","v":1}.
func GenerationMessage(uid, contactID string, gen int64) ([]byte, error) {
	if gen < 1 && gen != GenerationDeleted {
		return nil, errInvalid
	}
	return signed("fatoom-sesame-gen|1|"+uid+"|"+contactID, []string{uid, contactID},
		jcs.Object{"v": 1, "uid": uid, "contactId": contactID, "generation": gen})
}

// EnrolMessage rend le message d'une preuve d'enrôlement :
// "fatoom-sesame-enroler|1|" ‖ uid ‖ {"cleViePub","defi","generation","uid","v":1},
// cleViePub (32 octets) en base64.
func EnrolMessage(uid, defi string, gen int64, cleViePub []byte) ([]byte, error) {
	if err := checkDefi(defi); err != nil || gen < 1 || len(cleViePub) != 32 {
		return nil, errInvalid
	}
	return signed("fatoom-sesame-enroler|1|"+uid, []string{uid},
		jcs.Object{"v": 1, "uid": uid, "defi": defi, "generation": gen, "cleViePub": b64.EncodeToString(cleViePub)})
}

// PinMotifs sont les motifs d'un dépôt du PIN (§9.2).
var PinMotifs = []string{"nouveau_pin", "ouverture", "redepot"}

// PinDepositMessage rend le message d'un dépôt du PIN : "fatoom-pin-depot|1|"
// ‖ uid ‖ "|" ‖ grappeId ‖ {"defi","elementAveugle": hex(SHA-256(élément)),
// "grappeId","motif","uid","v":1}.
func PinDepositMessage(uid, grappeID string, element []byte, defi, motif string) ([]byte, error) {
	known := false
	for _, m := range PinMotifs {
		known = known || m == motif
	}
	if err := checkDefi(defi); err != nil || !known || !reHex64.MatchString(grappeID) || len(element) != 32 {
		return nil, errInvalid
	}
	return signed("fatoom-pin-depot|1|"+uid+"|"+grappeID, []string{uid}, jcs.Object{"v": 1, "uid": uid, "grappeId": grappeID,
		"elementAveugle": hex.EncodeToString(primitives.SHA256(element)), "defi": defi, "motif": motif})
}

// Alert est une fiche d'alerte (§11.10 ; API.md §2.7) ; Telephone nil vaut null.
type Alert struct {
	UID        string
	Generation int64
	Email      string
	Telephone  *string
}

// AlertAAD rend "fatoom-sesame-alerte|1|" ‖ uid ‖ "|" ‖ génération : l'AAD de
// ficheScellee sous alerte-seal et le préfixe du message signé.
func AlertAAD(uid string, gen int64) ([]byte, error) {
	if !primitives.ID(uid) || gen < 1 {
		return nil, errInvalid
	}
	return []byte("fatoom-sesame-alerte|1|" + uid + "|" + strconv.FormatInt(gen, 10)), nil
}

// JSON rend le JSON canonique de la fiche {"email","generation","telephone",
// "uid","v":1}, après contrôle : email normalisé (cles.EmailNormalized), téléphone E.164
// ou null.
func (a Alert) JSON() ([]byte, error) {
	var phone any
	if a.Telephone != nil {
		if !rePhone.MatchString(*a.Telephone) {
			return nil, errInvalid
		}
		phone = *a.Telephone
	}
	if _, err := AlertAAD(a.UID, a.Generation); err != nil || !cles.EmailNormalized(a.Email) {
		return nil, errInvalid
	}
	return jcs.Marshal(jcs.Object{"email": a.Email, "generation": a.Generation, "telephone": phone, "uid": a.UID, "v": 1})
}

// Message rend le message signé de la fiche : AlertAAD ‖ JSON.
func (a Alert) Message() ([]byte, error) {
	j, err := a.JSON()
	if err != nil {
		return nil, err
	}
	aad, _ := AlertAAD(a.UID, a.Generation)
	return append(aad, j...), nil
}

// ReadAlert lit la fiche descellée (clair rendu par alerte-seal) : exactement
// son JSON canonique, de cet uid et de cette génération.
func ReadAlert(plain []byte, uid string, gen int64) (*Alert, error) {
	v, err := jcs.ReadCanonical(plain)
	o, _ := v.(jcs.Object)
	email, _ := o["email"].(string)
	a := &Alert{UID: uid, Generation: gen, Email: email}
	if p, ok := o["telephone"].(string); ok {
		a.Telephone = &p
	}
	_, hasPhone := o["telephone"]
	if err != nil || len(o) != 5 || o["v"] != 1.0 || o["uid"] != uid || o["generation"] != float64(gen) || !hasPhone ||
		(o["telephone"] != nil && a.Telephone == nil) {
		return nil, errInvalid
	}
	if _, err := a.JSON(); err != nil {
		return nil, err
	}
	return a, nil
}

// Verify vérifie une signature Ed25519 (base64 de 64 octets) de msg par
// cleViePub (32 octets) ; toute forme fausse rend faux.
func Verify(cleViePub, msg []byte, signature string) bool {
	sig, err := primitives.B64(signature)
	if err != nil || len(cleViePub) != 32 {
		return false
	}
	ok, _ := primitives.Ed25519Verify(cleViePub, msg, sig)
	return ok
}
