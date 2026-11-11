package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// j recolle un texte : les motifs fautifs de ces tests ne s'écrivent jamais
// d'un seul tenant, pour que le contrôle lancé sur ce dépôt ne les trouve pas
// dans ce fichier.
func j(parts ...string) string { return strings.Join(parts, "") }

func TestRepositoryHasNoFault(t *testing.T) {
	faults, excluded, err := Check("")
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) > 0 {
		t.Fatalf("fautes dans ce dépôt :\n%s", strings.Join(faults, "\n"))
	}
	for _, e := range excluded {
		if !strings.HasPrefix(e, "testdata/vecteurs/") || !strings.HasSuffix(e, ".json") {
			t.Errorf("exclusion hors des copies de vecteurs : %s", e)
		}
	}
}

func TestScanAllows(t *testing.T) {
	allowed := []string{
		j("pour Fatoom : `gs:", "//<bucket-audit>`, `europe-west9`"),
		j("`gs:", "//<bucket-inscrits>/inscrits/<uid>`"),
		j("un `gs:", "//`, un chemin `-docker.pkg.dev/`, une adresse `.iam.gserviceaccount.com`, un chemin `projects/…`, un hôte `run.app`"),
		j("europe-west9-docker.pkg.dev/", "<projet-sesame>/<depot>/<image>@sha256:<empreinte>"),
		j("<compte-charge>@<projet-sesame>", ".iam.gserviceaccount.com"),
		j("projects/", "<numero-projet-sesame>/locations/global"),
		j("https://<service>.", "run.app"),
		"<numero-projet-sesame>",
		"12345678901 et 1234567890123 ne sont pas des numéros de projet",
		j("abc", "123456", "789012", "def"),
	}
	for _, text := range allowed {
		if faults := Scan("f", []byte(text)); len(faults) > 0 {
			t.Errorf("%q : fautes inattendues %v", text, faults)
		}
	}
}

// Contrôle positif : chaque règle trouve un nom réel.
func TestScanFindsNames(t *testing.T) {
	faulty := map[string]string{
		j("gs:", "//bucket-reel/bornes"):                               "bucket après gs://",
		j("gs:", "//<bucket>-reel"):                                    "bucket après gs://",
		j("projects/", "projet-reel/locations"):                        "projet après projects/",
		j("europe-west9-docker.pkg.dev/", "projet-reel/sesame/sesame"): "chemin d'image",
		j("europe-west9-docker.pkg.dev/", "<projet-sesame>/sesame"):    "chemin d'image",
		j("compte@projet-reel", ".iam.gserviceaccount.com"):            "compte de service",
		j("<compte>@projet-reel", ".iam.gserviceaccount.com"):          "compte de service",
		j("https://sesame-fake-abc123-ew.a.", "run.app"):               "hôte avant run.app",
		j("\"projet\": \"", "123456", "789012", "\""):                  "nombre de 12 chiffres",
		j("numéro ", "123456", "789012", "."):                          "nombre de 12 chiffres",
		j("ligne 1\nligne 2 avec gs:", "//bucket-reel"):                "f:2 : bucket",
	}
	for text, want := range faulty {
		faults := Scan("f", []byte(text))
		if len(faults) != 1 || !strings.Contains(faults[0], want) {
			t.Errorf("%q : attendu une faute « %s », obtenu %v", text, want, faults)
		}
	}
}

// Contrôle positif sur un dépôt Git : un fichier suivi fautif est trouvé, un
// fichier non suivi est ignoré, une copie de vecteurs de SHA256SUMS est
// exclue, compteurPin.json (écrit par ce dépôt) ne l'est pas.
func TestCheckOnRepository(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		f := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module exemple.test/m\n\ngo 1.27.4\n")
	write("docs/a.md", j("Bucket : gs:", "//bucket-reel\n"))
	write("testdata/vecteurs/attestation.json", j("{\"projet\": \"", "123456", "789012", "\"}\n"))
	write("testdata/vecteurs/SHA256SUMS", "0000000000000000000000000000000000000000000000000000000000000000  attestation.json\n")
	write("testdata/vecteurs/compteurPin.json", j("{\"n\": ", "123456", "789012", "}\n"))
	for _, args := range [][]string{{"init", "-q"}, {"add", "go.mod", "docs", "testdata"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v : %v\n%s", args, err, out)
		}
	}
	write("non-suivi.md", j("gs:", "//bucket-reel\n"))
	faults, excluded, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(excluded) != 1 || excluded[0] != "testdata/vecteurs/attestation.json" {
		t.Errorf("exclues = %v", excluded)
	}
	if len(faults) != 2 || !strings.HasPrefix(faults[0], "docs/a.md:1 ") || !strings.HasPrefix(faults[1], "testdata/vecteurs/compteurPin.json:1 ") {
		t.Errorf("fautes = %v, attendu docs/a.md et compteurPin.json", faults)
	}
}

func TestVectorCopiesRejectsPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "testdata", "vecteurs"), 0o755); err != nil {
		t.Fatal(err)
	}
	sums := "0000000000000000000000000000000000000000000000000000000000000000  ../../docs/a.md\n"
	if err := os.WriteFile(filepath.Join(root, "testdata", "vecteurs", "SHA256SUMS"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VectorCopies(root); err == nil {
		t.Fatal("un chemin dans SHA256SUMS ne doit pas exclure un fichier hors de testdata/vecteurs/")
	}
}
