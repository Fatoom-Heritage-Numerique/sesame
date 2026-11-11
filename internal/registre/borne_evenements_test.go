package registre

import (
	"fmt"
	"testing"
	"time"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// Les événements d'une période n'ont pas d'autre plafond que le nombre de
// ressources distinctes (ARCHITECTURE.md, borne du registre ; décision 0060) :
// le 10 001ᵉ rejet d'une période est porté comme le premier.
func TestSignalKeepsEveryDistinctEvent(t *testing.T) {
	b := &Borne{Horloge: fixedClock{time.Date(2026, 10, 12, 14, 3, 0, 0, time.UTC)}}
	const n = 10_001
	for i := 0; i < n; i++ {
		b.Signal("ligne_rejetee", fmt.Sprintf("essais/c%05d", i))
	}
	b.Signal("ligne_rejetee", "essais/c00000")
	b.Signal("registre_ancien", "chaine/tete")
	if got := len(b.events); got != n+1 {
		t.Fatalf("événements gardés : %d, attendus %d", got, n+1)
	}
	if _, ok := b.events[[2]string{"registre_ancien", "chaine/tete"}]; !ok {
		t.Fatal("événement signalé après 10 000 rejets perdu")
	}
}
