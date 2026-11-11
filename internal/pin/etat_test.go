package pin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

// CompteurPin est le vecteur compteurPin (testdata/vecteurs/compteurPin.json),
// écrit par outils/generer-compteurpin.
type CompteurPin struct {
	Appels []struct {
		Route, Motif string
		Le           time.Time
	}
	Apres []struct {
		Reponse         string
		ReessayerLe     *time.Time
		EssaisRestants  int
		ProchainEssaiLe *time.Time
		Evaluations     int64
	}
}

func readCompteurPin(t *testing.T) CompteurPin {
	t.Helper()
	f, err := vecteurs.Read("compteurPin.json")
	if err != nil || len(f.Vectors) != 1 || f.Vectors[0].Name != "compteurPin/suite" {
		t.Fatalf("compteurPin.json : %v", err)
	}
	var v CompteurPin
	if json.Unmarshal(f.Vectors[0].Inputs, &v) != nil || json.Unmarshal(f.Vectors[0].Outputs, &v) != nil || len(v.Appels) != len(v.Apres) {
		t.Fatal("compteurPin.json illisible")
	}
	return v
}

// TestCompteurPin rejoue le vecteur compteurPin sur Apply (PEV-02, PDP-01,
// PDP-03, PDP-13, API.md §2.12, §2.13 ; R-PIN-02, R-PIN-15).
func TestCompteurPin(t *testing.T) {
	v := readCompteurPin(t)
	records := map[string]*Record{}
	for i, a := range v.Appels {
		op := Op{UID: "u", Motif: a.Motif, At: a.Le}
		if a.Motif != "" && a.Motif != "ouverture" {
			op.Key = make([]byte, 32)
			op.Key[0] = byte(i + 1)
		}
		code, r := Apply(records, op)
		want := v.Apres[i]
		got := map[string]string{"": "evaluation", "pin_bloque": "pin_bloque", "trop_d_essais": "trop_d_essais"}[code]
		if code == "" && a.Motif == "ouverture" {
			got = "ouverture"
		}
		if code == "" && r.Left == 0 { // 10ᵉ essai : décompté, sans évaluation
			got = "pin_bloque"
		}
		next := (*time.Time)(nil)
		if !r.Next.IsZero() {
			next = &r.Next
		}
		if got != want.Reponse || r.Left != want.EssaisRestants || r.Evals != want.Evaluations ||
			(next == nil) != (want.ProchainEssaiLe == nil) || next != nil && !next.Equal(*want.ProchainEssaiLe) ||
			code == "trop_d_essais" && !r.Next.Equal(*want.ReessayerLe) || (r.Left == 0) != (r.Key == nil) {
			t.Fatalf("appel %d (%s %s) : %s, %+v ; attendu %+v", i+1, a.Route, a.Motif, code, r, want)
		}
	}
}

// TestApplySansPanique : une entrée fautive est rejetée sans panique.
func TestApplySansPanique(t *testing.T) { // « Entrées malformées »
	records := map[string]*Record{}
	for _, op := range []Op{{UID: "u"}, {UID: "u", Motif: "inconnu", Key: make([]byte, 32)}, {UID: "u", Motif: "redepot"},
		{UID: "u", Motif: "ouverture"}, {UID: "", Motif: "nouveau_pin", Key: make([]byte, 31)}} {
		if code, _ := Apply(records, op); code == "" {
			t.Fatalf("%+v admise", op)
		}
	}
	if len(records) != 0 {
		t.Fatal("enregistrement créé par une entrée refusée")
	}
}
