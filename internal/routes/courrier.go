package routes

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/cles"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/lettres"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
)

// Suivi des lettres (docs/technique/sesame/protocoles.md, « Suivi ») :
// renvois 8 et 38 jours après le premier envoi accepté, plus aucun envoi à
// la 3ᵉ erreur ; une invitation vaut 12 mois après la transmission.
var resendAfter = []time.Duration{8 * day, 38 * day}

const (
	maxErrors     = 3
	maxTitulaires = 100_000 // choix de ce dépôt : titulaires parcourus par la tâche horaire
	maxContacts   = 1_000
)

func inviteExpiry(transmissible time.Time) string {
	return stamp(transmissible.In(paris).AddDate(1, 0, 0))
}

// open rend le signalement dont la transmission est en cours (à terme, non
// annulé, non clos), le plus ancien, ou nil.
func open(reports []registre.Line, now time.Time) *registre.Line {
	var first *registre.Line
	for i, s := range reports {
		if s.Doc["annuleLe"] == nil && s.Doc["closLe"] == nil && !now.Before(at(s.Doc, "transmissibleLe")) &&
			(first == nil || at(s.Doc, "signaleLe").Before(at(first.Doc, "signaleLe"))) {
			first = &reports[i]
		}
	}
	return first
}

// post produit et commande la lettre du proche de la charte c, pour la
// transmission report (« Lettre », étapes 2 à 5). code vide : lettre neuve,
// nouveau code d'invitation tiré, scellé et inscrit avec invitations/{lookup}
// et lettres/{contactId}, envoi réservé ; sinon réimpression du code descellé
// (nouvel envoi, renvoi ou reprise), sans nouvelle inscription.
func (a *call) post(uid string, report *registre.Line, c cles.Charter, code string) error {
	contact := str(c, "contactId")
	letterPath := a.path(uid, "lettres", contact)
	if code == "" {
		rnd, err := a.in.cfg.Alea.Read(12)
		if err != nil {
			return err
		}
		code, _ = cles.DrawCode(rnd)
		lookup := cles.IndexAcces(code)
		aad, _ := cles.InvitationAAD(uid, contact, lookup)
		sealed, err := a.in.cfg.KMS.Encrypt(a.ctx, ports.InvitationSeal, []byte(code), aad)
		if err != nil {
			return registre.ErrUnavailable
		}
		err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
			w.Put("invitations/"+lookup, jcs.Object{"uid": uid, "contactId": contact, "expireLe": inviteExpiry(at(report.Doc, "transmissibleLe"))})
			w.Put(letterPath, jcs.Object{"invitationLookup": lookup, "codeInvitationScelle": b64(sealed), "reserveeLe": stamp(w.Now),
				"erreurs": 0, "renvoyeeLe": jcs.Array{}})
			return nil
		})
		if err != nil {
			return err
		}
	}
	name := "lettre_deces"
	if !isDeath(*report) {
		name = "lettre_inactivite"
	}
	_, text := a.in.render(name, "titulaire", str(c, "titulaireAffiche"), "prenom", str(c, "prenom"), "code", cles.DisplayCode(code))
	pdf := lettres.PDF(lettres.Letter{Address: strings.Split(str(c, "adresse"), "\n"), Text: text, Code: cles.DisplayCode(code)})
	id, err := a.in.cfg.Courrier.Send(a.ctx, a.in.cfg.reglages.CourrierOrganisation, pdf)
	if err != nil {
		return fail("fournisseur_indisponible")
	}
	err = a.edit(letterPath, func(l registre.Line) (jcs.Object, error) {
		if l.State != registre.Valid {
			return nil, registre.ErrUnavailable
		}
		if l.Doc["envoyeeLe"] == nil {
			l.Doc["envoyeeLe"] = stamp(a.now)
		}
		delete(l.Doc, "reserveeLe")
		l.Doc["pingenDocumentId"], l.Doc["statut"] = id, "commandee"
		return l.Doc, nil
	})
	if err == nil {
		a.in.notif.Notify(a.ctx, "lettreEnvoyee", jcs.Object{"uid": uid, "contactId": contact, "pingenDocumentId": id})
	}
	return err
}

// belongs dit si la lettre d est de la transmission report : envoyée, ou
// réservée, depuis son signalement (une lettre d'une transmission close ne
// compte pas, « D'une transmission à la suivante »).
func belongs(d jcs.Object, report *registre.Line) bool {
	sent := at(d, "envoyeeLe")
	if sent.IsZero() {
		sent = at(d, "reserveeLe")
	}
	return !sent.Before(at(report.Doc, "signaleLe"))
}

// postable dit si la charte c peut recevoir une lettre : adresse, prénom,
// nom et date de naissance du proche (R-PROCHE-18).
func postable(c cles.Charter) bool {
	return c["adresse"] != nil && c["nom"] != nil && c["dateNaissance"] != nil && str(c, "prenom") != ""
}

// secours suit §2.10 : la lettre de secours d'un proche dont chaque canal a
// échoué ; un refus de lettre rend commandee: false, motif journalisé.
func (a *call) secours() (jcs.Object, error) {
	uid, contact := str(a.body, "uid"), str(a.body, "contactId")
	if err := a.oidc("jetonFatoom", a.in.cfg.reglages.CompteDeces); err != nil {
		return nil, err
	}
	if err := a.limitKey("compte"); err != nil {
		return nil, err
	}
	l, marker, err := a.line(uid)
	if err == nil {
		err = a.unavailable(l, marker, true)
	}
	if err != nil {
		return nil, err
	}
	if !a.in.borne.UpToDate(a.ctx) {
		return nil, fail("reessayer_plus_tard")
	}
	refuse := func(motif string) (jcs.Object, error) {
		a.in.log("lettre_refusee", uid, motif)
		return jcs.Object{"commandee": false}, nil
	}
	reports, err := a.reports(uid)
	letter, err2 := a.in.reg.Read(a.ctx, a.path(uid, "lettres", contact))
	report := open(reports, a.now)
	switch {
	case err != nil || err2 != nil:
		return nil, or(err, err2)
	case report == nil:
		return refuse("sans_transmission")
	case letter.State != registre.Absent && belongs(letter.Doc, report):
		return refuse("deja_commandee")
	}
	b, err := a.bundle("uid="+uid+"&contactId="+contact, bundleFields)
	if err != nil || b == nil {
		return or2(err, refuse, "bundle_refuse")
	}
	if b["uid"] != uid || b["contactId"] != contact {
		return nil, fail("passerelle_indisponible")
	}
	c, kts, err := a.charter(b, b, l)
	clear(kts)
	if _, denied := err.(denied); denied || err == nil && !postable(c) {
		return refuse("charte")
	} else if err != nil {
		return nil, err
	}
	if err := a.post(uid, report, c, ""); err != nil {
		return nil, err
	}
	return jcs.Object{"commandee": true}, nil
}

func or2(err error, refuse func(string) (jcs.Object, error), motif string) (jcs.Object, error) {
	if err != nil {
		return nil, err
	}
	return refuse(motif)
}

// hourly est la tâche horaire de l'instance (docs/technique/sesame/
// protocoles.md, « Qui fait la tâche ») : sans grappe, chaque VM la fait ;
// chaque envoi est réservé au registre avant d'être fait.
func (in *Instance) hourly(ctx context.Context) {
	a := &call{ctx: ctx, in: in, now: in.cfg.Horloge.Now(), name: "tache"}
	holders, err := a.list("titulaires", maxTitulaires)
	instances, err2 := a.list("instances", maxTitulaires)
	if err = or(err, err2); err != nil {
		in.log("tache_horaire", "", "registre_indisponible")
		return
	}
	seen := map[string]time.Time{}
	for _, i := range instances {
		if i.State == registre.Valid {
			seen[lastSegment(i.Path)] = at(i.Doc, "vuLe")
		}
	}
	in.mu.Lock() // sessions comptées des instances au vuLe de plus de 2 minutes (API.md §7)
	for k, l := range in.counted {
		in.counted[k] = slices.DeleteFunc(l, func(i string) bool { return a.now.Sub(seen[i]) > 2*time.Minute })
	}
	in.mu.Unlock()
	for _, h := range holders {
		uid := lastSegment(h.Path)
		reports, err := a.reports(uid)
		if err == nil {
			a.lost(uid, seen)
			a.redactions(uid)
		}
		if err != nil || h.State != registre.Valid || h.Doc["etat"] == "supprime" {
			continue
		}
		a.reminders(uid, reports)
		if in.borne.UpToDate(ctx) {
			a.letters(uid, h, reports)
		}
	}
}

// reserve inscrit dans le document path ce que f y écrit, si f le rend ;
// faux si la réservation est déjà faite (f rend nil) ou impossible.
func (a *call) reserve(path string, f func(doc jcs.Object) bool) bool {
	done := false
	_ = a.edit(path, func(l registre.Line) (jcs.Object, error) {
		if done = l.State == registre.Valid && f(l.Doc); !done {
			return nil, nil
		}
		return l.Doc, nil
	})
	return done
}

// reminders envoie les rappels J3 et J6 d'un signalement non annulé et non
// clos, réservés dans alertesEnvoyeesLe (versions-et-alertes.md).
func (a *call) reminders(uid string, reports []registre.Line) {
	for _, s := range reports {
		sent, _ := s.Doc["alertesEnvoyeesLe"].(jcs.Array)
		if n := len(sent); s.Doc["annuleLe"] != nil || s.Doc["closLe"] != nil || n < 1 || n > 2 ||
			a.now.Before(at(s.Doc, "signaleLe").Add(time.Duration(n)*72*time.Hour)) {
			continue
		}
		if a.reserve(s.Path, func(d jcs.Object) bool {
			l, _ := d["alertesEnvoyeesLe"].(jcs.Array)
			d["alertesEnvoyeesLe"] = append(l, stamp(a.now))
			return len(l) == len(sent)
		}) {
			a.in.alertReport(a.ctx, uid, s, true)
		}
	}
}

// alertReport envoie l'alerte d'un signalement inscrit (J0) ou un rappel.
func (in *Instance) alertReport(ctx context.Context, uid string, s registre.Line, reminder bool) {
	name := "alerte_signalement_" + str(s.Doc, "source")
	if reminder {
		name += "_rappel"
	}
	in.alert(ctx, uid, name, lastSegment(s.Path), "signaleLe", parisTime(at(s.Doc, "signaleLe")),
		"transmissibleLe", parisDay(at(s.Doc, "transmissibleLe")), "preuveNom", str(s.Doc, "preuveNom"), "empreinte", str(s.Doc, "empreintePreuve"))
}

// lost notifie sessionTerminee pour chaque ouverture sans fermeeLe dont
// l'instance n'a pas écrit vuLe depuis 2 minutes ou n'a pas de document
// (registre.md, « Autres précisions »), fermeeLe = ce vuLe, écrit une fois.
func (a *call) lost(uid string, seen map[string]time.Time) {
	opened, _ := a.list(titulaire(uid)+"/ouvertures", maxOpenings)
	for _, o := range opened {
		vu, ok := seen[str(o.Doc, "instanceId")]
		if o.Doc["fermeeLe"] != nil || ok && a.now.Sub(vu) <= 2*time.Minute {
			continue
		}
		if !ok {
			vu = a.now
		}
		if a.reserve(o.Path, func(d jcs.Object) bool { d["fermeeLe"] = stamp(vu); return true }) {
			a.in.notif.Notify(a.ctx, "sessionTerminee", jcs.Object{"uid": uid, "contactId": str(o.Doc, "contactId"),
				"ouvertLe": str(o.Doc, "ouvertLe"), "fermeeLe": stamp(vu)})
		}
	}
}

// redactions représente chaque suppression refusée par Stripe pendant 7
// jours, sans réservation, puis l'abandonne (registre.md, identites).
func (a *call) redactions(uid string) {
	docs, _ := a.list(titulaire(uid)+"/identites", maxContacts)
	for _, d := range docs {
		pend, _ := d.Doc["redactionsEnAttente"].(jcs.Array)
		var keep jcs.Array
		for _, p := range pend {
			r, _ := p.(jcs.Object)
			switch {
			case a.in.cfg.Identite.Redact(a.ctx, str(r, "verificationId")) == nil:
			case !a.now.Before(at(r, "depuisLe").Add(redactFor)):
				a.in.log("redaction_identite_abandonnee", uid, "")
			default:
				keep = append(keep, r)
			}
		}
		if len(keep) != len(pend) && d.State == registre.Valid {
			a.reserve(d.Path, func(doc jcs.Object) bool { doc["redactionsEnAttente"] = append(jcs.Array{}, keep...); return true })
		}
	}
}

// letters fait les lettres d'un titulaire : premières lettres d'une
// transmission (« Lettre »), prolongation des invitations après un décès
// vérifié pendant une inactivité transmise (« Transmission après décès »,
// étape 7), puis suivi, nouveaux envois, renvois et reprises.
func (a *call) letters(uid string, h registre.Line, reports []registre.Line) {
	first := open(reports, a.now)
	for _, s := range reports {
		if s.Doc["annuleLe"] != nil || s.Doc["closLe"] != nil || a.now.Before(at(s.Doc, "transmissibleLe")) || s.Doc["lettresEnvoyeesLe"] != nil ||
			!a.reserve(s.Path, func(d jcs.Object) bool { d["lettresEnvoyeesLe"] = stamp(a.now); return true }) {
			continue
		}
		if first.Path != s.Path { // décès vérifié après une inactivité transmise : aucune lettre
			a.prolong(uid, at(s.Doc, "transmissibleLe"))
			continue
		}
		b, err := a.bundle("uid="+uid+"&courrier=1", []field{uidField, req("sealedKts", isText), req("ktsGeneration", gen1),
			req("titulaireAffiche", isText), req("chartes", arrayOf(maxContacts, object(charterFields...)))})
		if err != nil || b == nil {
			a.reserve(s.Path, func(d jcs.Object) bool { delete(d, "lettresEnvoyeesLe"); return true })
			continue
		}
		for _, v := range b["chartes"].(jcs.Array) {
			c, kts, err := a.charter(b, v.(jcs.Object), h)
			clear(kts)
			if err != nil || c["courrier"] != true || !postable(c) {
				a.in.log("lettre_ignoree", uid, "charte")
				continue
			}
			if err := a.post(uid, &s, c, ""); err != nil {
				a.in.log("lettre_non_envoyee", uid, "fournisseur_indisponible")
			}
		}
	}
	a.follow(uid, h, first)
}

// prolong porte expireLe de chaque invitation du titulaire à transmissibleLe
// du décès + 12 mois (R-VIE-07).
func (a *call) prolong(uid string, transmissible time.Time) {
	_ = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		letters, err := w.List(titulaire(uid)+"/lettres", "", "", maxContacts)
		for _, l := range letters {
			inv, err2 := w.Get("invitations/" + str(l.Doc, "invitationLookup"))
			if err = or(err, err2); err == nil && inv.State == registre.Valid {
				inv.Doc["expireLe"] = inviteExpiry(transmissible)
				w.Put(inv.Path, inv.Doc)
			}
		}
		return err
	})
}

// follow suit les lettres de la transmission en cours : état chez le
// prestataire (statutLettre), nouvel envoi après une erreur, lettreEchouee à
// la 3ᵉ, renvois de J+15 et J+45 tant que le code n'a pas servi, reprise
// d'un envoi réservé sans document du prestataire.
func (a *call) follow(uid string, h registre.Line, report *registre.Line) {
	if report == nil {
		return
	}
	docs, _ := a.list(titulaire(uid)+"/lettres", maxContacts)
	for _, l := range docs {
		contact, d := lastSegment(l.Path), l.Doc
		renvois, _ := d["renvoyeeLe"].(jcs.Array)
		if l.State != registre.Valid || !belongs(d, report) || num(d, "erreurs") >= maxErrors {
			continue
		}
		reprint := d["reserveeLe"] != nil
		if id := str(d, "pingenDocumentId"); !reprint && id != "" && d["statut"] != "distribuee" {
			st, err := a.in.cfg.Courrier.Status(a.ctx, a.in.cfg.reglages.CourrierOrganisation, id)
			if err == nil && st != d["statut"] && a.reserve(l.Path, func(doc jcs.Object) bool {
				doc["statut"] = st
				if st == "erreur" {
					doc["erreurs"], doc["reserveeLe"] = num(doc, "erreurs")+1, stamp(a.now)
				}
				return true
			}) {
				a.in.notif.Notify(a.ctx, "statutLettre", jcs.Object{"uid": uid, "contactId": contact, "pingenDocumentId": id, "statut": st})
				if reprint = st == "erreur"; reprint && num(d, "erreurs")+1 >= maxErrors {
					a.in.notif.Notify(a.ctx, "lettreEchouee", jcs.Object{"uid": uid, "contactId": contact})
					continue
				}
			}
		}
		if n := len(renvois); !reprint && d["invitationUtiliseeLe"] == nil && n < len(resendAfter) && !a.now.Before(at(d, "envoyeeLe").Add(resendAfter[n])) {
			reprint = a.reserve(l.Path, func(doc jcs.Object) bool {
				r, _ := doc["renvoyeeLe"].(jcs.Array)
				doc["renvoyeeLe"], doc["reserveeLe"] = append(r, stamp(a.now)), stamp(a.now)
				return len(r) == n
			})
		}
		if reprint {
			a.reprint(uid, h, report, l)
		}
	}
}

// reprint réimprime la lettre de l, code descellé, charte relue à la
// passerelle avec le contrôle de génération.
func (a *call) reprint(uid string, h registre.Line, report *registre.Line, l registre.Line) {
	b, err := a.bundle("uid="+uid+"&contactId="+lastSegment(l.Path), bundleFields)
	if err != nil || b == nil {
		return
	}
	c, kts, err := a.charter(b, b, h)
	clear(kts)
	lookup := str(l.Doc, "invitationLookup")
	aad, _ := cles.InvitationAAD(uid, lastSegment(l.Path), lookup)
	sealed, _ := primitives.B64(str(l.Doc, "codeInvitationScelle"))
	code, err2 := a.in.cfg.KMS.Decrypt(a.ctx, ports.InvitationSeal, sealed, aad)
	if err == nil && err2 == nil && postable(c) {
		err = a.post(uid, report, c, string(code))
	}
	clear(code)
	if err != nil || err2 != nil {
		a.in.log("lettre_non_envoyee", uid, "reimpression")
	}
}
