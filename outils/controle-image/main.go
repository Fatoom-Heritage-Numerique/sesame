// Commande controle-image : contrôle « Image » de RELEASE.md §8 (décision
// 0065), sur le Dockerfile et sur l'archive OCI qu'écrit build/reproduire.sh.
//
// Le contrôle échoue si :
//   - une image de base du Dockerfile n'est pas épinglée par digest ;
//   - l'image contient un shell (sh, bash, busybox…), quel que soit son
//     dossier ;
//   - les étiquettes tee.launch_policy.* de l'image ne sont pas exactement
//     celles d'ARCHITECTURE.md §2.1 (une de plus, une de moins ou une autre
//     valeur) ;
//   - l'image pose la variable du lanceur factice (ARCHITECTURE.md §2.4).
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

// EnvOverride est la liste exacte des variables d'ARCHITECTURE.md §2.3, dans
// son ordre : celles que la politique de lancement admet.
var EnvOverride = []string{
	"VERIFICATION_SERVICE",
	"COURRIER_ORGANISATION",
	"EMAIL_EXPEDITEUR",
	"PASSERELLE_URL",
	"SESAME_AUDIENCE",
	"SMS_EXPEDITEUR",
	"IDTOKEN_PROJET",
	"COMPTE_DECES",
	"COMPTE_EXPLOITATION",
	"SESAME_TEXTES",
}

// LaunchPolicy rend les étiquettes de la politique de lancement
// (ARCHITECTURE.md §2.1).
func LaunchPolicy() map[string]string {
	return map[string]string{
		"tee.launch_policy.allow_env_override":  strings.Join(EnvOverride, ","),
		"tee.launch_policy.allow_cmd_override":  "false",
		"tee.launch_policy.log_redirect":        "never",
		"tee.launch_policy.hardened_monitoring": "none",
		"tee.launch_policy.debug_monitoring":    "none",
	}
}

// shells sont les noms de fichier refusés dans l'image.
var shells = map[string]bool{
	"sh": true, "bash": true, "dash": true, "ash": true, "zsh": true, "ksh": true,
	"mksh": true, "csh": true, "tcsh": true, "fish": true, "busybox": true, "toybox": true,
}

// fakeLauncherVar est la variable du lanceur factice, écrite en deux morceaux
// pour que seul outils/controle-lanceur écrive le nom d'un seul tenant.
var fakeLauncherVar = "SESAME_FAKE_" + "LAUNCHER"

func main() {
	dockerfile := flag.String("dockerfile", "Dockerfile", "chemin du Dockerfile")
	archive := flag.String("oci", "", "archive OCI de l'image (build/reproduire.sh, variable SORTIE_OCI)")
	flag.Parse()
	var faults []string
	content, err := os.ReadFile(*dockerfile)
	if err != nil {
		fail(err)
	}
	faults = append(faults, CheckDockerfile(content)...)
	if *archive != "" {
		f, err := os.Open(*archive)
		if err != nil {
			fail(err)
		}
		img, err := ReadOCI(f)
		f.Close()
		if err != nil {
			fail(err)
		}
		faults = append(faults, CheckImage(img)...)
	}
	for _, f := range faults {
		fmt.Fprintln(os.Stderr, "controle-image :", f)
	}
	if len(faults) > 0 {
		os.Exit(1)
	}
	if *archive == "" {
		fmt.Println("Dockerfile : images de base épinglées par digest (archive OCI non donnée : image non contrôlée)")
		return
	}
	fmt.Println("image : sans shell, politique de lancement d'ARCHITECTURE.md §2.1, images de base épinglées par digest")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "controle-image :", err)
	os.Exit(1)
}

var (
	fromLine = regexp.MustCompile(`(?i)^FROM\s+(?:--platform=\S+\s+)?(\S+)(?:\s+AS\s+(\S+))?\s*$`)
	pinned   = regexp.MustCompile(`^[a-z0-9.\-/:]+@sha256:[0-9a-f]{64}$`)
)

// CheckDockerfile rend une faute par image de base qui n'est pas épinglée par
// digest (une étape nommée plus haut peut servir de base).
func CheckDockerfile(content []byte) []string {
	var faults []string
	stages := map[string]bool{}
	lines := bufio.NewScanner(bytes.NewReader(content))
	n, froms := 0, 0
	for lines.Scan() {
		n++
		line := strings.TrimSpace(lines.Text())
		if !strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			continue
		}
		froms++
		m := fromLine.FindStringSubmatch(line)
		if m == nil {
			faults = append(faults, fmt.Sprintf("Dockerfile:%d : FROM illisible : %s", n, line))
			continue
		}
		if !stages[strings.ToLower(m[1])] && !pinned.MatchString(m[1]) {
			faults = append(faults, fmt.Sprintf("Dockerfile:%d : image de base %s non épinglée par digest", n, m[1]))
		}
		if m[2] != "" {
			stages[strings.ToLower(m[2])] = true
		}
	}
	if froms == 0 {
		faults = append(faults, "Dockerfile sans FROM")
	}
	return faults
}

// Image est ce que le contrôle lit d'une image : sa configuration et les
// chemins de ses couches.
type Image struct {
	Labels map[string]string
	Env    []string
	Files  []string
}

type descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
}

type index struct {
	MediaType string       `json:"mediaType"`
	Manifests []descriptor `json:"manifests"`
}

type manifest struct {
	MediaType string       `json:"mediaType"`
	Config    descriptor   `json:"config"`
	Layers    []descriptor `json:"layers"`
}

type config struct {
	Config struct {
		Labels map[string]string `json:"Labels"`
		Env    []string          `json:"Env"`
	} `json:"config"`
}

const (
	mediaIndex       = "application/vnd.oci.image.index.v1+json"
	mediaManifest    = "application/vnd.oci.image.manifest.v1+json"
	mediaLayerGzip   = "application/vnd.oci.image.layer.v1.tar+gzip"
	mediaLayerTar    = "application/vnd.oci.image.layer.v1.tar"
	dockerLayerGzip  = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	dockerManifest   = "application/vnd.docker.distribution.manifest.v2+json"
	dockerList       = "application/vnd.docker.distribution.manifest.list.v2+json"
	maxIndexNestings = 4
)

// ReadOCI lit une archive tar au format OCI (index.json, blobs/sha256/…) qui
// porte une seule image.
func ReadOCI(r io.Reader) (*Image, error) {
	blobs := map[string][]byte{}
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		blobs[path.Clean(h.Name)] = content
	}
	blob := func(digest string) ([]byte, error) {
		algo, hex, ok := strings.Cut(digest, ":")
		if !ok || algo != "sha256" {
			return nil, fmt.Errorf("digest %q illisible", digest)
		}
		b, ok := blobs["blobs/sha256/"+hex]
		if !ok {
			return nil, fmt.Errorf("blob %s absent de l'archive", digest)
		}
		return b, nil
	}
	raw, ok := blobs["index.json"]
	if !ok {
		return nil, errors.New("archive sans index.json : attendu le format OCI de build/reproduire.sh")
	}
	var m manifest
	for depth := 0; ; depth++ {
		var idx index
		if err := json.Unmarshal(raw, &idx); err != nil {
			return nil, err
		}
		if len(idx.Manifests) != 1 {
			return nil, fmt.Errorf("index de %d manifestes : attendu une seule image", len(idx.Manifests))
		}
		d := idx.Manifests[0]
		b, err := blob(d.Digest)
		if err != nil {
			return nil, err
		}
		if d.MediaType == mediaManifest || d.MediaType == dockerManifest {
			if err := json.Unmarshal(b, &m); err != nil {
				return nil, err
			}
			break
		}
		if (d.MediaType != mediaIndex && d.MediaType != dockerList) || depth == maxIndexNestings {
			return nil, fmt.Errorf("type %q inattendu dans l'index", d.MediaType)
		}
		raw = b
	}
	cb, err := blob(m.Config.Digest)
	if err != nil {
		return nil, err
	}
	var c config
	if err := json.Unmarshal(cb, &c); err != nil {
		return nil, err
	}
	img := &Image{Labels: c.Config.Labels, Env: c.Config.Env}
	for _, l := range m.Layers {
		b, err := blob(l.Digest)
		if err != nil {
			return nil, err
		}
		var lr io.Reader = bytes.NewReader(b)
		switch l.MediaType {
		case mediaLayerGzip, dockerLayerGzip:
			gz, err := gzip.NewReader(lr)
			if err != nil {
				return nil, err
			}
			lr = gz
		case mediaLayerTar:
		default:
			return nil, fmt.Errorf("couche de type %q non lue", l.MediaType)
		}
		lt := tar.NewReader(lr)
		for {
			h, err := lt.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			if h.Typeflag != tar.TypeDir {
				img.Files = append(img.Files, path.Clean("/"+h.Name))
			}
		}
	}
	return img, nil
}

// CheckImage rend les fautes d'une image.
func CheckImage(img *Image) []string {
	var faults []string
	for _, f := range img.Files {
		if shells[path.Base(f)] {
			faults = append(faults, fmt.Sprintf("shell dans l'image : %s", f))
		}
	}
	want := LaunchPolicy()
	var keys []string
	for k := range img.Labels {
		if strings.HasPrefix(k, "tee.") {
			keys = append(keys, k)
		}
	}
	for k := range want {
		if _, ok := img.Labels[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		got, has := img.Labels[k]
		expected, known := want[k]
		switch {
		case !known:
			faults = append(faults, fmt.Sprintf("étiquette %s=%q hors de la politique d'ARCHITECTURE.md §2.1", k, got))
		case !has:
			faults = append(faults, fmt.Sprintf("étiquette %s absente, attendu %q", k, expected))
		case got != expected:
			faults = append(faults, fmt.Sprintf("étiquette %s=%q, attendu %q", k, got, expected))
		}
	}
	for _, e := range img.Env {
		if name, _, _ := strings.Cut(e, "="); name == fakeLauncherVar {
			faults = append(faults, fmt.Sprintf("l'image pose %s", name))
		}
	}
	return faults
}
