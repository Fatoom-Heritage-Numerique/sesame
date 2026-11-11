package main

import (
	"io"
	"strings"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/fixture"
	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/golist"
)

func TestCountLines(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   int
	}{
		{"vide", "", 0},
		{"paquet seul", "package p\n", 1},
		{"lignes vides et commentaires", "// Paquet p.\npackage p\n\n// commentaire\n\t// indenté\n/* bloc\n   sur deux lignes */\n", 1},
		{"commentaire en fin de ligne", "package p\n\nvar x = 1 // un\n", 2},
		{"bloc avant du code", "package p\n/* a */ var x = 1\n", 2},
		{"chaîne qui contient //", "package p\n\nvar s = \"// pas un commentaire\"\n", 2},
		{"chaîne brute sur trois lignes", "package p\n\nvar s = `a\n\nb`\n", 4},
		{"fonction", "package p\n\nfunc f() {\n\t// rien\n\n\treturn\n}\n", 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := CountLines("x.go", []byte(c.source))
			if err != nil {
				t.Fatal(err)
			}
			if n != c.want {
				t.Errorf("CountLines = %d, attendu %d", n, c.want)
			}
		})
	}
}

func TestCountLinesRejectsBadSource(t *testing.T) {
	if _, err := CountLines("x.go", []byte("package p\nvar s = \"non fermée\n")); err == nil {
		t.Fatal("une chaîne non fermée doit faire échouer le décompte")
	}
}

func TestSelected(t *testing.T) {
	const m = "exemple.test/m"
	pkgs := []golist.Package{
		{ImportPath: "fmt", Standard: true},
		{ImportPath: m + "/cmd/sesame", Module: &golist.Module{Path: m}},
		{ImportPath: m + "/internal/canal", Module: &golist.Module{Path: m}},
		{ImportPath: m + "/internal/factice", Module: &golist.Module{Path: m}},
		{ImportPath: m + "/internal/factice/kms", Module: &golist.Module{Path: m}},
		{ImportPath: m + "/internal/facticeux", Module: &golist.Module{Path: m}},
		{ImportPath: "go.etcd.io/raft/v3", Module: &golist.Module{Path: "go.etcd.io/raft/v3"}},
	}
	var names []string
	for _, p := range Selected(pkgs, m) {
		names = append(names, p.ImportPath)
	}
	want := m + "/cmd/sesame," + m + "/internal/canal," + m + "/internal/facticeux"
	if got := strings.Join(names, ","); got != want {
		t.Errorf("Selected = %s, attendu %s", got, want)
	}
}

// Module de test : cmd/sesame lie internal/a et internal/factice ; les tests,
// le factice et un paquet non lié ne comptent pas.
func testModule(t *testing.T) string {
	return fixture.Module(t, "exemple.test/m", map[string]string{
		"cmd/sesame/main.go":          "// Commande.\npackage main\n\nimport (\n\t\"exemple.test/m/internal/a\"\n\t\"exemple.test/m/internal/factice\"\n)\n\nfunc main() {\n\ta.F()\n\tfactice.F()\n}\n",
		"cmd/sesame/main_test.go":     "package main\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
		"internal/a/a.go":             "package a\n\n// F ne fait rien.\nfunc F() {}\n",
		"internal/factice/factice.go": "package factice\n\nfunc F() {}\n\nvar x = 1\nvar y = 2\n",
		"internal/b/b.go":             "package b\n\nfunc G() {}\n",
	})
}

func TestRunOnModule(t *testing.T) {
	root := testModule(t)
	// cmd/sesame : 9 lignes ; internal/a : 2 lignes.
	if err := run(root, 11, false, io.Discard); err != nil {
		t.Fatalf("11 lignes pour un plafond de 11 : %v", err)
	}
}

// Contrôle positif : le budget dépassé fait échouer la commande.
func TestRunFailsAboveLimit(t *testing.T) {
	root := testModule(t)
	err := run(root, 10, false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "budget dépassé : 11 lignes") {
		t.Fatalf("11 lignes pour un plafond de 10 : erreur attendue, obtenu %v", err)
	}
}

func TestLimit(t *testing.T) {
	if Limit != 6000 {
		t.Fatalf("le plafond d'ARCHITECTURE.md §2.2 est de 6 000 lignes, pas %d", Limit)
	}
}
