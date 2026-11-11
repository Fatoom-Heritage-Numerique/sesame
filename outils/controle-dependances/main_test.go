package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/fixture"
	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/golist"
)

func TestRepositoryHasNoFault(t *testing.T) {
	faults, err := Check("", AllowedModules)
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) > 0 {
		t.Fatalf("fautes dans ce dépôt : %v", faults)
	}
}

// Chaque module admis est nommé dans ARCHITECTURE.md §2.4, qui fait foi.
func TestAllowedModulesNamedInArchitecture(t *testing.T) {
	root, err := golist.Root()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "ARCHITECTURE.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	start := strings.Index(text, "### 2.4 ")
	end := strings.Index(text, "\n## 3.")
	if start < 0 || end < start {
		t.Fatal("ARCHITECTURE.md §2.4 introuvable")
	}
	section := text[start:end]
	for _, m := range AllowedModules {
		if !strings.Contains(section, "`"+m+"`") && !strings.Contains(section, "`"+m+"/") {
			t.Errorf("%s n'est pas nommé dans ARCHITECTURE.md §2.4", m)
		}
	}
}

// Contrôle positif : modules hors liste, module remplacé et paquet hors
// module sont trouvés.
func TestClassify(t *testing.T) {
	const m = "exemple.test/m"
	pkgs := []golist.Package{
		{ImportPath: "fmt", Standard: true},
		{ImportPath: m + "/cmd/sesame", Module: &golist.Module{Path: m}},
		{ImportPath: "github.com/cloudflare/circl/oprf", Module: &golist.Module{Path: "github.com/cloudflare/circl"}},
		{ImportPath: "github.com/cloudflare/circl/group", Module: &golist.Module{Path: "github.com/cloudflare/circl"}},
		{ImportPath: "golang.org/x/crypto/argon2", Module: &golist.Module{Path: "golang.org/x/crypto"}},
		{ImportPath: "github.com/autre/lib", Module: &golist.Module{Path: "github.com/autre"}},
		{ImportPath: "go.etcd.io/raft/v3", Module: &golist.Module{Path: "go.etcd.io/raft/v3", Replace: &golist.Module{Path: "../raft"}}},
		{ImportPath: "golang.org/x/net/http2", Module: &golist.Module{Path: "golang.org/x/net"}},
		{ImportPath: "vendored/sans/module"},
	}
	want := []string{
		"github.com/autre/lib : module github.com/autre hors de la liste d'ARCHITECTURE.md §2.4",
		"golang.org/x/net/http2 : module golang.org/x/net hors de la liste d'ARCHITECTURE.md §2.4",
		"module go.etcd.io/raft/v3 remplacé par ../raft",
		"vendored/sans/module : paquet hors module",
	}
	if got := Classify(pkgs, m, AllowedModules); !reflect.DeepEqual(got, want) {
		t.Errorf("Classify =\n%s\nattendu\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Contrôle positif sur un vrai `go list` : un module transitif hors liste,
// lié par un module admis, est trouvé.
func TestTransitiveModuleOutsideList(t *testing.T) {
	root := fixture.Module(t, "exemple.test/m", map[string]string{
		"go.mod":             "module exemple.test/m\n\ngo 1.27.4\n\nrequire exemple.test/admis v0.0.0\n\nrequire exemple.test/transitif v0.0.0 // indirect\n\nreplace exemple.test/admis => ./admis\n\nreplace exemple.test/transitif => ./transitif\n",
		"cmd/sesame/main.go": "package main\n\nimport \"exemple.test/admis\"\n\nfunc main() { admis.F() }\n",
		"admis/go.mod":       "module exemple.test/admis\n\ngo 1.27.4\n\nrequire exemple.test/transitif v0.0.0\n",
		"admis/admis.go":     "package admis\n\nimport \"exemple.test/transitif\"\n\nfunc F() { transitif.G() }\n",
		"transitif/go.mod":   "module exemple.test/transitif\n\ngo 1.27.4\n",
		"transitif/t.go":     "package transitif\n\nfunc G() {}\n",
	})
	faults, err := Check(root, []string{"exemple.test/admis"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"exemple.test/transitif : module exemple.test/transitif hors de la liste d'ARCHITECTURE.md §2.4",
		"module exemple.test/admis remplacé par ./admis",
		"module exemple.test/transitif remplacé par ./transitif",
	}
	if !reflect.DeepEqual(faults, want) {
		t.Errorf("fautes =\n%s\nattendu\n%s", strings.Join(faults, "\n"), strings.Join(want, "\n"))
	}
}
