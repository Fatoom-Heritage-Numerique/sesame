// Paquet registre : formats du registre de Sésame et de sa borne publique
// (ARCHITECTURE.md §6.2, §6.3, §8.1 ; décision 0021) : sig d'une ligne par
// registre-sign, JSON canonique d'une écriture et chaîne d'empreintes depuis
// l'époque 0, borne signée par session-sign, et contrôle anti-retour sur
// l'époque de « la dernière borne ». L'heure est un paramètre ; aucune E/S.
package registre

import (
	"encoding/hex"
	"regexp"
	"slices"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/canal"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

var (
	errInvalid = primitives.ErrInvalid
	reHex64    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reDigest   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// Head est le chemin du document de tête de la chaîne.
const Head = "chaine/tete"

// LineMessage rend "fatoom-sesame-registre|1|" ‖ JSON canonique de la ligne
// sans sig (§6).
func LineMessage(doc jcs.Object) ([]byte, error) {
	body := jcs.Object{}
	for k, v := range doc {
		if k != "sig" {
			body[k] = v
		}
	}
	j, err := jcs.Marshal(body)
	if err != nil {
		return nil, err
	}
	return append([]byte("fatoom-sesame-registre|1|"), j...), nil
}

// verifyAny vérifie une signature R ‖ S en base64url sans remplissage par
// l'une des clés P-256 données.
func verifyAny(msg []byte, sig any, keys [][]byte) bool {
	s, _ := sig.(string)
	raw, err := primitives.B64URL(s)
	if err != nil || len(raw) != 64 {
		return false
	}
	for _, k := range keys {
		if ok, _ := primitives.P256Verify(k, msg, raw); ok {
			return true
		}
	}
	return false
}

// VerifyLine vérifie la sig d'une ligne par l'une des versions activées de
// registre-sign ; toute autre forme de sig est fausse.
func VerifyLine(doc jcs.Object, keys [][]byte) bool {
	msg, err := LineMessage(doc)
	return err == nil && verifyAny(msg, doc["sig"], keys)
}

// Document est un document écrit par une transaction ; Doc nil vaut une
// suppression.
type Document struct {
	Path string
	Doc  jcs.Object
}

// Write est une écriture du registre : une transaction et sa nouvelle époque.
type Write struct {
	Epoch     int64
	Documents []Document
}

// JSON rend {"documents": [{"chemin","doc"}…], "epoque"} en JSON canonique,
// documents triés par chemin (unités UTF-16), chaine/tete exclu, sans chemin
// en double.
func (w Write) JSON() ([]byte, error) {
	docs := slices.Clone(w.Documents)
	slices.SortFunc(docs, func(a, b Document) int {
		switch {
		case jcs.LessUTF16(a.Path, b.Path):
			return -1
		case jcs.LessUTF16(b.Path, a.Path):
			return 1
		}
		return 0
	})
	list := make(jcs.Array, len(docs))
	for i, d := range docs {
		if d.Path == "" || d.Path == Head || (i > 0 && docs[i-1].Path == d.Path) {
			return nil, errInvalid
		}
		var doc any
		if d.Doc != nil {
			doc = d.Doc
		}
		list[i] = jcs.Object{"chemin": d.Path, "doc": doc}
	}
	if w.Epoch < 1 || len(list) == 0 {
		return nil, errInvalid
	}
	return jcs.Marshal(jcs.Object{"epoque": w.Epoch, "documents": list})
}

// Next rend SHA-256(empreinte précédente ‖ JSON canonique de l'écriture).
func Next(prev []byte, w Write) ([]byte, error) {
	j, err := w.JSON()
	if err != nil || len(prev) != 32 {
		return nil, errInvalid
	}
	return primitives.SHA256(prev, j), nil
}

// VerifyChain rejoue la chaîne depuis l'époque 0 (32 octets nuls) : époques
// 1, 2, … sans trou, sig de chaque document écrit, puis tête (epoque,
// empreinte en hexadécimal, sig) égale au résultat.
func VerifyChain(writes []Write, head jcs.Object, keys [][]byte) bool {
	sum := make([]byte, 32)
	for i, w := range writes {
		if w.Epoch != int64(i+1) {
			return false
		}
		for _, d := range w.Documents {
			if d.Doc != nil && !VerifyLine(d.Doc, keys) {
				return false
			}
		}
		var err error
		if sum, err = Next(sum, w); err != nil {
			return false
		}
	}
	s, _ := head["empreinte"].(string)
	got, err := hex.DecodeString(s)
	return head["epoque"] == float64(len(writes)) && reHex64.MatchString(s) && err == nil &&
		primitives.Equal(got, sum) && VerifyLine(head, keys)
}

// Événements d'une borne (§8.1).
var eventTypes = []string{"ligne_rejetee", "ligne_absente", "reprise", "alerte_refusee", "registre_ancien"}

func exact(v any, keys ...string) jcs.Object {
	o, _ := v.(jcs.Object)
	if len(o) != len(keys) {
		return nil
	}
	for _, k := range keys {
		if _, ok := o[k]; !ok {
			return nil
		}
	}
	return o
}

func text(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok && s != ""
}

func stamp(v any) (time.Time, bool) {
	s, _ := v.(string)
	t, err := primitives.ReadTime(s)
	return t, err == nil
}

func natural(v any) bool {
	f, ok := v.(float64)
	return ok && f >= 0 && f == float64(int64(f)) && f <= 1<<53
}

// CheckBorne contrôle la forme d'une borne : ses huit champs exacts,
// instanceId, imageDigest, borneLe, grappe ({grappeId: null, votants: 0} sans
// grappe), registre {epoque, empreinte}, événements typés.
func CheckBorne(v any) (jcs.Object, error) {
	b := exact(v, "v", "instanceId", "imageDigest", "gceInstanceId", "borneLe", "grappe", "registre", "evenements")
	g := exact(b["grappe"], "grappeId", "votants")
	r := exact(b["registre"], "epoque", "empreinte")
	id, _ := b["instanceId"].(string)
	_, errID := canal.ParseInstanceID(id)
	digest, _ := b["imageDigest"].(string)
	_, okGCE := text(b["gceInstanceId"])
	_, okLe := stamp(b["borneLe"])
	grappe, _ := g["grappeId"].(string)
	sum, _ := r["empreinte"].(string)
	events, okEv := b["evenements"].(jcs.Array)
	ok := b != nil && g != nil && r != nil && b["v"] == 1.0 && errID == nil && reDigest.MatchString(digest) && okGCE && okLe &&
		natural(g["votants"]) && ((g["grappeId"] == nil && g["votants"] == 0.0) || reHex64.MatchString(grappe)) &&
		natural(r["epoque"]) && reHex64.MatchString(sum) && okEv
	for _, e := range events {
		o := exact(e, "type", "ressource", "vuLe")
		t, _ := o["type"].(string)
		_, okRes := text(o["ressource"])
		_, okVu := stamp(o["vuLe"])
		ok = ok && o != nil && slices.Contains(eventTypes, t) && okRes && okVu
	}
	if !ok {
		return nil, errInvalid
	}
	return b, nil
}

// BorneMessage rend "fatoom-sesame-borne|1|" ‖ instanceId ‖ JSON canonique.
func BorneMessage(v any) ([]byte, error) {
	b, err := CheckBorne(v)
	if err != nil {
		return nil, err
	}
	j, err := jcs.Marshal(b)
	if err != nil {
		return nil, err
	}
	return append([]byte("fatoom-sesame-borne|1|"+b["instanceId"].(string)), j...), nil
}

// BorneName rend bornes/AAAA/MM/JJ/HH-mm-<instanceId>.json depuis borneLe.
func BorneName(v any) (string, error) {
	b, err := CheckBorne(v)
	if err != nil {
		return "", err
	}
	t, _ := stamp(b["borneLe"])
	return "bornes/" + t.Format("2006/01/02/15-04-") + b["instanceId"].(string) + ".json", nil
}

// VerifyBorne vérifie un fichier {borne, signature, kid} déposé sous name :
// nom conforme à borneLe et à l'instanceId, signature R ‖ S par la clé de
// session-sign de son kid (toute version activée), sur le message de domaine.
func VerifyBorne(name string, file any, keys map[string][]byte) bool {
	f := exact(file, "borne", "signature", "kid")
	kid, _ := f["kid"].(string)
	key, okKey := keys[kid]
	msg, err := BorneMessage(f["borne"])
	n, errName := BorneName(f["borne"])
	return f != nil && okKey && err == nil && errName == nil && n == name && verifyAny(msg, f["signature"], [][]byte{key})
}

// Deposited est une borne lue au bucket d'audit, avec le timeCreated que
// Cloud Storage a posé.
type Deposited struct {
	Name    string
	File    any
	Created time.Time
}

// Fenêtres du contrôle anti-retour (§6.3, §8.1).
const (
	window   = 3 * time.Hour
	maxAge   = 2 * time.Hour
	maxDrift = 5 * time.Minute
)

// Reference rend l'époque de « la dernière borne » à now : la plus haute des
// bornes valides déposées dans les 3 dernières heures (timeCreated), borneLe à
// moins de 5 minutes de timeCreated, hors bornes qui portent registre_ancien ;
// ok est faux s'il n'y en a aucune ou si la plus récente a plus de 2 heures.
func Reference(bornes []Deposited, keys map[string][]byte, now time.Time) (epoch int64, ok bool) {
	var newest time.Time
	for _, d := range bornes {
		if d.Created.After(now) || now.Sub(d.Created) > window || !VerifyBorne(d.Name, d.File, keys) {
			continue
		}
		b := exact(d.File, "borne", "signature", "kid")["borne"].(jcs.Object)
		le, _ := stamp(b["borneLe"])
		stale := false
		for _, e := range b["evenements"].(jcs.Array) {
			stale = stale || e.(jcs.Object)["type"] == "registre_ancien"
		}
		if le.Sub(d.Created).Abs() > maxDrift || stale {
			continue
		}
		epoch = max(epoch, int64(b["registre"].(jcs.Object)["epoque"].(float64)))
		if !ok || d.Created.After(newest) {
			newest = d.Created
		}
		ok = true
	}
	if !ok || now.Sub(newest) > maxAge {
		return 0, false
	}
	return epoch, true
}

// UpToDate dit si le registre lu (epoque de chaine/tete) n'est ni plus ancien
// que la référence, ni que la dernière époque écrite par l'instance (§6.3).
func UpToDate(read, reference int64, ok bool, lastWritten int64) bool {
	return ok && read >= reference && read >= lastWritten
}
