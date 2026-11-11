package attestation

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vecteurs"
)

type hexa = vecteurs.Hex

func TestAttestationVectors(t *testing.T) {
	tr := vecteurs.Track(t, "attestation.json", "attestation", "listeVersions")
	tr.Vectors(t, "attestation", func(t *testing.T, v vecteurs.Vector) {
		var e struct {
			JWT, Liste, PrefixeImage, Projet string
			KemPub, Racine, SessionPub       hexa
			Maintenant                       int64
		}
		var want struct{ ImageDigest, Motif, Statut, Version string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		list, err := ReadVersions([]byte(e.Liste))
		if err != nil {
			t.Fatal(err)
		}
		type result struct{ ImageDigest, Motif, Statut, Version string }
		var got result
		c, err := Verify(e.JWT, e.Racine, time.UnixMilli(e.Maintenant), ChannelNonces(e.SessionPub, e.KemPub))
		var ver *Version
		if err == nil {
			ver, err = CheckList(c, list, e.Projet, e.PrefixeImage)
		}
		if err != nil {
			got = result{Motif: err.Error(), Statut: "refusee"}
		} else {
			got = result{ImageDigest: c.ImageDigest, Statut: "acceptee", Version: ver.Version}
		}
		vecteurs.Same(t, got, want)
		// Entre VM, un jeton de client (deux valeurs) est toujours refusé.
		pair := []string{"a", "b", "c"}
		if _, err := Verify(e.JWT, e.Racine, time.UnixMilli(e.Maintenant), pair); err == nil {
			t.Error("jeton de client accepté entre VM")
		}
	})
	readList := func(t *testing.T, raw []byte) (*Versions, error) {
		var e struct{ Liste string }
		vecteurs.Load(t, raw, &e)
		return ReadVersions([]byte(e.Liste))
	}
	tr.Vectors(t, "listeVersions", func(t *testing.T, v vecteurs.Vector) {
		var want struct{ EnService, Kids, Retirees string }
		vecteurs.Load(t, v.Outputs, &want)
		l, err := readList(t, v.Inputs)
		if err != nil {
			t.Fatal(err)
		}
		by := func(status string) string {
			var s []string
			for _, r := range l.Versions {
				if r.Status == status {
					s = append(s, r.Version)
				}
			}
			return strings.Join(s, ",")
		}
		var kids []string
		for k := range l.SessionKeys {
			kids = append(kids, k)
		}
		slices.Sort(kids)
		vecteurs.Same(t, struct{ EnService, Kids, Retirees string }{by("en_service"), strings.Join(kids, ","), by("retiree")}, want)
	})
	tr.Refusals(t, "listeVersions", func(t *testing.T, r vecteurs.Refusal) {
		if _, err := readList(t, r.Inputs); err == nil || err.Error() != r.Error {
			t.Errorf("erreur %v, attendu %s", err, r.Error)
		}
	})
}

func TestNonceVectors(t *testing.T) {
	tr := vecteurs.Track(t, "sesame.json", "jetonPaire", "noncesClient", "noncesPaire")
	type pair struct {
		Cle, NoncePaire hexa
		NodeID          string
		EatNonce        string
	}
	tr.Vectors(t, "jetonPaire", func(t *testing.T, v vecteurs.Vector) {
		var e pair
		var want struct{ NonceCle, NonceNodeID, NoncePaire string }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		n, err := PairNonces(e.NoncePaire, e.NodeID, e.Cle)
		if err != nil {
			t.Fatal(err)
		}
		vecteurs.Same(t, struct{ NonceCle, NonceNodeID, NoncePaire string }{n[2], n[1], n[0]}, want)
	})
	tr.Refusals(t, "jetonPaire", func(t *testing.T, r vecteurs.Refusal) {
		var e pair
		vecteurs.Load(t, r.Inputs, &e)
		if _, err := PairNonces(e.NoncePaire, e.NodeID, e.Cle); err == nil || err.Error() != r.Error {
			t.Errorf("erreur %v, attendu %s", err, r.Error)
		}
	})
	accepted := func(t *testing.T, raw string, want []string) bool {
		v, err := jcs.Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		return NoncesAccepted(v, want)
	}
	tr.Vectors(t, "noncesPaire", func(t *testing.T, v vecteurs.Vector) {
		var e pair
		var want struct{ Accepte bool }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		n, err := PairNonces(e.NoncePaire, e.NodeID, e.Cle)
		if err != nil {
			t.Fatal(err)
		}
		if got := accepted(t, e.EatNonce, n); got != want.Accepte {
			t.Errorf("accepte = %v", got)
		}
	})
	tr.Vectors(t, "noncesClient", func(t *testing.T, v vecteurs.Vector) {
		var e struct {
			EatNonce           string
			KemPub, SessionPub hexa
		}
		var want struct{ Accepte bool }
		vecteurs.Load(t, v.Inputs, &e)
		vecteurs.Load(t, v.Outputs, &want)
		if got := accepted(t, e.EatNonce, ChannelNonces(e.SessionPub, e.KemPub)); got != want.Accepte {
			t.Errorf("accepte = %v", got)
		}
	})
}
