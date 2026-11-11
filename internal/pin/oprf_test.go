package pin

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/cloudflare/circl/oprf"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

func unhex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// rfc9497 sont les vecteurs de l'annexe A.1.1 de la RFC 9497 (ristretto255-
// SHA512, mode OPRF), recopiés de https://www.rfc-editor.org/rfc/rfc9497.txt.
var rfc9497 = struct {
	seed, keyInfo, skSm string
	vectors             []struct{ input, blind, blinded, evaluation, output string }
}{
	seed:    "a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3a3",
	keyInfo: "74657374206b6579",
	skSm:    "5ebcea5ee37023ccb9fc2d2019f9d7737be85591ae8652ffa9ef0f4d37063b0e",
	vectors: []struct{ input, blind, blinded, evaluation, output string }{
		{"00", "64d37aed22a27f5191de1c1d69fadb899d8862b58eb4220029e036ec4c1f6706",
			"609a0ae68c15a3cf6903766461307e5c8bb2f95e7e6550e1ffa2dc99e412803c",
			"7ec6578ae5120958eb2db1745758ff379e77cb64fe77b0b2d8cc917ea0869c7e",
			"527759c3d9366f277d8c6020418d96bb393ba2afb20ff90df23fb7708264e2f3ab9135e3bd69955851de4b1f9fe8a0973396719b7912ba9ee8aa7d0b5e24bcf6"},
		{"5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a", "64d37aed22a27f5191de1c1d69fadb899d8862b58eb4220029e036ec4c1f6706",
			"da27ef466870f5f15296299850aa088629945a17d1f5b7f5ff043f76b3c06418",
			"b4cbf5a4f1eeda5a63ce7b77c7d23f461db3fcab0dd28e4e17cecb5c90d02c25",
			"f4a74c9c592497375e796aa837e907b1a045d34306a749db9f34221f7e750cb4f2a6413a6bf6fa5e19ba6348eb673934a722a7ede2e7621306d18951e7cf2c73"},
	},
}

// blindAndFinalize joue le client par circl : aveuglement déterministe,
// évaluation par Evaluate, puis Finalize.
func blindAndFinalize(t *testing.T, input []byte, blind oprf.Blind, key []byte) (blinded, evaluation, output []byte) {
	t.Helper()
	client := oprf.NewClient(Suite)
	fin, req, err := client.DeterministicBlind([][]byte{input}, []oprf.Blind{blind})
	if err != nil {
		t.Fatal(err)
	}
	blinded, _ = req.Elements[0].MarshalBinaryCompress()
	if evaluation, err = Evaluate(key, blinded); err != nil {
		t.Fatal(err)
	}
	e := Suite.Group().NewElement()
	if err := e.UnmarshalBinary(evaluation); err != nil {
		t.Fatal(err)
	}
	outputs, err := client.Finalize(fin, &oprf.Evaluation{Elements: []oprf.Evaluated{e}})
	if err != nil {
		t.Fatal(err)
	}
	return blinded, evaluation, outputs[0]
}

// TestRFC9497 vérifie circl et Evaluate sur l'annexe A.1.1 : clé dérivée de
// Seed et KeyInfo, puis, pour chaque vecteur, élément aveuglé, évaluation et
// sortie.
func TestRFC9497(t *testing.T) {
	sk, err := oprf.DeriveKey(Suite, oprf.BaseMode, unhex(rfc9497.seed), unhex(rfc9497.keyInfo))
	if err != nil {
		t.Fatal(err)
	}
	key, _ := sk.MarshalBinary()
	if hex.EncodeToString(key) != rfc9497.skSm {
		t.Fatalf("skSm = %x", key)
	}
	for i, v := range rfc9497.vectors {
		blind := Suite.Group().NewScalar()
		if err := blind.UnmarshalBinary(unhex(v.blind)); err != nil {
			t.Fatal(err)
		}
		blinded, evaluation, output := blindAndFinalize(t, unhex(v.input), blind, key)
		if hex.EncodeToString(blinded) != v.blinded || hex.EncodeToString(evaluation) != v.evaluation || hex.EncodeToString(output) != v.output {
			t.Errorf("vecteur %d : aveuglé %x, évaluation %x, sortie %x", i+1, blinded, evaluation, output)
		}
	}
	if _, err := Evaluate(key, make([]byte, 32)); err == nil {
		t.Error("identité acceptée")
	}
	if _, err := Evaluate(key, bytes.Repeat([]byte{0xff}, 32)); err == nil {
		t.Error("élément hors forme accepté")
	}
}

type depositIn struct {
	CleOprf, Evaluation, GraineVie, Pin, Tirage, CleViePub, ElementAveugle vecteurs.Hex
	Defi, GrappeID, Motif, UID, Signature                                  string
}

// TestDepositVectors rejoue depotPin et verifierDepotPin : évaluation par
// Sésame de l'élément aveuglé du vecteur, égale à evaluation (cleOprf du
// vecteur), message signé par cleVie et sa vérification.
func TestDepositVectors(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "depotPin", "verifierDepotPin")
	b64 := func(b []byte) string { return primitives.B64Encode(b) }
	tr.Vectors(t, "depotPin", func(t *testing.T, v vecteurs.Vector) {
		var e depositIn
		var want struct {
			CleViePub, ElementAveugle, Signature         vecteurs.Hex
			ElementAveugleBase64, EmpreinteElement, JSON string
			Message, SignatureBase64                     string
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		// L'aveuglement du PIN par le tirage est fait par le client ;
		// Sésame évalue l'élément qu'il reçoit.
		blinded := []byte(want.ElementAveugle)
		if evaluation, err := Evaluate(e.CleOprf, blinded); err != nil || !bytes.Equal(evaluation, e.Evaluation) {
			t.Errorf("évaluation %x, %v ; attendu %x", evaluation, err, e.Evaluation)
		}
		msg, err := vie.PinDepositMessage(e.UID, e.GrappeID, blinded, e.Defi, e.Motif)
		if err != nil {
			t.Fatal(err)
		}
		pub, _ := primitives.Ed25519Public(e.GraineVie)
		sig, _ := primitives.Ed25519Sign(e.GraineVie, msg)
		if !vie.Verify(pub, msg, b64(sig)) {
			t.Error("signature refusée")
		}
		vecteurs.Same(t, struct {
			CleViePub, ElementAveugle, Signature         vecteurs.Hex
			ElementAveugleBase64, EmpreinteElement, JSON string
			Message, SignatureBase64                     string
		}{pub, blinded, sig, b64(blinded), hex.EncodeToString(primitives.SHA256(blinded)), string(msg[bytes.IndexByte(msg, '{'):]),
			string(msg), b64(sig)}, want)
	})
	tr.Refusals(t, "depotPin", func(t *testing.T, r vecteurs.Refusal) {
		var e depositIn
		vecteurs.Load(t, r.Inputs, &e)
		if _, err := vie.PinDepositMessage(e.UID, e.GrappeID, make([]byte, 32), e.Defi, e.Motif); err == nil || err.Error() != r.Error {
			t.Errorf("erreur %v, attendu %s", err, r.Error)
		}
	})
	tr.Vectors(t, "verifierDepotPin", func(t *testing.T, v vecteurs.Vector) {
		var e depositIn
		var want struct{ Valide bool }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		msg, err := vie.PinDepositMessage(e.UID, e.GrappeID, e.ElementAveugle, e.Defi, e.Motif)
		if got := err == nil && vie.Verify(e.CleViePub, msg, e.Signature); got != want.Valide {
			t.Errorf("valide = %v", got)
		}
	})
}
