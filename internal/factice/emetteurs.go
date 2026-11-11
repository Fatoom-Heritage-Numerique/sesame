package factice

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Émetteurs distants (écart E-5 de l'étape 3) : pour un projet réel dont
// les clés tournent, sesame-fake relit les clés publiques d'un émetteur à
// une adresse de sa configuration (emetteursDistants), sous l'une des deux
// formes que publie Google : un JWKS ({"keys": [{kid, kty: "RSA", n, e}]},
// comme https://www.googleapis.com/oauth2/v3/certs) ou un dictionnaire de
// certificats X.509 en PEM par kid (comme les certificats de securetoken).
// Les clés sont relues après la durée de Cache-Control max-age, une heure
// au plus, et gardées si la relecture échoue. Le code des décisions ne voit
// que le port Emetteurs et n'accepte toujours qu'un RS256 vérifié.

// MaxKeysBody borne une réponse de clés.
const MaxKeysBody = 1 << 20

// Remote est un port Emetteurs : clés statiques de la configuration, puis
// clés relues aux adresses des émetteurs distants.
type Remote struct {
	Static map[string]map[string][]byte
	URLs   map[string]string
	Client *http.Client
	Clock  *Clock
	mu     sync.Mutex
	cache  map[string]fetched
}

type fetched struct {
	keys  map[string][]byte
	until time.Time
}

func (r *Remote) Keys(ctx context.Context, issuer string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for k, v := range r.Static[issuer] {
		out[k] = v
	}
	u, ok := r.URLs[issuer]
	if !ok {
		return out, nil
	}
	r.mu.Lock()
	f := r.cache[issuer]
	r.mu.Unlock()
	if now := r.Clock.Now(); !now.Before(f.until) {
		if keys, life, err := r.fetch(ctx, u); err == nil {
			f = fetched{keys, now.Add(min(life, time.Hour))}
			r.mu.Lock()
			if r.cache == nil {
				r.cache = map[string]fetched{}
			}
			r.cache[issuer] = f
			r.mu.Unlock()
		} else if f.keys == nil {
			return nil, err
		}
	}
	for k, v := range f.keys {
		out[k] = v
	}
	return out, nil
}

func (r *Remote) fetch(ctx context.Context, u string) (map[string][]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxKeysBody))
	if err != nil || resp.StatusCode != 200 {
		return nil, 0, errors.New("clés de l'émetteur illisibles")
	}
	life := time.Hour
	for _, d := range strings.Split(resp.Header.Get("Cache-Control"), ",") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(d), "max-age="); ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				life = time.Duration(n) * time.Second
			}
		}
	}
	keys, err := ParseKeys(body)
	return keys, life, err
}

// ParseKeys lit un JWKS RSA ou un dictionnaire de certificats PEM par kid et
// rend chaque clé en SubjectPublicKeyInfo DER.
func ParseKeys(body []byte) (map[string][]byte, error) {
	out := map[string][]byte{}
	var jwks struct {
		Keys []struct{ Kid, Kty, N, E string } `json:"keys"`
	}
	if json.Unmarshal(body, &jwks) == nil && jwks.Keys != nil {
		for _, k := range jwks.Keys {
			n, err1 := base64.RawURLEncoding.DecodeString(k.N)
			e, err2 := base64.RawURLEncoding.DecodeString(k.E)
			if k.Kty != "RSA" || err1 != nil || err2 != nil || len(e) > 4 {
				continue
			}
			der, err := x509.MarshalPKIXPublicKey(&rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())})
			if err == nil {
				out[k.Kid] = der
			}
		}
		return out, nil
	}
	var certs map[string]string
	if err := json.Unmarshal(body, &certs); err != nil {
		return nil, err
	}
	for kid, p := range certs {
		b, _ := pem.Decode([]byte(p))
		if b == nil {
			continue
		}
		if c, err := x509.ParseCertificate(b.Bytes); err == nil {
			if der, err := x509.MarshalPKIXPublicKey(c.PublicKey); err == nil {
				out[kid] = der
			}
		}
	}
	return out, nil
}
