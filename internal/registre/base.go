package registre

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// State est l'état d'une ligne lue : absente, valide, ou rejetée (sig
// fausse, ou ecritLe à plus de 2 minutes de l'update_time, §6.3).
type State int

const (
	Absent State = iota
	Valid
	Rejected
)

// Freshness est l'écart admis entre ecritLe et update_time (§6.3).
const Freshness = 2 * time.Minute

// ErrUnavailable est une lecture ou une écriture impossible du registre,
// du bucket des marqueurs, de KMS (registre_indisponible, API.md §6).
var ErrUnavailable = errors.New("registre_indisponible")

// Line est une ligne lue et vérifiée.
type Line struct {
	Path  string
	Doc   jcs.Object
	State State
}

// Registre écrit et lit le registre d'une instance : chaque document écrit
// porte ecritLe et sa sig par registre-sign, chaque écriture avance l'époque
// et la chaîne de chaine/tete ; chaque lecture vérifie sig et fraîcheur.
// Event reçoit les événements de borne (ligne_rejetee) ; lastEpoch est la
// dernière époque écrite par l'instance (contrôle anti-retour).
type Registre struct {
	Base     ports.Base
	KMS      ports.KMS
	Stockage ports.Stockage
	Horloge  ports.Horloge
	Event    func(kind, resource string)
	keys     [][]byte
	last     lastEpoch
}

// Load lit les clés publiques de toutes les versions activées de
// registre-sign (au démarrage seulement : une version ajoutée ensuite n'est
// acceptée qu'au démarrage suivant).
func (r *Registre) Load(ctx context.Context) error {
	keys, err := r.KMS.PublicKeys(ctx, ports.RegistreSign)
	if err != nil || len(keys) == 0 {
		return ErrUnavailable
	}
	for _, k := range keys {
		r.keys = append(r.keys, k)
	}
	return nil
}

// Masque rend les 8 premiers hexadécimaux de SHA-256(id) (§7).
func Masque(id string) string { return hex.EncodeToString(primitives.SHA256([]byte(id)))[:8] }

func (r *Registre) verify(d ports.Doc) Line {
	l := Line{Path: d.Path, Doc: d.Data}
	if d.Data == nil {
		return l
	}
	l.State = Rejected
	le, _ := d.Data["ecritLe"].(string)
	t, err := primitives.ReadTime(le)
	if err == nil && d.UpdateTime.Sub(t).Abs() <= Freshness && VerifyLine(d.Data, r.keys) {
		l.State = Valid
	} else if parts := strings.Split(d.Path, "/"); parts[0] == "titulaires" && len(parts) >= 2 && r.Event != nil {
		coll := parts[0] // titulaires/{uid}, ou titulaires/{uid}/<collection>/{id}
		if len(parts) >= 4 {
			coll = parts[2]
		}
		r.Event("ligne_rejetee", Masque(parts[1])+"/"+coll)
	}
	return l
}

// Read lit et vérifie un document hors écriture.
func (r *Registre) Read(ctx context.Context, path string) (Line, error) {
	var l Line
	err := r.Base.Transaction(ctx, func(tx ports.Tx) error {
		d, err := tx.Get(path)
		l = r.verify(d)
		return err
	})
	if err != nil {
		return Line{}, ErrUnavailable
	}
	return l, nil
}

// Writer est une écriture en cours : ses lectures vérifiées, puis les
// documents qu'elle écrit.
type Writer struct {
	tx   ports.Tx
	r    *Registre
	docs []Document
	Now  time.Time
}

// Get lit et vérifie un document dans la transaction.
func (w *Writer) Get(path string) (Line, error) {
	d, err := w.tx.Get(path)
	if err != nil {
		return Line{}, ErrUnavailable
	}
	return w.r.verify(d), nil
}

// List lit et vérifie les documents d'une collection (ports.Tx.List).
func (w *Writer) List(collection, field, after string, max int) ([]Line, error) {
	ds, err := w.tx.List(collection, field, after, max)
	if err != nil {
		return nil, ErrUnavailable
	}
	ls := make([]Line, len(ds))
	for i, d := range ds {
		ls[i] = w.r.verify(d)
	}
	return ls, nil
}

// Put inscrit un document entier, écrit à la validation.
func (w *Writer) Put(path string, doc jcs.Object) { w.docs = append(w.docs, Document{path, doc}) }

func (r *Registre) sign(ctx context.Context, doc jcs.Object, now time.Time) error {
	delete(doc, "sig")
	doc["ecritLe"] = now.UTC().Format(primitives.TimeLayout)
	msg, err := LineMessage(doc)
	if err != nil {
		return err
	}
	der, err := r.KMS.Sign(ctx, ports.RegistreSign, msg)
	if err != nil {
		return err
	}
	raw, err := jetons.DERToRaw(der)
	if err != nil {
		return err
	}
	doc["sig"] = base64.RawURLEncoding.EncodeToString(raw)
	return nil
}

// Write exécute f dans une transaction, puis, si f a inscrit des documents,
// les date, les signe, avance l'époque et la chaîne et réécrit chaine/tete ;
// tout est recalculé à chaque reprise de la transaction (§6.2). Une erreur
// de f est rendue telle quelle ; toute autre panne est ErrUnavailable.
func (r *Registre) Write(ctx context.Context, f func(w *Writer) error) error {
	var fErr error
	var epoch int64
	err := r.Base.Transaction(ctx, func(tx ports.Tx) error {
		fErr = nil
		d, err := tx.Get(Head)
		if err != nil {
			return err
		}
		prev := make([]byte, 32)
		epoch = 0
		if head := r.verify(d); head.State == Rejected {
			return ErrUnavailable
		} else if head.State == Valid {
			s, _ := head.Doc["empreinte"].(string)
			e, _ := head.Doc["epoque"].(float64)
			if prev, err = hex.DecodeString(s); err != nil || len(prev) != 32 {
				return ErrUnavailable
			}
			epoch = int64(e)
		}
		w := &Writer{tx: tx, r: r, Now: r.Horloge.Now()}
		if fErr = f(w); fErr != nil || len(w.docs) == 0 {
			return fErr
		}
		for _, d := range w.docs {
			if err := r.sign(ctx, d.Doc, w.Now); err != nil {
				return err
			}
		}
		epoch++
		sum, err := Next(prev, Write{epoch, w.docs})
		if err != nil {
			return err
		}
		head := jcs.Object{"epoque": epoch, "empreinte": hex.EncodeToString(sum)}
		if err := r.sign(ctx, head, w.Now); err != nil {
			return err
		}
		for _, d := range w.docs {
			tx.Set(d.Path, d.Doc)
		}
		tx.Set(Head, head)
		return nil
	})
	if fErr != nil {
		return fErr
	}
	if err != nil {
		return ErrUnavailable
	}
	r.last.raise(epoch)
	return nil
}

// HeadEpoch lit l'époque de chaine/tete (0 sur un registre vide).
func (r *Registre) HeadEpoch(ctx context.Context) (int64, jcs.Object, error) {
	l, err := r.Read(ctx, Head)
	if err != nil || l.State == Rejected {
		return 0, nil, ErrUnavailable
	}
	e, _ := l.Doc["epoque"].(float64)
	return int64(e), l.Doc, nil
}

// Marker lit le marqueur d'inscription de l'uid (§6.2) : sa cleViePub, ou nil
// sans marqueur.
func (r *Registre) Marker(ctx context.Context, uid string) ([]byte, error) {
	b, err := r.Stockage.Read(ctx, ports.BucketInscrits, markerName(uid))
	if err != nil {
		return nil, ErrUnavailable
	}
	if b == nil {
		return nil, nil
	}
	v, err := jcs.ReadCanonical(b)
	o, _ := v.(jcs.Object)
	s, _ := o["cleViePub"].(string)
	pub, err2 := primitives.B64(s)
	if err != nil || err2 != nil || len(o) != 2 || o["v"] != 1.0 || len(pub) != 32 {
		return nil, ErrUnavailable
	}
	return pub, nil
}

func markerName(uid string) string {
	return "inscrits/" + hex.EncodeToString(primitives.SHA256([]byte(uid)))
}

// CreateMarker crée le marqueur d'un premier enrôlement, en création seule ;
// un marqueur déjà là doit porter la même cleViePub.
func (r *Registre) CreateMarker(ctx context.Context, uid string, pub []byte) error {
	b, _ := jcs.Marshal(jcs.Object{"v": 1, "cleViePub": primitives.B64Encode(pub)})
	err := r.Stockage.Create(ctx, ports.BucketInscrits, markerName(uid), b)
	if errors.Is(err, ports.ErrExists) {
		if got, err := r.Marker(ctx, uid); err == nil && primitives.Equal(got, pub) {
			return nil
		}
	}
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
