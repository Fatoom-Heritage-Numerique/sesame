package jcs

import (
	"sort"
	"strconv"
	"sync"
	"unicode/utf8"
)

// IsNFC dit si s est de l'UTF-8 bien formé déjà en forme normale C (UAX #15) :
// aucun caractère NFC_QC=No, marques combinantes dans l'ordre canonique, et
// aucun caractère qui se composerait avec le dernier caractère de base qui le
// précède sans être bloqué. Sésame refuse une chaîne hors NFC au lieu de la
// normaliser (API.md §1, ARCHITECTURE.md §10).
//
// Limite : un caractère précomposé suivi d'une marque de classe inférieure à
// celle des marques de sa décomposition (« ṧ » puis la cédille U+0327) est
// accepté alors que la normalisation réordonnerait ces marques ; les tables
// ne portent pas la décomposition complète de chaque caractère.
func IsNFC(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	loadTables()
	starter, sinceStarter, maxCCC, prevCCC := rune(-1), 0, 0, 0
	for _, c := range s {
		if inRanges(tables.qcNo, c) {
			return false
		}
		k := combiningClass(c)
		if k != 0 && prevCCC > k {
			return false
		}
		unblocked := (k == 0 && sinceStarter == 0) || (k != 0 && maxCCC < k)
		if starter >= 0 && unblocked && composes(starter, c) {
			return false
		}
		if k == 0 {
			starter, sinceStarter, maxCCC = c, 0, 0
		} else {
			sinceStarter++
			maxCCC = max(maxCCC, k)
		}
		prevCCC = k
	}
	return true
}

// Composition algorithmique du Hangul (Unicode, §3.12).
const (
	hangulS, hangulL, hangulV, hangulT = 0xAC00, 0x1100, 0x1161, 0x11A7
	hangulLCount, hangulVCount         = 19, 21
	hangulTCount, hangulSCount         = 28, 11172
)

func composes(a, b rune) bool {
	switch {
	case a >= hangulL && a < hangulL+hangulLCount && b >= hangulV && b < hangulV+hangulVCount:
		return true
	case a >= hangulS && a < hangulS+hangulSCount && (a-hangulS)%hangulTCount == 0 && b > hangulT && b < hangulT+hangulTCount:
		return true
	}
	key := uint64(a)<<21 | uint64(b)
	i := sort.Search(len(tables.compositions), func(i int) bool { return tables.compositions[i] >= key })
	return i < len(tables.compositions) && tables.compositions[i] == key
}

func combiningClass(c rune) int {
	i := sort.Search(len(tables.ccc), func(i int) bool { return tables.ccc[i].hi >= c })
	if i < len(tables.ccc) && tables.ccc[i].lo <= c {
		return tables.ccc[i].class
	}
	return 0
}

type runeRange struct {
	lo, hi rune
	class  int
}

func inRanges(rs []runeRange, c rune) bool {
	i := sort.Search(len(rs), func(i int) bool { return rs[i].hi >= c })
	return i < len(rs) && rs[i].lo <= c
}

var (
	tablesOnce sync.Once
	tables     struct {
		qcNo, ccc    []runeRange
		compositions []uint64
	}
)

// loadTables décode les tables de nfc_tables.go (outils/generer-nfc).
func loadTables() {
	tablesOnce.Do(func() {
		hex := func(s string) rune { v, _ := strconv.ParseUint(s, 16, 32); return rune(v) }
		for i := 0; i+10 <= len(qcNo); i += 10 {
			tables.qcNo = append(tables.qcNo, runeRange{hex(qcNo[i : i+5]), hex(qcNo[i+5 : i+10]), 0})
		}
		for i := 0; i+12 <= len(cccRuns); i += 12 {
			tables.ccc = append(tables.ccc, runeRange{hex(cccRuns[i : i+5]), hex(cccRuns[i+5 : i+10]), int(hex(cccRuns[i+10 : i+12]))})
		}
		for i := 0; i+10 <= len(compositions); i += 10 {
			tables.compositions = append(tables.compositions, uint64(hex(compositions[i:i+5]))<<21|uint64(hex(compositions[i+5:i+10])))
		}
	})
}
