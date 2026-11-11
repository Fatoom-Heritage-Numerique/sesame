package jetons

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

type hexa = vecteurs.Hex

type in struct {
	Cles         map[string]hexa
	Graines      map[string]hexa
	JWS, Typ     string
	SignatureDer hexa
	Maintenant   int64
}

func keys(e in) map[string][]byte {
	k := map[string][]byte{}
	for kid, v := range e.Cles {
		k[kid] = v
	}
	return k
}

// signSelf signe la même charge avec la graine du kid par crypto/ecdsa (un
// aléa, comme KMS) puis vérifie le jeton obtenu.
func signSelf(t *testing.T, e in, jwt bool, payload jcs.Object, kid string) {
	t.Helper()
	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), e.Graines[kid])
	if err != nil {
		t.Fatal(err)
	}
	input, err := SigningInput(kid, jwt, payload)
	if err != nil {
		t.Fatal(err)
	}
	der, err := ecdsa.SignASN1(rand.Reader, priv, primitives.SHA256([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	jws, err := Compact(input, der)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(jws, keys(e), jwt); err != nil {
		t.Errorf("jeton signé par Go refusé : %v", err)
	}
}

func replay(t *testing.T, v vecteurs.Vector, jwt bool) {
	var e in
	var want struct {
		Charge    string
		Signature hexa
	}
	vecteurs.Load(t, v.Inputs, &e)
	vecteurs.Load(t, v.Outputs, &want)
	var c jcs.Object
	var err error
	if jwt {
		if _, err = ReadSession(e.JWS, keys(e), time.UnixMilli(e.Maintenant)); err != nil {
			t.Fatal(err)
		}
	}
	if c, err = Verify(e.JWS, keys(e), jwt); err != nil {
		t.Fatal(err)
	}
	if !jwt && c["typ"] != e.Typ {
		t.Errorf("typ %v, attendu %s", c["typ"], e.Typ)
	}
	raw, err := DERToRaw(e.SignatureDer)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(e.JWS, ".")
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	if string(sig) != string(raw) {
		t.Error("signature du JWS différente de signatureDer convertie")
	}
	header, _ := primitives.B64URL(parts[0])
	h, _ := jcs.Parse(header)
	kid := h.(jcs.Object)["kid"].(string)
	if input, _ := SigningInput(kid, jwt, c); input != parts[0]+"."+parts[1] {
		t.Errorf("en-tête et charge recomposés %s", input)
	}
	signSelf(t, e, jwt, c, kid)
	charge, _ := jcs.Marshal(c)
	vecteurs.Same(t, struct {
		Charge    string
		Signature hexa
	}{string(charge), raw}, want)
}

func TestVectors(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "datePasserelle", "jetonSession", "passerelle", "recu")
	tr.Vectors(t, "recu", func(t *testing.T, v vecteurs.Vector) { replay(t, v, false) })
	tr.Refusals(t, "recu", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		c, err := Verify(e.JWS, keys(e), false)
		if err == nil && c["typ"] != e.Typ {
			err = primitives.ErrInvalid // reçu d'un autre type
		}
		if err == nil || err.Error() != r.Error {
			t.Errorf("erreur %v, attendu %s", err, r.Error)
		}
	})
	tr.Vectors(t, "jetonSession", func(t *testing.T, v vecteurs.Vector) { replay(t, v, true) })
	tr.Refusals(t, "jetonSession", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		if _, err := ReadSession(e.JWS, keys(e), time.UnixMilli(e.Maintenant)); err == nil || err.Error() != r.Error {
			t.Errorf("erreur %v, attendu %s", err, r.Error)
		}
	})
	tr.Vectors(t, "passerelle", func(t *testing.T, v vecteurs.Vector) {
		var e struct {
			CheminEtQuery, Date, Methode, RequestID, Signature string
			Cles                                               []hexa
			Corps, Graine                                      hexa
		}
		var want struct {
			Message string
			Valide  bool
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		msg, err := GatewayMessage(e.Methode, e.CheminEtQuery, e.Date, e.RequestID, e.Corps)
		if err != nil {
			t.Fatal(err)
		}
		// Vérification de la passerelle : base64 de 64 octets, contre chaque
		// clé de clesSession (décision 0044).
		valid := false
		if sig, err := primitives.B64(e.Signature); err == nil {
			for _, k := range e.Cles {
				ok, _ := primitives.P256Verify(k, msg, sig)
				valid = valid || ok
			}
		}
		if e.Graine != nil { // Sésame signe lui-même ce message
			priv, _ := ecdsa.ParseRawPrivateKey(elliptic.P256(), e.Graine)
			der, _ := ecdsa.SignASN1(rand.Reader, priv, primitives.SHA256(msg))
			raw, err := DERToRaw(der)
			if ok, _ := primitives.P256Verify(e.Cles[len(e.Cles)-1], msg, raw); err != nil || !ok {
				t.Error("signature de Go refusée")
			}
		}
		vecteurs.Same(t, struct {
			Message string
			Valide  bool
		}{string(msg), valid}, want)
	})
	// La date est contrôlée par la passerelle (API.md §5.1), pas par Sésame :
	// rejouée ici sur la lecture de l'horodatage de ce paquet.
	tr.Vectors(t, "datePasserelle", func(t *testing.T, v vecteurs.Vector) {
		var e struct {
			Date       string
			Maintenant int64
		}
		var want struct{ Admise bool }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		d, err := primitives.ReadTime(e.Date)
		gap := time.UnixMilli(e.Maintenant).Sub(d).Abs()
		if got := err == nil && gap <= 5*time.Minute; got != want.Admise {
			t.Errorf("admise = %v", got)
		}
	})
}

func TestDERToRawRefusals(t *testing.T) {
	for _, der := range []string{"", "3006020100020101", "300702020001020101", "30060201010201010000"} {
		b, _ := hex.DecodeString(der)
		if _, err := DERToRaw(b); err == nil {
			t.Errorf("%q accepté", der)
		}
	}
}
