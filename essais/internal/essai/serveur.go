package essai

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// Serveur sert les trois routes de l'essai (essais/README.md). Ses journaux
// ne portent que des compteurs et des mesures : jamais un jeton, ni
// d'attestation ni d'accès.
type Serveur struct {
	// Heure fait une mesure SNTP de l'horloge de la VM auprès du serveur
	// d'adresse donnée (ServeurHeure ou ServeurHeureIP).
	Heure func(ctx context.Context, adresse string) (Mesure, error)
	// Jeton demande un jeton PKI au lanceur.
	Jeton func(ctx context.Context, audience string, nonces []string) (string, error)
	// Ecrivain rend l'écriture d'une transaction dans la base nommée.
	Ecrivain func(ctx context.Context, base string) (Ecrivain, error)
	// DemarreLe est l'heure de démarrage du processus : un relevé après une
	// migration à chaud montre que le conteneur n'a pas redémarré.
	DemarreLe time.Time

	enCours sync.Mutex // un essai de débit à la fois
}

// Handler rend les routes du serveur.
func (s *Serveur) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		repondre(w, http.StatusOK, map[string]any{
			"routes":    []string{"/horloge?mesures=&intervalle=&reference=nom|ip", "/attestation?audience=&nonce=&nonce=", "/firestore?base=&duree=&debit=&concurrence=&taille="},
			"demarreLe": s.DemarreLe,
		})
	})
	mux.HandleFunc("GET /horloge", s.horloge)
	mux.HandleFunc("GET /attestation", s.attestation)
	mux.HandleFunc("GET /firestore", s.firestore)
	return mux
}

func repondre(w http.ResponseWriter, statut int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statut)
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	_ = e.Encode(v)
}

func refuser(w http.ResponseWriter, statut int, motif string) {
	repondre(w, statut, map[string]string{"erreur": motif})
}

// RapportHorloge est le résultat de /horloge.
type RapportHorloge struct {
	Serveur   string         `json:"serveur"`
	HeureVM   time.Time      `json:"heureVm"`
	DemarreLe time.Time      `json:"demarreLe"`
	Mesures   int            `json:"mesures"`
	Reussies  int            `json:"reussies"`
	Erreurs   map[string]int `json:"erreurs"`
	EcartMs   *Resume        `json:"ecartMs"`
	DelaiMs   *Resume        `json:"delaiMs"`
	Detail    []Mesure       `json:"detail"`
}

func (s *Serveur) horloge(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	n, err := Entier(q, "mesures", 16, 1, 64)
	if err != nil {
		refuser(w, http.StatusBadRequest, err.Error())
		return
	}
	intervalle, err := Entier(q, "intervalle", 250, 0, 5000)
	if err != nil {
		refuser(w, http.StatusBadRequest, err.Error())
		return
	}
	adresse := ServeurHeure
	switch q.Get("reference") {
	case "", "nom":
	case "ip":
		adresse = ServeurHeureIP
	default:
		refuser(w, http.StatusBadRequest, "paramètre reference : nom ou ip")
		return
	}
	rap := RapportHorloge{Serveur: adresse, HeureVM: time.Now().UTC(), DemarreLe: s.DemarreLe, Mesures: n, Erreurs: map[string]int{}, Detail: []Mesure{}}
	var ecarts, delais []float64
	for i := 0; i < n; i++ {
		if i > 0 && intervalle > 0 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(time.Duration(intervalle) * time.Millisecond):
			}
		}
		m, err := s.Heure(r.Context(), adresse)
		if err != nil {
			rap.Erreurs[err.Error()]++
			continue
		}
		rap.Reussies++
		rap.Detail = append(rap.Detail, m)
		ecarts = append(ecarts, m.EcartMs)
		delais = append(delais, m.DelaiMs)
	}
	rap.EcartMs, rap.DelaiMs = Resumer(ecarts), Resumer(delais)
	log.Printf("horloge : %d mesures, %d réussies", rap.Mesures, rap.Reussies)
	repondre(w, http.StatusOK, rap)
}

func (s *Serveur) attestation(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	audience, nonces := q.Get("audience"), q["nonce"]
	if nonces == nil {
		nonces = []string{}
	}
	if err := VerifierDemande(audience, nonces); err != nil {
		refuser(w, http.StatusBadRequest, err.Error())
		return
	}
	jeton, err := s.Jeton(r.Context(), audience, nonces)
	var el ErreurLanceur
	switch {
	case errors.As(err, &el):
		log.Printf("attestation : refus du lanceur, statut %d", el.Statut)
		repondre(w, http.StatusBadGateway, map[string]any{"erreur": "lanceur", "statut": el.Statut, "corps": el.Corps})
		return
	case err != nil:
		log.Printf("attestation : lanceur injoignable")
		refuser(w, http.StatusBadGateway, "lanceur injoignable : "+err.Error())
		return
	}
	log.Printf("attestation : jeton servi (%d octets)", len(jeton))
	repondre(w, http.StatusOK, map[string]any{"audience": audience, "nonces": nonces, "jeton": jeton})
}

// nomBase suit la forme d'un identifiant de base Firestore : minuscules,
// chiffres et tirets, une lettre en tête, 4 à 63 caractères.
var nomBase = regexp.MustCompile(`^[a-z][a-z0-9-]{2,61}[a-z0-9]$`)

func (s *Serveur) firestore(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	base := q.Get("base")
	if base == "" {
		base = "essai"
	}
	if !nomBase.MatchString(base) {
		refuser(w, http.StatusBadRequest, "paramètre base hors de la forme d'un identifiant de base Firestore")
		return
	}
	var p ParametresDebit
	var duree int
	var err error
	for _, lu := range []struct {
		nom                 string
		cible               *int
		defaut, minimum, mx int
	}{
		{"duree", &duree, 60, 1, 600},
		{"debit", &p.Debit, 10, 1, 500},
		{"concurrence", &p.Concurrence, 16, 1, 64},
		{"taille", &p.Taille, 512, 0, 16384},
	} {
		if *lu.cible, err = Entier(q, lu.nom, lu.defaut, lu.minimum, lu.mx); err != nil {
			refuser(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	p.Duree = time.Duration(duree) * time.Second
	if !s.enCours.TryLock() {
		refuser(w, http.StatusConflict, "un essai de débit est déjà en cours sur cette VM")
		return
	}
	defer s.enCours.Unlock()
	ecrire, err := s.Ecrivain(r.Context(), base)
	if err != nil {
		refuser(w, http.StatusBadGateway, "client Firestore : "+err.Error())
		return
	}
	rap := MesurerDebit(r.Context(), p, ecrire)
	log.Printf("firestore : %d lancées, %d validées, %d rejouées, %d échecs, %d non lancées", rap.Lancees, rap.Validees, rap.Rejouees, rap.Echecs, rap.NonLancees)
	repondre(w, http.StatusOK, rap)
}
