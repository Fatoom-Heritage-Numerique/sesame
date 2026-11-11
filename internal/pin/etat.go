package pin

import "time"

// Tries est le nombre d'essais par code PIN (R-PIN-02, API.md §2.12).
const Tries = 10

// Delays sont les délais du contrat (API.md §2.13), avant l'essai de rang 1
// à 10 depuis la dernière remise à 10, comptés depuis l'essai décompté
// précédent : aucun pour les 3 premiers, puis 1 min, 5 min, 15 min, 1 h,
// 4 h, 12 h, 24 h.
var Delays = [Tries]time.Duration{0, 0, 0, time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 4 * time.Hour, 12 * time.Hour, 24 * time.Hour}

// Record est l'enregistrement du PIN d'un titulaire, en mémoire de la grappe
// seulement (docs/technique/sesame/protocole-pin.md) : cleOprf (nil au
// plafond), essaisRestants, prochainEssaiLe (zéro après une remise à 10),
// creeLe et le compteur des évaluations décomptées.
type Record struct {
	Key     []byte
	Left    int
	Next    time.Time
	Created time.Time
	Evals   int64
}

// Op est une opération validée par la majorité : un dépôt (Motif non vide,
// Key tirée par le leader, sauf pour ouverture) ou un essai, à l'heure At du
// leader.
type Op struct {
	UID   string    `json:"uid"`
	Motif string    `json:"motif,omitempty"`
	Key   []byte    `json:"cle,omitempty"`
	At    time.Time `json:"le"`
}

// Apply applique op à records, sans E/S ni panique, et rend le code du refus
// ("" si appliquée) et l'enregistrement qui en résulte. Le leader a fait les
// mêmes contrôles avant de proposer : une réplique les refait, si bien
// qu'une entrée fautive est rejetée par les trois (« Mémoire et robustesse »).
func Apply(records map[string]*Record, op Op) (string, *Record) {
	r := records[op.UID]
	switch {
	case op.Motif == "" && r == nil:
		return "pin_indisponible", r
	case op.Motif == "" && r.Left == 0:
		return "pin_bloque", r
	case op.Motif == "" && op.At.Before(r.Next):
		return "trop_d_essais", r
	case op.Motif == "":
		r.Left--
		r.Evals++
		r.Next = op.At.Add(Delays[(Tries-r.Left)%Tries])
		if r.Left == 0 { // 10ᵉ essai : cleOprf effacée avant toute évaluation
			r.Key, r.Next = nil, time.Time{}
		}
		return "", r
	case op.Motif != "ouverture" && op.Motif != "redepot" && op.Motif != "nouveau_pin", op.Motif != "ouverture" && len(op.Key) != 32:
		return "message_refuse", r
	case r == nil && op.Motif == "ouverture":
		return "pin_indisponible", r
	case r == nil:
		r = &Record{Left: Tries, Created: op.At}
		records[op.UID] = r
	case r.Left == 0 && op.Motif != "nouveau_pin":
		return "pin_bloque", r
	}
	if op.Motif != "redepot" {
		r.Left, r.Next = Tries, time.Time{}
	}
	if op.Motif != "ouverture" {
		r.Key = op.Key
	}
	return "", r
}
