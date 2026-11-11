package jcs

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"unicode"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

func decode(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("vecteur illisible : %v", err)
	}
}

func TestVectors(t *testing.T) {
	tr := vecteurs.Track(t, "json.json", "jsonCanonique", "nombre")
	tr.Vectors(t, "jsonCanonique", func(t *testing.T, v vecteurs.Vector) {
		var in struct{ JSON string }
		var out struct{ Texte, UTF8 string }
		decode(t, v.Inputs, &in)
		decode(t, v.Outputs, &out)
		got, err := Canonicalize([]byte(in.JSON))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != out.Texte || hex.EncodeToString(got) != out.UTF8 {
			t.Errorf("obtenu %s", got)
		}
		if _, err := ReadCanonical(got); err != nil {
			t.Errorf("forme canonique relue refusée : %v", err)
		}
	})
	tr.Refusals(t, "jsonCanonique", func(t *testing.T, r vecteurs.Refusal) {
		var in struct{ JSON string }
		decode(t, r.Inputs, &in)
		if _, err := Canonicalize([]byte(in.JSON)); !errors.Is(err, ErrInvalid) || err.Error() != r.Error {
			t.Errorf("erreur %v, attendu %s", err, r.Error)
		}
	})
	number := func(t *testing.T, raw json.RawMessage) (string, error) {
		var in struct{ IEEE754 string }
		decode(t, raw, &in)
		b, err := hex.DecodeString(in.IEEE754)
		if err != nil || len(b) != 8 {
			t.Fatalf("ieee754 illisible : %q", in.IEEE754)
		}
		return FormatNumber(math.Float64frombits(binary.BigEndian.Uint64(b)))
	}
	tr.Vectors(t, "nombre", func(t *testing.T, v vecteurs.Vector) {
		var out struct{ Texte string }
		decode(t, v.Outputs, &out)
		if got, err := number(t, v.Inputs); err != nil || got != out.Texte {
			t.Errorf("obtenu %q, %v ; attendu %q", got, err, out.Texte)
		}
	})
	tr.Refusals(t, "nombre", func(t *testing.T, r vecteurs.Refusal) {
		if _, err := number(t, r.Inputs); err == nil || err.Error() != r.Error {
			t.Errorf("erreur %v, attendu %s", err, r.Error)
		}
	})
}

func TestTablesVersion(t *testing.T) {
	if unicodeVersion != unicode.Version {
		t.Fatalf("tables NFC d'Unicode %s, Go en %s : relancer outils/generer-nfc", unicodeVersion, unicode.Version)
	}
}

func TestNFC(t *testing.T) {
	for s, want := range map[string]bool{
		"\u00e9lise": true, "e\u0301lise": false, "\ufb33": false, "\u212b": false,
		"q\u0323\u0301": true, "q\u0301\u0323": false, "a\u0316\u0301": false, "a\u0301\u0316": false,
		"\u091c\u093c": true, "\u0928\u093c": false, "\u0bb0\u0bbe": true, "\u0bca": true, "\u0bc6\u0bbe": false,
		"\u1100\u1161": false, "\uac00\u11a8": false, "\uac01": true, "\xff": false,
	} {
		if got := IsNFC(s); got != want {
			t.Errorf("IsNFC(%+q) = %v, attendu %v", s, got, want)
		}
	}
}

func TestParseRefusals(t *testing.T) {
	for _, s := range []string{`{"a":1,"a":2}`, `[1,]`, `01`, `1.`, `"\x"`, `{"a":1} x`, `"\ud800A"`, `1e400`, "\"\x01\""} {
		if _, err := Parse([]byte(s)); err == nil {
			t.Errorf("%q accepté", s)
		}
	}
	if _, err := Marshal(Object{"a": int64(1 << 54)}); err == nil {
		t.Error("entier au-delà de 2⁵³ accepté")
	}
	if _, err := ReadCanonical([]byte(`{"b":1, "a":2}`)); err == nil {
		t.Error("forme non canonique acceptée")
	}
}
