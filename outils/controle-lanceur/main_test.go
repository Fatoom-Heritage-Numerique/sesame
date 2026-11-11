package main

import (
	"strings"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/fixture"
)

const reading = "package p\n\nimport \"os\"\n\nvar launcher = os.Getenv(\"" + LauncherVar + "\")\n"

func inPackage(name string) string { return strings.Replace(reading, "package p", "package "+name, 1) }

func TestRepositoryHasNoFault(t *testing.T) {
	faults, err := Check("")
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) > 0 {
		t.Fatalf("fautes dans ce dépôt : %v", faults)
	}
}

func TestReadingInInternalFacticeAllowed(t *testing.T) {
	root := fixture.Module(t, "exemple.test/m", map[string]string{
		"cmd/sesame/main.go":          "package main\n\nfunc main() {}\n",
		"cmd/sesame-fake/main.go":     "package main\n\nimport _ \"exemple.test/m/internal/factice\"\n\nfunc main() {}\n",
		"internal/factice/lanceur.go": inPackage("factice"),
	})
	faults, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) > 0 {
		t.Fatalf("fautes inattendues : %v", faults)
	}
}

// Contrôle positif : le nom écrit hors de internal/factice/ est trouvé.
func TestReadingOutsideInternalFactice(t *testing.T) {
	root := fixture.Module(t, "exemple.test/m", map[string]string{
		"cmd/sesame/main.go":           "package main\n\nfunc main() {}\n",
		"internal/routes/lanceur.go":   inPackage("routes"),
		"internal/routes/x_test.go":    inPackage("routes"),
		"internal/facticeux/x.go":      inPackage("facticeux"),
		"testdata/x.go":                reading,
		"Dockerfile":                   "FROM scratch\nENV " + LauncherVar + "=1\n",
		"docs/lanceur.md":              "La variable " + LauncherVar + " n'est lue que par internal/factice/.\n",
		"outils/controle-lanceur/x.go": inPackage("main"),
	})
	faults, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Dockerfile", "internal/facticeux/x.go", "internal/routes/lanceur.go", "internal/routes/x_test.go", "testdata/x.go"}
	if len(faults) != len(want) {
		t.Fatalf("fautes = %v, attendu une par fichier de %v", faults, want)
	}
	for i, f := range want {
		if !strings.HasPrefix(faults[i], f+" ") {
			t.Errorf("faute %d = %q, attendu %s", i, faults[i], f)
		}
	}
}

// Contrôle positif : un paquet de internal/factice/ lié par cmd/sesame, même
// par un intermédiaire, est trouvé.
func TestCmdSesameLinksInternalFactice(t *testing.T) {
	root := fixture.Module(t, "exemple.test/m", map[string]string{
		"cmd/sesame/main.go":          "package main\n\nimport _ \"exemple.test/m/internal/routes\"\n\nfunc main() {}\n",
		"internal/routes/routes.go":   "package routes\n\nimport _ \"exemple.test/m/internal/factice/kms\"\n",
		"internal/factice/kms/kms.go": "package kms\n",
		"internal/factice/factice.go": "package factice\n",
	})
	faults, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) != 1 || faults[0] != "cmd/sesame lie exemple.test/m/internal/factice/kms" {
		t.Fatalf("fautes = %v, attendu le seul paquet internal/factice/kms", faults)
	}
}
