// Paquet cles : hiérarchie et formats des clés que Sésame ouvre
// (ARCHITECTURE.md §10) : dérivations de la KTS, enveloppes d'un contenu pour
// un proche, charte, code d'accès et son index, AAD des scellés KMS (KTS,
// code d'invitation, fiche d'alerte). Aucune E/S ; les comparaisons de
// secrets sont en temps constant.
package cles

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

var errInvalid = primitives.ErrInvalid

// Generation contrôle une génération (ktsGeneration, génération de charte) :
// entier ≥ 1.
func Generation(g int64) error {
	if g < 1 {
		return errInvalid
	}
	return nil
}

func derive(kts []byte, uid, info string) ([]byte, error) {
	if len(kts) != 32 {
		return nil, primitives.ErrKey
	}
	if !primitives.ID(uid) {
		return nil, errInvalid
	}
	return primitives.HKDF(kts, []byte(uid), []byte(info), 32)
}

// KTSEnv rend KTS_env = HKDF(KTS, uid, "fatoom-sesame-enveloppe", 32) (§10.2).
func KTSEnv(kts []byte, uid string) ([]byte, error) {
	return derive(kts, uid, "fatoom-sesame-enveloppe")
}

// KTSAuth rend KTS_auth = HKDF(KTS, uid, "fatoom-sesame-charte", 32) (§10.2).
func KTSAuth(kts []byte, uid string) ([]byte, error) { return derive(kts, uid, "fatoom-sesame-charte") }

// Envelope désigne l'enveloppe d'un contenu pour un proche (§10.3).
type Envelope struct {
	UID, Type, ContentID, ContactID string
	KTSGeneration                   int64
}

func fileID(s string) bool {
	b, err := primitives.B64URL(s)
	return len(s) == 22 && err == nil && len(b) == 16
}

// AAD rend aad_env = "fatoom-sesame-env|1|" ‖ uid ‖ "|" ‖ type ‖ "|" ‖
// contentId ‖ "|" ‖ contactId ‖ "|" ‖ ktsGeneration ; contentId selon le
// type : fileId (fichier, motdepasse), contactId (mot), "courant" (volontes).
func (d Envelope) AAD() ([]byte, error) {
	ok := primitives.ID(d.UID) && primitives.ID(d.ContactID) && Generation(d.KTSGeneration) == nil
	switch d.Type {
	case "fichier", "motdepasse":
		ok = ok && fileID(d.ContentID)
	case "mot":
		ok = ok && d.ContentID == d.ContactID
	case "volontes":
		ok = ok && d.ContentID == "courant"
	default:
		ok = false
	}
	if !ok {
		return nil, errInvalid
	}
	return []byte(strings.Join([]string{"fatoom-sesame-env|1", d.UID, d.Type, d.ContentID, d.ContactID, strconv.FormatInt(d.KTSGeneration, 10)}, "|")), nil
}

// Open ouvre une enveloppe de 48 octets (IV de 12 octets) sous KTS_env : une
// enveloppe sous une autre AAD est refusée (dechiffrement_refuse).
func (d Envelope) Open(ktsEnv, sealed, iv []byte) ([]byte, error) {
	aad, err := d.AAD()
	if err != nil {
		return nil, err
	}
	if len(sealed) != 48 {
		return nil, errInvalid
	}
	return primitives.Open(ktsEnv, iv, sealed, aad)
}

// CharterAAD rend aad_charte = "fatoom-sesame-charte|1|" ‖ uid ‖ "|" ‖
// contactId ‖ "|" ‖ generation ‖ "|" ‖ ktsGeneration (§10.4).
func CharterAAD(uid, contactID string, gen, ktsGen int64) ([]byte, error) {
	if !primitives.ID(uid) || !primitives.ID(contactID) || Generation(gen) != nil || Generation(ktsGen) != nil {
		return nil, errInvalid
	}
	return []byte("fatoom-sesame-charte|1|" + uid + "|" + contactID + "|" + strconv.FormatInt(gen, 10) + "|" + strconv.FormatInt(ktsGen, 10)), nil
}

var (
	rePhone = regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`)
	reDate  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// charterFields sont les 15 champs de la charte (§10.4).
	charterFields = []string{"adresse", "codeVerifier", "contactId", "courrier", "creeLe", "dateNaissance", "email",
		"generation", "nom", "prenom", "role", "telephone", "titulaireAffiche", "uid", "v"}
)

// EmailNormalized dit si e est sous la forme normalisée de l'email de la
// charte et de la fiche d'alerte (§10 ; décision 0076) : au moins un
// caractère, tous ASCII de 0x21 à 0x7E, aucun de A à Z. Sésame ne normalise
// rien : le client a retiré les espaces de bord et mis A-Z en minuscules.
func EmailNormalized(e string) bool {
	return e != "" && strings.IndexFunc(e, func(r rune) bool { return r < 0x21 || r > 0x7E || (r >= 'A' && r <= 'Z') }) < 0
}

// Charter est une charte ouverte et contrôlée.
type Charter = jcs.Object

// CheckCharter contrôle la forme d'une charte : exactement ses 15 champs,
// facultatifs à null (jamais vides), téléphone E.164, email normalisé, date de
// naissance réelle, creeLe à la milliseconde, courrier vrai si et seulement
// si téléphone et email sont null (et alors adresse, nom et date de naissance
// présents), téléphone exigé pour le rôle gardien.
func CheckCharter(c jcs.Object) error {
	if len(c) != len(charterFields) {
		return errInvalid
	}
	for _, k := range charterFields {
		if _, ok := c[k]; !ok {
			return errInvalid
		}
	}
	text := func(k string) (string, bool) { s, ok := c[k].(string); return s, ok && s != "" }
	optional := func(k string) (string, bool) {
		if c[k] == nil {
			return "", true
		}
		return text(k)
	}
	uid, okUID := text("uid")
	contact, okContact := text("contactId")
	gen, _ := c["generation"].(float64)
	verifier, _ := c["codeVerifier"].(string)
	cv, errCV := primitives.B64(verifier)
	phone, okPhone := optional("telephone")
	email, okEmail := optional("email")
	nom, okNom := optional("nom")
	adresse, okAdr := optional("adresse")
	birth, okBirth := optional("dateNaissance")
	_, okPrenom := text("prenom")
	_, okTit := text("titulaireAffiche")
	created, _ := c["creeLe"].(string)
	_, errCreated := primitives.ReadTime(created)
	courrier, okCourrier := c["courrier"].(bool)
	role := c["role"]
	noContact := c["telephone"] == nil && c["email"] == nil
	switch {
	case c["v"] != 1.0 || (role != "proche" && role != "gardien") || !okCourrier,
		!okUID || !primitives.ID(uid) || !okContact || !primitives.ID(contact),
		gen != float64(int64(gen)) || Generation(int64(gen)) != nil, errCV != nil || len(cv) != 32,
		!okPhone || (phone != "" && !rePhone.MatchString(phone)), !okEmail || (email != "" && !EmailNormalized(email)),
		!okPrenom || !okTit || !okNom || !okAdr || !okBirth || (birth != "" && !realDate(birth)), errCreated != nil,
		courrier != noContact, courrier && (adresse == "" || nom == "" || birth == ""),
		role == "gardien" && c["telephone"] == nil:
		return errInvalid
	}
	return nil
}

func realDate(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	return reDate.MatchString(s) && err == nil && t.Format("2006-01-02") == s
}

// OpenCharter ouvre une charte sous KTS_auth et l'AAD attendue, puis exige que
// le clair soit exactement le JSON canonique d'une charte de forme contrôlée,
// de cet uid, de ce proche et de cette génération.
func OpenCharter(ktsAuth, sealed, iv []byte, uid, contactID string, gen, ktsGen int64) (Charter, error) {
	aad, err := CharterAAD(uid, contactID, gen, ktsGen)
	if err != nil {
		return nil, err
	}
	plain, err := primitives.Open(ktsAuth, iv, sealed, aad)
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	v, err := jcs.ReadCanonical(plain)
	c, _ := v.(jcs.Object)
	if err != nil || c == nil || CheckCharter(c) != nil || c["uid"] != uid || c["contactId"] != contactID || c["generation"] != float64(gen) {
		return nil, errInvalid
	}
	return c, nil
}

// CodeAlphabet est l'alphabet du code d'accès (§10.4) ; un code a 12 de ses
// caractères.
const CodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// NormalizeCode applique normalize à une saisie : tirets et espaces retirés,
// a à z en majuscules ; le résultat doit être 12 caractères de l'alphabet.
func NormalizeCode(input string) (string, error) {
	var b strings.Builder
	for _, r := range input {
		switch {
		case r == '-' || r == ' ':
			continue
		case r >= 'a' && r <= 'z':
			r -= 'a' - 'A'
		}
		if r > unicode.MaxASCII || !strings.ContainsRune(CodeAlphabet, r) {
			return "", errInvalid
		}
		b.WriteRune(r)
	}
	if b.Len() != 12 {
		return "", errInvalid
	}
	return b.String(), nil
}

// CodeVerifier rend base64(SHA-256("fatoom-code|" ‖ code normalisé)).
func CodeVerifier(code string) string {
	return base64.StdEncoding.EncodeToString(primitives.SHA256([]byte("fatoom-code|" + code)))
}

// IndexAcces rend hex(Argon2id(code normalisé, "fatoom-lookup-v1", m = 65 536
// Kio, t = 3, p = 1, 32 octets)) : l'index d'un code d'accès et le lookup
// d'un code d'invitation (§10.4).
func IndexAcces(code string) string {
	k, _ := primitives.Argon2id([]byte(code), []byte("fatoom-lookup-v1"), 65536, 3, 1, 32) // paramètres valides
	return hex.EncodeToString(k)
}

// CheckCode dit, en temps constant, si un code normalisé correspond au
// codeVerifier d'une charte.
func CheckCode(code, verifier string) bool {
	return primitives.Equal([]byte(CodeVerifier(code)), []byte(verifier))
}

var reLookup = regexp.MustCompile(`^[0-9a-f]{64}$`)

// SealedKTSAAD rend l'AAD de sealedKts : uid ‖ "|" ‖ ktsGeneration (§4).
func SealedKTSAAD(uid string, ktsGen int64) ([]byte, error) {
	if !primitives.ID(uid) || Generation(ktsGen) != nil {
		return nil, errInvalid
	}
	return []byte(uid + "|" + strconv.FormatInt(ktsGen, 10)), nil
}

// InvitationAAD rend l'AAD du code d'invitation scellé :
// "fatoom-sesame-invitation|1|" ‖ uid ‖ "|" ‖ contactId ‖ "|" ‖ lookup (§4).
func InvitationAAD(uid, contactID, lookup string) ([]byte, error) {
	if !primitives.ID(uid) || !primitives.ID(contactID) || !reLookup.MatchString(lookup) {
		return nil, errInvalid
	}
	return []byte("fatoom-sesame-invitation|1|" + uid + "|" + contactID + "|" + lookup), nil
}

// DrawCode tire un code de 12 caractères depuis 12 octets d'aléa, sans biais :
// alphabet[octet mod 32] (§10.4) ; Sésame tire ainsi le code d'invitation.
func DrawCode(random []byte) (string, error) {
	if len(random) != 12 {
		return "", errInvalid
	}
	b := make([]byte, 12)
	for i, x := range random {
		b[i] = CodeAlphabet[x%32]
	}
	return string(b), nil
}

// DisplayCode rend un code normalisé sous la forme affichée XXXX-XXXX-XXXX
// (vide pour une chaîne qui n'a pas 12 caractères).
func DisplayCode(code string) string {
	if len(code) != 12 {
		return ""
	}
	return code[:4] + "-" + code[4:8] + "-" + code[8:]
}
