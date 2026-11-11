// Commande decompte-lignes : budget de code de Sésame (ARCHITECTURE.md §2.2,
// RELEASE.md §8, contrôle « Budget »).
//
// Le décompte porte sur les fichiers .go des paquets que lie cmd/sesame
// (`go list -deps`, plateforme de l'image), du seul module de ce dépôt : la
// bibliothèque standard et les modules épinglés n'y entrent pas, ni les
// fichiers _test.go (absents de GoFiles), ni les paquets du lanceur factice et
// des doubles (internal/factice/). Une ligne compte si elle porte au moins un
// élément de code : les lignes vides et celles qui ne portent qu'un
// commentaire ne comptent pas. La commande échoue au-delà du plafond.
package main

import (
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/golist"
)

// Limit est le plafond dur du budget de code (ARCHITECTURE.md §2.2).
const Limit = 6000

func main() {
	limit := flag.Int("plafond", Limit, "nombre de lignes au-delà duquel le contrôle échoue")
	detail := flag.Bool("detail", false, "imprime le décompte de chaque paquet")
	root := flag.String("racine", "", "racine du module (par défaut : celle du dossier courant)")
	flag.Parse()
	if err := run(*root, *limit, *detail, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "decompte-lignes :", err)
		os.Exit(1)
	}
}

func run(root string, limit int, detail bool, out io.Writer) error {
	root, err := golist.RootOr(root)
	if err != nil {
		return err
	}
	mainModule, err := golist.MainModule(root)
	if err != nil {
		return err
	}
	pkgs, err := golist.Deps(root, "./cmd/sesame")
	if err != nil {
		return err
	}
	perPackage, total, err := Count(pkgs, mainModule)
	if err != nil {
		return err
	}
	if detail {
		names := make([]string, 0, len(perPackage))
		for name := range perPackage {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(out, "%6d  %s\n", perPackage[name], name)
		}
	}
	fmt.Fprintf(out, "%d lignes de Go liées par cmd/sesame (plafond %d)\n", total, limit)
	if total > limit {
		return fmt.Errorf("budget dépassé : %d lignes pour un plafond de %d", total, limit)
	}
	return nil
}

// Count compte les lignes des paquets retenus et rend le détail par paquet et
// le total.
func Count(pkgs []golist.Package, mainModule string) (map[string]int, int, error) {
	perPackage := map[string]int{}
	total := 0
	for _, p := range Selected(pkgs, mainModule) {
		n := 0
		for _, f := range append(append([]string{}, p.GoFiles...), p.CgoFiles...) {
			path := filepath.Join(p.Dir, f)
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, 0, err
			}
			lines, err := CountLines(path, content)
			if err != nil {
				return nil, 0, err
			}
			n += lines
		}
		perPackage[p.ImportPath] = n
		total += n
	}
	return perPackage, total, nil
}

// Selected garde les paquets du module principal, hors internal/factice/.
func Selected(pkgs []golist.Package, mainModule string) []golist.Package {
	var kept []golist.Package
	for _, p := range pkgs {
		if p.Standard || p.Module == nil || p.Module.Path != mainModule {
			continue
		}
		if golist.Within(p.ImportPath, mainModule+"/internal/factice") {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

// CountLines compte les lignes d'un fichier Go qui portent au moins un
// élément de code. Une chaîne brute sur plusieurs lignes compte pour chacune.
func CountLines(name string, content []byte) (int, error) {
	fset := token.NewFileSet()
	file := fset.AddFile(name, fset.Base(), len(content))
	var errs scanner.ErrorList
	var s scanner.Scanner
	s.Init(file, content, func(pos token.Position, msg string) { errs.Add(pos, msg) }, 0)
	lines := map[int]bool{}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON && lit == "\n" {
			continue // point-virgule inséré en fin de ligne
		}
		first := fset.Position(pos).Line
		last := first + strings.Count(lit, "\n")
		for l := first; l <= last; l++ {
			lines[l] = true
		}
	}
	if len(errs) > 0 {
		return 0, errs.Err()
	}
	return len(lines), nil
}
