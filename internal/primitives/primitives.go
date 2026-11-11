// Paquet primitives : les primitives de la bibliothèque standard qu'emploie
// Sésame, avec les longueurs exigées et les refus de bord sur lesquels les
// clients et l'enclave s'accordent (ARCHITECTURE.md §10 et §5 ; vecteurs
// primitives.json). AES-256-GCM (IV 12 octets, tag 16 octets à la fin),
// HKDF-SHA256, X25519 (secret nul refusé), Ed25519 (S ≥ ℓ refusé par
// crypto/ed25519), ECDSA P-256 en R ‖ S, RSA PKCS#1 v1.5 SHA-256 (chaîne x5c)
// et ML-KEM-768 tiré de sa graine d ‖ z. Aucune E/S hormis l'aléa du système.
package primitives

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"math/big"
	"regexp"
	"time"
)

// Error est un refus typé ; son texte est le code d'ErreurCrypto des vecteurs.
type Error string

func (e Error) Error() string { return string(e) }

// Refus communs à tous les formats.
const (
	ErrInvalid Error = "parametres_invalides"
	ErrDecrypt Error = "dechiffrement_refuse"
	ErrKey     Error = "cle_invalide"
	ErrZero    Error = "secret_nul"
)

// Random rend n octets de l'aléa du système.
func Random(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrInvalid
	}
	b := make([]byte, n)
	_, _ = rand.Read(b) // crypto/rand.Read n'échoue pas (documentation de Go)
	return b, nil
}

// SHA256 rend l'empreinte SHA-256 de b.
func SHA256(b ...[]byte) []byte {
	h := sha256.New()
	for _, p := range b {
		h.Write(p)
	}
	return h.Sum(nil)
}

// Equal compare deux octets en temps constant.
func Equal(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }

// HKDF rend n octets de HKDF-SHA256.
func HKDF(ikm, salt, info []byte, n int) ([]byte, error) {
	if n < 1 {
		return nil, ErrInvalid
	}
	k, err := hkdf.Key(sha256.New, ikm, salt, string(info), n)
	if err != nil {
		return nil, ErrInvalid
	}
	return k, nil
}

func gcm(key, iv []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, ErrKey
	}
	if len(iv) != 12 {
		return nil, ErrInvalid
	}
	block, _ := aes.NewCipher(key)
	return cipher.NewGCM(block)
}

// Seal chiffre en AES-256-GCM : chiffré ‖ tag.
func Seal(key, iv, plain, aad []byte) ([]byte, error) {
	a, err := gcm(key, iv)
	if err != nil {
		return nil, err
	}
	return a.Seal(nil, iv, plain, aad), nil
}

// Open déchiffre chiffré ‖ tag ; tout échec d'authentification est ErrDecrypt.
func Open(key, iv, sealed, aad []byte) ([]byte, error) {
	a, err := gcm(key, iv)
	if err != nil {
		return nil, err
	}
	plain, err := a.Open(nil, iv, sealed, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plain, nil
}

// X25519Public rend la clé publique d'une clé privée X25519.
func X25519Public(priv []byte) ([]byte, error) {
	k, err := ecdh.X25519().NewPrivateKey(priv)
	if err != nil {
		return nil, ErrKey
	}
	return k.PublicKey().Bytes(), nil
}

// X25519 rend le secret partagé, refusé s'il est entièrement nul.
func X25519(priv, pub []byte) ([]byte, error) {
	k, err := ecdh.X25519().NewPrivateKey(priv)
	if err != nil {
		return nil, ErrKey
	}
	p, err := ecdh.X25519().NewPublicKey(pub)
	if err != nil {
		return nil, ErrKey
	}
	s, err := k.ECDH(p)
	if err != nil { // crypto/ecdh refuse le point d'ordre faible : secret nul
		return nil, ErrZero
	}
	return s, nil
}

// Ed25519Public rend la clé publique d'une graine Ed25519.
func Ed25519Public(seed []byte) ([]byte, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, ErrKey
	}
	return ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey), nil
}

// Ed25519Sign signe msg avec la clé de graine seed.
func Ed25519Sign(seed, msg []byte) ([]byte, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, ErrKey
	}
	return ed25519.Sign(ed25519.NewKeyFromSeed(seed), msg), nil
}

// Ed25519Verify vérifie une signature de 64 octets ; S ≥ ℓ est refusé.
func Ed25519Verify(pub, msg, sig []byte) (bool, error) {
	if len(pub) != ed25519.PublicKeySize {
		return false, ErrKey
	}
	return len(sig) == ed25519.SignatureSize && ed25519.Verify(pub, msg, sig), nil
}

// P256Key lit un point P-256 non compressé (65 octets).
func P256Key(pub []byte) (*ecdsa.PublicKey, error) {
	k, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), pub)
	if err != nil {
		return nil, ErrKey
	}
	return k, nil
}

// P256Verify vérifie une signature ECDSA P-256 / SHA-256 en R ‖ S (64 octets).
func P256Verify(pub, msg, sig []byte) (bool, error) {
	k, err := P256Key(pub)
	if err != nil {
		return false, err
	}
	if len(sig) != 64 {
		return false, nil
	}
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	return ecdsa.Verify(k, SHA256(msg), r, s), nil
}

// RSAVerify vérifie une signature RSA PKCS#1 v1.5 / SHA-256 par une clé
// SubjectPublicKeyInfo DER.
func RSAVerify(spki, msg, sig []byte) (bool, error) {
	k, err := x509.ParsePKIXPublicKey(spki)
	if err != nil {
		return false, ErrKey
	}
	rk, ok := k.(*rsa.PublicKey)
	if !ok {
		return false, ErrKey
	}
	return rsa.VerifyPKCS1v15(rk, crypto.SHA256, SHA256(msg), sig) == nil, nil
}

// MLKEMDecapsulate retrouve le secret ML-KEM-768 d'un chiffré de 1 088
// octets, la clé de décapsulation tirée de sa graine d ‖ z de 64 octets.
func MLKEMDecapsulate(seed, ct []byte) ([]byte, error) {
	dk, err := mlkem.NewDecapsulationKey768(seed)
	if err != nil {
		return nil, ErrKey
	}
	if len(ct) != mlkem.CiphertextSize768 {
		return nil, ErrInvalid
	}
	ss, err := dk.Decapsulate(ct)
	if err != nil {
		return nil, ErrInvalid
	}
	return ss, nil
}

// MLKEMPublic rend la clé d'encapsulation (1 184 octets) d'une graine.
func MLKEMPublic(seed []byte) ([]byte, error) {
	dk, err := mlkem.NewDecapsulationKey768(seed)
	if err != nil {
		return nil, ErrKey
	}
	return dk.EncapsulationKey().Bytes(), nil
}

// B64 décode le base64 standard avec remplissage, forme canonique seule.
func B64(s string) ([]byte, error) {
	b, err := base64.StdEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, ErrInvalid
	}
	return b, nil
}

// B64URL décode le base64url sans remplissage, forme canonique seule.
func B64URL(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, ErrInvalid
	}
	return b, nil
}

// TimeLayout est l'horodatage des formats : ISO 8601 UTC à la milliseconde.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// ReadTime lit un horodatage exactement sous la forme TimeLayout.
func ReadTime(s string) (time.Time, error) {
	t, err := time.Parse(TimeLayout, s)
	if err != nil || t.Format(TimeLayout) != s {
		return time.Time{}, ErrInvalid
	}
	return t, nil
}

var reID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// ID dit si s est un identifiant des formats (uid, contactId, contentId,
// signalementId…) : 1 à 128 caractères de [A-Za-z0-9_-] (ARCHITECTURE.md
// §10 ; API.md §1).
func ID(s string) bool { return reID.MatchString(s) }
