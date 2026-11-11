package factice

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
)

// LauncherEnv est la variable qui autorise le lanceur factice ; ce paquet est
// le seul à la lire (RELEASE.md §8).
const LauncherEnv = "SESAME_FAKE_LAUNCHER"

// ErrLauncher : lanceur factice demandé sans SESAME_FAKE_LAUNCHER=1.
var ErrLauncher = errors.New("lanceur factice refusé : " + LauncherEnv + " n'est pas 1")

// Claims sont les claims propres à une VM factice, que le test peut changer.
type Claims struct {
	InstanceName, Zone, InstanceID string
	Edit                           func(c jcs.Object) // retouche du test, appliquée en dernier
}

// Launcher est le lanceur factice d'une VM : il signe ses jetons PKI par une
// feuille tirée au démarrage, signée par l'intermédiaire de test de la
// configuration ; x5c porte feuille, intermédiaire, racine.
type Launcher struct {
	cfg    *Config
	clock  ports.Horloge
	key    *rsa.PrivateKey
	leaf   []byte
	mu     sync.Mutex
	Claims Claims
	Fail   bool
}

// NewLauncher rend le lanceur factice, seulement sous SESAME_FAKE_LAUNCHER=1.
func NewLauncher(cfg *Config, clock ports.Horloge, claims Claims) (*Launcher, error) {
	if os.Getenv(LauncherEnv) != "1" {
		return nil, ErrLauncher
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	inter, err := x509.ParseCertificate(cfg.inter)
	if err != nil {
		return nil, err
	}
	t := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "Feuille de test de sesame-fake"},
		NotBefore: inter.NotBefore, NotAfter: inter.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, SignatureAlgorithm: x509.SHA256WithRSA}
	leaf, err := x509.CreateCertificate(rand.Reader, t, inter, &key.PublicKey, cfg.interKey)
	if err != nil {
		return nil, err
	}
	return &Launcher{cfg: cfg, clock: clock, key: key, leaf: leaf, Claims: claims}, nil
}

var b64url = base64.RawURLEncoding

// Token rend un jeton valable 1 h dont eat_nonce porte nonces.
func (l *Launcher) Token(_ context.Context, nonces []string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Fail {
		return "", errDouble
	}
	now := l.clock.Now().Unix()
	env := jcs.Object{}
	for k, v := range l.cfg.EnvOverride {
		env[k] = v
	}
	n := jcs.Array{}
	for _, s := range nonces {
		n = append(n, s)
	}
	c := jcs.Object{"iss": attestation.Issuer, "aud": attestation.Audience, "iat": now, "nbf": now, "exp": now + 3600,
		"eat_nonce": n, "swname": "CONFIDENTIAL_SPACE", "hwmodel": l.cfg.hwmodel, "dbgstat": "disabled-since-boot", "secboot": true,
		"submods": jcs.Object{
			"confidential_space": jcs.Object{"support_attributes": jcs.Array{"LATEST", "STABLE", "USABLE"}},
			"container": jcs.Object{"image_digest": l.cfg.imageDigest, "image_reference": l.cfg.RegistreImages + "sesame@" + l.cfg.imageDigest,
				"env_override": env, "restart_policy": "Always"},
			"gce": jcs.Object{"project_number": l.cfg.Projet, "zone": l.Claims.Zone, "instance_name": l.Claims.InstanceName, "instance_id": l.Claims.InstanceID},
		}}
	if l.Claims.Edit != nil {
		l.Claims.Edit(c)
	}
	x5c := jcs.Array{}
	for _, d := range [][]byte{l.leaf, l.cfg.inter, l.cfg.root} {
		x5c = append(x5c, base64.StdEncoding.EncodeToString(d))
	}
	h, _ := jcs.Marshal(jcs.Object{"alg": "RS256", "typ": "JWT", "x5c": x5c})
	p, err := jcs.Marshal(c)
	if err != nil {
		return "", err
	}
	input := b64url.EncodeToString(h) + "." + b64url.EncodeToString(p)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, l.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return input + "." + b64url.EncodeToString(sig), nil
}
