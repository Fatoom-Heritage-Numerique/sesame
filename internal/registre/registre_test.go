package registre

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

type hexa = vecteurs.Hex

type in struct {
	Cles                                     any
	Doc, Ecriture, Ecritures, Tete, Fichier  string
	Nom, Bornes                              string
	Graine, SigDer, SignatureDer, Precedente hexa
	EpoqueLue, DerniereEcrite, Maintenant    int64
}

func parse(t *testing.T, text string) any {
	t.Helper()
	v, err := jcs.Parse([]byte(text))
	if err != nil {
		t.Fatalf("JSON du vecteur illisible : %v", err)
	}
	return v
}

func keyList(t *testing.T, e in) [][]byte {
	var out [][]byte
	for _, k := range e.Cles.([]any) {
		var h hexa
		if err := h.UnmarshalJSON([]byte(`"` + k.(string) + `"`)); err != nil {
			t.Fatal(err)
		}
		out = append(out, h)
	}
	return out
}

func keyMap(t *testing.T, e in) map[string][]byte {
	out := map[string][]byte{}
	for kid, k := range e.Cles.(map[string]any) {
		var h hexa
		if err := h.UnmarshalJSON([]byte(`"` + k.(string) + `"`)); err != nil {
			t.Fatal(err)
		}
		out[kid] = h
	}
	return out
}

// toWrite lit une écriture du vecteur {"epoque", "documents": [{chemin, doc}]}.
func toWrite(v any) Write {
	o := v.(jcs.Object)
	w := Write{Epoch: int64(o["epoque"].(float64))}
	for _, d := range o["documents"].(jcs.Array) {
		doc, _ := d.(jcs.Object)["doc"].(jcs.Object)
		w.Documents = append(w.Documents, Document{d.(jcs.Object)["chemin"].(string), doc})
	}
	return w
}

// signSelf signe msg par la graine P-256 avec crypto/ecdsa (aléa, comme KMS)
// et rend la signature R ‖ S en base64url, convertie depuis DER.
func signSelf(t *testing.T, seed, msg []byte) string {
	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), seed)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := ecdsa.SignASN1(rand.Reader, priv, primitives.SHA256(msg))
	raw, err := jetons.DERToRaw(der)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestRegistre(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "chaineRegistre", "empreinteRegistre", "ligneRegistre")
	tr.Vectors(t, "empreinteRegistre", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			Empreinte hexa
			JSON      string
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		w := toWrite(parse(t, e.Ecriture))
		j, err := w.JSON()
		if err != nil {
			t.Fatal(err)
		}
		sum, err := Next(e.Precedente, w)
		if err != nil {
			t.Fatal(err)
		}
		vecteurs.Same(t, struct {
			Empreinte hexa
			JSON      string
		}{sum, string(j)}, want)
	})
	tr.Vectors(t, "ligneRegistre", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			Message string
			Sig     hexa
			Valide  bool
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		doc := parse(t, e.Doc).(jcs.Object)
		msg, err := LineMessage(doc)
		if err != nil {
			t.Fatal(err)
		}
		got := struct {
			Message string
			Sig     hexa
			Valide  bool
		}{Message: string(msg), Valide: VerifyLine(doc, keyList(t, e))}
		if e.SigDer != nil { // sig convertie depuis la signature DER de KMS
			raw, err := jetons.DERToRaw(e.SigDer)
			if err != nil {
				t.Fatal(err)
			}
			if doc["sig"] != base64.RawURLEncoding.EncodeToString(raw) {
				t.Error("sig différente de sigDer convertie")
			}
			got.Sig = raw
			signed := jcs.Object{}
			for k, x := range doc {
				signed[k] = x
			}
			signed["sig"] = signSelf(t, e.Graine, msg)
			if !VerifyLine(signed, keyList(t, e)) {
				t.Error("ligne signée par Go refusée")
			}
		}
		vecteurs.Same(t, got, want)
	})
	tr.Vectors(t, "chaineRegistre", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ Valide bool }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		var writes []Write
		for _, w := range parse(t, e.Ecritures).(jcs.Array) {
			writes = append(writes, toWrite(w))
		}
		if got := VerifyChain(writes, parse(t, e.Tete).(jcs.Object), keyList(t, e)); got != want.Valide {
			t.Errorf("valide = %v", got)
		}
	})
}

func TestBorne(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "antiRetour", "borne", "verifierBorne")
	tr.Vectors(t, "borne", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			JSON, Message, Nom string
			Signature          hexa
			Valide             bool
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		f := parse(t, e.Fichier).(jcs.Object)
		msg, err := BorneMessage(f["borne"])
		if err != nil {
			t.Fatal(err)
		}
		name, _ := BorneName(f["borne"])
		raw, err := jetons.DERToRaw(e.SignatureDer)
		if err != nil || f["signature"] != base64.RawURLEncoding.EncodeToString(raw) {
			t.Fatal("signature différente de signatureDer convertie")
		}
		mine := jcs.Object{"borne": f["borne"], "kid": f["kid"], "signature": signSelf(t, e.Graine, msg)}
		if !VerifyBorne(name, mine, keyMap(t, e)) {
			t.Error("borne signée par Go refusée")
		}
		j, _ := jcs.Marshal(f["borne"])
		vecteurs.Same(t, struct {
			JSON, Message, Nom string
			Signature          hexa
			Valide             bool
		}{string(j), string(msg), name, raw, VerifyBorne(e.Nom, f, keyMap(t, e))}, want)
	})
	tr.Vectors(t, "verifierBorne", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct{ Valide bool }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		if got := VerifyBorne(e.Nom, parse(t, e.Fichier), keyMap(t, e)); got != want.Valide {
			t.Errorf("valide = %v", got)
		}
	})
	tr.Vectors(t, "antiRetour", func(t *testing.T, v vecteurs.Vector) {
		var e in
		var want struct {
			Admis     bool
			Reference *int64
		}
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		var bornes []Deposited
		for _, b := range parse(t, e.Bornes).(jcs.Array) {
			o := b.(jcs.Object)
			created, err := primitives.ReadTime(o["timeCreated"].(string))
			if err != nil {
				t.Fatal(err)
			}
			bornes = append(bornes, Deposited{o["nom"].(string), o["fichier"], created})
		}
		ref, ok := Reference(bornes, keyMap(t, e), time.UnixMilli(e.Maintenant))
		got := struct {
			Admis     bool
			Reference *int64
		}{UpToDate(e.EpoqueLue, ref, ok, e.DerniereEcrite), nil}
		if ok {
			got.Reference = &ref
		}
		vecteurs.Same(t, got, want)
	})
}
