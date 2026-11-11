package factice_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Comptage des appels par route et par clé de limite (uid, canal) dans les
// parcours joués par les tests du contrat (tests.md du client, « Grappe du
// PIN », mesures) : MESURES_APPELS=1 go test ./internal/factice imprime, par
// route, le plus grand nombre d'appels d'une même clé dans un test, hors
// tests qui éprouvent une limite (réponse trop_d_appels) ou un refus.
var appels = struct {
	sync.Mutex
	n      map[string]map[string]int // test → route|clé → appels
	limite map[string]bool
}{n: map[string]map[string]int{}, limite: map[string]bool{}}

func compte(t *testing.T, c *ch, route string, uid string, r map[string]any) {
	if os.Getenv("MESURES_APPELS") == "" {
		return
	}
	appels.Lock()
	defer appels.Unlock()
	m := appels.n[t.Name()]
	if m == nil {
		m = map[string]int{}
		appels.n[t.Name()] = m
	}
	m[route+"|canal"+c.id+string(c.cliPub)]++
	if uid != "" {
		m[route+"|uid|"+uid]++
	}
	if r["erreur"] == "trop_d_appels" {
		appels.limite[t.Name()] = true
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	if os.Getenv("MESURES_APPELS") != "" {
		max := map[string][2]any{}
		for test, m := range appels.n {
			if appels.limite[test] || strings.Contains(test, "Limite") || strings.Contains(test, "Refus") {
				continue
			}
			for k, n := range m {
				route, cle, _ := strings.Cut(k, "|")
				k := route + " par " + map[bool]string{true: "uid", false: "canal"}[strings.HasPrefix(cle, "uid")]
				if old, _ := max[k][0].(int); n > old {
					max[k] = [2]any{n, test}
				}
			}
		}
		var keys []string
		for k := range max {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("APPELS\t%s\t%d\t%s\n", k, max[k][0], max[k][1])
		}
	}
	os.Exit(code)
}
