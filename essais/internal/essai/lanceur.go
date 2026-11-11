package essai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// SocketLanceur est la socket du lanceur de Confidential Space qui sert les
// jetons d'attestation (ARCHITECTURE.md §2.1 ; documentation de Google,
// « Connect to external resources » : POST http://localhost/v1/token sur
// /run/container_launcher/teeserver.sock).
const SocketLanceur = "/run/container_launcher/teeserver.sock"

// Bornes d'une demande de jeton, d'après la même documentation : audience
// d'au plus 512 octets, au plus 6 nonces de 10 à 74 octets chacun.
const (
	AudienceMax   = 512
	NoncesMax     = 6
	NonceMinOctet = 10
	NonceMaxOctet = 74
	jetonMax      = 64 << 10
)

// ErreurDemande est une demande de jeton hors des bornes du lanceur.
type ErreurDemande struct{ Motif string }

func (e ErreurDemande) Error() string { return e.Motif }

// VerifierDemande contrôle une demande avant de l'envoyer au lanceur.
func VerifierDemande(audience string, nonces []string) error {
	if audience == "" || len(audience) > AudienceMax {
		return ErreurDemande{fmt.Sprintf("audience de 1 à %d octets", AudienceMax)}
	}
	if len(nonces) > NoncesMax {
		return ErreurDemande{fmt.Sprintf("au plus %d nonces", NoncesMax)}
	}
	for _, n := range nonces {
		if len(n) < NonceMinOctet || len(n) > NonceMaxOctet {
			return ErreurDemande{fmt.Sprintf("nonce de %d à %d octets", NonceMinOctet, NonceMaxOctet)}
		}
	}
	return nil
}

// Lanceur demande des jetons au lanceur de Confidential Space.
type Lanceur struct {
	client *http.Client
}

// NouveauLanceur rend un client de la socket du lanceur au chemin donné.
func NouveauLanceur(socket string) *Lanceur {
	return &Lanceur{client: &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socket)
			},
		},
	}}
}

// ErreurLanceur est une réponse du lanceur autre que 200.
type ErreurLanceur struct {
	Statut int
	Corps  string
}

func (e ErreurLanceur) Error() string { return fmt.Sprintf("lanceur : statut %d", e.Statut) }

// JetonPKI demande un jeton de type PKI pour l'audience et les nonces donnés
// et rend le jeton brut (JWS compact) tel que le lanceur le sert.
func (l *Lanceur) JetonPKI(ctx context.Context, audience string, nonces []string) (string, error) {
	if err := VerifierDemande(audience, nonces); err != nil {
		return "", err
	}
	corps, err := json.Marshal(struct {
		Audience  string   `json:"audience"`
		TokenType string   `json:"token_type"`
		Nonces    []string `json:"nonces,omitempty"`
	}{audience, "PKI", nonces})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/v1/token", bytes.NewReader(corps))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	rep, err := l.client.Do(req)
	if err != nil {
		return "", err
	}
	defer rep.Body.Close()
	brut, err := io.ReadAll(io.LimitReader(rep.Body, jetonMax+1))
	if err != nil {
		return "", err
	}
	if rep.StatusCode != http.StatusOK {
		return "", ErreurLanceur{Statut: rep.StatusCode, Corps: tronquer(string(brut), 1024)}
	}
	if len(brut) > jetonMax {
		return "", errors.New("jeton du lanceur trop long")
	}
	jeton := strings.TrimSpace(string(brut))
	if strings.Count(jeton, ".") != 2 {
		// Forme inattendue : rendue telle quelle (tronquée) pour la séance.
		return "", ErreurLanceur{Statut: rep.StatusCode, Corps: tronquer(jeton, 1024)}
	}
	return jeton, nil
}

func tronquer(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
