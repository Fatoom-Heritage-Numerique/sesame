package lettres

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// La lettre est identique octet pour octet pour des entrées fixes (tests.md
// de Fatoom, « Lettres ») : l'empreinte ci-dessous ne change qu'avec le
// générateur, par un commit qui le dit.
func TestPDFDeterministe(t *testing.T) {
	l := Letter{Address: []string{"Léa Martin", "1 rue de l'Exemple", "75001 Paris"},
		Text: "Texte de test lettre_deces. Léa (Jean Dupont) \\ œuvre — 12 € ✓\nSecond paragraphe.", Code: "ABCD-EFGH-JKLM"}
	a, b := PDF(l), PDF(l)
	sum := sha256.Sum256(a)
	if !bytes.Equal(a, b) || hex.EncodeToString(sum[:]) != want {
		t.Fatalf("PDF : %s", hex.EncodeToString(sum[:]))
	}
	for _, s := range []string{"%PDF-1.4", "/BaseFont/Helvetica/Encoding/WinAnsiEncoding", "(L\xe9a Martin) Tj", "(ABCD-EFGH-JKLM) Tj", "\\(Jean Dupont\\) \\\\ \x9cuvre \x97 12 \x80 ?", "%%EOF"} {
		if !bytes.Contains(a, []byte(s)) {
			t.Fatalf("PDF sans %q", s)
		}
	}
}

const want = "76c07562b5c1066a342bc4909832fd2acb94d1fed0d0d1e11a88bcc99b747a34"
