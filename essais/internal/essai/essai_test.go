package essai

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResumer(t *testing.T) {
	if Resumer(nil) != nil {
		t.Fatal("série vide : nil attendu")
	}
	r := Resumer([]float64{5, 1, 3})
	if r.N != 3 || r.Min != 1 || r.Mediane != 3 || r.Max != 5 {
		t.Fatalf("série impaire : %+v", r)
	}
	if r := Resumer([]float64{4, 1, 3, 2}); r.Mediane != 2.5 {
		t.Fatalf("série paire : médiane %v, attendu 2,5", r.Mediane)
	}
	var cent []float64
	for i := 100; i >= 1; i-- {
		cent = append(cent, float64(i))
	}
	if r := Resumer(cent); r.P90 != 90 || r.P99 != 99 || r.Max != 100 {
		t.Fatalf("centiles au rang le plus proche : %+v", r)
	}
	if r := Resumer([]float64{7}); r.P90 != 7 || r.P99 != 7 || r.Mediane != 7 {
		t.Fatalf("une valeur : %+v", r)
	}
}

func TestEntier(t *testing.T) {
	q := url.Values{"a": {"12"}, "b": {"x"}, "c": {"0"}, "d": {"99"}}
	if n, err := Entier(q, "absent", 7, 1, 10); err != nil || n != 7 {
		t.Fatalf("absent : %d, %v", n, err)
	}
	if n, err := Entier(q, "a", 1, 1, 20); err != nil || n != 12 {
		t.Fatalf("lu : %d, %v", n, err)
	}
	for _, nom := range []string{"b", "c", "d"} {
		if _, err := Entier(q, nom, 1, 1, 20); err == nil {
			t.Errorf("%s : refus attendu", nom)
		}
	}
}

// serveurSNTP répond aux requêtes SNTP avec une horloge décalée de decalage
// sur celle du test ; modifier altère chaque réponse.
func serveurSNTP(t *testing.T, decalage time.Duration, modifier func([]byte)) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		b := make([]byte, 128)
		for {
			n, adr, err := pc.ReadFrom(b)
			if err != nil {
				return
			}
			if n != 48 || b[0] != 0x23 {
				continue
			}
			r := make([]byte, 48)
			r[0] = 0<<6 | 4<<3 | 4
			r[1] = 2
			copy(r[24:32], b[40:48])
			putNTP(r[32:40], time.Now().Add(decalage))
			putNTP(r[40:48], time.Now().Add(decalage))
			if modifier != nil {
				modifier(r)
			}
			_, _ = pc.WriteTo(r, adr)
		}
	}()
	return pc.LocalAddr().String()
}

func TestMesurerSNTP(t *testing.T) {
	adresse := serveurSNTP(t, 1500*time.Millisecond, nil)
	m, err := MesurerSNTP(context.Background(), adresse, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(m.EcartMs-1500) > 50 {
		t.Errorf("écart %v ms, attendu 1 500 ± 50", m.EcartMs)
	}
	if m.DelaiMs < 0 || m.DelaiMs > 50 || m.Strate != 2 {
		t.Errorf("délai %v ms ou strate %d inattendus", m.DelaiMs, m.Strate)
	}
	// Une VM qui avance donne un écart négatif.
	if m, err := MesurerSNTP(context.Background(), serveurSNTP(t, -2*time.Second, nil), time.Now); err != nil || math.Abs(m.EcartMs+2000) > 50 {
		t.Errorf("VM en avance : %+v, %v", m, err)
	}
}

func TestMesurerSNTPRefus(t *testing.T) {
	cas := map[string]func([]byte){
		"mode client":         func(r []byte) { r[0] = 4<<3 | 3 },
		"non synchronisé":     func(r []byte) { r[0] |= 3 << 6 },
		"strate nulle":        func(r []byte) { r[1] = 0 },
		"origine étrangère":   func(r []byte) { r[24] ^= 0xff },
		"réponse trop courte": nil,
	}
	for nom, modifier := range cas {
		t.Run(nom, func(t *testing.T) {
			if modifier == nil {
				if _, err := lireReponseSNTP(make([]byte, 47), make([]byte, 8), time.Now(), time.Now()); err == nil {
					t.Fatal("refus attendu")
				}
				return
			}
			if _, err := MesurerSNTP(context.Background(), serveurSNTP(t, 0, modifier), time.Now); err == nil {
				t.Fatal("refus attendu")
			}
		})
	}
}

func TestHorodatageNTP(t *testing.T) {
	b := make([]byte, 8)
	for _, h := range []time.Time{time.Unix(0, 0), time.Date(2036, 2, 7, 6, 28, 15, 999_999_000, time.UTC), time.Date(2026, 11, 11, 12, 0, 0, 123_456_789, time.UTC)} {
		putNTP(b, h)
		if d := getNTP(b).Sub(h); d < -time.Nanosecond || d > time.Nanosecond {
			t.Errorf("%v : aller-retour décalé de %v", h, d)
		}
	}
}

func TestVerifierDemande(t *testing.T) {
	bon := strings.Repeat("n", 43)
	if err := VerifierDemande("Fatoom-Heritage-Numerique/sesame", []string{bon, bon}); err != nil {
		t.Fatal(err)
	}
	refus := map[string]struct {
		audience string
		nonces   []string
	}{
		"audience vide":        {"", nil},
		"audience trop longue": {strings.Repeat("a", 513), nil},
		"sept nonces":          {"a", []string{bon, bon, bon, bon, bon, bon, bon}},
		"nonce de 9 octets":    {"a", []string{strings.Repeat("n", 9)}},
		"nonce de 75 octets":   {"a", []string{strings.Repeat("n", 75)}},
	}
	for nom, c := range refus {
		if err := VerifierDemande(c.audience, c.nonces); err == nil {
			t.Errorf("%s : refus attendu", nom)
		}
	}
	if err := VerifierDemande(strings.Repeat("a", 512), []string{strings.Repeat("n", 10), strings.Repeat("n", 74)}); err != nil {
		t.Errorf("bornes incluses : %v", err)
	}
}

// fauxLanceur sert la socket du lanceur dans un dossier court (un chemin de
// socket Unix est borné à 108 octets).
func fauxLanceur(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "lanceur")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "teeserver.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { srv.Close() })
	return socket
}

func TestJetonPKI(t *testing.T) {
	n1, n2 := strings.Repeat("a", 43), strings.Repeat("b", 43)
	socket := fauxLanceur(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/token" || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "route", http.StatusNotFound)
			return
		}
		var d map[string]any
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if d["token_type"] != "PKI" || d["audience"] != "aud-essai" || fmt.Sprint(d["nonces"]) != fmt.Sprint([]any{n1, n2}) || len(d) != 3 {
			http.Error(w, "demande inattendue", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, "eyJh.eyJi.c2ln\n")
	})
	jeton, err := NouveauLanceur(socket).JetonPKI(context.Background(), "aud-essai", []string{n1, n2})
	if err != nil || jeton != "eyJh.eyJi.c2ln" {
		t.Fatalf("jeton %q, %v", jeton, err)
	}
}

func TestJetonPKIRefus(t *testing.T) {
	socket := fauxLanceur(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("forme") != "" {
			_, _ = io.WriteString(w, `{"pas":"un jeton"}`)
			return
		}
		http.Error(w, "audience refusée", http.StatusBadRequest)
	})
	l := NouveauLanceur(socket)
	_, err := l.JetonPKI(context.Background(), "aud-essai", nil)
	var el ErreurLanceur
	if !errors.As(err, &el) || el.Statut != http.StatusBadRequest || !strings.Contains(el.Corps, "audience refusée") {
		t.Fatalf("refus du lanceur attendu, obtenu %v", err)
	}
	if _, err := l.JetonPKI(context.Background(), "", nil); !errors.As(err, new(ErreurDemande)) {
		t.Fatalf("demande hors bornes : %v", err)
	}
	if _, err := NouveauLanceur(filepath.Join(t.TempDir(), "absente.sock")).JetonPKI(context.Background(), "a", nil); err == nil {
		t.Fatal("socket absente : erreur attendue")
	}
}

func TestMetadonnees(t *testing.T) {
	var appels atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			http.Error(w, "en-tête", http.StatusForbidden)
			return
		}
		appels.Add(1)
		switch r.URL.Path {
		case "/computeMetadata/v1/project/project-id":
			_, _ = io.WriteString(w, "projet-essai")
		case "/computeMetadata/v1/instance/service-accounts/default/token":
			_, _ = io.WriteString(w, `{"access_token":"jeton-de-test","expires_in":3599,"token_type":"Bearer"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := NouvellesMetadonnees(srv.URL + "/computeMetadata/v1")
	if p, err := m.Projet(context.Background()); err != nil || p != "projet-essai" {
		t.Fatalf("projet %q, %v", p, err)
	}
	for i := 0; i < 3; i++ {
		if j, err := m.Jeton(context.Background()); err != nil || j != "jeton-de-test" {
			t.Fatalf("jeton %q, %v", j, err)
		}
	}
	if appels.Load() != 2 {
		t.Fatalf("%d appels, attendu 2 : le jeton est gardé jusqu'à son expiration", appels.Load())
	}
}

// fauxFirestore tient chaine/tete et les lignes en mémoire, avec la
// concurrence optimiste : un commit dont la transaction a lu une tête depuis
// réécrite est rejeté (409 ABORTED).
type fauxFirestore struct {
	t         *testing.T
	mu        sync.Mutex
	epoque    int64
	empreinte string
	lignes    map[string]bool
	lu        map[string]int64 // époque lue par chaque transaction
	suivante  int
	abandons  int
	toujours  string // si non vide : état d'erreur rendu à chaque commit
}

// projetTest est le projet du faux Firestore (concaténé : jamais écrit
// après « projects/ », contrôle des noms de ressources).
const (
	projetTest = "projet-essai"
	racineTest = "/v1/projects/" + projetTest + "/databases/essai/documents"
)

func (f *fauxFirestore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer jeton-de-test" {
		erreurAPI(w, http.StatusUnauthorized, "UNAUTHENTICATED")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == racineTest+":beginTransaction":
		f.suivante++
		tx := base64.StdEncoding.EncodeToString([]byte("tx/" + strconv.Itoa(f.suivante) + "?"))
		f.lu[tx] = -1
		_ = json.NewEncoder(w).Encode(map[string]string{"transaction": tx})
	case r.Method == http.MethodGet && r.URL.Path == racineTest+"/chaine/tete":
		tx := r.URL.Query().Get("transaction")
		if _, ok := f.lu[tx]; !ok {
			erreurAPI(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		f.lu[tx] = f.epoque
		if f.epoque == 0 {
			erreurAPI(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{
			"epoque":    map[string]string{"integerValue": strconv.FormatInt(f.epoque, 10)},
			"empreinte": map[string]string{"stringValue": f.empreinte},
		}})
	case r.Method == http.MethodPost && r.URL.Path == racineTest+":rollback":
		f.abandons++
	case r.Method == http.MethodPost && r.URL.Path == racineTest+":commit":
		f.commit(w, r)
	default:
		erreurAPI(w, http.StatusNotFound, "NOT_FOUND")
	}
}

func (f *fauxFirestore) commit(w http.ResponseWriter, r *http.Request) {
	var c struct {
		Transaction string `json:"transaction"`
		Writes      []struct {
			Update struct {
				Name   string                       `json:"name"`
				Fields map[string]map[string]string `json:"fields"`
			} `json:"update"`
			CurrentDocument *struct {
				Exists bool `json:"exists"`
			} `json:"currentDocument"`
		} `json:"writes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil || len(c.Writes) != 2 {
		erreurAPI(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if f.toujours != "" {
		erreurAPI(w, http.StatusConflict, f.toujours)
		return
	}
	lu, ok := f.lu[c.Transaction]
	delete(f.lu, c.Transaction)
	if !ok || lu != f.epoque {
		erreurAPI(w, http.StatusConflict, "ABORTED")
		return
	}
	tete, ligne := c.Writes[0], c.Writes[1]
	if tete.Update.Name != racineTest[len("/v1/"):]+"/chaine/tete" || tete.CurrentDocument != nil ||
		ligne.CurrentDocument == nil || ligne.CurrentDocument.Exists {
		f.t.Errorf("écritures inattendues : %+v", c.Writes)
		erreurAPI(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if f.lignes[ligne.Update.Name] {
		erreurAPI(w, http.StatusConflict, "ALREADY_EXISTS")
		return
	}
	n, _ := strconv.ParseInt(tete.Update.Fields["epoque"]["integerValue"], 10, 64)
	if n != f.epoque+1 || ligne.Update.Fields["epoque"]["integerValue"] != tete.Update.Fields["epoque"]["integerValue"] {
		f.t.Errorf("époque %d après %d", n, f.epoque)
	}
	f.lignes[ligne.Update.Name] = true
	f.epoque = n
	f.empreinte = tete.Update.Fields["empreinte"]["stringValue"]
	_, _ = io.WriteString(w, `{"commitTime":"2026-11-11T00:00:00Z"}`)
}

func erreurAPI(w http.ResponseWriter, statut int, etat string) {
	w.WriteHeader(statut)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": statut, "status": etat}})
}

func nouveauFaux(t *testing.T) (*fauxFirestore, *Firestore) {
	f := &fauxFirestore{t: t, lignes: map[string]bool{}, lu: map[string]int64{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	jeton := func(context.Context) (string, error) { return "jeton-de-test", nil }
	return f, NouveauFirestore(srv.URL+"/v1", "projet-essai", "essai", jeton)
}

func TestTransactionChaine(t *testing.T) {
	f, fs := nouveauFaux(t)
	attendue := make([]byte, 32)
	for i := int64(1); i <= 3; i++ {
		n, err := fs.Transaction(context.Background(), 16)
		if err != nil || n != 1 {
			t.Fatalf("transaction %d : %d tentatives, %v", i, n, err)
		}
		h := sha256.New()
		h.Write(attendue)
		fmt.Fprintf(h, `{"documents":[{"chemin":"lignes/e%020d"}],"epoque":%d}`, i, i)
		attendue = h.Sum(nil)
	}
	if f.epoque != 3 || f.empreinte != hex.EncodeToString(attendue) || len(f.lignes) != 3 {
		t.Fatalf("tête %d %s, %d lignes", f.epoque, f.empreinte, len(f.lignes))
	}
}

func TestTransactionRejetee(t *testing.T) {
	f, fs := nouveauFaux(t)
	f.toujours = "ABORTED"
	n, err := fs.Transaction(context.Background(), 0)
	if n != EssaisMax || Cle(err) != "409 ABORTED" {
		t.Fatalf("%d tentatives, %v", n, err)
	}
	f.toujours = "ALREADY_EXISTS"
	if n, err := fs.Transaction(context.Background(), 0); n != 1 || Cle(err) != "409 ALREADY_EXISTS" {
		t.Fatalf("collision : %d tentatives, %v", n, err)
	}
}

func TestMesurerDebit(t *testing.T) {
	f, fs := nouveauFaux(t)
	p := ParametresDebit{Duree: 500 * time.Millisecond, Debit: 200, Concurrence: 8, Taille: 64}
	r := MesurerDebit(context.Background(), p, fs.Transaction)
	if r.Lancees == 0 || r.Validees+r.Echecs != r.Lancees {
		t.Fatalf("comptes incohérents : %+v", r)
	}
	if int64(r.Validees) != f.epoque || r.Collisions != 0 {
		t.Fatalf("%d validées pour une tête à %d, %d collisions", r.Validees, f.epoque, r.Collisions)
	}
	if r.LatenceMs == nil || r.LatenceMs.N != r.Validees || r.DebitTenu <= 0 || r.EssaisParValidee < 1 {
		t.Fatalf("résumé : %+v", r)
	}
}

func TestMesurerDebitComptes(t *testing.T) {
	var n atomic.Int32
	ecrire := func(ctx context.Context, _ int) (int, error) {
		switch n.Add(1) % 3 {
		case 0:
			return 2, nil
		case 1:
			return 1, &ErreurAPI{Statut: 409, Etat: "ALREADY_EXISTS"}
		default:
			time.Sleep(30 * time.Millisecond) // garde la seule place prise
			return EssaisMax, &ErreurAPI{Statut: 409, Etat: "ABORTED"}
		}
	}
	r := MesurerDebit(context.Background(), ParametresDebit{Duree: 300 * time.Millisecond, Debit: 100, Concurrence: 1}, ecrire)
	if r.NonLancees == 0 {
		t.Error("une seule place occupée 30 ms à 100 départs par seconde : des départs non lancés attendus")
	}
	if r.Collisions == 0 || r.Erreurs["409 ALREADY_EXISTS"] != r.Collisions || r.Erreurs["409 ABORTED"] == 0 || r.Rejouees == 0 {
		t.Errorf("comptes : %+v", r)
	}
}

func TestServeur(t *testing.T) {
	_, fs := nouveauFaux(t)
	var heures atomic.Int32
	s := &Serveur{
		Heure: func(_ context.Context, adresse string) (Mesure, error) {
			if adresse != ServeurHeure && adresse != ServeurHeureIP {
				t.Errorf("adresse %s", adresse)
			}
			if heures.Add(1) == 2 {
				return Mesure{}, errors.New("délai")
			}
			return Mesure{EcartMs: 1, DelaiMs: 0.5, Strate: 2}, nil
		},
		Jeton: func(_ context.Context, audience string, nonces []string) (string, error) {
			if audience == "refus" {
				return "", ErreurLanceur{Statut: 400, Corps: "non"}
			}
			return "a.b.c", nil
		},
		Ecrivain: func(context.Context, string) (Ecrivain, error) { return fs.Transaction, nil },
	}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	lire := func(chemin string, statut int) map[string]any {
		t.Helper()
		rep, err := http.Get(srv.URL + chemin)
		if err != nil {
			t.Fatal(err)
		}
		defer rep.Body.Close()
		var v map[string]any
		if err := json.NewDecoder(rep.Body).Decode(&v); err != nil {
			t.Fatal(err)
		}
		if rep.StatusCode != statut {
			t.Fatalf("%s : statut %d, attendu %d (%v)", chemin, rep.StatusCode, statut, v)
		}
		return v
	}
	if v := lire("/horloge?mesures=3&intervalle=0", 200); v["reussies"] != 2.0 || v["erreurs"].(map[string]any)["délai"] != 1.0 {
		t.Errorf("horloge : %v", v)
	}
	lire("/horloge?mesures=65", 400)
	lire("/horloge?reference=autre", 400)
	if v := lire("/horloge?mesures=1&reference=ip", 200); v["serveur"] != ServeurHeureIP {
		t.Errorf("horloge par adresse IP : %v", v)
	}
	n := strings.Repeat("n", 43)
	if v := lire("/attestation?audience=aud&nonce="+n+"&nonce="+n, 200); v["jeton"] != "a.b.c" {
		t.Errorf("attestation : %v", v)
	}
	lire("/attestation?audience=aud&nonce=court", 400)
	if v := lire("/attestation?audience=refus", 502); v["statut"] != 400.0 {
		t.Errorf("refus du lanceur : %v", v)
	}
	lire("/firestore?base=Essai", 400)
	lire("/firestore?duree=601", 400)
	if v := lire("/firestore?duree=1&debit=20&concurrence=2&taille=8", 200); v["validees"].(float64) < 1 {
		t.Errorf("firestore : %v", v)
	}
	lire("/", 200)
}
