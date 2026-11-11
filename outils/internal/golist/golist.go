// Paquet golist : lecture de `go list -deps -json`, commune aux contrôles de
// outils/ (RELEASE.md §8). Les paquets sont ceux que lie le binaire pour la
// plateforme de l'image (linux/amd64, sans cgo), comme le construit le
// Dockerfile.
package golist

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Module est la part de `go list -json` qui décrit le module d'un paquet.
type Module struct {
	Path    string
	Version string
	Main    bool
	Dir     string
	Replace *Module
}

// Package est la part de `go list -json` dont les contrôles ont besoin.
type Package struct {
	ImportPath string
	Dir        string
	Standard   bool
	Module     *Module
	GoFiles    []string
	CgoFiles   []string
}

// ImageEnv fixe la plateforme de l'image et écarte un espace de travail
// (go.work) qui changerait les modules liés.
var ImageEnv = []string{"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=-mod=readonly"}

// Deps rend les paquets que lient les motifs donnés (`go list -deps`), lancé
// dans le dossier root.
func Deps(root string, patterns ...string) ([]Package, error) {
	args := append([]string{"list", "-deps", "-json"}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), ImageEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list -deps %s : %v\n%s", strings.Join(patterns, " "), err, stderr.String())
	}
	return Decode(&stdout)
}

// Decode lit la suite d'objets JSON qu'écrit `go list -json`.
func Decode(r io.Reader) ([]Package, error) {
	dec := json.NewDecoder(r)
	var pkgs []Package
	for {
		var p Package
		err := dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			return pkgs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("sortie de go list illisible : %v", err)
		}
		pkgs = append(pkgs, p)
	}
}

// MainModule lit le chemin du module dans le go.mod du dossier root.
func MainModule(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`), nil
		}
	}
	return "", errors.New("go.mod sans ligne module")
}

// Root rend le dossier du go.mod qui contient le dossier courant.
func Root() (string, error) {
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

// RootOr rend root s'il est donné, sinon Root.
func RootOr(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return Root()
}

// Within dit si path est base ou un paquet sous base.
func Within(path, base string) bool {
	return path == base || strings.HasPrefix(path, base+"/")
}
