package essai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// APIFirestore est la racine de l'API REST de Firestore. L'essai l'appelle
// par net/http, avec le jeton du compte de service de la VM, sans SDK : le
// module du SDK n'est pas dans la liste d'ARCHITECTURE.md §2.4 (décision
// 0075).
const APIFirestore = "https://firestore.googleapis.com/v1"

// EssaisMax est le nombre de tentatives d'une transaction rejetée par
// Firestore (ABORTED) avant de la compter en échec.
const EssaisMax = 5

// ErreurAPI est une réponse de Firestore autre que 200.
type ErreurAPI struct {
	Statut int
	Etat   string // champ error.status de la réponse (ABORTED, ALREADY_EXISTS…)
}

func (e *ErreurAPI) Error() string { return fmt.Sprintf("firestore : %d %s", e.Statut, e.Etat) }

// Cle rend la clé de comptage d'une erreur dans un rapport.
func Cle(err error) string {
	var e *ErreurAPI
	if errors.As(err, &e) {
		return strconv.Itoa(e.Statut) + " " + e.Etat
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "délai dépassé"
	}
	return "réseau"
}

// Firestore écrit le document unique chaine/tete d'une base, comme le
// registre (spécification de l'opérateur, registre, « Époque et chaîne ») :
// chaque transaction relit la tête, la réécrit (époque + 1, empreinte) et
// crée un document de ligne.
type Firestore struct {
	documents string // …/projects/<p>/databases/<d>/documents
	jeton     func(context.Context) (string, error)
	client    *http.Client
}

// NouveauFirestore rend un client de la base base du projet projet.
func NouveauFirestore(api, projet, base string, jeton func(context.Context) (string, error)) *Firestore {
	return &Firestore{
		documents: strings.TrimSuffix(api, "/") + "/projects/" + projet + "/databases/" + base + "/documents",
		jeton:     jeton,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (f *Firestore) appel(ctx context.Context, methode, url string, corps, sortie any) error {
	var lecteur io.Reader
	if corps != nil {
		b, err := json.Marshal(corps)
		if err != nil {
			return err
		}
		lecteur = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, methode, url, lecteur)
	if err != nil {
		return err
	}
	jeton, err := f.jeton(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+jeton)
	req.Header.Set("Content-Type", "application/json")
	rep, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer rep.Body.Close()
	b, err := io.ReadAll(io.LimitReader(rep.Body, 1<<20))
	if err != nil {
		return err
	}
	if rep.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Status string `json:"status"`
			} `json:"error"`
		}
		_ = json.Unmarshal(b, &e)
		return &ErreurAPI{Statut: rep.StatusCode, Etat: e.Error.Status}
	}
	if sortie != nil {
		return json.Unmarshal(b, sortie)
	}
	return nil
}

type valeur map[string]any

type document struct {
	Name   string            `json:"name,omitempty"`
	Fields map[string]valeur `json:"fields"`
}

// lireTete rend l'époque et l'empreinte de chaine/tete, ou l'époque 0 et
// 32 octets nuls si la tête n'existe pas encore.
func (f *Firestore) lireTete(ctx context.Context, transaction string) (int64, []byte, error) {
	var d document
	err := f.appel(ctx, http.MethodGet, f.documents+"/chaine/tete?transaction="+urlBase64(transaction), nil, &d)
	var e *ErreurAPI
	if errors.As(err, &e) && e.Statut == http.StatusNotFound {
		return 0, make([]byte, 32), nil
	}
	if err != nil {
		return 0, nil, err
	}
	brut, _ := d.Fields["epoque"]["integerValue"].(string)
	epoque, err := strconv.ParseInt(brut, 10, 64)
	if err != nil {
		return 0, nil, errors.New("chaine/tete : époque illisible")
	}
	hexa, _ := d.Fields["empreinte"]["stringValue"].(string)
	empreinte, err := hex.DecodeString(hexa)
	if err != nil || len(empreinte) != 32 {
		return 0, nil, errors.New("chaine/tete : empreinte illisible")
	}
	return epoque, empreinte, nil
}

// urlBase64 échappe les caractères d'une chaîne base64 standard qu'une
// requête ne transporte pas tels quels.
func urlBase64(s string) string {
	return strings.NewReplacer("+", "%2B", "/", "%2F", "=", "%3D").Replace(s)
}

// Transaction fait une écriture du registre d'essai : relit chaine/tete, la
// réécrit et crée lignes/e<époque> (création seule), dans une transaction
// rejouée en entier si Firestore la rejette (ABORTED), au plus EssaisMax
// fois. Elle rend le nombre de tentatives.
func (f *Firestore) Transaction(ctx context.Context, taille int) (int, error) {
	contenu := strings.Repeat("x", taille)
	var err error
	for essai := 1; essai <= EssaisMax; essai++ {
		err = f.tentative(ctx, contenu)
		var e *ErreurAPI
		if !errors.As(err, &e) || e.Etat != "ABORTED" {
			return essai, err
		}
	}
	return EssaisMax, err
}

func (f *Firestore) tentative(ctx context.Context, contenu string) error {
	var tx struct {
		Transaction string `json:"transaction"`
	}
	ouverture := map[string]any{"options": map[string]any{"readWrite": map[string]any{}}}
	if err := f.appel(ctx, http.MethodPost, f.documents+":beginTransaction", ouverture, &tx); err != nil {
		return err
	}
	epoque, precedente, err := f.lireTete(ctx, tx.Transaction)
	if err != nil {
		f.abandonner(tx.Transaction)
		return err
	}
	epoque++
	h := sha256.New()
	h.Write(precedente)
	fmt.Fprintf(h, `{"documents":[{"chemin":"lignes/e%020d"}],"epoque":%d}`, epoque, epoque)
	empreinte := hex.EncodeToString(h.Sum(nil))
	racine := f.documents[strings.Index(f.documents, "/projects/")+1:]
	ecriture := map[string]any{
		"transaction": tx.Transaction,
		"writes": []any{
			map[string]any{"update": document{
				Name: racine + "/chaine/tete",
				Fields: map[string]valeur{
					"epoque":    {"integerValue": strconv.FormatInt(epoque, 10)},
					"empreinte": {"stringValue": empreinte},
				},
			}},
			map[string]any{
				"update": document{
					Name: fmt.Sprintf("%s/lignes/e%020d", racine, epoque),
					Fields: map[string]valeur{
						"epoque":  {"integerValue": strconv.FormatInt(epoque, 10)},
						"ecritLe": {"timestampValue": time.Now().UTC().Format(time.RFC3339Nano)},
						"contenu": {"stringValue": contenu},
					},
				},
				"currentDocument": map[string]any{"exists": false},
			},
		},
	}
	return f.appel(ctx, http.MethodPost, f.documents+":commit", ecriture, nil)
}

// abandonner libère une transaction ouverte (au mieux, sans attendre).
func (f *Firestore) abandonner(transaction string) {
	ctx, annuler := context.WithTimeout(context.Background(), 10*time.Second)
	defer annuler()
	_ = f.appel(ctx, http.MethodPost, f.documents+":rollback", map[string]any{"transaction": transaction}, nil)
}

// ParametresDebit règle un essai de débit.
type ParametresDebit struct {
	Duree       time.Duration
	Debit       int // transactions lancées par seconde
	Concurrence int // transactions en vol au plus
	Taille      int // octets du contenu de chaque ligne
}

// RapportDebit est le résultat d'un essai de débit, rendu en JSON.
type RapportDebit struct {
	Duree            float64        `json:"duree"`
	DebitVise        int            `json:"debitVise"`
	Concurrence      int            `json:"concurrence"`
	Taille           int            `json:"taille"`
	Lancees          int            `json:"lancees"`
	NonLancees       int            `json:"nonLancees"`
	Validees         int            `json:"validees"`
	Rejouees         int            `json:"rejouees"`
	Echecs           int            `json:"echecs"`
	Collisions       int            `json:"collisions"`
	Erreurs          map[string]int `json:"erreurs"`
	DebitTenu        float64        `json:"debitTenu"`
	EssaisParValidee float64        `json:"essaisParValidee"`
	LatenceMs        *Resume        `json:"latenceMs"`
	Debut            time.Time      `json:"debut"`
	Fin              time.Time      `json:"fin"`
}

// Ecrivain fait une transaction et rend le nombre de tentatives.
type Ecrivain func(ctx context.Context, taille int) (int, error)

// MesurerDebit lance des transactions au débit visé pendant la durée, sans
// dépasser la concurrence : un départ qui trouve toutes les places prises
// n'est pas lancé et compte dans NonLancees (le débit visé n'est alors pas
// tenu par ce client).
func MesurerDebit(ctx context.Context, p ParametresDebit, ecrire Ecrivain) RapportDebit {
	r := RapportDebit{DebitVise: p.Debit, Concurrence: p.Concurrence, Taille: p.Taille, Erreurs: map[string]int{}, Debut: time.Now().UTC()}
	var (
		mu       sync.Mutex
		latences []float64
		essais   int
		enVol    sync.WaitGroup
	)
	places := make(chan struct{}, p.Concurrence)
	rythme := time.NewTicker(time.Second / time.Duration(p.Debit))
	defer rythme.Stop()
	fin := time.NewTimer(p.Duree)
	defer fin.Stop()
	debut := time.Now()
boucle:
	for {
		select {
		case <-ctx.Done():
			break boucle
		case <-fin.C:
			break boucle
		case <-rythme.C:
		}
		select {
		case places <- struct{}{}:
		default:
			r.NonLancees++
			continue
		}
		r.Lancees++
		enVol.Add(1)
		go func() {
			defer enVol.Done()
			defer func() { <-places }()
			t := time.Now()
			n, err := ecrire(ctx, p.Taille)
			d := ms(time.Since(t))
			mu.Lock()
			defer mu.Unlock()
			essais += n
			if n > 1 {
				r.Rejouees += n - 1
			}
			if err != nil {
				r.Echecs++
				cle := Cle(err)
				r.Erreurs[cle]++
				if strings.HasSuffix(cle, " ALREADY_EXISTS") {
					r.Collisions++
				}
				return
			}
			r.Validees++
			latences = append(latences, d)
		}()
	}
	enVol.Wait()
	ecoule := time.Since(debut)
	r.Fin = time.Now().UTC()
	r.Duree = ecoule.Seconds()
	if ecoule > 0 {
		r.DebitTenu = float64(r.Validees) / ecoule.Seconds()
	}
	if r.Validees > 0 {
		r.EssaisParValidee = float64(essais) / float64(r.Validees)
	}
	r.LatenceMs = Resumer(latences)
	return r
}
