package canal

import (
	"bytes"
	"crypto/ecdh"
	"crypto/mlkem"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

type hexa = vecteurs.Hex

type in struct {
	SessionPriv, SessionPub, CliPriv, CliPub, GraineKem, KemPub, KemCt, AleaEncapsulation hexa
	KReq, KRes, IVPrefixe                                                                 hexa
	InstanceID, Requete, Enveloppe                                                        string
	N                                                                                     int64
	Octets                                                                                hexa
	Prefixe                                                                               string
}

func errIs(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Errorf("erreur %v, attendu %s", err, want)
	}
}

func canonical(t *testing.T, text string) jcs.Object {
	t.Helper()
	v, err := jcs.Parse([]byte(text))
	o, _ := v.(jcs.Object)
	if err != nil || o == nil {
		t.Fatalf("JSON du vecteur illisible : %v", err)
	}
	return o
}

// openResponse est la lecture d'une réponse par le client (API.md §1.3), pour
// rejouer ses refus sur les IV, aad et réponses que Sésame produit.
func openResponse(kRes, prefix []byte, n int64, envelope string) (jcs.Object, error) {
	v, err := jcs.Parse([]byte(envelope))
	o, _ := v.(jcs.Object)
	if err != nil || len(o) != 2 || o["n"] != float64(n) {
		return nil, primitives.ErrInvalid
	}
	s, _ := o["body"].(string)
	body, err := primitives.B64(s)
	if err != nil {
		return nil, err
	}
	iv, err := IV(prefix, n)
	if err != nil {
		return nil, err
	}
	aad, _ := AAD(false, n)
	plain, err := primitives.Open(kRes, iv, body, aad)
	if err != nil {
		return nil, err
	}
	r, err := jcs.ReadCanonical(plain)
	ro, _ := r.(jcs.Object)
	if err != nil || ro == nil {
		return nil, primitives.ErrInvalid
	}
	return ro, nil
}

func TestVectors(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "canal", "canalReponse", "canalRequete", "instanceId", "lireInstanceId", "ouvrirCanal")
	tr.Vectors(t, "canal", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			CliPub, KReq, KRes, Shared, SsKem, Transcription hexa
			EatNonceKemPub, EatNonceSessionPub               string
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		inst, err := NewInstance(e.SessionPriv, e.GraineKem)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(inst.SessionPub, e.SessionPub) || !bytes.Equal(inst.KemPub, e.KemPub) {
			t.Fatal("clés publiques de l'instance différentes du vecteur")
		}
		cliPub, _ := primitives.X25519Public(e.CliPriv) // côté client
		shared, err := primitives.X25519(e.SessionPriv, cliPub)
		ssKem, err2 := primitives.MLKEMDecapsulate(e.GraineKem, e.KemCt)
		transcript, err3 := Transcript(inst.SessionPub, cliPub, inst.KemPub, e.KemCt)
		kReq, kRes, err4 := inst.Keys(cliPub, e.KemCt)
		for _, err := range []error{err, err2, err3, err4} {
			if err != nil {
				t.Fatal(err)
			}
		}
		n := attestation.ChannelNonces(inst.SessionPub, inst.KemPub)
		vecteurs.Same(t, struct {
			CliPub, KReq, KRes, Shared, SsKem, Transcription hexa
			EatNonceKemPub, EatNonceSessionPub               string
		}{cliPub, kReq, kRes, shared, ssKem, transcript, n[1], n[0]}, want)
	})
	tr.Refusals(t, "canal", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := primitives.X25519(e.CliPriv, e.SessionPub) // côté client
		errIs(t, err, r.Error)
		// Côté enclave, le même point d'ordre faible reçu comme cliPub.
		inst, err := NewInstance(bytes.Repeat([]byte{7}, 32), e.GraineKem)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = inst.Keys(e.SessionPub, e.KemCt)
		errIs(t, err, r.Error)
	})
	tr.Vectors(t, "ouvrirCanal", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			LongueurKemCt    int
			N                int64
			RequeteRetrouvee bool
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		// Client tiré au hasard : paire X25519, encapsulation, ivPrefixe.
		cli, _ := ecdh.X25519().GenerateKey(rand.Reader)
		ek, err := mlkem.NewEncapsulationKey768(e.KemPub)
		if err != nil {
			t.Fatal(err)
		}
		ss, kemCt := ek.Encapsulate()
		shared, _ := primitives.X25519(cli.Bytes(), e.SessionPub)
		transcript, _ := Transcript(e.SessionPub, cli.PublicKey().Bytes(), e.KemPub, kemCt)
		cReq, _, _ := DeriveKeys(ss, shared, transcript)
		prefix, _ := primitives.Random(PrefixSize)
		iv, _ := IV(prefix, 1)
		aad, _ := AAD(true, 1)
		plain, _ := jcs.Canonicalize([]byte(e.Requete))
		body, _ := primitives.Seal(cReq, iv, plain, aad)
		envelope, _ := jcs.Marshal(jcs.Object{"instanceId": e.InstanceID, "cliPub": b64.EncodeToString(cli.PublicKey().Bytes()),
			"kemCt": b64.EncodeToString(kemCt), "ivPrefixe": b64.EncodeToString(prefix), "n": 1, "body": b64.EncodeToString(body)})
		// Côté enclave.
		inst, err := NewInstance(e.SessionPriv, e.GraineKem)
		if err != nil {
			t.Fatal(err)
		}
		env, err := ReadEnvelope(envelope)
		if err != nil {
			t.Fatal(err)
		}
		kReq, _, err := inst.Keys(env.CliPub, env.KemCt)
		if err != nil {
			t.Fatal(err)
		}
		route, corps, err := OpenRequest(kReq, env)
		if err != nil {
			t.Fatal(err)
		}
		back, _ := jcs.Marshal(jcs.Object{"route": route, "corps": corps})
		vecteurs.Same(t, struct {
			LongueurKemCt    int
			N                int64
			RequeteRetrouvee bool
		}{len(env.KemCt), env.N, string(back) == e.Requete}, want)
	})
	tr.Refusals(t, "ouvrirCanal", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		// Le client refuse d'encapsuler vers une kemPub hors module ou de
		// longueur fausse (FIPS 203, §7.2).
		if _, err := mlkem.NewEncapsulationKey768(e.KemPub); err == nil || r.Error != string(primitives.ErrKey) {
			t.Errorf("kemPub acceptée, ou refus attendu %s", r.Error)
		}
	})
	tr.Vectors(t, "canalRequete", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			AAD, Enveloppe string
			Body, IV       hexa
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		env, err := ReadEnvelope([]byte(want.Enveloppe))
		if err != nil {
			t.Fatal(err)
		}
		if env.InstanceID != e.InstanceID || !bytes.Equal(env.CliPub, e.CliPub) || !bytes.Equal(env.KemCt, e.KemCt) ||
			!bytes.Equal(env.IVPrefix, e.IVPrefixe) || env.N != e.N || !bytes.Equal(env.Body, want.Body) {
			t.Error("enveloppe lue différente des entrées")
		}
		route, corps, err := OpenRequest(e.KReq, env)
		if err != nil {
			t.Fatal(err)
		}
		if back, _ := jcs.Marshal(jcs.Object{"route": route, "corps": corps}); string(back) != e.Requete {
			t.Errorf("requête ouverte %s", back)
		}
		iv, _ := IV(e.IVPrefixe, e.N)
		aad, _ := AAD(true, e.N)
		plain, _ := jcs.Canonicalize([]byte(e.Requete))
		body, _ := primitives.Seal(e.KReq, iv, plain, aad) // ce que scelle le client
		vecteurs.Same(t, struct {
			AAD, Enveloppe string
			Body, IV       hexa
		}{string(aad), want.Enveloppe, body, iv}, want)
	})
	tr.Refusals(t, "canalRequete", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := IV(e.IVPrefixe, e.N)
		errIs(t, err, r.Error)
		_, err = AAD(true, e.N)
		errIs(t, err, r.Error)
		env, _ := jcs.Marshal(jcs.Object{"instanceId": e.InstanceID, "cliPub": b64.EncodeToString(e.CliPub), "kemCt": b64.EncodeToString(e.KemCt),
			"ivPrefixe": b64.EncodeToString(e.IVPrefixe), "n": e.N, "body": base64.StdEncoding.EncodeToString(make([]byte, 16))})
		_, err = ReadEnvelope(env)
		errIs(t, err, r.Error)
	})
	tr.Vectors(t, "canalReponse", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			AAD, Reponse string
			IV           hexa
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		sealed, err := SealResponse(e.KRes, e.IVPrefixe, e.N, canonical(t, want.Reponse))
		if err != nil {
			t.Fatal(err)
		}
		if string(sealed) != e.Enveloppe {
			t.Errorf("réponse scellée %s", sealed)
		}
		resp, err := openResponse(e.KRes, e.IVPrefixe, e.N, e.Enveloppe)
		if err != nil {
			t.Fatal(err)
		}
		text, _ := jcs.Marshal(resp)
		iv, _ := IV(e.IVPrefixe, e.N)
		aad, _ := AAD(false, e.N)
		vecteurs.Same(t, struct {
			AAD, Reponse string
			IV           hexa
		}{string(aad), string(text), iv}, want)
	})
	tr.Refusals(t, "canalReponse", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := openResponse(e.KRes, e.IVPrefixe, e.N, e.Enveloppe)
		errIs(t, err, r.Error)
	})
	tr.Vectors(t, "instanceId", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ InstanceID string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		id, err := InstanceID(e.Prefixe, e.Octets)
		if err != nil || id != want.InstanceID {
			t.Errorf("obtenu %q, %v", id, err)
		}
	})
	tr.Vectors(t, "lireInstanceId", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ Chemin, Prefixe string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		p, err := ParseInstanceID(e.InstanceID)
		if err != nil {
			t.Fatal(err)
		}
		vecteurs.Same(t, struct{ Chemin, Prefixe string }{"/n/" + p + "/canal", p}, want)
	})
	tr.Refusals(t, "lireInstanceId", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := ParseInstanceID(e.InstanceID)
		errIs(t, err, r.Error)
	})
}
