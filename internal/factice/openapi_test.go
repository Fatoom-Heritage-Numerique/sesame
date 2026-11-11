package factice_test

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/routes"
)

// aVenir nomme les routes d'API.md (§2 et §4) que sesame-fake ne sert pas
// encore : chacune en sort avec son code, sa description dans l'OpenAPI et
// ses tests (décision 0064). Une route servie absente de l'OpenAPI, ou
// décrite et non servie, fait échouer TestOpenAPIRoutes.
var aVenir = []string{}

// communes sont les erreurs que toute route du canal peut rendre (API.md §2).
var communes = []string{"message_refuse", "trop_d_appels", "registre_indisponible"}

type spec struct {
	Components struct {
		Schemas map[string]any `json:"schemas"`
	} `json:"components"`
	Routes map[string]struct {
		Section string   `json:"section"`
		Requete any      `json:"requete"`
		Reponse any      `json:"reponse"`
		Erreurs []string `json:"erreurs"`
	} `json:"x-sesame-routes"`
	Erreurs map[string]*int `json:"x-sesame-erreurs"`
	EntreVM map[string]struct {
		Section string   `json:"section"`
		Requete any      `json:"requete"`
		Reponse any      `json:"reponse"`
		Erreurs []string `json:"erreurs"`
	} `json:"x-sesame-entre-vm"`
}

var (
	specOnce sync.Once
	theSpec  spec
	specErr  error
)

func loadSpec(t *testing.T) *spec {
	t.Helper()
	specOnce.Do(func() {
		b, err := os.ReadFile("../../api/openapi.json")
		if err == nil {
			err = json.Unmarshal(b, &theSpec)
		}
		specErr = err
	})
	if specErr != nil {
		t.Fatal(specErr)
	}
	return &theSpec
}

// keywords sont les seuls mots-clés que lit le validateur réduit.
var keywords = []string{"type", "properties", "required", "additionalProperties", "enum", "pattern", "minLength", "maxLength",
	"minimum", "maximum", "items", "maxItems", "$ref", "anyOf", "description"}

// check valide v contre le schéma s ; rend le chemin de la première faute.
func (sp *spec) check(s any, v any, path string) string {
	m, ok := s.(map[string]any)
	if !ok {
		return path + " : schéma illisible"
	}
	for k := range m {
		if !slices.Contains(keywords, k) {
			return path + " : mot-clé non lu par le validateur : " + k
		}
	}
	if ref, ok := m["$ref"].(string); ok {
		return sp.check(sp.Components.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")], v, path)
	}
	if any, ok := m["anyOf"].([]any); ok {
		for _, a := range any {
			if sp.check(a, v, path) == "" {
				return ""
			}
		}
		return path + " : aucune branche d'anyOf"
	}
	if t, ok := m["type"]; ok {
		types := []any{t}
		if a, ok := t.([]any); ok {
			types = a
		}
		match := false
		for _, ty := range types {
			match = match || typeOf(v, ty.(string))
		}
		if !match {
			return path + " : type"
		}
	}
	if e, ok := m["enum"].([]any); ok && !slices.ContainsFunc(e, func(x any) bool { return reflect.DeepEqual(x, v) }) {
		return path + " : enum"
	}
	switch v := v.(type) {
	case string:
		if p, ok := m["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(v) {
			return path + " : motif"
		}
		if n, ok := m["minLength"].(float64); ok && float64(len([]rune(v))) < n {
			return path + " : longueur"
		}
		if n, ok := m["maxLength"].(float64); ok && float64(len([]rune(v))) > n {
			return path + " : longueur"
		}
	case float64:
		if n, ok := m["minimum"].(float64); ok && v < n {
			return path + " : minimum"
		}
		if n, ok := m["maximum"].(float64); ok && v > n {
			return path + " : maximum"
		}
	case []any:
		if n, ok := m["maxItems"].(float64); ok && float64(len(v)) > n {
			return path + " : maxItems"
		}
		for i, e := range v {
			if f := sp.check(m["items"], e, path+"/"+strconv.Itoa(i)); m["items"] != nil && f != "" {
				return f
			}
		}
	case map[string]any:
		props, _ := m["properties"].(map[string]any)
		req, _ := m["required"].([]any)
		for _, r := range req {
			if _, ok := v[r.(string)]; !ok {
				return path + " : champ requis " + r.(string)
			}
		}
		for k, e := range v {
			ps, ok := props[k]
			if !ok && m["additionalProperties"] == false {
				return path + " : champ en plus " + k
			}
			if ok {
				if f := sp.check(ps, e, path+"/"+k); f != "" {
					return f
				}
			}
		}
	}
	return ""
}

func typeOf(v any, t string) bool {
	switch v := v.(type) {
	case nil:
		return t == "null"
	case bool:
		return t == "boolean"
	case string:
		return t == "string"
	case float64:
		return t == "number" || t == "integer" && v == math.Trunc(v)
	case []any:
		return t == "array"
	case map[string]any:
		return t == "object"
	}
	return false
}

func validateRequest(t *testing.T, route string, corps jcs.Object) {
	t.Helper()
	sp := loadSpec(t)
	r, ok := sp.Routes[route]
	if !ok {
		t.Fatalf("route %s absente de l'OpenAPI", route)
	}
	if f := sp.check(r.Requete, jsonValue(corps), route); f != "" {
		t.Fatalf("requête hors de son schéma : %s", f)
	}
}

func validateResponse(t *testing.T, route string, resp jcs.Object) {
	t.Helper()
	sp := loadSpec(t)
	r := sp.Routes[route]
	code := str(resp, "erreur")
	if code == "" {
		if f := sp.check(r.Reponse, jsonValue(sub(resp, "corps")), route+" (200)"); f != "" {
			t.Fatalf("réponse hors de son schéma : %s", f)
		}
		return
	}
	st := sp.Erreurs[code]
	if !slices.Contains(r.Erreurs, code) && !slices.Contains(communes, code) || st == nil || float64(*st) != resp["statut"] {
		t.Fatalf("code %s (statut %v) hors des erreurs de %s dans l'OpenAPI", code, resp["statut"], route)
	}
	for k := range sub(resp, "details") {
		if !slices.Contains([]string{"essaisRestants", "reessayerLe", "reprise"}, k) {
			t.Fatalf("details.%s hors contrat", k)
		}
	}
}

// jsonValue convertit les types de jcs (entiers Go compris) en valeurs de
// encoding/json.
func jsonValue(v any) any {
	b, _ := jcs.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// TestOpenAPIRoutes : routes de x-sesame-routes = routes servies par
// sesame-fake ; routes d'API.md (§2.2 à §2.21, §4) = servies ∪ à venir ;
// codes du §6 d'API.md = x-sesame-erreurs (décision 0064).
func TestOpenAPIRoutes(t *testing.T) {
	sp := loadSpec(t)
	var described []string
	for k, r := range sp.Routes {
		described = append(described, k)
		for _, s := range []any{r.Requete, r.Reponse} {
			if f := sp.walk(s, k); f != "" {
				t.Error(f)
			}
		}
	}
	served := routes.Routes()
	sort.Strings(described)
	sort.Strings(served)
	if !slices.Equal(described, served) {
		t.Errorf("x-sesame-routes %v\nservies par sesame-fake %v", described, served)
	}
	for _, n := range aVenir {
		if slices.Contains(served, n) {
			t.Errorf("%s est servie : la retirer d'aVenir", n)
		}
	}
	api, err := os.ReadFile("../../API.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(api)
	titled := regexp.MustCompile("(?m)^#{3,4} (?:[0-9.]+ )?`([a-z0-9/-]+)`$").FindAllStringSubmatch(text, -1)
	var names []string
	for _, m := range titled {
		names = append(names, m[1])
	}
	want := append(append([]string{}, served...), aVenir...)
	for k, r := range sp.EntreVM { // routes entre VM (§4) et messages de protocole-pin.md, « Transport »
		for _, s := range []any{r.Requete, r.Reponse} {
			if f := sp.walk(s, k); f != "" {
				t.Error(f)
			}
		}
		if g, ok := strings.CutPrefix(k, "/grappe/"); ok {
			want = append(want, "grappe/"+g)
		}
	}
	sort.Strings(names)
	sort.Strings(want)
	if !slices.Equal(names, want) {
		t.Errorf("routes d'API.md %v\nservies et à venir %v", names, want)
	}
	start, end := strings.Index(text, "\n## 6. "), strings.Index(text, "\n## 7. ")
	codes := map[string]*int{}
	for _, m := range regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\| ([0-9]+|—)").FindAllStringSubmatch(text[start:end], -1) {
		if n, err := strconv.Atoi(m[2]); err == nil {
			codes[m[1]] = &n
		} else {
			codes[m[1]] = nil
		}
	}
	if !reflect.DeepEqual(codes, sp.Erreurs) {
		t.Errorf("codes du §6 d'API.md et x-sesame-erreurs diffèrent")
	}
	for k, s := range sp.Components.Schemas {
		if f := sp.walk(s, k); f != "" {
			t.Error(f)
		}
	}
}

// walk refuse tout mot-clé que le validateur ne lit pas.
func (sp *spec) walk(s any, path string) string {
	m, _ := s.(map[string]any)
	for k, v := range m {
		if !slices.Contains(keywords, k) {
			return path + " : mot-clé non lu : " + k
		}
		switch k {
		case "properties":
			for p, ps := range v.(map[string]any) {
				if f := sp.walk(ps, path+"/"+p); f != "" {
					return f
				}
			}
		case "items":
			if f := sp.walk(v, path+"/items"); f != "" {
				return f
			}
		case "anyOf":
			for _, a := range v.([]any) {
				if f := sp.walk(a, path); f != "" {
					return f
				}
			}
		}
	}
	return ""
}
