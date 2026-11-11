// Commande essai : programme jetable de la séance d'essais sur une VM
// Confidential Space (essais/README.md, décision 0075). Il sert en HTTP :
//
//   - GET /horloge : écart de l'horloge de la VM sur le serveur de
//     métadonnées, en SNTP ;
//   - GET /attestation : jeton PKI du lanceur pour une audience et des nonces ;
//   - GET /firestore : débit d'écriture du document unique chaine/tete.
//
// Il écrit aussi, chaque minute, une mesure de l'horloge dans son journal,
// pour suivre la dérive pendant une migration à chaud. Ce n'est ni Sésame ni
// un adaptateur de Sésame : aucun binaire publié ne le lie.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/essais/internal/essai"
)

func main() {
	ecoute := flag.String("ecoute", ":8080", "adresse d'écoute HTTP")
	rythme := flag.Duration("journal-horloge", time.Minute, "intervalle des mesures de l'horloge écrites au journal (0 : aucune)")
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.LUTC | log.Lmicroseconds)

	meta := essai.NouvellesMetadonnees(essai.ServeurMetadonnees)
	lanceur := essai.NouveauLanceur(essai.SocketLanceur)
	heure := func(ctx context.Context, adresse string) (essai.Mesure, error) {
		return essai.MesurerSNTP(ctx, adresse, time.Now)
	}
	s := &essai.Serveur{
		Heure:     heure,
		Jeton:     lanceur.JetonPKI,
		DemarreLe: time.Now().UTC(),
		Ecrivain: func(ctx context.Context, base string) (essai.Ecrivain, error) {
			projet, err := meta.Projet(ctx)
			if err != nil {
				return nil, err
			}
			return essai.NouveauFirestore(essai.APIFirestore, projet, base, meta.Jeton).Transaction, nil
		},
	}
	if *rythme > 0 {
		go journaliserHorloge(heure, *rythme)
	}
	srv := &http.Server{Addr: *ecoute, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("essai : écoute sur %s", *ecoute)
	log.Fatal(srv.ListenAndServe())
}

// journaliserHorloge écrit une mesure SNTP à chaque intervalle : des
// nombres seulement.
func journaliserHorloge(heure func(context.Context, string) (essai.Mesure, error), intervalle time.Duration) {
	for {
		for _, ref := range []struct{ nom, adresse string }{{"nom", essai.ServeurHeure}, {"ip", essai.ServeurHeureIP}} {
			ctx, annuler := context.WithTimeout(context.Background(), 10*time.Second)
			m, err := heure(ctx, ref.adresse)
			annuler()
			if err != nil {
				log.Printf("horloge : mesure en échec (reference=%s)", ref.nom)
				continue
			}
			log.Printf("horloge : reference=%s ecartMs=%.3f delaiMs=%.3f strate=%d", ref.nom, m.EcartMs, m.DelaiMs, m.Strate)
			break
		}
		time.Sleep(intervalle)
	}
}
