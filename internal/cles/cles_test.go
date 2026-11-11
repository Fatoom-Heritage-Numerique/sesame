package cles

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

type hexa = vecteurs.Hex

var b64 = base64.StdEncoding

type in struct {
	UID, ContactID, ContentID, Type                   string
	KTSGeneration, Generation                         int64
	IV, KMU, KTS, KMF, KME, KMO, KMV, KTSEnv, KTSAuth hexa
	MessageChiffre                                    string
	KTSChiffree, KTSIv, Chiffree, Charte, CharteIv    string
	CleKms, SealedKts, CodeInvitationScelle           hexa
	Code, Lookup, Saisie                              string
	Tirage                                            hexa
	Argon2M, Argon2P, Argon2T                         int
	Sel                                               string
}

func errIs(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Errorf("erreur %v, attendu %s", err, want)
	}
}

// must rend v, ou fait échouer le test (panique) sur une erreur inattendue.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// sealKMS et openKMS sont le format du KMS simulé des doubles : IV ‖ chiffré
// ‖ tag, AES-256-GCM.
func sealKMS(key, iv, plain, aad []byte) ([]byte, error) {
	c, err := primitives.Seal(key, iv, plain, aad)
	return append(append([]byte{}, iv...), c...), err
}

func openKMS(key, sealed, aad []byte) ([]byte, error) {
	if len(sealed) < 28 {
		return nil, primitives.ErrDecrypt
	}
	return primitives.Open(key, sealed[:12], sealed[12:], aad)
}

func TestKTSAndEnvelopes(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "enveloppeFichier", "enveloppeMot", "enveloppeMotDePasse", "enveloppeVolontes",
		"kts", "messageFichier", "ouvrirEnveloppe", "ouvrirKts")
	// aad de ktsChiffree (§10.2) : seul le client chiffre la KTS sous la KMU.
	ktsAAD := func(uid string) []byte { return []byte("fatoom-sesame-kts|1|" + uid) }
	tr.Vectors(t, "kts", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			AAD, KTSChiffreeBase64, KTSIv string
			KTSAuth, KTSChiffree, KTSEnv  hexa
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		sealed := must(primitives.Seal(e.KMU, e.IV, e.KTS, ktsAAD(e.UID)))
		vecteurs.Same(t, struct {
			AAD, KTSChiffreeBase64, KTSIv string
			KTSAuth, KTSChiffree, KTSEnv  hexa
		}{string(ktsAAD(e.UID)), b64.EncodeToString(sealed), b64.EncodeToString(e.IV),
			must(KTSAuth(e.KTS, e.UID)), sealed, must(KTSEnv(e.KTS, e.UID))}, want)
	})
	tr.Refusals(t, "kts", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := KTSEnv(e.KTS, e.UID)
		errIs(t, err, r.Error)
	})
	openKTS := func(t *testing.T, e in) ([]byte, error) {
		c, err1 := primitives.B64(e.KTSChiffree)
		iv, err2 := primitives.B64(e.KTSIv)
		if err1 != nil || err2 != nil {
			t.Fatal("ktsChiffree illisible")
		}
		return primitives.Open(e.KMU, iv, c, ktsAAD(e.UID))
	}
	tr.Vectors(t, "ouvrirKts", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ KTS hexa }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		vecteurs.Same(t, struct{ KTS hexa }{must(openKTS(t, e))}, want)
	})
	tr.Refusals(t, "ouvrirKts", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := openKTS(t, e)
		errIs(t, err, r.Error)
	})
	for op, typ := range map[string]string{"enveloppeFichier": "fichier", "enveloppeMotDePasse": "motdepasse",
		"enveloppeMot": "mot", "enveloppeVolontes": "volontes", "messageFichier": "fichier"} {
		key := func(e in) []byte {
			return map[string][]byte{"fichier": e.KMF, "motdepasse": e.KME, "mot": e.KMO, "volontes": e.KMV}[typ]
		}
		tr.Vectors(t, op, func(t *testing.T, v vecteurs.Vector) {
			var e in
			var raw map[string]any
			vecteurs.Load(t, v.Inputs, &e)
			vecteurs.Load(t, v.Outputs, &raw)
			d := Envelope{e.UID, typ, e.ContentID, e.ContactID, e.KTSGeneration}
			aad := must(d.AAD())
			ktsEnv := must(KTSEnv(e.KTS, e.UID))
			sealed := must(primitives.Seal(ktsEnv, e.IV, key(e), aad)) // ce que chiffre le client
			if opened := must(d.Open(ktsEnv, sealed, e.IV)); !bytes.Equal(opened, key(e)) {
				t.Error("enveloppe rouverte différente")
			}
			p := map[string]string{"fichier": "kmf", "motdepasse": "kme", "mot": "kmo", "volontes": "kmv"}[typ]
			got := map[string]any{"aad": string(aad), p + "Chiffree": hex.EncodeToString(sealed), p + "ChiffreeBase64": b64.EncodeToString(sealed),
				p + "Iv": b64.EncodeToString(e.IV), "ktsEnv": hex.EncodeToString(ktsEnv)}
			vecteurs.Same(t, got, raw)
		})
		tr.Refusals(t, op, func(t *testing.T, r vecteurs.Refusal) {
			var e in
			vecteurs.Load(t, r.Inputs, &e)
			_, err := Envelope{e.UID, typ, e.ContentID, e.ContactID, e.KTSGeneration}.AAD()
			errIs(t, err, r.Error)
		})
	}
	type openIn struct {
		UID, ContactID, ContentID, Type, Chiffree, IV string
		KTSGeneration                                 int64
		KTSEnv                                        hexa
	}
	open := func(t *testing.T, e openIn) ([]byte, error) {
		c, err1 := primitives.B64(e.Chiffree)
		iv, err2 := primitives.B64(e.IV)
		if err1 != nil || err2 != nil {
			t.Fatal("enveloppe illisible")
		}
		return Envelope{e.UID, e.Type, e.ContentID, e.ContactID, e.KTSGeneration}.Open(e.KTSEnv, c, iv)
	}
	tr.Vectors(t, "ouvrirEnveloppe", func(t *testing.T, v vecteurs.Vector) {
		var e openIn
		var want struct{ Cle hexa }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		vecteurs.Same(t, struct{ Cle hexa }{must(open(t, e))}, want)
	})
	tr.Refusals(t, "ouvrirEnveloppe", func(t *testing.T, r vecteurs.Refusal) {
		var e openIn
		vecteurs.Load(t, r.Inputs, &e)
		_, err := open(t, e)
		errIs(t, err, r.Error)
	})
}

func parseObject(t *testing.T, text string) jcs.Object {
	t.Helper()
	v, err := jcs.Parse([]byte(text))
	o, _ := v.(jcs.Object)
	if err != nil || o == nil {
		t.Fatalf("JSON du vecteur illisible : %v", err)
	}
	return o
}

func TestCharter(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "charte", "ouvrirCharte")
	tr.Vectors(t, "charte", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			AAD, CharteBase64, CharteIv, CharteJSON string
			Charte, KTSAuth                         hexa
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		c := parseObject(t, e.Charte)
		if err := CheckCharter(c); err != nil {
			t.Fatal(err)
		}
		gen := int64(c["generation"].(float64))
		aad := must(CharterAAD(e.UID, c["contactId"].(string), gen, e.KTSGeneration))
		ktsAuth := must(KTSAuth(e.KTS, e.UID))
		plain := must(jcs.Marshal(c))
		sealed := must(primitives.Seal(ktsAuth, e.IV, plain, aad)) // ce que chiffre le client
		opened := must(OpenCharter(ktsAuth, sealed, e.IV, e.UID, c["contactId"].(string), gen, e.KTSGeneration))
		text := must(jcs.Marshal(opened))
		vecteurs.Same(t, struct {
			AAD, CharteBase64, CharteIv, CharteJSON string
			Charte, KTSAuth                         hexa
		}{string(aad), b64.EncodeToString(sealed), b64.EncodeToString(e.IV), string(text), sealed, ktsAuth}, want)
	})
	tr.Refusals(t, "charte", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		errIs(t, CheckCharter(parseObject(t, e.Charte)), r.Error)
	})
	open := func(t *testing.T, e in) (Charter, error) {
		c, err1 := primitives.B64(e.Charte)
		iv, err2 := primitives.B64(e.CharteIv)
		if err1 != nil || err2 != nil {
			t.Fatal("charte illisible")
		}
		return OpenCharter(e.KTSAuth, c, iv, e.UID, e.ContactID, e.Generation, e.KTSGeneration)
	}
	tr.Vectors(t, "ouvrirCharte", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ CharteJSON string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		vecteurs.Same(t, struct{ CharteJSON string }{string(must(jcs.Marshal(must(open(t, e)))))}, want)
	})
	tr.Refusals(t, "ouvrirCharte", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := open(t, e)
		errIs(t, err, r.Error)
	})
}

func TestCodesAndSeals(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "codeAcces", "codeInvitation", "codeVerifier", "descellerInvitation",
		"descellerKts", "scelle", "tirerCodeAcces")
	tr.Vectors(t, "tirerCodeAcces", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ Affiche, Code string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		code := must(DrawCode(e.Tirage))
		vecteurs.Same(t, struct{ Affiche, Code string }{DisplayCode(code), code}, want)
	})
	tr.Vectors(t, "codeAcces", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ Affiche, Normalise string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		code := must(NormalizeCode(e.Saisie))
		vecteurs.Same(t, struct{ Affiche, Normalise string }{DisplayCode(code), code}, want)
	})
	tr.Refusals(t, "codeAcces", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := NormalizeCode(e.Saisie)
		errIs(t, err, r.Error)
	})
	tr.Vectors(t, "codeVerifier", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			CodeVerifier string
			Empreinte    hexa
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		code := must(NormalizeCode(e.Saisie))
		cv := CodeVerifier(code)
		if !CheckCode(code, want.CodeVerifier) || CheckCode("UEX4J2E7WDV8", want.CodeVerifier) {
			t.Error("contrôle du code faux")
		}
		vecteurs.Same(t, struct {
			CodeVerifier string
			Empreinte    hexa
		}{cv, must(primitives.B64(cv))}, want)
	})
	tr.Vectors(t, "scelle", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			AAD, SealedKtsBase64 string
			SealedKts            hexa
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		aad := must(SealedKTSAAD(e.UID, e.KTSGeneration))
		sealed := must(sealKMS(e.CleKms, e.IV, e.KTS, aad))
		vecteurs.Same(t, struct {
			AAD, SealedKtsBase64 string
			SealedKts            hexa
		}{string(aad), b64.EncodeToString(sealed), sealed}, want)
	})
	unsealKTS := func(e in) ([]byte, error) {
		aad, err := SealedKTSAAD(e.UID, e.KTSGeneration)
		if err != nil {
			return nil, err
		}
		return openKMS(e.CleKms, e.SealedKts, aad)
	}
	tr.Vectors(t, "descellerKts", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ KTS hexa }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		vecteurs.Same(t, struct{ KTS hexa }{must(unsealKTS(e))}, want)
	})
	tr.Refusals(t, "descellerKts", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := unsealKTS(e)
		errIs(t, err, r.Error)
	})
	tr.Vectors(t, "codeInvitation", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			AAD, CodeInvitationScelleBase64, Normalise, NormaliseRetrouve string
			CodeInvitationScelle                                          hexa
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		code := must(NormalizeCode(e.Code))
		aad := must(InvitationAAD(e.UID, e.ContactID, e.Lookup))
		sealed := must(sealKMS(e.CleKms, e.IV, []byte(code), aad))
		back := must(openKMS(e.CleKms, sealed, aad))
		vecteurs.Same(t, struct {
			AAD, CodeInvitationScelleBase64, Normalise, NormaliseRetrouve string
			CodeInvitationScelle                                          hexa
		}{string(aad), b64.EncodeToString(sealed), code, string(back), sealed}, want)
	})
	unsealInvitation := func(e in) (string, error) {
		aad, err := InvitationAAD(e.UID, e.ContactID, e.Lookup)
		if err != nil {
			return "", err
		}
		plain, err := openKMS(e.CleKms, e.CodeInvitationScelle, aad)
		if err != nil {
			return "", err
		}
		return NormalizeCode(string(plain))
	}
	tr.Vectors(t, "descellerInvitation", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ Normalise string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		vecteurs.Same(t, struct{ Normalise string }{must(unsealInvitation(e))}, want)
	})
	tr.Refusals(t, "descellerInvitation", func(t *testing.T, r vecteurs.Refusal) {
		var e in
		vecteurs.Load(t, r.Inputs, &e)
		_, err := unsealInvitation(e)
		errIs(t, err, r.Error)
	})
}
