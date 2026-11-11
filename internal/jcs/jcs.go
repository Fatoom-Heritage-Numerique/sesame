// Paquet jcs : JSON canonique RFC 8785 (JCS) de tous les messages signés, des
// AAD qui portent un JSON, des lignes et de la chaîne du registre, de la borne
// et d'env_override (ARCHITECTURE.md §10, §6.2, §8.1 ; vecteurs json.json).
// Clés triées par unités UTF-16, sans espace, nombres sérialisés comme
// ECMAScript, échappements de la RFC, UTF-8 ; une chaîne ou une clé hors NFC,
// une demi-paire de substitution isolée, un nombre non fini, une clé en double
// et toute valeur qui n'est pas du JSON sont refusés (ErrInvalid). Aucune E/S.
package jcs

import (
	"bytes"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// ErrInvalid est le refus de toute valeur ou de tout texte hors forme.
const ErrInvalid = primitives.ErrInvalid

// Object est un objet JSON ; Array, un tableau. Les autres valeurs sont nil,
// bool, string, float64 et les entiers Go (int, int64, uint32), bornés à
// ±2⁵³ comme les entiers sûrs d'ECMAScript.
type (
	Object = map[string]any
	Array  = []any
)

// Marshal rend le JSON canonique de v, en octets UTF-8.
func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := encode(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Canonicalize lit un texte JSON (RFC 8259, UTF-8) et rend son JSON canonique.
func Canonicalize(text []byte) ([]byte, error) {
	v, err := Parse(text)
	if err != nil {
		return nil, err
	}
	return Marshal(v)
}

// ReadCanonical lit un JSON qui doit être exactement sa forme canonique.
func ReadCanonical(text []byte) (any, error) {
	v, err := Parse(text)
	if err != nil {
		return nil, err
	}
	if c, err := Marshal(v); err != nil || !bytes.Equal(c, text) {
		return nil, ErrInvalid
	}
	return v, nil
}

const maxSafe = 1 << 53

func encode(b *bytes.Buffer, v any) error {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case string:
		return encodeString(b, v)
	case float64:
		s, err := FormatNumber(v)
		if err != nil {
			return err
		}
		b.WriteString(s)
	case int:
		return encodeInt(b, int64(v))
	case int64:
		return encodeInt(b, v)
	case uint32:
		return encodeInt(b, int64(v))
	case Array:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := encode(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case Object:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return LessUTF16(keys[i], keys[j]) })
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := encodeString(b, k); err != nil {
				return err
			}
			b.WriteByte(':')
			if err := encode(b, v[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return ErrInvalid
	}
	return nil
}

func encodeInt(b *bytes.Buffer, i int64) error {
	if i > maxSafe || i < -maxSafe {
		return ErrInvalid
	}
	b.WriteString(strconv.FormatInt(i, 10))
	return nil
}

// LessUTF16 compare deux chaînes par unités UTF-16 (RFC 8785, §3.2.3).
func LessUTF16(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

func encodeString(b *bytes.Buffer, s string) error {
	if !IsNFC(s) {
		return ErrInvalid
	}
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			if c < 0x20 {
				b.WriteString(`\u00`)
				b.WriteString(strconv.FormatUint(uint64(c)>>4, 16) + strconv.FormatUint(uint64(c)&0xf, 16))
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return nil
}

// FormatNumber sérialise un nombre comme Number::toString d'ECMAScript
// (RFC 8785, §3.2.2.3) : -0 donne 0, NaN et les infinis sont refusés.
func FormatNumber(f float64) (string, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", ErrInvalid
	}
	if f == 0 {
		return "0", nil
	}
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}
	// Chiffres les plus courts qui relisent f, et exposant décimal n tel que
	// f = 0.digits × 10ⁿ.
	mant, exp, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	digits := strings.Replace(mant, ".", "", 1)
	e, _ := strconv.Atoi(exp)
	k, n := len(digits), e+1
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k), nil
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:], nil
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits, nil
	}
	s := digits[:1]
	if k > 1 {
		s += "." + digits[1:]
	}
	if n-1 >= 0 {
		return sign + s + "e+" + strconv.Itoa(n-1), nil
	}
	return sign + s + "e" + strconv.Itoa(n-1), nil
}

// Parse lit un texte JSON en valeurs Object, Array, string, float64, bool et
// nil. UTF-8 invalide, demi-paire isolée, clé en double et donnée après la
// valeur sont refusés.
func Parse(text []byte) (any, error) {
	if !utf8.Valid(text) {
		return nil, ErrInvalid
	}
	p := parser{s: text}
	v, err := p.value(0)
	if err != nil {
		return nil, err
	}
	if p.skip(); p.i != len(p.s) {
		return nil, ErrInvalid
	}
	return v, nil
}

// maxDepth borne l'imbrication : aucun texte reçu ne fait déborder la pile.
const maxDepth = 64

type parser struct {
	s []byte
	i int
}

func (p *parser) skip() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t' || p.s[p.i] == '\n' || p.s[p.i] == '\r') {
		p.i++
	}
}

func (p *parser) eat(c byte) bool {
	if p.skip(); p.i < len(p.s) && p.s[p.i] == c {
		p.i++
		return true
	}
	return false
}

func (p *parser) value(depth int) (any, error) {
	if depth > maxDepth {
		return nil, ErrInvalid
	}
	p.skip()
	if p.i >= len(p.s) {
		return nil, ErrInvalid
	}
	switch c := p.s[p.i]; {
	case c == '{':
		p.i++
		obj := Object{}
		if p.eat('}') {
			return obj, nil
		}
		for {
			if p.skip(); p.i >= len(p.s) || p.s[p.i] != '"' {
				return nil, ErrInvalid
			}
			k, err := p.str()
			if err != nil {
				return nil, err
			}
			if _, dup := obj[k]; dup || !p.eat(':') {
				return nil, ErrInvalid
			}
			if obj[k], err = p.value(depth + 1); err != nil {
				return nil, err
			}
			if p.eat('}') {
				return obj, nil
			}
			if !p.eat(',') {
				return nil, ErrInvalid
			}
		}
	case c == '[':
		p.i++
		arr := Array{}
		if p.eat(']') {
			return arr, nil
		}
		for {
			v, err := p.value(depth + 1)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
			if p.eat(']') {
				return arr, nil
			}
			if !p.eat(',') {
				return nil, ErrInvalid
			}
		}
	case c == '"':
		return p.str()
	case c == '-' || c >= '0' && c <= '9':
		return p.number()
	}
	for _, lit := range []struct {
		text string
		v    any
	}{{"true", true}, {"false", false}, {"null", nil}} {
		if bytes.HasPrefix(p.s[p.i:], []byte(lit.text)) {
			p.i += len(lit.text)
			return lit.v, nil
		}
	}
	return nil, ErrInvalid
}

func (p *parser) number() (any, error) {
	start := p.i
	digits := func() int {
		n := 0
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i, n = p.i+1, n+1
		}
		return n
	}
	if p.s[p.i] == '-' {
		p.i++
	}
	if p.i < len(p.s) && p.s[p.i] == '0' {
		p.i++
	} else if digits() == 0 {
		return nil, ErrInvalid
	}
	if p.i < len(p.s) && p.s[p.i] == '.' {
		if p.i++; digits() == 0 {
			return nil, ErrInvalid
		}
	}
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		if p.i++; p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
			p.i++
		}
		if digits() == 0 {
			return nil, ErrInvalid
		}
	}
	f, err := strconv.ParseFloat(string(p.s[start:p.i]), 64)
	if err != nil || math.IsInf(f, 0) {
		return nil, ErrInvalid
	}
	return f, nil
}

func (p *parser) str() (string, error) {
	p.i++ // guillemet ouvrant
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			return b.String(), nil
		case c < 0x20:
			return "", ErrInvalid
		case c != '\\':
			b.WriteByte(c)
			p.i++
			continue
		}
		if p.i+1 >= len(p.s) {
			return "", ErrInvalid
		}
		e := p.s[p.i+1]
		p.i += 2
		if r, ok := map[byte]byte{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}[e]; ok {
			b.WriteByte(r)
			continue
		}
		if e != 'u' {
			return "", ErrInvalid
		}
		r, ok := p.hex4()
		if !ok {
			return "", ErrInvalid
		}
		if utf16.IsSurrogate(r) {
			lo, ok2 := rune(-1), false
			if r < 0xdc00 && p.i+1 < len(p.s) && p.s[p.i] == '\\' && p.s[p.i+1] == 'u' {
				p.i += 2
				lo, ok2 = p.hex4()
			}
			if r = utf16.DecodeRune(r, lo); !ok2 || r == utf8.RuneError {
				return "", ErrInvalid
			}
		}
		b.WriteRune(r)
	}
	return "", ErrInvalid
}

func (p *parser) hex4() (rune, bool) {
	if p.i+4 > len(p.s) {
		return 0, false
	}
	v, err := strconv.ParseUint(string(p.s[p.i:p.i+4]), 16, 32)
	p.i += 4
	return rune(v), err == nil
}
