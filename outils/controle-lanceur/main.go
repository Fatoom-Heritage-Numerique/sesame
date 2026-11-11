// Commande controle-lanceur : le lanceur factice reste hors de l'image
// (ARCHITECTURE.md §2.4, RELEASE.md §8, contrôle « Lanceur factice » ;
// décision 0063).
//
// Le contrôle échoue si :
//   - le nom de la variable du lanceur factice figure dans un fichier .go hors
//     de internal/factice/ (tests et testdata compris), ou dans le Dockerfile ;
//   - cmd/sesame lie un paquet de internal/factice/ (`go list -deps`).
//
// Seul ce programme écrit aussi le nom, pour le chercher : c'est un paquet
// main, qu'aucun autre paquet ne peut importer, et il n'entre dans aucune
// image.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/golist"
)

// LauncherVar est le nom de la variable du lanceur factice.
const LauncherVar = "SESAME_FAKE_LAUNCHER"

// allowedDirs sont les seuls dossiers dont un fichier .go peut écrire le nom.
var allowedDirs = []string{"internal/factice", "outils/controle-lanceur"}

func main() {
	root := flag.String("racine", "", "racine du module (par défaut : celle du dossier courant)")
	flag.Parse()
	faults, err := Check(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "controle-lanceur :", err)
		os.Exit(1)
	}
	for _, f := range faults {
		fmt.Fprintln(os.Stderr, "controle-lanceur :", f)
	}
	if len(faults) > 0 {
		os.Exit(1)
	}
	fmt.Println("lanceur factice hors de cmd/sesame : aucune faute")
}

// Check rend les fautes trouvées dans le module de la racine donnée.
func Check(root string) ([]string, error) {
	root, err := golist.RootOr(root)
	if err != nil {
		return nil, err
	}
	faults, err := FindVariable(root)
	if err != nil {
		return nil, err
	}
	mainModule, err := golist.MainModule(root)
	if err != nil {
		return nil, err
	}
	pkgs, err := golist.Deps(root, "./cmd/sesame")
	if err != nil {
		return nil, err
	}
	return append(faults, FakePackages(pkgs, mainModule)...), nil
}

// FindVariable rend chaque fichier qui écrit le nom de la variable hors des
// dossiers admis : tout fichier .go du dépôt et le Dockerfile.
func FindVariable(root string) ([]string, error) {
	var faults []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		isGo := strings.HasSuffix(rel, ".go")
		if !isGo && rel != "Dockerfile" {
			return nil
		}
		if isGo && allowed(rel) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(content, []byte(LauncherVar)) {
			faults = append(faults, fmt.Sprintf("%s écrit %s hors de internal/factice/", rel, LauncherVar))
		}
		return nil
	})
	sort.Strings(faults)
	return faults, err
}

func allowed(rel string) bool {
	for _, d := range allowedDirs {
		if strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// FakePackages rend chaque paquet de internal/factice/ que lie cmd/sesame.
func FakePackages(pkgs []golist.Package, mainModule string) []string {
	var faults []string
	for _, p := range pkgs {
		if p.Module != nil && p.Module.Path == mainModule && golist.Within(p.ImportPath, mainModule+"/internal/factice") {
			faults = append(faults, fmt.Sprintf("cmd/sesame lie %s", p.ImportPath))
		}
	}
	return faults
}
