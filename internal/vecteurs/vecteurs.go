// Paquet vecteurs : lecture, pour les tests, des vecteurs recopiés depuis le
// client de référence sous testdata/vecteurs/ (CONTRIBUTING.md, « Formats et
// vecteurs » ; décision 0064). Aucun binaire ne le lie : seuls des fichiers
// _test.go l'importent.
//
// Chaque groupe (une opération, la partie du nom d'un vecteur avant « / »)
// est rejoué par le test du paquet qui porte son format. Ce test le réclame
// par Track, avec le nom du fichier et ceux des groupes écrits en clair ;
// chaque vecteur et chaque refus des groupes réclamés passe alors par
// Tracker.Vectors ou Tracker.Refusals, et le test échoue sur un nom qu'il n'a
// pas lu. Le test de ce paquet échoue sur un groupe des fichiers qu'aucun
// test ne réclame et qui n'est pas dans la liste des groupes en attente de
// leur test Go (couverture_test.go).
package vecteurs

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Files sont les copies que porte SHA256SUMS, dans l'ordre de leur nom.
var Files = []string{"attestation.json", "json.json", "primitives.json", "sesame.json"}

// Vector est un calcul attendu : entrées et sorties, telles quelles en JSON.
type Vector struct {
	Name    string          `json:"nom"`
	Inputs  json.RawMessage `json:"entrees"`
	Outputs json.RawMessage `json:"sorties"`
}

// Refusal est une entrée refusée et le code d'ErreurCrypto attendu.
type Refusal struct {
	Name   string          `json:"nom"`
	Inputs json.RawMessage `json:"entrees"`
	Error  string          `json:"erreur"`
}

// File est le contenu d'un fichier de vecteurs.
type File struct {
	Vectors  []Vector  `json:"vecteurs"`
	Refusals []Refusal `json:"refus"`
}

// Operation rend la partie du nom avant « / » : le groupe du vecteur.
func Operation(name string) string {
	op, _, _ := strings.Cut(name, "/")
	return op
}

// Operations rend, triés, les groupes d'un fichier.
func (f *File) Operations() []string {
	seen := map[string]bool{}
	for _, v := range f.Vectors {
		seen[Operation(v.Name)] = true
	}
	for _, r := range f.Refusals {
		seen[Operation(r.Name)] = true
	}
	ops := make([]string, 0, len(seen))
	for op := range seen {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	return ops
}

// ModuleRoot rend le dossier du go.mod qui contient le dossier courant.
func ModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("aucun go.mod au-dessus du dossier courant")
		}
		dir = parent
	}
}

// Dir rend testdata/vecteurs/ de la racine du module.
func Dir() (string, error) {
	root, err := ModuleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "testdata", "vecteurs"), nil
}

// Decode lit un fichier de vecteurs, sans champ inconnu ni donnée après
// l'objet. Les entrées et les sorties restent en JSON brut : un décodage en
// valeurs Go perdrait ce qu'un vecteur éprouve (demi-paire isolée, que
// encoding/json remplace par U+FFFD ; nombre exact).
func Decode(content []byte) (*File, error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err == nil {
		return nil, errors.New("données après l'objet")
	}
	return &f, nil
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*File{}
)

// Read rend un fichier de testdata/vecteurs/, lu une fois par processus.
func Read(name string) (*File, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if f, ok := cache[name]; ok {
		return f, nil
	}
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	f, err := Decode(content)
	if err != nil {
		return nil, fmt.Errorf("%s : %v", name, err)
	}
	cache[name] = f
	return f, nil
}

// Tracker tient les noms lus des groupes qu'un test réclame.
type Tracker struct {
	file       string
	content    *File
	operations map[string]bool
	mu         sync.Mutex
	read       map[string]bool
}

// Track réclame des groupes d'un fichier pour le test t : à la fin de t,
// chaque vecteur et chaque refus de ces groupes doit avoir été lu par
// Vectors ou Refusals, sinon t échoue. Le nom du fichier et ceux des groupes
// s'écrivent en chaînes littérales : le test de ce paquet les relit dans les
// sources.
func Track(t testing.TB, file string, operations ...string) *Tracker {
	t.Helper()
	content, err := Read(file)
	if err != nil {
		t.Fatalf("vecteurs : %v", err)
		return nil
	}
	return track(t, file, content, operations)
}

func track(t testing.TB, file string, content *File, operations []string) *Tracker {
	t.Helper()
	tr := &Tracker{file: file, content: content, operations: map[string]bool{}, read: map[string]bool{}}
	present := map[string]bool{}
	for _, op := range content.Operations() {
		present[op] = true
	}
	for _, op := range operations {
		if !present[op] {
			t.Fatalf("vecteurs : %s n'a aucun groupe %s", file, op)
			return nil
		}
		tr.operations[op] = true
	}
	t.Cleanup(func() {
		if t.Skipped() {
			t.Errorf("vecteurs : un test qui réclame des groupes de %s ne se saute pas", file)
			return
		}
		if unread := tr.Unread(); len(unread) > 0 {
			t.Errorf("vecteurs : %s, noms qu'aucun test n'a lus : %s", file, strings.Join(unread, ", "))
		}
	})
	return tr
}

// Unread rend, triés, les noms des groupes réclamés qui n'ont pas été lus.
func (tr *Tracker) Unread() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	var unread []string
	for _, v := range tr.content.Vectors {
		if tr.operations[Operation(v.Name)] && !tr.read["vecteurs/"+v.Name] {
			unread = append(unread, v.Name)
		}
	}
	for _, r := range tr.content.Refusals {
		if tr.operations[Operation(r.Name)] && !tr.read["refus/"+r.Name] {
			unread = append(unread, "refus "+r.Name)
		}
	}
	sort.Strings(unread)
	return unread
}

func (tr *Tracker) checkOperation(t testing.TB, op string) {
	t.Helper()
	if !tr.operations[op] {
		t.Fatalf("vecteurs : le groupe %s de %s n'est pas réclamé par Track", op, tr.file)
	}
}

func (tr *Tracker) mark(key string) {
	tr.mu.Lock()
	tr.read[key] = true
	tr.mu.Unlock()
}

// Vectors rejoue chaque vecteur du groupe op dans un sous-test à son nom. Un
// vecteur est lu quand son sous-test s'est déroulé sans être sauté, qu'il
// réussisse ou non.
func (tr *Tracker) Vectors(t *testing.T, op string, replay func(t *testing.T, v Vector)) {
	t.Helper()
	tr.checkOperation(t, op)
	for _, v := range tr.content.Vectors {
		if Operation(v.Name) != op {
			continue
		}
		skipped := true
		t.Run(v.Name, func(t *testing.T) {
			defer func() { skipped = t.Skipped() }()
			replay(t, v)
		})
		if !skipped {
			tr.mark("vecteurs/" + v.Name)
		}
	}
}

// Refusals rejoue chaque refus du groupe op dans un sous-test à son nom.
func (tr *Tracker) Refusals(t *testing.T, op string, replay func(t *testing.T, r Refusal)) {
	t.Helper()
	tr.checkOperation(t, op)
	for _, r := range tr.content.Refusals {
		if Operation(r.Name) != op {
			continue
		}
		skipped := true
		t.Run(r.Name, func(t *testing.T) {
			defer func() { skipped = t.Skipped() }()
			replay(t, r)
		})
		if !skipped {
			tr.mark("refus/" + r.Name)
		}
	}
}

// Hex est un champ d'octets écrit en hexadécimal minuscule.
type Hex []byte

// UnmarshalJSON lit l'hexadécimal minuscule d'une chaîne JSON.
func (h *Hex) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if strings.ToLower(s) != s {
		return errors.New("hexadécimal en majuscules")
	}
	v, err := hex.DecodeString(s)
	*h = v
	return err
}

// Load lit des entrées ou des sorties JSON dans dst, sans champ inconnu : un
// champ que le test ne lit pas le fait échouer.
func Load(t testing.TB, raw json.RawMessage, dst any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("vecteur illisible : %v", err)
	}
}

// MarshalJSON écrit l'hexadécimal minuscule (vide pour nil).
func (h Hex) MarshalJSON() ([]byte, error) { return json.Marshal(hex.EncodeToString(h)) }

// Same compare deux valeurs par leur JSON : sorties obtenues et attendues.
func Same(t testing.TB, got, want any) {
	t.Helper()
	g, err1 := json.Marshal(got)
	w, err2 := json.Marshal(want)
	if err1 != nil || err2 != nil || !bytes.Equal(g, w) {
		t.Errorf("obtenu  %s\nattendu %s", g, w)
	}
}
