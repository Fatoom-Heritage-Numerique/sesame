package primitives

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

type hex = vecteurs.Hex

// in rassemble les champs d'entrée de tous les groupes de primitives.json ;
// chaque groupe lit les siens.
type in struct {
	N                                *int
	Donnees, Cle, Ikm, Sel, Info     hex
	Longueur                         int
	Aad, Clair, Chiffre, IV          hex
	Prive, Publique, Graine, Message hex
	Signature                        hex
}

type out struct {
	Longueur                                              *int
	TiragesDistincts                                      *bool
	Empreinte, Mac, Okm, Chiffre, Clair, Publique, Secret hex
	Signature                                             hex
	Valide                                                *bool
	LongueurPrive, LongueurPublique                       int
	PubliqueRecalculee                                    bool
	LongueurChiffre, LongueurSecret                       int
	SecretDecapsule                                       bool
}

// ops rejoue chaque groupe sur la primitive de ce paquet (ou de la
// bibliothèque standard quand Sésame l'appelle directement) et rend ses
// sorties au format du fichier.
var ops = map[string]func(e in) (out, error){
	"aleatoire": func(e in) (out, error) {
		a, err := Random(*e.N)
		if err != nil {
			return out{}, err
		}
		b, _ := Random(*e.N)
		n, distinct := len(a), !bytes.Equal(a, b)
		return out{Longueur: &n, TiragesDistincts: &distinct}, nil
	},
	"sha256": func(e in) (out, error) { return out{Empreinte: SHA256(e.Donnees)}, nil },
	"hmacSha256": func(e in) (out, error) {
		m := hmac.New(sha256.New, e.Cle)
		m.Write(e.Donnees)
		return out{Mac: m.Sum(nil)}, nil
	},
	"hkdfSha256": func(e in) (out, error) {
		k, err := HKDF(e.Ikm, e.Sel, e.Info, e.Longueur)
		return out{Okm: k}, err
	},
	"aesGcmChiffrer": func(e in) (out, error) {
		c, err := Seal(e.Cle, e.IV, e.Clair, e.Aad)
		return out{Chiffre: c}, err
	},
	"aesGcmDechiffrer": func(e in) (out, error) {
		c, err := Open(e.Cle, e.IV, e.Chiffre, e.Aad)
		return out{Clair: c}, err
	},
	"x25519Publique": func(e in) (out, error) {
		p, err := X25519Public(e.Prive)
		return out{Publique: p}, err
	},
	"x25519": func(e in) (out, error) {
		s, err := X25519(e.Prive, e.Publique)
		return out{Secret: s}, err
	},
	"x25519Paire": func(e in) (out, error) {
		k, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			return out{}, err
		}
		p, err := X25519Public(k.Bytes())
		return out{LongueurPrive: len(k.Bytes()), LongueurPublique: len(k.PublicKey().Bytes()), PubliqueRecalculee: bytes.Equal(p, k.PublicKey().Bytes())}, err
	},
	"ed25519Publique": func(e in) (out, error) {
		p, err := Ed25519Public(e.Graine)
		return out{Publique: p}, err
	},
	"ed25519Signer": func(e in) (out, error) {
		s, err := Ed25519Sign(e.Graine, e.Message)
		return out{Signature: s}, err
	},
	"ed25519Verifier": func(e in) (out, error) {
		ok, err := Ed25519Verify(e.Publique, e.Message, e.Signature)
		return out{Valide: &ok}, err
	},
	"ecdsaP256Verifier": func(e in) (out, error) {
		ok, err := P256Verify(e.Publique, e.Message, e.Signature)
		return out{Valide: &ok}, err
	},
	"rsaPkcs1Sha256Verifier": func(e in) (out, error) {
		ok, err := RSAVerify(e.Publique, e.Message, e.Signature)
		return out{Valide: &ok}, err
	},
	"mlKem768Decapsuler": func(e in) (out, error) {
		s, err := MLKEMDecapsulate(e.Graine, e.Chiffre)
		return out{Secret: s}, err
	},
	// Sésame n'encapsule pas (le client le fait) : propriétés de
	// crypto/mlkem et refus d'une clé hors module, secret retrouvé par la
	// décapsulation de ce paquet.
	"mlKem768Encapsuler": func(e in) (out, error) {
		ek, err := mlkem.NewEncapsulationKey768(e.Publique)
		if err != nil {
			return out{}, ErrKey
		}
		ss, ct := ek.Encapsulate()
		back, err := MLKEMDecapsulate(e.Graine, ct)
		return out{LongueurChiffre: len(ct), LongueurSecret: len(ss), SecretDecapsule: bytes.Equal(back, ss)}, err
	},
}

func TestVectors(t *testing.T) {
	tr := vecteurs.Track(t, "primitives.json",
		"aesGcmChiffrer", "aesGcmDechiffrer", "aleatoire",
		"ecdsaP256Verifier", "ed25519Publique", "ed25519Signer", "ed25519Verifier",
		"hkdfSha256", "hmacSha256", "mlKem768Decapsuler", "mlKem768Encapsuler",
		"rsaPkcs1Sha256Verifier", "sha256", "x25519", "x25519Paire", "x25519Publique")
	for op, f := range ops {
		tr.Vectors(t, op, func(t *testing.T, v vecteurs.Vector) {
			var e in
			var want out
			vecteurs.Load(t, v.Inputs, &e)
			vecteurs.Load(t, v.Outputs, &want)
			got, err := f(e)
			if err != nil {
				t.Fatal(err)
			}
			vecteurs.Same(t, got, want)
		})
		tr.Refusals(t, op, func(t *testing.T, r vecteurs.Refusal) {
			var e in
			vecteurs.Load(t, r.Inputs, &e)
			if _, err := f(e); err == nil || err.Error() != r.Error {
				t.Errorf("erreur %v, attendu %s", err, r.Error)
			}
		})
	}
}
