package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/outils/internal/golist"
)

func repositoryFile(t *testing.T, name string) []byte {
	t.Helper()
	root, err := golist.Root()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// La liste des variables est celle d'ARCHITECTURE.md §2.3, qui fait foi.
func TestEnvOverrideMatchesArchitecture(t *testing.T) {
	text := string(repositoryFile(t, "ARCHITECTURE.md"))
	start := strings.Index(text, "dans des **variables d'environnement**")
	end := strings.Index(text[start+1:], "\n- ")
	if start < 0 || end < 0 {
		t.Fatal("ARCHITECTURE.md §2.3 : liste des variables introuvable")
	}
	item := text[start : start+1+end]
	var names []string
	for _, m := range regexp.MustCompile("`([A-Z][A-Z_]+)` \\(").FindAllStringSubmatch(item, -1) {
		names = append(names, m[1])
	}
	if strings.Join(names, ",") != strings.Join(EnvOverride, ",") {
		t.Errorf("ARCHITECTURE.md §2.3 : %v\noutils/controle-image : %v", names, EnvOverride)
	}
}

// Le Dockerfile du dépôt épingle ses images de base et porte la politique de
// lancement ; l'image construite est contrôlée en CI (archive OCI).
func TestRepositoryDockerfile(t *testing.T) {
	content := repositoryFile(t, "Dockerfile")
	if faults := CheckDockerfile(content); len(faults) > 0 {
		t.Errorf("fautes : %v", faults)
	}
	for k, v := range LaunchPolicy() {
		if !bytes.Contains(content, []byte(`"`+k+`"="`+v+`"`)) {
			t.Errorf("Dockerfile sans l'étiquette %q=%q", k, v)
		}
	}
}

// Contrôle positif : une image de base sans digest est trouvée.
func TestCheckDockerfile(t *testing.T) {
	digest := "@sha256:" + strings.Repeat("ab", 32)
	cases := map[string]int{
		"FROM golang:1.27.4-bookworm" + digest + " AS construction\nFROM gcr.io/distroless/static-debian12:nonroot" + digest + "\n": 0,
		"FROM golang" + digest + " AS c\nFROM c\n":                    0,
		"FROM golang:1.27.4-bookworm AS construction\nFROM scratch\n": 2,
		"FROM gcr.io/distroless/static-debian12:nonroot\n":            1,
		"FROM --platform=linux/amd64 golang:1.27.4 AS c\nFROM c\n":    1,
		"FROM golang@sha256:abc\n":                                    1,
		"RUN true\n":                                                  1,
	}
	for content, want := range cases {
		if faults := CheckDockerfile([]byte(content)); len(faults) != want {
			t.Errorf("%q : %d fautes %v, attendu %d", content, len(faults), faults, want)
		}
	}
}

// ociArchive écrit une archive OCI d'une image à une couche.
func ociArchive(t *testing.T, labels map[string]string, env []string, files []string, gz bool) []byte {
	t.Helper()
	var layer bytes.Buffer
	var lw *tar.Writer
	var gzw *gzip.Writer
	if gz {
		gzw = gzip.NewWriter(&layer)
		lw = tar.NewWriter(gzw)
	} else {
		lw = tar.NewWriter(&layer)
	}
	lw.WriteHeader(&tar.Header{Name: "bin/", Typeflag: tar.TypeDir, Mode: 0o755})
	for _, f := range files {
		lw.WriteHeader(&tar.Header{Name: f, Typeflag: tar.TypeReg, Mode: 0o755, Size: 1})
		lw.Write([]byte{0})
	}
	lw.Close()
	if gzw != nil {
		gzw.Close()
	}
	blobs := map[string][]byte{}
	put := func(b []byte) string {
		s := sha256.Sum256(b)
		d := "sha256:" + hex.EncodeToString(s[:])
		blobs[d] = b
		return d
	}
	cfg := map[string]any{"config": map[string]any{"Labels": labels, "Env": env}}
	cb, _ := json.Marshal(cfg)
	layerType := mediaLayerTar
	if gz {
		layerType = mediaLayerGzip
	}
	mb, _ := json.Marshal(map[string]any{
		"mediaType": mediaManifest,
		"config":    map[string]string{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": put(cb)},
		"layers":    []map[string]string{{"mediaType": layerType, "digest": put(layer.Bytes())}},
	})
	ib, _ := json.Marshal(map[string]any{"mediaType": mediaIndex, "manifests": []map[string]string{{"mediaType": mediaManifest, "digest": put(mb)}}})
	var out bytes.Buffer
	w := tar.NewWriter(&out)
	add := func(name string, b []byte) {
		w.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(b))})
		w.Write(b)
	}
	add("oci-layout", []byte(`{"imageLayoutVersion":"1.0.0"}`))
	add("index.json", ib)
	for d, b := range blobs {
		add("blobs/sha256/"+strings.TrimPrefix(d, "sha256:"), b)
	}
	w.Close()
	return out.Bytes()
}

func readImage(t *testing.T, archive []byte) *Image {
	t.Helper()
	img, err := ReadOCI(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestCheckImageAcceptsPolicy(t *testing.T) {
	for _, gz := range []bool{true, false} {
		img := readImage(t, ociArchive(t, LaunchPolicy(), []string{"PATH=/usr/bin"}, []string{"sesame", "etc/passwd"}, gz))
		if faults := CheckImage(img); len(faults) > 0 {
			t.Errorf("gzip %v : fautes inattendues %v", gz, faults)
		}
	}
}

// Contrôles positifs : shell, étiquette absente, de plus ou fausse, lanceur
// factice.
func TestCheckImageFindsFaults(t *testing.T) {
	with := func(change func(map[string]string)) map[string]string {
		l := LaunchPolicy()
		change(l)
		return l
	}
	cases := []struct {
		name   string
		labels map[string]string
		env    []string
		files  []string
		want   string
	}{
		{"shell", LaunchPolicy(), nil, []string{"sesame", "bin/sh"}, "shell dans l'image : /bin/sh"},
		{"busybox ailleurs", LaunchPolicy(), nil, []string{"usr/local/lib/busybox"}, "shell dans l'image"},
		{"étiquette absente", with(func(l map[string]string) { delete(l, "tee.launch_policy.debug_monitoring") }), nil, nil, "debug_monitoring absente"},
		{"étiquette de plus", with(func(l map[string]string) { l["tee.launch_policy.allow_capabilities"] = "true" }), nil, nil, "allow_capabilities=\"true\" hors de la politique"},
		{"valeur fausse", with(func(l map[string]string) { l["tee.launch_policy.log_redirect"] = "always" }), nil, nil, "log_redirect=\"always\", attendu \"never\""},
		{"variable de plus admise", with(func(l map[string]string) {
			l["tee.launch_policy.allow_env_override"] += ",AUTRE"
		}), nil, nil, "allow_env_override"},
		{"lanceur factice", LaunchPolicy(), []string{fakeLauncherVar + "=1"}, nil, "l'image pose " + fakeLauncherVar},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			img := readImage(t, ociArchive(t, c.labels, c.env, c.files, true))
			faults := CheckImage(img)
			if len(faults) != 1 || !strings.Contains(faults[0], c.want) {
				t.Errorf("fautes = %v, attendu « %s »", faults, c.want)
			}
		})
	}
}

func TestReadOCIRejectsOtherArchives(t *testing.T) {
	var out bytes.Buffer
	w := tar.NewWriter(&out)
	b := []byte(`[{"Config":"x.json","Layers":[]}]`)
	w.WriteHeader(&tar.Header{Name: "manifest.json", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(b))})
	w.Write(b)
	w.Close()
	if _, err := ReadOCI(&out); err == nil || !strings.Contains(err.Error(), "index.json") {
		t.Fatalf("archive sans index.json : erreur attendue, obtenu %v", err)
	}
}
