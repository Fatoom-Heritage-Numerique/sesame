package vecteurs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// Forme des fichiers : CONTRIBUTING.md, « Formats et vecteurs ».
var (
	nameForm   = regexp.MustCompile(`^[a-z][A-Za-z0-9]*/[a-z0-9]+(-[a-z0-9]+)*$`)
	sumLine    = regexp.MustCompile(`^([0-9a-f]{64})  ([A-Za-z0-9.]+)$`)
	errorCodes = map[string]bool{"parametres_invalides": true, "dechiffrement_refuse": true, "cle_invalide": true, "secret_nul": true}
)

// otherFiles sont les seuls fichiers de testdata/vecteurs/ hors des copies :
// SHA256SUMS et compteurPin.json, écrit par outils/generer-compteurpin.
var otherFiles = map[string]bool{"SHA256SUMS": true, "compteurPin.json": true}

func TestSHA256SUMS(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	faults := checkSums(sums, func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, name)) })
	for _, f := range faults {
		t.Error(f)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	copies := map[string]bool{}
	for _, f := range Files {
		copies[f] = true
	}
	for _, e := range entries {
		if !copies[e.Name()] && !otherFiles[e.Name()] {
			t.Errorf("testdata/vecteurs/%s n'est ni une copie de SHA256SUMS ni un fichier de ce dépôt", e.Name())
		}
	}
}

// checkSums rend les fautes de SHA256SUMS : une ligne par copie de Files,
// dans cet ordre, au format de sha256sum, et l'empreinte de chaque copie.
func checkSums(sums []byte, read func(string) ([]byte, error)) []string {
	var faults []string
	lines := strings.Split(strings.TrimSuffix(string(sums), "\n"), "\n")
	if !bytes.HasSuffix(sums, []byte("\n")) {
		faults = append(faults, "SHA256SUMS ne finit pas par une fin de ligne")
	}
	if len(lines) != len(Files) {
		return append(faults, fmt.Sprintf("SHA256SUMS a %d lignes, attendu une par copie : %s", len(lines), strings.Join(Files, ", ")))
	}
	for i, line := range lines {
		m := sumLine.FindStringSubmatch(line)
		if m == nil {
			faults = append(faults, fmt.Sprintf("SHA256SUMS ligne %d illisible : %q", i+1, line))
			continue
		}
		if m[2] != Files[i] {
			faults = append(faults, fmt.Sprintf("SHA256SUMS ligne %d : %s, attendu %s", i+1, m[2], Files[i]))
			continue
		}
		content, err := read(m[2])
		if err != nil {
			faults = append(faults, err.Error())
			continue
		}
		sum := sha256.Sum256(content)
		if got := hex.EncodeToString(sum[:]); got != m[1] {
			faults = append(faults, fmt.Sprintf("%s : empreinte %s, SHA256SUMS porte %s ; la copie se refait depuis le client, jamais à la main", m[2], got, m[1]))
		}
	}
	return faults
}

func TestFormOfFiles(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, name := range Files {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			for _, fault := range checkForm(content) {
				t.Error(fault)
			}
			f, err := Decode(content)
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range names(f) {
				if other, ok := seen[n]; ok {
					t.Errorf("nom %s aussi dans %s", n, other)
				}
				seen[n] = name
			}
			t.Logf("%s : %d vecteurs, %d refus, %d groupes", name, len(f.Vectors), len(f.Refusals), len(f.Operations()))
		})
	}
}

func names(f *File) []string {
	var n []string
	for _, v := range f.Vectors {
		n = append(n, v.Name)
	}
	for _, r := range f.Refusals {
		n = append(n, r.Name)
	}
	return n
}

// checkForm rend les fautes de forme d'un fichier de vecteurs.
func checkForm(content []byte) []string {
	var faults []string
	if !utf8.Valid(content) {
		return []string{"fichier hors UTF-8"}
	}
	if err := sortedKeys(content); err != nil {
		faults = append(faults, err.Error())
	}
	f, err := Decode(content)
	if err != nil {
		return append(faults, err.Error())
	}
	if len(f.Vectors) == 0 {
		faults = append(faults, "aucun vecteur")
	}
	unique := map[string]bool{}
	checkName := func(n string) {
		if !nameForm.MatchString(n) {
			faults = append(faults, fmt.Sprintf("nom %q hors de la forme <opération>/<cas>", n))
		}
		if unique[n] {
			faults = append(faults, fmt.Sprintf("nom %s en double", n))
		}
		unique[n] = true
	}
	for _, v := range f.Vectors {
		checkName(v.Name)
		if !isObject(v.Inputs) || !isObject(v.Outputs) {
			faults = append(faults, fmt.Sprintf("%s : entrees et sorties sont des objets", v.Name))
		}
	}
	for _, r := range f.Refusals {
		checkName(r.Name)
		if !isObject(r.Inputs) {
			faults = append(faults, fmt.Sprintf("refus %s : entrees est un objet", r.Name))
		}
		if !errorCodes[r.Error] {
			faults = append(faults, fmt.Sprintf("refus %s : erreur %q hors des codes d'ErreurCrypto", r.Name, r.Error))
		}
	}
	return faults
}

func isObject(raw json.RawMessage) bool {
	return len(raw) > 0 && raw[0] == '{'
}

// sortedKeys vérifie que les clés de chaque objet sont triées, dans l'ordre
// des unités UTF-16 du générateur du client (JSON canonique, RFC 8785).
func sortedKeys(content []byte) error {
	dec := json.NewDecoder(bytes.NewReader(content))
	var walk func(path string) error
	walk = func(path string) error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch d := tok.(type) {
		case json.Delim:
			switch d {
			case '{':
				prev := ""
				for i := 0; dec.More(); i++ {
					k, err := dec.Token()
					if err != nil {
						return err
					}
					key := k.(string)
					if i > 0 && !lessUTF16(prev, key) {
						return fmt.Errorf("%s : clé %q après %q, hors de l'ordre", path, key, prev)
					}
					prev = key
					if err := walk(path + "." + key); err != nil {
						return err
					}
				}
				_, err := dec.Token()
				return err
			case '[':
				for i := 0; dec.More(); i++ {
					if err := walk(fmt.Sprintf("%s[%d]", path, i)); err != nil {
						return err
					}
				}
				_, err := dec.Token()
				return err
			}
		}
		return nil
	}
	if err := walk("$"); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("données après l'objet")
	}
	return nil
}

func lessUTF16(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// Contrôles positifs de la forme et des empreintes.

func TestCheckFormFindsFaults(t *testing.T) {
	cases := map[string]string{
		`{"refus":[],"vecteurs":[{"entrees":{},"nom":"op/cas","sorties":{}}],"x":1}`:                                           "unknown field",
		`{"vecteurs":[{"entrees":{},"nom":"op/cas","sorties":{}}],"refus":[]}`:                                                 "hors de l'ordre",
		`{"refus":[],"vecteurs":[{"entrees":{},"nom":"op_cas","sorties":{}}]}`:                                                 "hors de la forme",
		`{"refus":[],"vecteurs":[{"entrees":{},"nom":"op/cas","sorties":{}},{"entrees":{},"nom":"op/cas","sorties":{}}]}`:      "en double",
		`{"refus":[],"vecteurs":[{"entrees":[],"nom":"op/cas","sorties":{}}]}`:                                                 "sont des objets",
		`{"refus":[{"entrees":{},"erreur":"autre","nom":"op/refus"}],"vecteurs":[{"entrees":{},"nom":"op/cas","sorties":{}}]}`: "hors des codes",
		`{"refus":[],"vecteurs":[]}`: "aucun vecteur",
		`{"refus":[],"vecteurs":[{"entrees":{"b":1,"a":2},"nom":"op/cas","sorties":{}}]}`: "hors de l'ordre",
		`{"refus":[],"vecteurs":[{"entrees":{},"nom":"op/cas","sorties":{}}]}{}`:          "après l'objet",
	}
	for content, want := range cases {
		faults := checkForm([]byte(content))
		if !strings.Contains(strings.Join(faults, "\n"), want) {
			t.Errorf("%s : fautes %v, attendu « %s »", content, faults, want)
		}
	}
	good := `{"refus":[{"entrees":{},"erreur":"secret_nul","nom":"op/refus"}],"vecteurs":[{"entrees":{"a":1,"b":[{"c":2,"d":3}]},"nom":"op/cas","sorties":{}}]}`
	if faults := checkForm([]byte(good)); len(faults) > 0 {
		t.Errorf("fautes inattendues : %v", faults)
	}
}

func TestCheckSumsFindsFaults(t *testing.T) {
	contents := map[string][]byte{}
	var good strings.Builder
	for _, f := range Files {
		contents[f] = []byte(f + "\n")
		sum := sha256.Sum256(contents[f])
		fmt.Fprintf(&good, "%s  %s\n", hex.EncodeToString(sum[:]), f)
	}
	read := func(n string) ([]byte, error) { return contents[n], nil }
	if faults := checkSums([]byte(good.String()), read); len(faults) > 0 {
		t.Fatalf("fautes inattendues : %v", faults)
	}
	contents["json.json"] = []byte("json.json modifié à la main\n")
	faults := checkSums([]byte(good.String()), read)
	if len(faults) != 1 || !strings.HasPrefix(faults[0], "json.json : empreinte") {
		t.Errorf("copie modifiée : fautes %v", faults)
	}
	lines := strings.SplitAfter(good.String(), "\n")
	if faults := checkSums([]byte(strings.Join(lines[:3], "")), read); len(faults) == 0 {
		t.Error("une copie absente de SHA256SUMS doit être une faute")
	}
	if faults := checkSums([]byte(lines[1]+lines[0]+lines[2]+lines[3]), read); len(faults) == 0 {
		t.Error("des lignes dans le désordre doivent être une faute")
	}
	if faults := checkSums([]byte(strings.Replace(good.String(), "  ", " *", 1)), read); len(faults) == 0 {
		t.Error("une ligne hors du format de sha256sum doit être une faute")
	}
}

func TestDecodeKeepsRawInputs(t *testing.T) {
	f, err := Decode([]byte(`{"refus":[{"entrees":{"s":"\ud800"},"erreur":"parametres_invalides","nom":"jsonCanonique/demi-paire"}],"vecteurs":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(f.Refusals[0].Inputs); got != `{"s":"\ud800"}` {
		t.Errorf("entrees = %s : la demi-paire isolée doit rester telle quelle", got)
	}
}
