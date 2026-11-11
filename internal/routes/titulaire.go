package routes

import (
	"encoding/base64"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/canal"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/cles"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

const (
	hour = time.Hour
	day  = 24 * time.Hour
)

var (
	defiField = req("defi", bytesN(32))
	sigField  = req("signature", bytesN(64))
	uidField  = req("uid", isID)
	tokField  = req("idToken", isText)
	sidField  = req("sessionId", isSession)
)

// table est la table des routes du canal servies (API.md §2) ; Routes en
// rend les noms.
var table map[string]route

func init() {
	table = map[string]route{
		"defi": {fields: []field{uidField}, ip: limit{120, hour}, perCh: limit{30, hour}, run: (*call).defi},
		"enroler": {fields: []field{uidField, tokField, req("ktsGeneration", integer(1, maxSafe)), req("kts", bytesN(32)),
			req("cleViePub", bytesN(32)), opt("preuve", object(defiField, sigField)), opt("cleViePubAttendue", bytesN(32))},
			ip: limit{20, hour}, uid: limit{5, hour}, run: (*call).enroler},
		"vie": {fields: []field{uidField, defiField, sigField}, uid: limit{10, day}, run: (*call).vie},
		"etat": {fields: []field{uidField, tokField, req("repere", object(req("ouvertures", nullable(isStamp)), req("evaluations", integer(0, maxSafe))))},
			ip: limit{60, time.Minute}, uid: limit{100, day}, run: (*call).etat},
		"charte/generation": {fields: []field{uidField, req("contactId", isID), req("generation", generation), sigField},
			uid: limit{30, time.Minute}, run: (*call).charteGeneration},
		"alerte/fiche": {fields: []field{uidField, req("generation", integer(1, maxSafe)), req("telephone", nullable(matches(rePhone))),
			req("email", isText), sigField}, uid: limit{10, day}, run: (*call).alerteFiche},
		"titulaire/supprimer": {fields: []field{req("jetonFatoom", isText), uidField}, uid: limit{100, time.Minute}, run: (*call).supprimer},
		"deces/signaler": {fields: []field{req("jetonFatoom", isText), uidField, req("source", oneOf("insee", "acte", "inactivite")),
			opt("dateDeces", matches(reDate)), opt("preuve", matches(reHex64)), opt("preuveNom", matches(reFile))},
			uid: limit{100, time.Minute}, run: (*call).signaler},
		"deces/annuler": {fields: []field{uidField, req("signalementId", matches(reDrawn)), defiField, sigField},
			uid: limit{10, day}, run: (*call).annuler},
		"lettre/secours": {fields: []field{req("jetonFatoom", isText), uidField, req("contactId", isID)}, uid: limit{100, time.Minute}, run: (*call).secours},
		"session/ouvrir": {fields: []field{opt("code", isText), opt("lookup", matches(reHex64)), opt("sessionId", isSession)},
			ip: limit{20, 15 * time.Minute}, run: (*call).ouvrir},
		"session/verifier":          {fields: []field{sidField, req("otp", matches(reDigits))}, perSess: limit{20, 15 * time.Minute}, run: (*call).verifier},
		"session/identite/demarrer": {fields: []field{sidField}, perSess: limit{5, hour}, run: (*call).demarrer},
		"session/identite/verifier": {fields: []field{sidField}, perSess: limit{5, hour}, run: (*call).identiteVerifier},
		"session/identite/bascule":  {fields: []field{sidField}, perSess: limit{5, hour}, run: (*call).bascule},
		"session/ouvrir-cles": {fields: []field{sidField, req("enveloppes", arrayOf(500, object(req("type", oneOf("fichier", "motdepasse", "mot", "volontes")),
			req("id", isID), req("contactId", isID), req("enveloppe", bytesN(48)), req("iv", bytesN(12)))))},
			max: MaxBodyCles, perSess: limit{60, hour}, run: (*call).ouvrirCles},
		"session/fermer":       {fields: []field{sidField}, perCh: limit{10, hour}, run: (*call).fermer},
		"session/refuser-role": {fields: []field{sidField}, perSess: limit{5, hour}, run: (*call).refuserRole},
	}
	pinRoutes(table)
	for k, r := range table {
		r.max = max(r.max, MaxBody)
		table[k] = r
	}
}

func generation(v any) bool { return integer(1, maxSafe)(v) || v == -1.0 }

// Routes rend les noms des routes du canal servies.
func Routes() []string {
	var n []string
	for k := range table {
		n = append(n, k)
	}
	return n
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func titulaire(uid string) string { return "titulaires/" + uid }

func (a *call) challenge(uid string) error {
	if !a.in.table.ConsumeChallenge(a.ch, str(a.body, "defi"), uid, a.now) {
		return fail("defi_invalide")
	}
	return nil
}

// line lit la ligne du titulaire et, absente, son marqueur.
func (a *call) line(uid string) (registre.Line, []byte, error) {
	l, err := a.in.reg.Read(a.ctx, titulaire(uid))
	if err != nil {
		return l, nil, err
	}
	var m []byte
	if l.State == registre.Absent {
		if m, err = a.in.reg.Marker(a.ctx, uid); err == nil && m != nil {
			a.in.borne.Signal("ligne_absente", registre.Masque(uid)+"/titulaires")
		}
	}
	return l, m, err
}

// unavailable rend titulaire_indisponible pour une ligne absente, supprime,
// ou rejetée si rejected ; details.reprise quand la ligne manque, que le
// marqueur existe et qu'une reprise publiée est en cours (API.md §2.4).
func (a *call) unavailable(l registre.Line, marker []byte, rejected bool) error {
	switch {
	case l.State == registre.Absent && marker != nil && a.in.cfg.Reprise != nil && (a.name == "vie" || a.name == "etat"):
		return &refusal{"titulaire_indisponible", jcs.Object{"reprise": true}}
	case l.State == registre.Absent, l.State == registre.Rejected && rejected, l.Doc["etat"] == "supprime":
		return fail("titulaire_indisponible")
	}
	return nil
}

func (a *call) signed(l registre.Line, msg []byte, err error) error {
	pub, _ := primitives.B64(str(l.Doc, "cleViePub"))
	if err != nil || !vie.Verify(pub, msg, str(a.body, "signature")) {
		return fail("message_refuse")
	}
	return nil
}

func (a *call) defi() (jcs.Object, error) {
	if err := a.limits(); err != nil {
		return nil, err
	}
	d, err := a.in.cfg.Alea.Read(32)
	if err != nil {
		return nil, err
	}
	a.in.table.DrawChallenge(a.ch, b64(d), str(a.body, "uid"), a.now)
	return jcs.Object{"defi": b64(d), "expireLe": stamp(a.now.Add(canal.ChallengeLife)), "grappeId": a.in.grappeID()}, nil
}

func (in *Instance) grappeID() any {
	if id := in.grappe.ID(); id != "" {
		return id
	}
	return nil
}

// enroler suit l'ordre des contrôles du §2.3.
func (a *call) enroler() (jcs.Object, error) {
	b, uid := a.body, str(a.body, "uid")
	if err := a.idToken(uid); err != nil {
		return nil, err
	}
	if err := a.limits(); err != nil {
		return nil, err
	}
	l, marker, err := a.line(uid)
	if err != nil {
		return nil, err
	}
	if err := a.unavailable(l, nil, true); err != nil && l.State != registre.Absent {
		return nil, err
	}
	_, hasExpected := b["cleViePubAttendue"]
	if hasExpected && a.in.cfg.Reprise == nil {
		return nil, fail("reprise_absente")
	}
	if hasExpected && (l.State != registre.Absent || marker == nil) {
		return nil, fail("message_refuse")
	}
	proof, hasProof := b["preuve"].(jcs.Object)
	if hasProof && !a.in.table.ConsumeChallenge(a.ch, str(proof, "defi"), uid, a.now) {
		return nil, fail("defi_invalide")
	}
	gen, pub, inscribed := num(b, "ktsGeneration"), raw(b, "cleViePub"), raw(l.Doc, "cleViePub")
	proves := func(key []byte) bool {
		msg, err := vie.EnrolMessage(uid, str(proof, "defi"), gen, pub)
		return hasProof && err == nil && vie.Verify(key, msg, str(proof, "signature"))
	}
	switch {
	case l.State == registre.Valid && !proves(inscribed), hasExpected && !proves(raw(b, "cleViePubAttendue")):
		return nil, fail("preuve_requise")
	case l.State == registre.Absent && !hasExpected && marker != nil && (!proves(marker) || !primitives.Equal(pub, marker)):
		return nil, fail("titulaire_indisponible")
	case l.State == registre.Absent && marker == nil && hasProof && !proves(pub):
		return nil, fail("message_refuse")
	}
	if err := a.limitKey(uid); err != nil {
		return nil, err
	}
	current := num(l.Doc, "ktsGeneration")
	if l.State == registre.Valid && gen < current {
		return nil, fail("generation_non_croissante")
	}
	if l.State == registre.Valid && gen == current && !primitives.Equal(pub, inscribed) {
		return nil, fail("message_refuse")
	}
	aad, _ := cles.SealedKTSAAD(uid, gen)
	kts := raw(b, "kts")
	sealed, err := a.in.cfg.KMS.Encrypt(a.ctx, ports.KtsSeal, kts, aad)
	clear(kts)
	if err != nil {
		return nil, err
	}
	if l.State == registre.Absent && marker == nil {
		if err := a.in.reg.CreateMarker(a.ctx, uid, pub); err != nil {
			return nil, err
		}
	}
	var enroleLe string
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		cur, err := w.Get(titulaire(uid))
		if err != nil || cur.State != l.State || stampOf(cur) != stampOf(l) {
			return registre.ErrUnavailable
		}
		doc := cur.Doc
		if doc == nil {
			now := stamp(w.Now)
			doc = jcs.Object{"enroleLe": now, "vivantLe": now, "chartes": jcs.Object{}, "rolesRefuses": jcs.Object{},
				"etat": "actif", "pinEvaluations": 0}
		}
		doc["ktsGeneration"], doc["cleViePub"] = gen, b64(pub)
		enroleLe = str(doc, "enroleLe")
		w.Put(titulaire(uid), doc)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if l.State == registre.Valid && gen > current { // rotation : sessions des proches fermées (§2.3)
		a.in.closeSessions(uid, "", a.now, nil)
	}
	return jcs.Object{"sealedKts": b64(sealed), "ktsGeneration": gen, "imageDigest": a.in.cfg.ImageDigest, "cleViePub": b64(pub), "enroleLe": enroleLe}, nil
}

func stampOf(l registre.Line) string { return str(l.Doc, "ecritLe") }

func (a *call) vie() (jcs.Object, error) {
	uid := str(a.body, "uid")
	if err := a.challenge(uid); err != nil {
		return nil, err
	}
	l, marker, err := a.line(uid)
	if err != nil {
		return nil, err
	}
	if err := a.unavailable(l, marker, false); err != nil {
		return nil, err
	}
	msg, err := vie.LifeMessage(uid, str(a.body, "defi"))
	if err := a.signed(l, msg, err); err != nil {
		return nil, err
	}
	if err := a.limitKey(uid); err != nil {
		return nil, err
	}
	var out jcs.Object
	var notes []note
	retour := false
	if l.State == registre.Rejected {
		reports, opened, err := a.history(uid, str(l.Doc, "vivantLe"))
		return a.lifeAnswer(l.Doc, reports, opened, false), err
	}
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		cur, err := w.Get(titulaire(uid))
		if err != nil || cur.State != registre.Valid {
			return registre.ErrUnavailable
		}
		reports, err := w.List(titulaire(uid)+"/signalements", "", "", maxReports)
		if err != nil {
			return err
		}
		previous := str(cur.Doc, "vivantLe")
		opened, err := w.List(titulaire(uid)+"/ouvertures", "ouvertLe", previous, maxOpenings)
		if err != nil {
			return err
		}
		notes = nil
		closed := false
		for _, s := range reports {
			if str(s.Doc, "source") != "inactivite" || s.Doc["annuleLe"] != nil || s.Doc["closLe"] != nil {
				continue
			}
			id := lastSegment(s.Path)
			if w.Now.Before(at(s.Doc, "transmissibleLe")) {
				s.Doc["annuleLe"], s.Doc["annulePar"] = stamp(w.Now), "preuve_de_vie"
				notes = append(notes, note{"signalementAnnule", jcs.Object{"uid": uid, "signalementId": id, "annulePar": "preuve_de_vie", "annuleLe": stamp(w.Now)}})
			} else {
				s.Doc["closLe"], s.Doc["closPar"], closed = stamp(w.Now), "retour", true
				notes = append(notes, note{"vieApresTransmission", jcs.Object{"uid": uid, "signalementId": id, "vivantLe": stamp(w.Now)}})
			}
			w.Put(s.Path, s.Doc)
		}
		cur.Doc["vivantLe"] = stamp(w.Now)
		w.Put(cur.Path, cur.Doc)
		out, retour = a.lifeAnswer(cur.Doc, reports, opened, closed), closed
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		a.in.notif.Notify(a.ctx, n.name, n.body)
	}
	if retour { // retour du titulaire : sessions de ses proches fermées (§2.4)
		a.in.closeSessions(uid, "", a.now, nil)
	}
	return out, nil
}

type note struct {
	name string
	body jcs.Object
}

// Plafonds des lectures de collections d'un titulaire (choix de ce dépôt).
const (
	maxReports  = 1_000
	maxOpenings = 10_000
)

// lifeAnswer compose la réponse de vie (API.md §2.4) ; closedNow dit que
// cette preuve vient de clore une inactivité transmise.
func (a *call) lifeAnswer(doc jcs.Object, reports, opened []registre.Line, closedNow bool) jcs.Object {
	present, blocked, _ := a.pinState(registre.Line{Doc: doc})
	return jcs.Object{"vivantLe": str(doc, "vivantLe"), "ktsGeneration": num(doc, "ktsGeneration"), "signalement": reportOut(current(reports)),
		"transmis": closedNow || transmitted(reports, a.now), "pinPresent": present, "pinBloque": blocked, "ouvertures": openings(opened)}
}
