// Paquet lettres : la lettre d'un proche, PDF texte d'une page, sans
// dépendance (ARCHITECTURE.md §11.9 ; docs/technique/sesame/protocoles.md,
// « Lettre », étape 3) : A4, police standard Helvetica en encodage WinAnsi,
// aucune police embarquée, adresse du destinataire dans la zone fenêtre
// gauche, texte attesté, code en grand. Le générateur est déterministe : une
// même lettre donne les mêmes octets (test par comparaison d'octets).
package lettres

import (
	"bytes"
	"fmt"
	"strings"
)

// Zone d'adresse (fenêtre gauche) et marges, en points (1 mm = 2,8346 pt) ;
// la zone exacte qu'accepte le prestataire d'impression est relevée au lot 9
// (docs/technique/actions-manuelles.md, « Pingen pour Sésame »).
const (
	pageW, pageH = 595.28, 841.89
	left         = 70.87  // 25 mm
	addrTop      = 714.33 // 45 mm sous le haut de la page
	textTop      = 566.93 // 100 mm sous le haut de la page
	wrapAt       = 90     // caractères par ligne du texte
)

// Letter est le contenu d'une lettre : lignes de l'adresse, texte et code.
type Letter struct {
	Address []string
	Text    string
	Code    string
}

// winAnsi rend s en WinAnsiEncoding, échappé pour une chaîne PDF ; un
// caractère hors de l'encodage devient « ? ».
func winAnsi(s string) string {
	extra := map[rune]byte{'€': 0x80, '…': 0x85, 'Œ': 0x8C, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97, 'œ': 0x9C}
	var b strings.Builder
	for _, r := range s {
		c, ok := extra[r]
		switch {
		case ok:
		case r == '(' || r == ')' || r == '\\':
			b.WriteByte('\\')
			c = byte(r)
		case r >= 0x20 && r < 0x7F || r >= 0xA0 && r <= 0xFF:
			c = byte(r)
		default:
			c = '?'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// wrap coupe un paragraphe en lignes d'au plus n caractères, aux espaces.
func wrap(p string, n int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(p) {
		if line != "" && len([]rune(line))+1+len([]rune(w)) > n {
			lines, line = append(lines, line), ""
		}
		line = strings.TrimSpace(line + " " + w)
	}
	return append(lines, line)
}

// PDF rend la lettre.
func PDF(l Letter) []byte {
	var c bytes.Buffer
	text := func(size, x, y float64, lines []string) {
		fmt.Fprintf(&c, "BT /F1 %.0f Tf %.1f TL %.2f %.2f Td\n", size, size*1.3, x, y)
		for _, s := range lines {
			fmt.Fprintf(&c, "(%s) Tj T*\n", winAnsi(s))
		}
		c.WriteString("ET\n")
	}
	text(11, left, addrTop, l.Address)
	var body []string
	for _, p := range strings.Split(l.Text, "\n") {
		body = append(body, wrap(p, wrapAt)...)
	}
	text(10, left, textTop, body)
	text(22, left, textTop-float64(len(body)+3)*13, []string{l.Code})
	objs := []string{"<</Type/Catalog/Pages 2 0 R>>", "<</Type/Pages/Kids[3 0 R]/Count 1>>",
		fmt.Sprintf("<</Type/Page/Parent 2 0 R/MediaBox[0 0 %.2f %.2f]/Resources<</Font<</F1 4 0 R>>>>/Contents 5 0 R>>", pageW, pageH),
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica/Encoding/WinAnsiEncoding>>",
		fmt.Sprintf("<</Length %d>>\nstream\n%sendstream", c.Len(), c.String())}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes()
}
