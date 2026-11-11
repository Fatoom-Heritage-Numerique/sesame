package jcs

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// IsNFC dit si s est de l'UTF-8 bien formé déjà en forme normale C (UAX #15).
// Sésame refuse une chaîne hors NFC au lieu de la normaliser (API.md §1.7,
// ARCHITECTURE.md §10) ; le contrôle est celui de golang.org/x/text, extension
// officielle de Go épinglée (décision 0068), aux tables de la version Unicode
// de la bibliothèque standard.
func IsNFC(s string) bool {
	return utf8.ValidString(s) && norm.NFC.IsNormalString(s)
}
