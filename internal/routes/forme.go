package routes

import (
	"regexp"
	"sync"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// field est un champ d'un corps (API.md §1.7) : nom, facultatif ou non, et
// le contrôle de sa valeur ; nil n'est admis que si ce contrôle l'admet.
type field struct {
	name string
	opt  bool
	ok   func(any) bool
}

func req(name string, ok func(any) bool) field { return field{name, false, ok} }
func opt(name string, ok func(any) bool) field { return field{name, true, ok} }

// valid exige les champs obligatoires, refuse tout champ inconnu, et
// contrôle chaque valeur.
func valid(o jcs.Object, fields []field) bool {
	seen := 0
	for _, f := range fields {
		v, ok := o[f.name]
		if !ok && !f.opt || ok && !f.ok(v) {
			return false
		}
		if ok {
			seen++
		}
	}
	return seen == len(o)
}

var (
	reHex64   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	rePhone   = regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`)
	reDate    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	reFile    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	reDrawn   = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)
	reDigits  = regexp.MustCompile(`^[0-9]{4,10}$`)
	isSession = matches(regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`))
)

func isID(v any) bool { s, ok := v.(string); return ok && primitives.ID(s) }

func isText(v any) bool { s, ok := v.(string); return ok && s != "" }

func matches(re *regexp.Regexp) func(any) bool {
	return func(v any) bool { s, ok := v.(string); return ok && re.MatchString(s) }
}

// bytesN contrôle du base64 standard de n octets.
func bytesN(n int) func(any) bool {
	return func(v any) bool {
		s, ok := v.(string)
		b, err := primitives.B64(s)
		return ok && err == nil && len(b) == n
	}
}

// integer contrôle un entier JSON de lo à hi.
func integer(lo, hi int64) func(any) bool {
	return func(v any) bool {
		f, ok := v.(float64)
		return ok && f == float64(int64(f)) && f >= float64(lo) && f <= float64(hi)
	}
}

func oneOf(vals ...string) func(any) bool {
	return func(v any) bool {
		for _, s := range vals {
			if v == s {
				return true
			}
		}
		return false
	}
}

// arrayOf contrôle un tableau d'au plus n éléments, chacun contrôlé par ok.
func arrayOf(n int, ok func(any) bool) func(any) bool {
	return func(v any) bool {
		l, is := v.(jcs.Array)
		for _, e := range l {
			is = is && ok(e)
		}
		return is && len(l) <= n
	}
}

func nullable(ok func(any) bool) func(any) bool { return func(v any) bool { return v == nil || ok(v) } }

func isStamp(v any) bool { s, _ := v.(string); _, err := primitives.ReadTime(s); return err == nil }

func object(fields ...field) func(any) bool {
	return func(v any) bool { o, ok := v.(jcs.Object); return ok && valid(o, fields) }
}

const maxSafe = 1<<53 - 1

// Lectures de champs déjà contrôlés.
func str(o jcs.Object, k string) string     { s, _ := o[k].(string); return s }
func num(o jcs.Object, k string) int64      { f, _ := o[k].(float64); return int64(f) }
func raw(o jcs.Object, k string) []byte     { b, _ := primitives.B64(str(o, k)); return b }
func sub(o jcs.Object, k string) jcs.Object { m, _ := o[k].(jcs.Object); return m }

// limiter tient des fenêtres fixes par clé, au plus maxKeys clés : pleine
// sans clé expirée, elle refuse (échec fermé).
type limiter struct {
	mu   sync.Mutex
	wins map[string]*window
}

type window struct {
	start time.Time
	per   time.Duration
	count int
}

const maxKeys = 100_000

func newLimiter() *limiter { return &limiter{wins: map[string]*window{}} }

// hit décompte un appel pour key et dit s'il est sous la limite ; sinon,
// retry est la fin de la fenêtre.
func (l *limiter) hit(key string, lim limit, now time.Time) (ok bool, retry time.Time) {
	if lim.n == 0 {
		return true, time.Time{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.wins[key]
	if w == nil || !now.Before(w.start.Add(w.per)) {
		if w == nil && len(l.wins) >= maxKeys {
			for k, o := range l.wins {
				if !now.Before(o.start.Add(o.per)) {
					delete(l.wins, k)
				}
			}
			if len(l.wins) >= maxKeys {
				return false, now.Add(lim.per)
			}
		}
		w = &window{start: now, per: lim.per}
		l.wins[key] = w
	}
	w.count++
	return w.count <= lim.n, w.start.Add(w.per)
}
