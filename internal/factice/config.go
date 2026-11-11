// Paquet factice : sesame-fake, le contrat exécutable (décision 0063). Il
// branche le code des décisions d'internal/routes sur des doubles : lanceur
// factice (seul lecteur de SESAME_FAKE_LAUNCHER) qui signe ses jetons par la
// chaîne d'attestation de test reçue en configuration, KMS, registre
// (Firestore), Cloud Storage, Secret Manager, Cloud Logging, émetteurs de
// jetons, Twilio et SendGrid en mémoire, passerelle de test. Jamais lié par
// cmd/sesame (RELEASE.md §8, contrôle « Lanceur factice »).
package factice

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Config est la configuration que le client fournit à sesame-fake, lue en
// JSON à un chemin donné au démarrage. Les clés sont des clés de test.
type Config struct {
	Attention   string          `json:"attention"`
	Adresses    json.RawMessage `json:"adresses"` // propres au client, ignorées
	Attestation struct {
		EmpreinteRacine     string `json:"empreinteRacine"`
		Racine              string `json:"racine"`
		Intermediaire       string `json:"intermediaire"`
		IntermediairePrivee string `json:"intermediairePrivee"`
	} `json:"attestation"`
	EnvOverride    map[string]string `json:"envOverride"`
	Projet         string            `json:"projet"`
	RegistreImages string            `json:"registreImages"`
	SessionSign    struct {
		Kid      string `json:"kid"`
		Privee   string `json:"privee"`
		Publique string `json:"publique"`
	} `json:"sessionSign"`
	Versions json.RawMessage `json:"versions"`
	// Emetteurs : par émetteur (idToken, google), clés publiques RS256 de
	// test par kid, en SubjectPublicKeyInfo DER base64.
	Emetteurs map[string]map[string]string `json:"emetteurs"`
	// EmetteursDistants : par émetteur, adresse HTTPS d'un JWKS ou de
	// certificats PEM par kid, relue périodiquement (emetteurs.go).
	EmetteursDistants map[string]string `json:"emetteursDistants"`
	// Textes : le document des textes, d'empreinte SESAME_TEXTES.
	Textes json.RawMessage `json:"textes"`
	// Mode : "grappe-locale" (trois instances a, b, c) ou "instance".
	Mode    string `json:"mode"`
	Ecoute  string `json:"ecoute"`
	NomVM   string `json:"nomVm"`
	Prefixe string `json:"prefixe"`
	// PasserelleReelle : adresse où envoyer les requêtes à la passerelle,
	// quand ce n'est pas PASSERELLE_URL (émulateur local) ; vide et
	// PASSERELLE_URL injoignable, la passerelle de test en mémoire répond.
	PasserelleReelle string `json:"passerelleReelle"`

	root, inter []byte
	rootSum     []byte
	interKey    *rsa.PrivateKey
	sessionKey  *ecdsa.PrivateKey
	versions    *attestation.Versions
	imageDigest string
	hwmodel     string
	textes      []byte
	issuers     map[string]map[string][]byte
}

// ReadConfig lit et contrôle la configuration : JSON sans champ inconnu,
// chaîne d'attestation (racine d'empreinte empreinteRacine), clé
// session-sign présente dans clesSession, version en service dont l'envDigest
// est celui d'envOverride, document des textes d'empreinte SESAME_TEXTES.
func ReadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfig(b)
}

// ParseConfig lit la configuration depuis ses octets (ReadConfig).
func ParseConfig(b []byte) (*Config, error) {
	c := &Config{}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(c); err != nil {
		return nil, fmt.Errorf("configuration illisible : %w", err)
	}
	bad := func(what string) (*Config, error) { return nil, errors.New("configuration de sesame-fake : " + what) }
	var err error
	if c.root, err = primitives.B64(c.Attestation.Racine); err != nil {
		return bad("racine")
	}
	sum := primitives.SHA256(c.root)
	if hex.EncodeToString(sum) != c.Attestation.EmpreinteRacine {
		return bad("empreinteRacine")
	}
	c.rootSum = sum
	if c.inter, err = primitives.B64(c.Attestation.Intermediaire); err != nil {
		return bad("intermediaire")
	}
	der, err := primitives.B64(c.Attestation.IntermediairePrivee)
	k, err2 := x509.ParsePKCS8PrivateKey(der)
	if c.interKey, _ = k.(*rsa.PrivateKey); err != nil || err2 != nil || c.interKey == nil {
		return bad("intermediairePrivee")
	}
	der, err = primitives.B64(c.SessionSign.Privee)
	k, err2 = x509.ParsePKCS8PrivateKey(der)
	if c.sessionKey, _ = k.(*ecdsa.PrivateKey); err != nil || err2 != nil || c.sessionKey == nil {
		return bad("sessionSign.privee")
	}
	if c.versions, err = attestation.ReadVersions(c.Versions); err != nil {
		return bad("versions")
	}
	pub, _ := c.sessionKey.PublicKey.ECDH()
	if got := c.versions.SessionKeys[c.SessionSign.Kid]; got == nil || !bytes.Equal(got, pub.Bytes()) || c.SessionSign.Publique != primitives.B64Encode(pub.Bytes()) {
		return bad("sessionSign absente de clesSession")
	}
	env := jcs.Object{}
	for k, v := range c.EnvOverride {
		env[k] = v
	}
	digest, err := attestation.EnvDigest(env)
	for _, v := range c.versions.Versions {
		if v.Status == "en_service" && v.EnvDigest == digest && err == nil {
			c.imageDigest, c.hwmodel = v.ImageDigest, v.HWModels[0]
			break
		}
	}
	if c.imageDigest == "" {
		return bad("aucune version en service à l'envDigest d'envOverride")
	}
	if c.textes, err = jcs.Canonicalize(c.Textes); err != nil || "sha256:"+hex.EncodeToString(primitives.SHA256(c.textes)) != c.EnvOverride["SESAME_TEXTES"] {
		return bad("textes absents ou d'une autre empreinte que SESAME_TEXTES")
	}
	c.issuers = map[string]map[string][]byte{}
	for iss, keys := range c.Emetteurs {
		c.issuers[iss] = map[string][]byte{}
		for kid, s := range keys {
			if c.issuers[iss][kid], err = primitives.B64(s); err != nil {
				return bad("emetteurs")
			}
		}
	}
	for iss, u := range c.EmetteursDistants {
		if (iss != "idToken" && iss != "google") || !strings.HasPrefix(u, "https://") {
			return bad("emetteursDistants")
		}
	}
	switch {
	case c.Mode == "":
		c.Mode = "grappe-locale"
	case c.Mode != "grappe-locale" && c.Mode != "instance":
		return bad("mode")
	}
	if c.Mode == "instance" && c.Prefixe != "a" && c.Prefixe != "b" && c.Prefixe != "c" {
		return bad("prefixe")
	}
	if c.NomVM == "" {
		c.NomVM = "sesame-fake"
	}
	if c.Ecoute == "" {
		c.Ecoute = "127.0.0.1:8090"
	}
	return c, nil
}

// ImageDigest est l'empreinte que présente le jeton de test.
func (c *Config) ImageDigest() string { return c.imageDigest }

// RootSum est l'empreinte de la racine de test, que le client épingle.
func (c *Config) RootSum() []byte { return c.rootSum }

// VersionList est la liste des versions de test, compilée par le client.
func (c *Config) VersionList() *attestation.Versions { return c.versions }
