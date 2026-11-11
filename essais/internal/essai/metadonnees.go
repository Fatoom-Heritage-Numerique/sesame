package essai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ServeurMetadonnees est l'adresse du serveur de métadonnées de Compute
// Engine, d'où viennent le jeton d'accès du compte de service de la VM et
// l'identifiant du projet ; par son adresse IP, sans résolution DNS.
const ServeurMetadonnees = "http://169.254.169.254/computeMetadata/v1"

// Metadonnees lit le serveur de métadonnées et garde le jeton d'accès
// jusqu'à une minute avant son expiration. Le jeton ne quitte jamais ce
// type : il n'est ni journalisé ni rendu par une route.
type Metadonnees struct {
	base   string
	client *http.Client

	mu       sync.Mutex
	jeton    string
	expireLe time.Time
}

// NouvellesMetadonnees rend un lecteur du serveur de métadonnées à base.
func NouvellesMetadonnees(base string) *Metadonnees {
	return &Metadonnees{base: strings.TrimSuffix(base, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

func (m *Metadonnees) lire(ctx context.Context, chemin string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.base+chemin, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	rep, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rep.Body.Close()
	corps, err := io.ReadAll(io.LimitReader(rep.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	if rep.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("métadonnées %s : statut %d", chemin, rep.StatusCode)
	}
	return corps, nil
}

// Projet rend l'identifiant du projet de la VM.
func (m *Metadonnees) Projet(ctx context.Context) (string, error) {
	corps, err := m.lire(ctx, "/project/project-id")
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(corps))
	if p == "" {
		return "", errors.New("métadonnées : projet vide")
	}
	return p, nil
}

// Jeton rend un jeton d'accès du compte de service de la VM.
func (m *Metadonnees) Jeton(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jeton != "" && time.Now().Before(m.expireLe) {
		return m.jeton, nil
	}
	corps, err := m.lire(ctx, "/instance/service-accounts/default/token")
	if err != nil {
		return "", err
	}
	var r struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(corps, &r); err != nil || r.AccessToken == "" {
		return "", errors.New("métadonnées : jeton d'accès illisible")
	}
	m.jeton = r.AccessToken
	m.expireLe = time.Now().Add(time.Duration(r.ExpiresIn)*time.Second - time.Minute)
	return m.jeton, nil
}
