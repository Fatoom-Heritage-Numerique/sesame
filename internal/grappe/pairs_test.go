package grappe

import (
	"slices"
	"testing"
)

// GAM-06 : les pairs se dérivent du nom de la VM (même radical, autre place) ;
// une VM d'un autre radical (autre version) n'est jamais jointe
// (protocole-pin.md, « Noms et pairs »).
func TestPairsDuNom(t *testing.T) {
	for name, want := range map[string][]string{
		"sesame-v2-b": {"sesame-v2-a", "sesame-v2-c"},
		"sesame-v1-a": {"sesame-v1-b", "sesame-v1-c"},
		"":            nil,
	} {
		if got := peers(name); !slices.Equal(got, want) {
			t.Fatalf("%q : %v", name, got)
		}
	}
}
