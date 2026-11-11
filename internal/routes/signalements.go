package routes

import (
	"encoding/base64"
	"strings"
	"time"
	_ "time/tzdata" // heure de Paris dans une image sans fuseaux (registre.md, « Durées en mois »)

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

var paris, _ = time.LoadLocation("Europe/Paris")

// Transmissible rend 23 h 59 min 59 s, heure de Paris, du 7ᵉ jour calendaire
// après la date de signaleLe à Paris (API.md §2.8).
func Transmissible(signaled time.Time) time.Time {
	y, m, d := signaled.In(paris).Date()
	return time.Date(y, m, d+7, 23, 59, 59, 0, paris).UTC()
}

// InactivityFloor rend vivantLe + 3 mois calendaires + 30 jours, heure de
// Paris : avant, un signalement d'inactivité est refusé (API.md §2.8).
func InactivityFloor(alive time.Time) time.Time {
	return alive.In(paris).AddDate(0, 3, 0).AddDate(0, 0, 30).UTC()
}

func at(o jcs.Object, k string) time.Time {
	t, _ := primitives.ReadTime(str(o, k))
	return t
}

func lastSegment(p string) string { return p[strings.LastIndex(p, "/")+1:] }

func isDeath(s registre.Line) bool { return s.Doc["deces"] == true }

// current rend le signalement courant : le dernier décès non annulé s'il en
// existe un, sinon le dernier inscrit (API.md §2.4) ; nil sans signalement.
func current(reports []registre.Line) *registre.Line {
	var last, death *registre.Line
	for i := range reports {
		s := &reports[i]
		if last == nil || at(s.Doc, "signaleLe").After(at(last.Doc, "signaleLe")) {
			last = s
		}
		if isDeath(*s) && s.Doc["annuleLe"] == nil && (death == nil || at(s.Doc, "signaleLe").After(at(death.Doc, "signaleLe"))) {
			death = s
		}
	}
	if death != nil {
		return death
	}
	return last
}

func reportOut(s *registre.Line) any {
	if s == nil {
		return nil
	}
	return jcs.Object{"signalementId": lastSegment(s.Path), "source": s.Doc["source"], "signaleLe": s.Doc["signaleLe"],
		"transmissibleLe": s.Doc["transmissibleLe"], "annuleLe": s.Doc["annuleLe"], "closLe": s.Doc["closLe"]}
}

// transmitted dit si un signalement non annulé et non clos est à terme.
func transmitted(reports []registre.Line, now time.Time) bool {
	for _, s := range reports {
		if s.Doc["annuleLe"] == nil && s.Doc["closLe"] == nil && !now.Before(at(s.Doc, "transmissibleLe")) {
			return true
		}
	}
	return false
}

func openings(opened []registre.Line) jcs.Array {
	out := jcs.Array{}
	for _, o := range opened {
		out = append(out, jcs.Object{"contactId": str(o.Doc, "contactId"), "role": str(o.Doc, "role"), "ouvertLe": str(o.Doc, "ouvertLe")})
	}
	return out
}

// history lit les signalements du titulaire et ses ouvertures postérieures
// à since, sans rien écrire.
func (a *call) history(uid, since string) (reports, opened []registre.Line, err error) {
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		if reports, err = w.List(titulaire(uid)+"/signalements", "", "", maxReports); err != nil {
			return err
		}
		opened, err = w.List(titulaire(uid)+"/ouvertures", "ouvertLe", since, maxOpenings)
		return err
	})
	return reports, opened, err
}

func (a *call) etat() (jcs.Object, error) {
	uid := str(a.body, "uid")
	if err := a.idToken(uid); err != nil {
		return nil, err
	}
	if err := a.limits(); err != nil {
		return nil, err
	}
	if err := a.limitKey(uid); err != nil {
		return nil, err
	}
	l, marker, err := a.line(uid)
	if err != nil {
		return nil, err
	}
	if err := a.unavailable(l, marker, true); err != nil {
		return nil, err
	}
	mark := sub(a.body, "repere")
	since, _ := mark["ouvertures"].(string)
	reports, opened, err := a.history(uid, since)
	if err != nil {
		return nil, err
	}
	present, blocked, count := a.pinState(l)
	seen := num(mark, "evaluations")
	if seen > count {
		seen = 0
	}
	for _, o := range opened {
		if s := str(o.Doc, "ouvertLe"); s > since {
			since = s
		}
	}
	var sinceOut any
	if since != "" {
		sinceOut = since
	}
	jws, err := a.in.signJWS(a.ctx, false, jcs.Object{"iss": "sesame", "typ": "etat", "uid": uid, "iat": a.now.Unix(),
		"signalement": reportOut(current(reports)), "ouvertures": openings(opened), "pinPresent": present, "pinBloque": blocked,
		"evaluationsDepuis": count - seen, "repere": jcs.Object{"ouvertures": sinceOut, "evaluations": max(count, num(mark, "evaluations"))}})
	if err != nil {
		return nil, err
	}
	return jcs.Object{"etat": jws}, nil
}

// signedLine lit la ligne d'un titulaire pour une route signée par cleVie
// (étapes 5 et 6 du §6) et vérifie la signature de msg ; rejected admet une
// ligne rejetée (vie, deces/annuler).
func (a *call) signedLine(uid string, msg []byte, msgErr error, rejected bool) (registre.Line, error) {
	l, marker, err := a.line(uid)
	if err != nil {
		return l, err
	}
	if err := a.unavailable(l, marker, !rejected); err != nil {
		return l, err
	}
	if err := a.signed(l, msg, msgErr); err != nil {
		return l, err
	}
	return l, a.limitKey(uid)
}

func (a *call) charteGeneration() (jcs.Object, error) {
	uid, contact, gen := str(a.body, "uid"), str(a.body, "contactId"), num(a.body, "generation")
	msg, err := vie.GenerationMessage(uid, contact, gen)
	if _, err := a.signedLine(uid, msg, err, false); err != nil {
		return nil, err
	}
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		cur, err := w.Get(titulaire(uid))
		if err != nil || cur.State != registre.Valid || cur.Doc["etat"] == "supprime" {
			return registre.ErrUnavailable
		}
		charters := sub(cur.Doc, "chartes")
		if charters == nil {
			charters = jcs.Object{}
			cur.Doc["chartes"] = charters
		}
		if old, ok := charters[contact].(float64); ok && (old == vie.GenerationDeleted || gen != vie.GenerationDeleted && float64(gen) <= old) {
			return fail("generation_non_croissante")
		}
		charters[contact] = gen
		w.Put(cur.Path, cur.Doc)
		return nil
	})
	if err != nil {
		return nil, err
	}
	recu, err := a.receipt("generation", jcs.Object{"uid": uid, "contactId": contact, "generation": gen})
	return jcs.Object{"contactId": contact, "generation": gen, "recu": recu}, err
}

func (a *call) alerteFiche() (jcs.Object, error) {
	uid, gen := str(a.body, "uid"), num(a.body, "generation")
	fiche := vie.Alert{UID: uid, Generation: gen, Email: str(a.body, "email")}
	if p, ok := a.body["telephone"].(string); ok {
		fiche.Telephone = &p
	}
	plain, err := fiche.JSON()
	if err != nil {
		return nil, fail("message_refuse")
	}
	msg, err := fiche.Message()
	if _, err := a.signedLine(uid, msg, err, false); err != nil {
		return nil, err
	}
	aad, _ := vie.AlertAAD(uid, gen)
	sealed, err := a.in.cfg.KMS.Encrypt(a.ctx, ports.AlerteSeal, plain, aad)
	if err != nil {
		return nil, err
	}
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		cur, err := w.Get(titulaire(uid))
		if err != nil || cur.State != registre.Valid {
			return registre.ErrUnavailable
		}
		if gen <= num(cur.Doc, "ficheGeneration") {
			return fail("generation_non_croissante")
		}
		cur.Doc["ficheGeneration"], cur.Doc["ficheScellee"] = gen, b64(sealed)
		w.Put(cur.Path, cur.Doc)
		return nil
	})
	return jcs.Object{"ficheGeneration": gen}, err
}

func (a *call) supprimer() (jcs.Object, error) {
	uid := str(a.body, "uid")
	if err := a.oidc("jetonFatoom", a.in.cfg.reglages.CompteDeces); err != nil {
		return nil, err
	}
	if err := a.limitKey("compte"); err != nil {
		return nil, err
	}
	l, marker, err := a.line(uid)
	switch {
	case err != nil:
		return nil, err
	case l.State == registre.Rejected || l.State == registre.Absent && marker != nil:
		return nil, fail("titulaire_indisponible")
	case l.State == registre.Absent:
		return jcs.Object{"etat": "absent"}, nil
	}
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		cur, err := w.Get(titulaire(uid))
		if err != nil || cur.State != registre.Valid {
			return registre.ErrUnavailable
		}
		if cur.Doc["etat"] != "supprime" {
			cur.Doc["etat"] = "supprime"
			w.Put(cur.Path, cur.Doc)
		}
		return nil
	})
	return jcs.Object{"etat": "supprime"}, err
}

func (a *call) signaler() (jcs.Object, error) {
	b, uid, source := a.body, str(a.body, "uid"), str(a.body, "source")
	_, hasDate := b["dateDeces"]
	_, hasProof := b["preuve"]
	_, hasName := b["preuveNom"]
	if death := source != "inactivite"; death != hasDate || death != hasProof || (source == "insee") != hasName {
		return nil, fail("message_refuse")
	}
	if err := a.oidc("jetonFatoom", a.in.cfg.reglages.CompteDeces); err != nil {
		return nil, err
	}
	if err := a.limitKey("compte"); err != nil {
		return nil, err
	}
	l, marker, err := a.line(uid)
	if err != nil {
		return nil, err
	}
	if err := a.unavailable(l, marker, true); err != nil {
		return nil, err
	}
	rnd, err := a.in.cfg.Alea.Read(16)
	if err != nil {
		return nil, err
	}
	var out jcs.Object
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		reports, err := w.List(titulaire(uid)+"/signalements", "", "", maxReports)
		if err != nil {
			return err
		}
		for _, s := range reports {
			if source == "inactivite" && isDeath(s) && s.Doc["annuleLe"] == nil {
				return fail("message_refuse")
			}
		}
		for _, s := range reports {
			if isDeath(s) == (source != "inactivite") && s.Doc["annuleLe"] == nil && s.Doc["closLe"] == nil {
				out = jcs.Object{"signalementId": lastSegment(s.Path), "signaleLe": s.Doc["signaleLe"], "transmissibleLe": s.Doc["transmissibleLe"]}
				return nil
			}
		}
		cur, err := w.Get(titulaire(uid))
		if err != nil || cur.State != registre.Valid {
			return registre.ErrUnavailable
		}
		if source == "inactivite" && w.Now.Before(InactivityFloor(at(cur.Doc, "vivantLe"))) {
			return fail("inactivite_trop_recente")
		}
		id := base64URL(rnd)
		doc := jcs.Object{"signaleLe": stamp(w.Now), "source": source, "deces": source != "inactivite",
			"transmissibleLe": stamp(Transmissible(w.Now)), "alertesEnvoyeesLe": jcs.Array{}}
		if source != "inactivite" {
			doc["dateDeces"], doc["empreintePreuve"] = b["dateDeces"], b["preuve"]
		}
		if source == "insee" {
			doc["preuveNom"] = b["preuveNom"]
		}
		w.Put(titulaire(uid)+"/signalements/"+id, doc)
		out = jcs.Object{"signalementId": id, "signaleLe": doc["signaleLe"], "transmissibleLe": doc["transmissibleLe"]}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out["recu"], err = a.receipt("signalement", jcs.Object{"uid": uid, "signalementId": out["signalementId"], "source": source,
		"signaleLe": out["signaleLe"], "transmissibleLe": out["transmissibleLe"]})
	return out, err
}

func (a *call) annuler() (jcs.Object, error) {
	uid, id := str(a.body, "uid"), str(a.body, "signalementId")
	if err := a.challenge(uid); err != nil {
		return nil, err
	}
	msg, err := vie.CancelMessage(uid, id, str(a.body, "defi"))
	if _, err := a.signedLine(uid, msg, err, true); err != nil {
		return nil, err
	}
	var out jcs.Object
	var notify bool
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		reports, err := w.List(titulaire(uid)+"/signalements", "", "", maxReports)
		if err != nil {
			return err
		}
		s := current(reports)
		switch {
		case s == nil || lastSegment(s.Path) != id || !isDeath(*s):
			return fail("message_refuse")
		case s.Doc["annuleLe"] == nil && !w.Now.Before(at(s.Doc, "transmissibleLe")):
			return fail("deja_transmis")
		case s.Doc["annuleLe"] == nil:
			s.Doc["annuleLe"], s.Doc["annulePar"], notify = stamp(w.Now), "annulation", true
			w.Put(s.Path, s.Doc)
		}
		out = jcs.Object{"signalementId": id, "source": s.Doc["source"], "signaleLe": s.Doc["signaleLe"],
			"transmissibleLe": s.Doc["transmissibleLe"], "annuleLe": s.Doc["annuleLe"]}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if notify {
		a.in.notif.Notify(a.ctx, "signalementAnnule", jcs.Object{"uid": uid, "signalementId": id, "annulePar": "annulation", "annuleLe": out["annuleLe"]})
	}
	recu, err := a.receipt("annulation", jcs.Object{"uid": uid, "signalementId": id, "annuleLe": out["annuleLe"]})
	return jcs.Object{"signalement": out, "recu": recu}, err
}

func base64URL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
