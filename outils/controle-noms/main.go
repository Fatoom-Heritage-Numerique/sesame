// Commande controle-noms : part structurelle du contrôle « Noms de
// ressources » (RELEASE.md §8 ; décisions 0065 et 0066).
//
// Dans chaque fichier suivi par Git, tout nom de ressource du déploiement
// s'écrit sous la forme <…> de CONTRIBUTING.md (« Règles du code ») :
//   - après gs:// (bucket) et après projects/ (projet), le nom est un <…> ;
//   - après -docker.pkg.dev/, tout le chemin de l'image est fait de <…>
//     (seul « @sha256: » s'y écrit en clair) ;
//   - avant .iam.gserviceaccount.com (compte et projet) et avant run.app
//     (hôte), le nom est fait de <…> ;
//   - aucun nombre de 12 chiffres (numéro de projet) n'est écrit.
//
// Une mention du motif seul (`gs://`, `run.app`, `projects/…`) ne porte aucun
// nom et passe. Ce contrôle ne connaît aucun nom réel : leur recherche se fait
// dans la CI du client (décision 0065). Les copies des vecteurs que nomme
// testdata/vecteurs/SHA256SUMS en sont exclues : leur contenu est fixé par le
// client de référence, octet pour octet (décision 0066).
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/golist"
)

// rule décrit un motif et le côté où se trouve le nom qu'il désigne.
type rule struct {
	pattern string
	after   bool   // le nom suit le motif ; sinon il le précède
	chars   string // caractères qui prolongent le nom
	label   string
}

const (
	nameChars    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-<>"
	pathChars    = nameChars + "/:@"
	accountChars = nameChars + "@+"
)

var rules = []rule{
	{"gs://", true, nameChars, "bucket après gs://"},
	{"projects/", true, nameChars + ":", "projet après projects/"},
	{"-docker.pkg.dev/", true, pathChars, "chemin d'image après -docker.pkg.dev/"},
	{".iam.gserviceaccount.com", false, accountChars, "compte de service avant .iam.gserviceaccount.com"},
	{"run.app", false, nameChars, "hôte avant run.app"},
}

var (
	placeholder  = regexp.MustCompile(`<[^<>\s]+>`)
	alphanumeric = regexp.MustCompile(`[A-Za-z0-9]`)
	twelveDigits = regexp.MustCompile(`(?:^|[^0-9A-Za-z_])([0-9]{12})(?:[^0-9A-Za-z_]|$)`)
)

func main() {
	root := flag.String("racine", "", "racine du dépôt (par défaut : celle du module du dossier courant)")
	flag.Parse()
	faults, excluded, err := Check(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "controle-noms :", err)
		os.Exit(1)
	}
	for _, f := range faults {
		fmt.Fprintln(os.Stderr, "controle-noms :", f)
	}
	if len(faults) > 0 {
		os.Exit(1)
	}
	list := "aucune"
	if len(excluded) > 0 {
		list = strings.Join(excluded, ", ")
	}
	fmt.Printf("noms de ressources sous la forme <…> : aucune faute (copies de vecteurs exclues : %s)\n", list)
}

// Check rend les fautes des fichiers suivis et la liste des copies de
// vecteurs exclues.
func Check(root string) ([]string, []string, error) {
	root, err := golist.RootOr(root)
	if err != nil {
		return nil, nil, err
	}
	files, err := TrackedFiles(root)
	if err != nil {
		return nil, nil, err
	}
	excluded, err := VectorCopies(root)
	if err != nil {
		return nil, nil, err
	}
	isExcluded := map[string]bool{}
	for _, e := range excluded {
		isExcluded[e] = true
	}
	var faults []string
	for _, f := range files {
		if isExcluded[f] {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			return nil, nil, err
		}
		faults = append(faults, Scan(f, content)...)
	}
	return faults, excluded, nil
}

// TrackedFiles rend les fichiers suivis par Git, relatifs à la racine.
func TrackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files : %v", err)
	}
	var files []string
	for _, f := range bytes.Split(out, []byte{0}) {
		if len(f) > 0 {
			files = append(files, string(f))
		}
	}
	return files, nil
}

// VectorCopies rend les fichiers que nomme testdata/vecteurs/SHA256SUMS.
func VectorCopies(root string) ([]string, error) {
	f, err := os.Open(filepath.Join(root, "testdata", "vecteurs", "SHA256SUMS"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var copies []string
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) != 2 || strings.ContainsAny(fields[1], "/\\") {
			return nil, fmt.Errorf("SHA256SUMS : ligne illisible %q", lines.Text())
		}
		copies = append(copies, "testdata/vecteurs/"+strings.TrimPrefix(fields[1], "*"))
	}
	return copies, lines.Err()
}

// Scan rend les fautes d'un fichier, une par occurrence.
func Scan(name string, content []byte) []string {
	var faults []string
	for i, line := range strings.Split(string(content), "\n") {
		for _, r := range rules {
			for from := 0; ; {
				j := strings.Index(line[from:], r.pattern)
				if j < 0 {
					break
				}
				j += from
				var found string
				if r.after {
					found = extend(line[j+len(r.pattern):], r.chars, true)
				} else {
					found = extend(line[:j], r.chars, false)
				}
				rest := placeholder.ReplaceAllString(strings.ReplaceAll(found, "@sha256:", ""), "")
				if alphanumeric.MatchString(rest) {
					faults = append(faults, fmt.Sprintf("%s:%d : %s hors de la forme <…> : %q", name, i+1, r.label, found))
				}
				from = j + len(r.pattern)
			}
		}
		for _, m := range twelveDigits.FindAllStringSubmatch(line, -1) {
			faults = append(faults, fmt.Sprintf("%s:%d : nombre de 12 chiffres %s ; un numéro de projet s'écrit <numero-projet-sesame>", name, i+1, m[1]))
		}
	}
	return faults
}

// extend rend la plus longue suite de caractères permis au début (ou à la
// fin) de s.
func extend(s, chars string, atStart bool) string {
	if atStart {
		n := 0
		for n < len(s) && strings.IndexByte(chars, s[n]) >= 0 {
			n++
		}
		return s[:n]
	}
	n := len(s)
	for n > 0 && strings.IndexByte(chars, s[n-1]) >= 0 {
		n--
	}
	return s[n:]
}
