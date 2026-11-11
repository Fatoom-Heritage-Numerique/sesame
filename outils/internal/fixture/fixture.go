// Paquet fixture : modules Go de test écrits dans un dossier temporaire, pour
// les contrôles positifs des outils (chaque contrôle doit savoir trouver une
// faute). Aucun binaire ne le lie.
package fixture

import (
	"os"
	"path/filepath"
	"testing"
)

// Module écrit un module nommé path, dont les fichiers sont donnés par leur
// chemin relatif, et rend sa racine. go.mod est écrit s'il n'est pas donné.
func Module(t testing.TB, path string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if _, ok := files["go.mod"]; !ok {
		files["go.mod"] = "module " + path + "\n\ngo 1.27.4\n"
	}
	for name, content := range files {
		f := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
