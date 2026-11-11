// Commande controle-dependances : chaque paquet que lie cmd/sesame est de la
// bibliothèque standard, de ce module ou d'un module de la liste
// d'ARCHITECTURE.md §2.4, modules transitifs compris (RELEASE.md §8, contrôle
// « Dépendances » ; décision 0065).
//
// Le contrôle échoue aussi sur un module remplacé (directive replace) : la
// somme de go.sum ne le couvre plus. La part « dépendance réseau » du
// contrôle (ARCHITECTURE.md §2.2) n'est pas ici : elle porte sur les adresses
// que le code appelle, qui n'existe pas encore.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/golist"
)

// AllowedModules est la liste d'ARCHITECTURE.md §2.4, par chemin de module :
// les deux exceptions cryptographiques, la normalisation NFC (décision 0068),
// Raft, et les modules qu'elles requièrent. Les SDK Google officiels y entrent par leur chemin avec le code
// qui les appelle, et leurs modules transitifs avec eux, après les avoir
// nommés dans ARCHITECTURE.md §2.4.
var AllowedModules = []string{
	"github.com/cloudflare/circl",
	"golang.org/x/crypto",
	"golang.org/x/sys",
	"github.com/bwesterb/go-ristretto",
	"golang.org/x/text",
	"go.etcd.io/raft/v3",
	"google.golang.org/protobuf",
}

func main() {
	root := flag.String("racine", "", "racine du module (par défaut : celle du dossier courant)")
	flag.Parse()
	faults, err := Check(*root, AllowedModules)
	if err != nil {
		fmt.Fprintln(os.Stderr, "controle-dependances :", err)
		os.Exit(1)
	}
	for _, f := range faults {
		fmt.Fprintln(os.Stderr, "controle-dependances :", f)
	}
	if len(faults) > 0 {
		os.Exit(1)
	}
	fmt.Println("dépendances de cmd/sesame : aucune hors de la liste d'ARCHITECTURE.md §2.4")
}

// Check rend les fautes des paquets que lie cmd/sesame.
func Check(root string, allowed []string) ([]string, error) {
	root, err := golist.RootOr(root)
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
	return Classify(pkgs, mainModule, allowed), nil
}

// Classify rend, triée, une faute par paquet hors de la liste et par module
// remplacé.
func Classify(pkgs []golist.Package, mainModule string, allowed []string) []string {
	list := map[string]bool{}
	for _, m := range allowed {
		list[m] = true
	}
	var faults []string
	replaced := map[string]bool{}
	for _, p := range pkgs {
		if p.Standard {
			continue
		}
		if p.Module == nil {
			faults = append(faults, fmt.Sprintf("%s : paquet hors module", p.ImportPath))
			continue
		}
		if p.Module.Replace != nil && !replaced[p.Module.Path] {
			replaced[p.Module.Path] = true
			faults = append(faults, fmt.Sprintf("module %s remplacé par %s", p.Module.Path, p.Module.Replace.Path))
		}
		if p.Module.Path == mainModule {
			continue
		}
		if !list[p.Module.Path] {
			faults = append(faults, fmt.Sprintf("%s : module %s hors de la liste d'ARCHITECTURE.md §2.4", p.ImportPath, p.Module.Path))
		}
	}
	sort.Strings(faults)
	return faults
}
