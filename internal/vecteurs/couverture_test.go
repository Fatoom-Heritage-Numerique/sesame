package vecteurs

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// pending sont les groupes dont le test Go n'est pas encore écrit
// (docs/technique/tests.md du client de référence, « Vecteurs repris par le
// dépôt sesame » : ce que le code Go recalcule pour chaque groupe). Un groupe
// sort de cette liste dans le commit qui écrit le test qui le réclame par
// Track ; le test ci-dessous échoue s'il est à la fois réclamé et en attente.
var pending = map[string][]string{
	"attestation.json": {"attestation", "listeVersions"},
	"primitives.json": {
		"aesGcmChiffrer", "aesGcmDechiffrer", "aleatoire", "argon2id",
		"ecdsaP256Verifier", "ed25519Publique", "ed25519Signer", "ed25519Verifier",
		"hkdfSha256", "hmacSha256", "mlKem768Decapsuler", "mlKem768Encapsuler",
		"rsaPkcs1Sha256Verifier", "sha256", "x25519", "x25519Paire", "x25519Publique",
	},
	"sesame.json": {
		"annulation", "antiRetour", "borne", "canal", "canalReponse", "canalRequete",
		"chaineRegistre", "charte", "charteGen", "codeAcces", "codeInvitation",
		"codeVerifier", "datePasserelle", "depotPin", "descellerFiche",
		"descellerInvitation", "descellerKts", "empreinteRegistre", "enveloppeFichier",
		"enveloppeMot", "enveloppeMotDePasse", "enveloppeVolontes", "ficheAlerte",
		"indexAcces", "instanceId", "jetonPaire", "jetonSession", "kts", "ligneRegistre",
		"lireInstanceId", "messageFichier", "noncesClient", "noncesPaire", "ouvrirCanal",
		"ouvrirCharte", "ouvrirEnveloppe", "ouvrirKts", "passerelle", "preuveDeVie",
		"preuveEnrolement", "recu", "scelle", "tirerCodeAcces", "verifierAnnulation",
		"verifierBorne", "verifierCharteGen", "verifierDepotPin", "verifierFicheAlerte",
		"verifierPreuveDeVie", "verifierPreuveEnrolement",
	},
}

const importPath = "github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"

// claim est un groupe réclamé par un appel à Track, et l'endroit de l'appel.
type claim struct {
	file, operation, at string
}

// findClaims relit les fichiers _test.go sous root et rend chaque groupe
// réclamé par un appel <paquet vecteurs>.Track(t, "<fichier>", "<groupe>"…).
// Un argument qui n'est pas une chaîne littérale est une erreur : la
// réclamation doit se lire sans exécuter le test.
func findClaims(root string) ([]claim, error) {
	var claims []claim
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (d.Name() == ".git" || d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		alias := ""
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == importPath {
				alias = "vecteurs"
				if imp.Name != nil {
					alias = imp.Name.Name
				}
			}
		}
		if alias == "" {
			return nil
		}
		var inner error
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || inner != nil {
				return inner == nil
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Track" {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); !ok || x.Name != alias {
				return true
			}
			at := fset.Position(call.Pos()).String()
			if len(call.Args) < 3 || call.Ellipsis.IsValid() {
				inner = fmt.Errorf("%s : Track réclame au moins un groupe, en arguments écrits un à un", at)
				return false
			}
			var lits []string
			for _, a := range call.Args[1:] {
				lit, ok := a.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					inner = fmt.Errorf("%s : Track prend le fichier et les groupes en chaînes littérales", at)
					return false
				}
				s, _ := strconv.Unquote(lit.Value)
				lits = append(lits, s)
			}
			for _, op := range lits[1:] {
				claims = append(claims, claim{file: lits[0], operation: op, at: at})
			}
			return true
		})
		return inner
	})
	return claims, err
}

// checkCoverage rend les fautes de couverture : groupe ni réclamé ni en
// attente, réclamé et en attente, réclamé deux fois, ou inconnu des fichiers.
func checkCoverage(files map[string][]string, pending map[string][]string, claims []claim) []string {
	var faults []string
	known := map[string]bool{}
	for file, ops := range files {
		for _, op := range ops {
			known[file+" "+op] = true
		}
	}
	waiting := map[string]bool{}
	for file, ops := range pending {
		for _, op := range ops {
			key := file + " " + op
			if waiting[key] {
				faults = append(faults, fmt.Sprintf("%s : groupe %s deux fois en attente", file, op))
			}
			waiting[key] = true
			if !known[key] {
				faults = append(faults, fmt.Sprintf("%s : groupe %s en attente mais absent du fichier", file, op))
			}
		}
	}
	claimed := map[string]string{}
	for _, c := range claims {
		key := c.file + " " + c.operation
		if prev, ok := claimed[key]; ok {
			faults = append(faults, fmt.Sprintf("%s : groupe %s réclamé deux fois (%s, %s)", c.file, c.operation, prev, c.at))
		}
		claimed[key] = c.at
		if !known[key] {
			faults = append(faults, fmt.Sprintf("%s : %s réclame le groupe %s, absent du fichier", c.at, c.file, c.operation))
		}
		if waiting[key] {
			faults = append(faults, fmt.Sprintf("%s : %s réclame le groupe %s, encore en attente ; retirez-le de pending", c.at, c.file, c.operation))
		}
	}
	for key := range known {
		if !waiting[key] && claimed[key] == "" {
			file, op, _ := strings.Cut(key, " ")
			faults = append(faults, fmt.Sprintf("%s : groupe %s qu'aucun test ne réclame par Track et qui n'est pas en attente", file, op))
		}
	}
	sort.Strings(faults)
	return faults
}

func TestCoverageOfGroups(t *testing.T) {
	root, err := ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]string{}
	total := 0
	for _, name := range Files {
		f, err := Read(name)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = f.Operations()
		total += len(files[name])
	}
	claims, err := findClaims(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range checkCoverage(files, pending, claims) {
		t.Error(fault)
	}
	waiting := 0
	for _, ops := range pending {
		waiting += len(ops)
	}
	t.Logf("groupes de vecteurs : %d réclamés par un test Go, %d en attente de leur test, sur %d", len(claims), waiting, total)
}

// Contrôles positifs de la mécanique de couverture.

func TestCheckCoverageFindsFaults(t *testing.T) {
	files := map[string][]string{"a.json": {"x", "y", "z"}}
	cases := []struct {
		name    string
		pending map[string][]string
		claims  []claim
		want    string
	}{
		{"groupe ni réclamé ni en attente", map[string][]string{"a.json": {"x", "y"}}, nil, "groupe z qu'aucun test ne réclame"},
		{"groupe en attente absent", map[string][]string{"a.json": {"x", "y", "z", "w"}}, nil, "groupe w en attente mais absent"},
		{"réclamé et en attente", map[string][]string{"a.json": {"x", "y", "z"}}, []claim{{"a.json", "x", "p_test.go:1"}}, "encore en attente"},
		{"réclamé deux fois", map[string][]string{"a.json": {"y", "z"}}, []claim{{"a.json", "x", "p_test.go:1"}, {"a.json", "x", "q_test.go:2"}}, "réclamé deux fois"},
		{"réclamation inconnue", map[string][]string{"a.json": {"x", "y", "z"}}, []claim{{"a.json", "v", "p_test.go:1"}}, "absent du fichier"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			faults := checkCoverage(files, c.pending, c.claims)
			if len(faults) == 0 || !strings.Contains(strings.Join(faults, "\n"), c.want) {
				t.Errorf("fautes = %v, attendu « %s »", faults, c.want)
			}
		})
	}
	if faults := checkCoverage(files, map[string][]string{"a.json": {"y"}}, []claim{{"a.json", "x", "p"}, {"a.json", "z", "q"}}); len(faults) > 0 {
		t.Errorf("couverture complète : fautes inattendues %v", faults)
	}
}

func TestFindClaims(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("canal/canal_test.go", "package canal\n\nimport (\n\t\"testing\"\n\n\t\""+importPath+"\"\n)\n\nfunc TestCanal(t *testing.T) {\n\ttr := vecteurs.Track(t, \"sesame.json\", \"canal\", \"ouvrirCanal\")\n\t_ = tr\n}\n")
	write("prim/prim_test.go", "package prim\n\nimport (\n\t\"testing\"\n\n\tv \""+importPath+"\"\n)\n\nfunc TestP(t *testing.T) { v.Track(t, \"primitives.json\", \"sha256\") }\n")
	write("autre/autre_test.go", "package autre\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { vecteurs.Track(t, \"sesame.json\", \"kts\") }\n")
	write("autre/testdata/x_test.go", "ce fichier n'est pas du Go")
	claims, err := findClaims(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range claims {
		got = append(got, c.file+" "+c.operation)
	}
	sort.Strings(got)
	want := "primitives.json sha256,sesame.json canal,sesame.json ouvrirCanal"
	if strings.Join(got, ",") != want {
		t.Errorf("réclamations = %v, attendu %s", got, want)
	}

	write("variable/v_test.go", "package variable\n\nimport (\n\t\"testing\"\n\n\t\""+importPath+"\"\n)\n\nvar groupe = \"kts\"\n\nfunc TestV(t *testing.T) { vecteurs.Track(t, \"sesame.json\", groupe) }\n")
	if _, err := findClaims(root); err == nil || !strings.Contains(err.Error(), "chaînes littérales") {
		t.Errorf("un groupe en variable doit être refusé, obtenu %v", err)
	}
}
