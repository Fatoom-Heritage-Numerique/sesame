package routes

import (
	"context"
	"encoding/hex"
	"regexp"
	"slices"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/pin"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// recentDays est l'écart exigé entre l'entrée dans la grappe inscrite et un
// dépôt dans une autre grappe (LIM-19 ; API.md §2.12).
const recentDays = 30

var (
	reVM   = regexp.MustCompile(`^[a-z][a-z0-9-]{0,26}-[abc]$`)
	scalar = append([]byte{1}, make([]byte, 31)...)
)

// isElement contrôle un élément ristretto255 de 32 octets (API.md §1.7).
func isElement(v any) bool {
	s, _ := v.(string)
	b, err := primitives.B64(s)
	_, err2 := pin.Evaluate(scalar, b)
	return err == nil && err2 == nil
}

// isPlaces contrôle la liste de grappe/membres : trois membres triés par vm,
// un par préfixe de routage (API.md §4.2).
func isPlaces(v any) bool {
	l, _ := v.(jcs.Array)
	var vms []string
	for _, e := range l {
		if o, _ := e.(jcs.Object); valid(o, []field{req("vm", matches(reVM)), req("zone", matches(reZone))}) {
			vms = append(vms, str(o, "vm"))
		}
	}
	return len(vms) == 3 && len(l) == 3 && slices.IsSorted(vms) && vms[0][len(vms[0])-1] == 'a' && vms[1][len(vms[1])-1] == 'b' && vms[2][len(vms[2])-1] == 'c'
}

var reZone = regexp.MustCompile(`^[a-z]+-[a-z]+[0-9]+-[a-z]$`)

// pinRoutes ajoute à la table les routes du PIN et de la grappe du canal.
func pinRoutes(table map[string]route) {
	elem := req("elementAveugle", isElement)
	table["pin/deposer"] = route{fields: []field{uidField, tokField, req("grappeId", matches(reHex64)), elem, defiField,
		req("motif", oneOf(vie.PinMotifs...)), sigField}, max: MaxBody, uid: limit{10, hour}, run: (*call).deposer}
	table["pin/evaluer"] = route{fields: []field{uidField, tokField, elem}, max: MaxBody, ip: limit{60, hour}, uid: limit{20, hour}, run: (*call).evaluer}
	table["grappe/membres"] = route{fields: []field{req("jetonExploitation", isText), req("grappeId", matches(reHex64)), req("membres", isPlaces)},
		max: MaxBody, uid: limit{2, hour}, run: (*call).membres}
	table["grappe/amorcer"] = route{fields: []field{req("jetonExploitation", isText)}, max: MaxBody, ip: limit{2, time.Minute}, run: (*call).amorcer}
}

// relayed est la réponse entière ({statut, …}) d'une route exécutée au leader.
type relayed jcs.Object

func (relayed) Error() string { return "relayee" }

// toLeader exécute la route au leader : ici si cette VM l'est, sinon par
// relais (POST /interne/relais) ; sans leader joignable,
// pin_indisponible_temporaire (§6, étape 9).
func (a *call) toLeader() (jcs.Object, error) {
	if a.in.grappe.Leader() {
		return a.atLeader()
	}
	r, ok := a.in.grappe.Relay(a.ctx, "/interne/relais", jcs.Object{"route": a.name, "corps": a.body,
		"canal": jcs.Object{"instanceId": a.in.ID, "cliPub": b64([]byte(a.ch.Key))}})
	if _, st := r["statut"].(float64); !ok || !st {
		return nil, fail("pin_indisponible_temporaire")
	}
	return nil, relayed(r)
}

// Relay exécute au leader une route relayée par une autre VM.
func (in *Instance) Relay(ctx context.Context, req jcs.Object) jcs.Object {
	a := &call{ctx: ctx, in: in, name: str(req, "route"), body: sub(req, "corps"), now: in.cfg.Horloge.Now()}
	r, ok := table[a.name]
	if a.r = &r; !ok || a.name != "pin/deposer" && a.name != "pin/evaluer" && a.name != "grappe/membres" || !valid(a.body, r.fields) {
		return response(nil, fail("message_refuse"))
	}
	return response(a.atLeader())
}

func (a *call) atLeader() (jcs.Object, error) {
	switch a.name {
	case "pin/deposer":
		return a.depot()
	case "pin/evaluer":
		return a.essai()
	}
	if err := a.limitKey("grappe"); err != nil {
		return nil, err
	}
	places := map[string]string{}
	for _, e := range a.body["membres"].(jcs.Array) {
		places[str(e.(jcs.Object), "vm")] = str(e.(jcs.Object), "zone")
	}
	if code := a.in.grappe.Membres(a.ctx, str(a.body, "grappeId"), places); code != "" {
		return nil, fail(code)
	}
	return jcs.Object{"grappeId": str(a.body, "grappeId"), "inscrite": true}, nil
}

func digest(b []byte) string { return hex.EncodeToString(primitives.SHA256(b)) }

// deposer suit §2.12 et l'ordre du §6 : idToken, défi et grappeId signé, et
// pour ouverture l'évaluation notée sur ce canal, par la VM qui reçoit ;
// le reste au leader.
func (a *call) deposer() (jcs.Object, error) {
	uid, id := str(a.body, "uid"), a.in.grappe.ID()
	if err := a.idToken(uid); err != nil {
		return nil, err
	}
	if err := a.challenge(uid); err != nil {
		return nil, err
	}
	switch {
	case id == "":
		return nil, fail("pin_indisponible_temporaire")
	case str(a.body, "grappeId") != id:
		return nil, fail("defi_invalide")
	case str(a.body, "motif") == "ouverture" && !a.in.table.Noted(a.ch, uid+"|"+digest(raw(a.body, "elementAveugle"))):
		return nil, fail("message_refuse")
	}
	return a.toLeader()
}

// evaluer suit §2.13 : idToken et limite par IP par la VM qui reçoit, qui
// note l'évaluation sur son canal pour un dépôt ouverture ; le reste au
// leader.
func (a *call) evaluer() (jcs.Object, error) {
	uid := str(a.body, "uid")
	if err := a.idToken(uid); err != nil {
		return nil, err
	}
	if err := a.limits(); err != nil {
		return nil, err
	}
	if a.in.grappe.ID() == "" {
		return nil, fail("pin_indisponible_temporaire")
	}
	out, err := a.toLeader()
	if r, _ := err.(relayed); err == nil || r["statut"] == 200.0 {
		a.in.table.Note(a.ch, uid+"|"+digest(raw(a.body, "elementAveugle")))
	}
	return out, err
}

// pinLine lit la ligne au leader (§6, étape 5) : registre illisible ⇒
// pin_indisponible_temporaire ; absente, rejetée ou supprime ⇒
// pin_indisponible.
func (a *call) pinLine(uid string) (registre.Line, error) {
	l, ok, err := a.actif(uid)
	if err != nil {
		return l, fail("pin_indisponible_temporaire")
	}
	if !ok {
		return l, fail("pin_indisponible")
	}
	return l, nil
}

// depot : ligne, signature puis limite par uid, contrôle anti-retour avant
// un changement de grappe, plafond, grappe_recente, mémoire, grappePin écrit
// en transaction avant la validation (protocole-pin.md, « Écriture sûre »),
// validation par la majorité, recopie, évaluation.
func (a *call) depot() (jcs.Object, error) {
	uid, motif, g := str(a.body, "uid"), str(a.body, "motif"), a.in.grappe
	id, elem := g.ID(), raw(a.body, "elementAveugle")
	l, err := a.pinLine(uid)
	if err != nil {
		return nil, err
	}
	msg, err := vie.PinDepositMessage(uid, str(a.body, "grappeId"), elem, str(a.body, "defi"), motif)
	if err := a.signed(l, msg, err); err != nil {
		return nil, err
	}
	if err := a.limitKey(uid); err != nil {
		return nil, err
	}
	gp := sub(l.Doc, "grappePin")
	other := str(gp, "grappeId") != id && motif != "ouverture"
	rec, has := g.Record(uid)
	switch {
	case other && !a.in.borne.UpToDate(a.ctx):
		return nil, fail("reessayer_plus_tard")
	case has && rec.Left == 0 && motif != "nouveau_pin":
		return nil, fail("pin_bloque")
	case other && gp != nil && a.now.Before(at(gp, "depuisLe").AddDate(0, 0, recentDays)):
		return nil, fail("grappe_recente")
	case !has && g.Full(), other && a.edit(titulaire(uid), func(cur registre.Line) (jcs.Object, error) {
		if cur.State != registre.Valid || stampOf(cur) != stampOf(l) {
			return nil, registre.ErrUnavailable
		}
		cur.Doc["grappePin"] = jcs.Object{"grappeId": id, "depuisLe": stamp(a.now)}
		return cur.Doc, nil
	}) != nil:
		return nil, fail("pin_indisponible_temporaire")
	}
	var key []byte
	if motif != "ouverture" {
		if key, err = a.in.cfg.Alea.Read(32); err == nil {
			key, err = pin.NewKey(key)
		}
		if err != nil {
			return nil, fail("pin_indisponible_temporaire")
		}
	}
	code, rec, err := g.Propose(a.ctx, pin.Op{UID: uid, Motif: motif, Key: key, At: a.now})
	switch {
	case err != nil:
		return nil, fail("pin_indisponible_temporaire")
	case code != "":
		return nil, fail(code)
	case motif == "ouverture":
		return jcs.Object{"evaluation": nil, "grappeId": id}, nil
	}
	if err := a.recopy(uid, rec, motif); err != nil {
		return nil, err
	}
	ev, err := pin.Evaluate(rec.Key, elem)
	if err != nil {
		return nil, fail("pin_indisponible_temporaire")
	}
	return jcs.Object{"evaluation": b64(ev), "grappeId": id}, nil
}

// essai : limite par uid, ligne, enregistrement, plafond et délai, essai
// décompté et validé par la majorité, alerte au titulaire (au plus une par
// heure, au leader), recopie, puis évaluation ; le 10ᵉ essai n'est pas
// évalué (R-PIN-02).
func (a *call) essai() (jcs.Object, error) {
	uid, g := str(a.body, "uid"), a.in.grappe
	if err := a.limitKey(uid); err != nil {
		return nil, err
	}
	if _, err := a.pinLine(uid); err != nil {
		return nil, err
	}
	rec, has := g.Record(uid)
	switch {
	case !has:
		return nil, fail("pin_indisponible")
	case rec.Left == 0:
		return nil, fail("pin_bloque")
	case a.now.Before(rec.Next):
		return nil, failAt("trop_d_essais", "reessayerLe", rec.Next)
	}
	code, rec, err := g.Propose(a.ctx, pin.Op{UID: uid, At: a.now})
	switch {
	case err != nil:
		return nil, fail("pin_indisponible_temporaire")
	case code == "trop_d_essais":
		return nil, failAt(code, "reessayerLe", rec.Next)
	case code != "":
		return nil, fail(code)
	}
	if ok, _ := a.in.local.hit("alerte_pin|"+uid, limit{1, hour}, a.now); ok {
		a.in.alert(a.ctx, uid, "alerte_pin", "pin", "date", parisTime(a.now))
	}
	if err := a.recopy(uid, rec, ""); err != nil {
		return nil, err
	}
	if rec.Left == 0 {
		return nil, fail("pin_bloque")
	}
	ev, err := pin.Evaluate(rec.Key, raw(a.body, "elementAveugle"))
	if err != nil {
		return nil, fail("pin_indisponible_temporaire")
	}
	return jcs.Object{"evaluation": b64(ev)}, nil
}

// recopy écrit le dernier état connu du PIN dans la ligne, avant la réponse
// (registre.md : pinDeposeLe, pinBloqueLe, pinEvaluations, compteur entier).
func (a *call) recopy(uid string, rec pin.Record, motif string) error {
	err := a.edit(titulaire(uid), func(l registre.Line) (jcs.Object, error) {
		if l.State != registre.Valid {
			return nil, registre.ErrUnavailable
		}
		if motif != "" {
			l.Doc["pinDeposeLe"] = stamp(a.now)
		}
		if motif == "nouveau_pin" {
			delete(l.Doc, "pinBloqueLe")
		}
		if rec.Left == 0 {
			l.Doc["pinBloqueLe"] = stamp(a.now)
		}
		l.Doc["pinEvaluations"] = rec.Evals
		return l.Doc, nil
	})
	if err != nil {
		return fail("pin_indisponible_temporaire")
	}
	return nil
}

// pinState rend pinPresent, pinBloque et le compteur des évaluations : hors
// pause, la copie appliquée par cette VM ; pendant une pause, le dernier état
// connu de la ligne s'il vient de cette grappe ; sinon, et sur une VM sans
// grappe, les valeurs neutres (API.md §2.4, §2.5).
func (a *call) pinState(l registre.Line) (bool, bool, int64) {
	g, uid := a.in.grappe, str(a.body, "uid")
	id := g.ID()
	switch {
	case id != "" && !g.Paused():
		rec, has := g.Record(uid)
		return has, has && rec.Left == 0, rec.Evals
	case id == "" || str(sub(l.Doc, "grappePin"), "grappeId") != id:
		return true, false, 0
	}
	return l.Doc["pinDeposeLe"] != nil, l.Doc["pinBloqueLe"] != nil, num(l.Doc, "pinEvaluations")
}

// membres suit §4.2 : jeton d'exploitation vérifié par la VM qui reçoit, puis
// la commande au leader.
func (a *call) membres() (jcs.Object, error) {
	if err := a.oidc("jetonExploitation", a.in.cfg.reglages.CompteExploitation); err != nil {
		return nil, err
	}
	if a.in.event("grappe_membres_commandee", a.in.cfg.reglages.CompteExploitation); a.in.grappe.ID() == "" {
		return nil, fail("membres_refuses")
	}
	return a.toLeader()
}

// amorcer suit §4.3.
func (a *call) amorcer() (jcs.Object, error) {
	if err := a.oidc("jetonExploitation", a.in.cfg.reglages.CompteExploitation); err != nil {
		return nil, err
	}
	if err := a.limits(); err != nil {
		return nil, err
	}
	out, code := a.in.grappe.Amorcer(a.ctx, a.in.cfg.reglages.CompteExploitation)
	if code != "" {
		return nil, fail(code)
	}
	return out, nil
}
