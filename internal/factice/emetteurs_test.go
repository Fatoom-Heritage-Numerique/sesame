package factice_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
)

// Émetteurs distants de sesame-fake (écart E-5) : JWKS et certificats PEM
// relus à l'expiration de leur cache, gardés si la relecture échoue.
func TestEmetteursDistants(t *testing.T) {
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: t0, NotAfter: t0.Add(time.Hour)}
	cert, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	want, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
	hits, down := 0, false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if down {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=600")
		if r.URL.Path == "/jwks" {
			_, _ = w.Write([]byte(`{"keys":[{"kid":"k1","kty":"RSA","alg":"RS256","n":"` + base64.RawURLEncoding.EncodeToString(k.N.Bytes()) + `","e":"AQAB"}]}`))
			return
		}
		p := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
		_, _ = w.Write([]byte(`{"k2":` + jsonString(string(p)) + `}`))
	}))
	defer srv.Close()
	clock := &factice.Clock{}
	clock.Set(t0)
	r := &factice.Remote{URLs: map[string]string{"google": srv.URL + "/jwks", "idToken": srv.URL + "/certs"}, Client: srv.Client(), Clock: clock}
	for iss, kid := range map[string]string{"google": "k1", "idToken": "k2"} {
		keys, err := r.Keys(t.Context(), iss)
		if err != nil || string(keys[kid]) != string(want) {
			t.Fatalf("%s : %v %v", iss, keys, err)
		}
	}
	_, _ = r.Keys(t.Context(), "google")
	if hits != 2 {
		t.Fatalf("cache : %d lectures", hits)
	}
	clock.Advance(11 * time.Minute)
	down = true
	if keys, err := r.Keys(t.Context(), "google"); err != nil || keys["k1"] == nil || hits != 3 {
		t.Fatalf("relecture en échec : %v %v %d", keys, err, hits)
	}
}

func jsonString(s string) string {
	out := []byte{'"'}
	for _, c := range []byte(s) {
		if c == '\n' {
			out = append(out, '\\', 'n')
			continue
		}
		out = append(out, c)
	}
	return string(append(out, '"'))
}
