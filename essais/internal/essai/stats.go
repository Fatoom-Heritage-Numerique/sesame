// Package essai porte les trois essais de la séance sur Confidential Space
// (essais/README.md, décision 0075) : horloge de la VM, jeton d'attestation
// du lanceur, débit d'écriture d'un document unique de Firestore. Ce code
// n'est lié par aucun binaire publié (ni cmd/sesame, ni cmd/sesame-fake) et
// ne décide rien de Sésame.
package essai

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
)

// Resume résume une série de valeurs (millisecondes).
type Resume struct {
	N       int     `json:"n"`
	Min     float64 `json:"min"`
	Mediane float64 `json:"mediane"`
	P90     float64 `json:"p90"`
	P99     float64 `json:"p99"`
	Max     float64 `json:"max"`
}

// Resumer rend le résumé des valeurs, ou nil pour une série vide. Les
// centiles sont ceux du rang le plus proche ; la médiane d'une série paire
// est la moyenne des deux valeurs du milieu.
func Resumer(valeurs []float64) *Resume {
	if len(valeurs) == 0 {
		return nil
	}
	v := append([]float64(nil), valeurs...)
	sort.Float64s(v)
	n := len(v)
	mediane := v[n/2]
	if n%2 == 0 {
		mediane = (v[n/2-1] + v[n/2]) / 2
	}
	return &Resume{N: n, Min: v[0], Mediane: mediane, P90: rang(v, 0.90), P99: rang(v, 0.99), Max: v[n-1]}
}

// rang rend le centile p (0 < p ≤ 1) d'une série triée, au rang le plus
// proche : la plus petite valeur dont au moins p des valeurs sont inférieures
// ou égales.
func rang(trie []float64, p float64) float64 {
	k := int(math.Ceil(p*float64(len(trie)))) - 1
	if k < 0 {
		k = 0
	}
	return trie[k]
}

// Entier lit le paramètre entier nom de la requête : defaut s'il est absent,
// une erreur s'il est illisible ou hors de [min, max].
func Entier(q url.Values, nom string, defaut, min, max int) (int, error) {
	brut := q.Get(nom)
	if brut == "" {
		return defaut, nil
	}
	n, err := strconv.Atoi(brut)
	if err != nil {
		return 0, fmt.Errorf("paramètre %s illisible", nom)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("paramètre %s hors de [%d, %d]", nom, min, max)
	}
	return n, nil
}
