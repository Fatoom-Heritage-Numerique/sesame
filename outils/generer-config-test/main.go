// Commande generer-config-test : écrit la configuration de test générique de
// sesame-fake (testdata/factice/configuration.json) et les clés privées de test
// des émetteurs de jetons (testdata/factice/emetteurs-prives.json), clés de
// test publiques et sans valeur hors des tests (décision 0063). Usage :
//
//	go run ./outils/generer-config-test testdata/factice
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/attestation"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/routes"
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

var b64 = base64.StdEncoding.EncodeToString

func main() {
	out := os.Args[1]
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2036, 1, 1, 0, 0, 0, 0, time.UTC)
	rootKey := must(rsa.GenerateKey(rand.Reader, 2048))
	interKey := must(rsa.GenerateKey(rand.Reader, 2048))
	ca := func(serial int64, cn string) *x509.Certificate {
		return &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn, Organization: []string{"Sesame (test)"}},
			NotBefore: from, NotAfter: to, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, SignatureAlgorithm: x509.SHA256WithRSA}
	}
	rootT := ca(1, "Racine de test de sesame-fake")
	root := must(x509.CreateCertificate(rand.Reader, rootT, rootT, &rootKey.PublicKey, rootKey))
	inter := must(x509.CreateCertificate(rand.Reader, ca(2, "Intermediaire de test de sesame-fake"), rootT, &interKey.PublicKey, rootKey))
	sum := sha256.Sum256(root)
	sess := must(ecdsa.GenerateKey(elliptic.P256(), rand.Reader))
	sessPub := must(sess.PublicKey.ECDH()).Bytes()
	issuers := map[string]any{}
	privates := map[string]any{}
	for _, n := range []string{"idToken", "google"} {
		k := must(rsa.GenerateKey(rand.Reader, 2048))
		issuers[n] = map[string]string{n + "-test-1": b64(must(x509.MarshalPKIXPublicKey(&k.PublicKey)))}
		privates[n] = map[string]string{n + "-test-1": b64(must(x509.MarshalPKCS8PrivateKey(k)))}
	}
	textes := jcs.Object{}
	for _, n := range append(append([]string{}, routes.Textes...), "alerte_signalement_insee_email", "alerte_signalement_insee_rappel_email", "alerte_signalement_insee_sms",
		"alerte_signalement_acte_email", "alerte_signalement_acte_rappel_email", "alerte_signalement_acte_sms",
		"alerte_signalement_inactivite_email", "alerte_signalement_inactivite_rappel_email", "alerte_signalement_inactivite_sms") {
		textes[n] = "Texte de test " + n + "."
	}
	doc := must(jcs.Marshal(jcs.Object{"v": 1, "textes": textes}))
	dsum := sha256.Sum256(doc)
	env := map[string]string{
		"COMPTE_DECES": "deces@comptes.exemple.test", "COMPTE_EXPLOITATION": "exploitation@comptes.exemple.test",
		"COURRIER_ORGANISATION": "organisation-de-test", "EMAIL_EXPEDITEUR": "Sesame de test <sesame@exemple.test>",
		"IDTOKEN_PROJET": "projet-de-test", "PASSERELLE_URL": "https://passerelle.exemple.test",
		"SESAME_AUDIENCE": "https://sesame.exemple.test", "SESAME_TEXTES": "sha256:" + hex.EncodeToString(dsum[:]),
		"SMS_EXPEDITEUR": "SesameTest", "VERIFICATION_SERVICE": "VA-service-de-test",
	}
	envObj := jcs.Object{}
	for k, v := range env {
		envObj[k] = v
	}
	envDigest := must(attestation.EnvDigest(envObj))
	digest := sha256.Sum256([]byte("image de test de sesame-fake"))
	cfg := map[string]any{
		"attention": "Configuration de test du dépôt sesame : clés de test publiques, sans valeur hors des tests.",
		"attestation": map[string]string{"empreinteRacine": hex.EncodeToString(sum[:]), "racine": b64(root), "intermediaire": b64(inter),
			"intermediairePrivee": b64(must(x509.MarshalPKCS8PrivateKey(interKey)))},
		"envOverride": env, "projet": "1234567", "registreImages": "registre.exemple.test/projet-de-test/sesame/",
		"sessionSign": map[string]string{"kid": "session-sign/1", "privee": b64(must(x509.MarshalPKCS8PrivateKey(sess))), "publique": b64(sessPub)},
		"versions": map[string]any{"v": 1, "clesSession": map[string]any{"session-sign/1": map[string]string{"cle": b64(sessPub), "depuis": "2026-01-01T00:00:00.000Z"}},
			"versions": []any{map[string]any{"commit": "0000000000000000000000000000000000000001", "envDigest": envDigest, "hwmodels": []string{"GCP_AMD_SEV", "GCP_INTEL_TDX"},
				"imageDigest": "sha256:" + hex.EncodeToString(digest[:]), "notes": "Version de test de sesame-fake.", "publieLe": "2026-01-01T00:00:00.000Z",
				"retireLe": nil, "statut": "en_service", "tag": "v0.0.1", "version": "0.0.1"}}},
		"emetteurs": issuers, "textes": json.RawMessage(doc), "mode": "grappe-locale", "ecoute": "127.0.0.1:8090", "nomVm": "sesame-test",
	}
	write := func(name string, v any) {
		b := must(json.MarshalIndent(v, "", "  "))
		must(0, os.WriteFile(out+"/"+name, append(b, '\n'), 0o644))
	}
	write("configuration.json", cfg)
	write("emetteurs-prives.json", map[string]any{"attention": "Clés privées de test des émetteurs de jetons, publiques et sans valeur hors des tests.", "emetteurs": privates})
}
