package essai

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"time"
)

// ServeurHeure est la référence de l'heure des VM que retient la
// spécification : le serveur de métadonnées, en NTP (ARCHITECTURE.md §2.2,
// « Heure »).
const ServeurHeure = "metadata.google.internal:123"

// ServeurHeureIP est le même serveur par son adresse, sans résolution DNS
// (spécification de l'opérateur, enclave, « Heure des VM »).
const ServeurHeureIP = "169.254.169.254:123"

// ecartEpoqueNTP est le nombre de secondes du 1er janvier 1900 (époque de
// NTP) au 1er janvier 1970.
const ecartEpoqueNTP = 2208988800

// Mesure est une mesure SNTP (RFC 4330, §5) : écart de l'horloge du serveur
// sur celle de la VM (positif si la VM retarde) et délai aller-retour, en
// millisecondes.
type Mesure struct {
	EcartMs float64 `json:"ecartMs"`
	DelaiMs float64 `json:"delaiMs"`
	Strate  int     `json:"strate"`
}

// Horloge rend l'heure de la VM ; remplacée par les tests.
type Horloge func() time.Time

// MesurerSNTP envoie une requête SNTP à adresse et calcule l'écart et le
// délai. maintenant donne l'heure murale de la VM au départ et au retour.
func MesurerSNTP(ctx context.Context, adresse string, maintenant Horloge) (Mesure, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", adresse)
	if err != nil {
		return Mesure{}, err
	}
	defer conn.Close()
	echeance := time.Now().Add(5 * time.Second)
	if e, ok := ctx.Deadline(); ok && e.Before(echeance) {
		echeance = e
	}
	if err := conn.SetDeadline(echeance); err != nil {
		return Mesure{}, err
	}
	requete := make([]byte, 48)
	requete[0] = 0<<6 | 4<<3 | 3 // LI 0, version 4, mode client
	t1 := maintenant()
	putNTP(requete[40:48], t1) // horodatage d'émission, recopié par le serveur
	if _, err := conn.Write(requete); err != nil {
		return Mesure{}, err
	}
	reponse := make([]byte, 128)
	n, err := conn.Read(reponse)
	t4 := maintenant()
	if err != nil {
		return Mesure{}, err
	}
	return lireReponseSNTP(reponse[:n], requete[40:48], t1, t4)
}

func lireReponseSNTP(r, emis []byte, t1, t4 time.Time) (Mesure, error) {
	if len(r) < 48 {
		return Mesure{}, errors.New("réponse SNTP trop courte")
	}
	if mode := r[0] & 7; mode != 4 {
		return Mesure{}, errors.New("réponse SNTP : mode autre que serveur")
	}
	if r[0]>>6 == 3 {
		return Mesure{}, errors.New("réponse SNTP : serveur non synchronisé")
	}
	strate := int(r[1])
	if strate == 0 || strate > 15 {
		return Mesure{}, errors.New("réponse SNTP : strate hors de 1 à 15")
	}
	if string(r[24:32]) != string(emis) {
		return Mesure{}, errors.New("réponse SNTP : horodatage d'origine étranger")
	}
	t2, t3 := getNTP(r[32:40]), getNTP(r[40:48])
	ecart := (t2.Sub(t1) + t3.Sub(t4)) / 2
	delai := t4.Sub(t1) - t3.Sub(t2)
	return Mesure{EcartMs: ms(ecart), DelaiMs: ms(delai), Strate: strate}, nil
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func putNTP(b []byte, t time.Time) {
	secondes := uint64(t.Unix() + ecartEpoqueNTP)
	fraction := uint64(t.Nanosecond()) << 32 / 1e9
	binary.BigEndian.PutUint64(b, secondes<<32|fraction)
}

func getNTP(b []byte) time.Time {
	v := binary.BigEndian.Uint64(b)
	secondes := int64(v>>32) - ecartEpoqueNTP
	nanos := int64((v & 0xffffffff) * 1e9 >> 32)
	return time.Unix(secondes, nanos)
}
