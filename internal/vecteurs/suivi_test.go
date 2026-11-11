package vecteurs

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fakeTB capte les erreurs et les nettoyages d'un test, pour éprouver Track
// sans faire échouer le test qui l'éprouve.
type fakeTB struct {
	testing.TB
	errors   []string
	cleanups []func()
	skipped  bool
}

func (f *fakeTB) Helper()          {}
func (f *fakeTB) Cleanup(c func()) { f.cleanups = append(f.cleanups, c) }
func (f *fakeTB) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}
func (f *fakeTB) Fatalf(format string, args ...any) { f.Errorf(format, args...) }
func (f *fakeTB) Skipped() bool                     { return f.skipped }
func (f *fakeTB) finish() {
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
}

func sample() *File {
	obj := json.RawMessage(`{}`)
	return &File{
		Vectors:  []Vector{{"x/a", obj, obj}, {"x/b", obj, obj}, {"y/a", obj, obj}},
		Refusals: []Refusal{{"x/c", obj, "parametres_invalides"}},
	}
}

func TestTrackAllRead(t *testing.T) {
	fake := &fakeTB{}
	tr := track(fake, "f.json", sample(), []string{"x"})
	tr.Vectors(t, "x", func(t *testing.T, v Vector) {})
	tr.Refusals(t, "x", func(t *testing.T, r Refusal) {})
	fake.finish()
	if len(fake.errors) > 0 {
		t.Fatalf("erreurs inattendues : %v", fake.errors)
	}
}

// Contrôle positif : un vecteur ou un refus non lus font échouer le test qui
// réclame leur groupe.
func TestTrackFindsUnread(t *testing.T) {
	fake := &fakeTB{}
	tr := track(fake, "f.json", sample(), []string{"x", "y"})
	tr.Vectors(t, "x", func(t *testing.T, v Vector) {})
	fake.finish()
	if len(fake.errors) != 1 {
		t.Fatalf("erreurs = %v, attendu une seule", fake.errors)
	}
	for _, want := range []string{"refus x/c", "y/a"} {
		if !strings.Contains(fake.errors[0], want) {
			t.Errorf("l'erreur %q ne nomme pas %s", fake.errors[0], want)
		}
	}
	for _, read := range []string{"x/a", "x/b"} {
		if strings.Contains(fake.errors[0], read+",") || strings.HasSuffix(fake.errors[0], read) {
			t.Errorf("l'erreur %q nomme %s, qui a été lu", fake.errors[0], read)
		}
	}
}

func TestTrackRefusesUnknownGroup(t *testing.T) {
	fake := &fakeTB{}
	if tr := track(fake, "f.json", sample(), []string{"z"}); tr != nil {
		t.Fatal("un groupe absent du fichier doit être refusé")
	}
	if len(fake.errors) != 1 || !strings.Contains(fake.errors[0], "aucun groupe z") {
		t.Fatalf("erreurs = %v", fake.errors)
	}
}

func TestTrackRefusesSkippedTest(t *testing.T) {
	fake := &fakeTB{skipped: true}
	track(fake, "f.json", sample(), []string{"y"})
	fake.finish()
	if len(fake.errors) != 1 || !strings.Contains(fake.errors[0], "ne se saute pas") {
		t.Fatalf("erreurs = %v", fake.errors)
	}
}
