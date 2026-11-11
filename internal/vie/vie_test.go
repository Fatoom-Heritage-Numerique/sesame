package vie

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

type hexa = vecteurs.Hex

type in struct {
	UID, Defi, SignalementID, ContactID, GrappeID, Motif, Signature, Email string
	Telephone                                                              json.RawMessage
	Generation                                                             int64
	GraineVie, CleViePub, CleViePubEnvoyee, ElementAveugle                 hexa
	CleOprf, Evaluation, Pin, Tirage                                       hexa
	CleAlerte, IV, FicheScellee                                            hexa
}

// alert lit la fiche des entrées ; telephone absent n'est pas null.
func (e in) alert(t *testing.T) (Alert, bool) {
	a := Alert{UID: e.UID, Generation: e.Generation, Email: e.Email}
	if e.Telephone == nil {
		return a, false
	}
	if string(e.Telephone) != "null" {
		var p string
		if err := json.Unmarshal(e.Telephone, &p); err != nil {
			t.Fatal(err)
		}
		a.Telephone = &p
	}
	return a, true
}

// message rend le message de l'opération op depuis les entrées.
func message(t *testing.T, op string, e in) ([]byte, error) {
	switch op {
	case "preuveDeVie":
		return LifeMessage(e.UID, e.Defi)
	case "annulation":
		return CancelMessage(e.UID, e.SignalementID, e.Defi)
	case "charteGen":
		return GenerationMessage(e.UID, e.ContactID, e.Generation)
	case "preuveEnrolement":
		return EnrolMessage(e.UID, e.Defi, e.Generation, e.CleViePubEnvoyee)
	case "ficheAlerte":
		a, ok := e.alert(t)
		if !ok {
			return nil, errInvalid // telephone absent au lieu de null
		}
		return a.Message()
	}
	t.Fatalf("opération %s", op)
	return nil, nil
}

func TestVectors(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "annulation", "charteGen", "descellerFiche", "ficheAlerte", "preuveDeVie",
		"preuveEnrolement", "verifierAnnulation", "verifierCharteGen", "verifierFicheAlerte", "verifierPreuveDeVie",
		"verifierPreuveEnrolement")
	for _, op := range []string{"preuveDeVie", "annulation", "charteGen", "preuveEnrolement", "ficheAlerte"} {
		tr.Vectors(t, op, func(t *testing.T, v vecteurs.Vector) {
			var e in
			var want map[string]any
			vecteurs.Load(t, v.Inputs, &e)
			vecteurs.Load(t, v.Outputs, &want)
			msg, err := message(t, op, e)
			if err != nil {
				t.Fatal(err)
			}
			pub, _ := primitives.Ed25519Public(e.GraineVie)
			sig, _ := primitives.Ed25519Sign(e.GraineVie, msg) // ce que signe le client
			if !Verify(pub, msg, b64.EncodeToString(sig)) {
				t.Error("signature refusée")
			}
			got := map[string]any{"cleViePub": hexa(pub), "json": string(msg[bytes.IndexByte(msg, '{'):]), "message": string(msg),
				"signature": hexa(sig), "signatureBase64": b64.EncodeToString(sig)}
			if op == "ficheAlerte" { // scellée par alerte-seal (KMS simulé : IV ‖ chiffré ‖ tag)
				a, _ := e.alert(t)
				aad, _ := AlertAAD(a.UID, a.Generation)
				j, _ := a.JSON()
				c, _ := primitives.Seal(e.CleAlerte, e.IV, j, aad)
				sealed := append(append([]byte{}, e.IV...), c...)
				got["aad"], got["ficheScellee"], got["ficheScelleeBase64"] = string(aad), hexa(sealed), b64.EncodeToString(sealed)
			}
			vecteurs.Same(t, got, want)
		})
		tr.Refusals(t, op, func(t *testing.T, r vecteurs.Refusal) {
			var e in
			vecteurs.Load(t, r.Inputs, &e)
			if _, err := message(t, op, e); err == nil || err.Error() != r.Error {
				t.Errorf("erreur %v, attendu %s", err, r.Error)
			}
		})
		tr.Vectors(t, "verifier"+strings.ToUpper(op[:1])+op[1:], func(t *testing.T, v vecteurs.Vector) {
			var e in
			var want struct{ Valide bool }
			vecteurs.Load(t, v.Inputs, &e)
			vecteurs.Load(t, v.Outputs, &want)
			msg, err := message(t, op, e)
			if got := err == nil && Verify(e.CleViePub, msg, e.Signature); got != want.Valide {
				t.Errorf("valide = %v", got)
			}
		})
	}
	unseal := func(e in) (*Alert, error) {
		aad, err := AlertAAD(e.UID, e.Generation)
		if err != nil {
			return nil, err
		}
		plain, err := primitives.Open(e.CleAlerte, e.FicheScellee[:12], e.FicheScellee[12:], aad)
		if err != nil {
			return nil, err
		}
		return ReadAlert(plain, e.UID, e.Generation)
	}
	tr.Vectors(t, "descellerFiche", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ JSON string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		a, err := unseal(e)
		if err != nil {
			t.Fatal(err)
		}
		j, _ := a.JSON()
		vecteurs.Same(t, struct{ JSON string }{string(j)}, want)
	})
	tr.Refusals(t, "descellerFiche", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		if _, err := unseal(e); err == nil || err.Error() != r.Error {
			t.Errorf("erreur %v, attendu %s", err, r.Error)
		}
	})
}
