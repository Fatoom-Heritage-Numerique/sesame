// Commande generer-compteurpin : écrit le vecteur compteurPin
// (testdata/vecteurs/compteurPin.json), propre à ce dépôt et hors de
// SHA256SUMS (CONTRIBUTING.md, « Formats et vecteurs »). Les réponses
// attendues se calculent ici depuis la table des délais du contrat (API.md
// §2.13) et les règles du compteur par code PIN (API.md §2.12), sans importer
// le code du protocole de PIN (internal/pin), que le vecteur éprouve. Avec
// -verifier, la commande recalcule le vecteur en mémoire et échoue s'il
// diffère du fichier commité (contrôle de la CI). Usage :
//
//	go run ./outils/generer-compteurpin [-verifier] [-fichier testdata/vecteurs/compteurPin.json]
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
)

// delais est la table du contrat, recopiée de API.md §2.13 : aucun délai pour
// les 3 premiers essais depuis la dernière remise à 10, puis 1 min, 5 min,
// 15 min, 1 h, 4 h, 12 h, 24 h, comptés depuis l'essai décompté précédent.
var delais = []time.Duration{0, 0, 0, time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 4 * time.Hour, 12 * time.Hour, 24 * time.Hour}

const layout = "2006-01-02T15:04:05.000Z"

// appel est un appel de la suite : un dépôt (motif) ou une évaluation, à
// l'instant debut + apres.
type appel struct {
	motif string
	apres time.Duration
}

// suite : redépôt sur un enregistrement absent, un essai, nouveau_pin (le
// compteur des évaluations n'est pas remis à zéro), puis les dix essais du
// nouveau code avec deux essais dans un délai en cours et un redépôt entre
// deux essais, un 11ᵉ essai au plafond, ouverture et redepot refusés,
// nouveau_pin et un essai sans délai.
var suite = []appel{
	{"redepot", 0}, {"", time.Second}, {"nouveau_pin", 2 * time.Second},
	{"", 3 * time.Second}, {"", 4 * time.Second}, {"", 5 * time.Second}, {"", 30 * time.Second}, {"redepot", 40 * time.Second},
	{"", 65 * time.Second}, {"", 6*time.Minute + 5*time.Second}, {"", 21*time.Minute + 5*time.Second},
	{"", 81*time.Minute + 5*time.Second}, {"", 321*time.Minute + 5*time.Second}, {"", 1041*time.Minute + 5*time.Second},
	{"", 2481*time.Minute + 4*time.Second}, {"", 2481*time.Minute + 5*time.Second}, {"", 2482 * time.Minute},
	{"ouverture", 2483 * time.Minute}, {"redepot", 2484 * time.Minute}, {"nouveau_pin", 2485 * time.Minute}, {"", 2485*time.Minute + time.Second},
}

func stamp(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(layout)
}

// vecteur calcule les sorties attendues de la suite.
func vecteur() jcs.Object {
	debut := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	present, restants, evaluations := false, 0, int64(0)
	var prochain time.Time
	appels, apres := jcs.Array{}, jcs.Array{}
	for _, a := range suite {
		le := debut.Add(a.apres)
		entree := jcs.Object{"route": "pin/evaluer", "le": stamp(le)}
		sortie := jcs.Object{"reponse": "evaluation"}
		switch {
		case a.motif != "":
			entree = jcs.Object{"route": "pin/deposer", "motif": a.motif, "le": stamp(le)}
			if a.motif == "ouverture" {
				sortie["reponse"] = "ouverture"
			}
			switch {
			case present && restants == 0 && a.motif != "nouveau_pin":
				sortie["reponse"] = "pin_bloque"
			case !present:
				present, restants, evaluations, prochain = true, 10, 0, time.Time{}
			case a.motif != "redepot":
				restants, prochain = 10, time.Time{}
			}
		case restants == 0:
			sortie["reponse"] = "pin_bloque"
		case le.Before(prochain):
			sortie["reponse"], sortie["reessayerLe"] = "trop_d_essais", stamp(prochain)
		default:
			rang := 11 - restants
			restants--
			evaluations++
			if restants == 0 {
				sortie["reponse"], prochain = "pin_bloque", time.Time{}
			} else {
				prochain = le.Add(delais[rang])
			}
		}
		sortie["essaisRestants"], sortie["prochainEssaiLe"], sortie["evaluations"] = restants, stamp(prochain), evaluations
		appels, apres = append(appels, entree), append(apres, sortie)
	}
	return jcs.Object{"vecteurs": jcs.Array{jcs.Object{"nom": "compteurPin/suite",
		"entrees": jcs.Object{"appels": appels}, "sorties": jcs.Object{"apres": apres}}}}
}

func main() {
	verifier := flag.Bool("verifier", false, "compare le vecteur recalculé au fichier commité")
	fichier := flag.String("fichier", "testdata/vecteurs/compteurPin.json", "fichier du vecteur")
	flag.Parse()
	b, err := jcs.Marshal(vecteur())
	if err != nil {
		fmt.Fprintln(os.Stderr, "generer-compteurpin :", err)
		os.Exit(1)
	}
	b = append(b, '\n')
	if !*verifier {
		err = os.WriteFile(*fichier, b, 0o644)
	} else if got, e := os.ReadFile(*fichier); e != nil || !bytes.Equal(got, b) {
		err = fmt.Errorf("%s diffère du vecteur recalculé : le régénérer par go run ./outils/generer-compteurpin", *fichier)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "generer-compteurpin :", err)
		os.Exit(1)
	}
	if *verifier {
		fmt.Println("compteurPin.json identique au vecteur recalculé")
	}
}
