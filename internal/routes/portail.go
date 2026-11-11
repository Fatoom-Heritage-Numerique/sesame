package routes

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/cles"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jetons"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/sessions"
)

// Essais faux (API.md §7) : 5 dans une fenêtre de 15 min, puis blocage de
// 15 min à compter du 5ᵉ ; 3 échecs d'identité, puis blocage de 24 h ;
// dernier code par email valable 10 min ; 3 renvois par session.
const (
	strikeWin, strikeMax = 15 * time.Minute, 5
	idBlock, idMax       = 24 * time.Hour, 3
	otpLife, maxResends  = 10 * time.Minute, 3
	redactFor            = 7 * 24 * time.Hour
)

// tally est un compte d'essais faux : n dans la fenêtre ouverte à since,
// blocage jusqu'à until (docs/technique/sesame/registre.md, essais).
type tally struct {
	n            int64
	since, until time.Time
}

func (t tally) left(now time.Time) int64 {
	if t.n == 0 || !now.Before(t.since.Add(strikeWin)) {
		return strikeMax
	}
	return max(strikeMax-t.n, 0)
}

// fail compte un essai faux à now : le premier après la fin d'une fenêtre
// en ouvre une nouvelle, le 5ᵉ pose le blocage.
func (t *tally) fail(now time.Time) {
	if t.left(now) == strikeMax {
		t.n, t.since = 0, now
	}
	if t.n++; t.n >= strikeMax {
		t.until = now.Add(strikeWin)
	}
}

// Champs d'un compte au document essais/{contactId} : kind "Code" (code
// d'accès ou d'invitation) ou "DernierCode".
func readTally(doc jcs.Object, kind string) tally {
	return tally{num(doc, "essais"+kind), at(doc, "essais"+kind+"DepuisLe"), at(doc, untilKey(kind))}
}

func untilKey(kind string) string { return strings.ToLower(kind[:1]) + kind[1:] + "BloqueJusquau" }

// strike inscrit, signé, un code faux du proche avant la réponse ; une
// écriture refusée rend registre_indisponible (registre.md, essais).
func (a *call) strike(uid, contact, kind string) (t tally, err error) {
	err = a.edit(a.path(uid, "essais", contact), func(l registre.Line) (jcs.Object, error) {
		doc := l.Doc
		if l.State != registre.Valid {
			doc = jcs.Object{}
		}
		t = readTally(doc, kind)
		t.fail(a.now)
		doc["essais"+kind], doc["essais"+kind+"DepuisLe"] = t.n, stamp(t.since)
		if !t.until.IsZero() {
			doc[untilKey(kind)] = stamp(t.until)
		}
		return doc, nil
	})
	return t, err
}

// strikes compte en mémoire les codes faux par IP (API.md §2.14), au plus
// maxKeys IP ; pleine, la table bloque (échec fermé).
type strikes struct {
	mu sync.Mutex
	m  map[string]*tally
}

func (s *strikes) get(ip string, now time.Time, hit bool) tally {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]*tally{}
	}
	t := s.m[ip]
	if t == nil && len(s.m) >= maxKeys {
		for k, o := range s.m {
			if o.left(now) == strikeMax && !now.Before(o.until) {
				delete(s.m, k)
			}
		}
		if len(s.m) >= maxKeys {
			return tally{strikeMax, now, now.Add(strikeWin)}
		}
	}
	if t == nil {
		t = &tally{}
		s.m[ip] = t
	}
	if hit {
		t.fail(now)
	}
	return *t
}

// denied est un code refusé (code_refuse) ; motif est journalisé, jamais
// rendu (API.md §2.14).
type denied struct{ motif string }

func (d denied) Error() string { return "code_refuse" }

func deny(motif string) error { return denied{motif} }

func (a *call) path(uid, coll, id string) string { return titulaire(uid) + "/" + coll + "/" + id }

// edit relit le document path dans une écriture et y inscrit ce que f rend
// (rien si f rend nil).
func (a *call) edit(path string, f func(l registre.Line) (jcs.Object, error)) error {
	return a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		l, err := w.Get(path)
		if err != nil {
			return err
		}
		doc, err := f(l)
		if doc != nil {
			w.Put(path, doc)
		}
		return err
	})
}

// bundle lit une charte à la passerelle (API.md §5.2 à §5.4) : nil pour un
// 404, passerelle_indisponible pour toute réponse hors contrat.
func (a *call) bundle(query string, fields []field) (jcs.Object, error) {
	st, body, err := a.in.notif.Send(a.ctx, "GET", "/bundle?"+query, "", nil)
	if err == nil && st == 404 {
		return nil, nil
	}
	v, perr := jcs.Parse(body)
	o, _ := v.(jcs.Object)
	if err != nil || perr != nil || st != 200 || !valid(o, fields) {
		return nil, fail("passerelle_indisponible")
	}
	return o, nil
}

var (
	gen1          = integer(1, maxSafe)
	charterFields = []field{req("contactId", isID), req("charte", isText), req("charteIv", bytesN(12)), req("charteGeneration", gen1)}
	bundleFields  = append([]field{uidField, req("sealedKts", isText), req("ktsGeneration", gen1), req("isUnlocked", func(v any) bool { _, ok := v.(bool); return ok }),
		req("titulaireAffiche", isText)}, charterFields...)
)

// charter descelle la KTS de b et ouvre la charte du proche contact,
// exigée de la génération du registre (absente : 1 seulement ; -1 : jamais)
// et de la ktsGeneration de la ligne (API.md §2.14, étapes 1 et 2).
func (a *call) charter(b jcs.Object, ch jcs.Object, l registre.Line) (cles.Charter, []byte, error) {
	uid, contact, gen, ktsGen := str(b, "uid"), str(ch, "contactId"), num(ch, "charteGeneration"), num(b, "ktsGeneration")
	want, ok := sub(l.Doc, "chartes")[contact].(float64)
	if !ok {
		want = 1
	}
	if float64(gen) != want || ktsGen != num(l.Doc, "ktsGeneration") {
		return nil, nil, deny("generation")
	}
	aad, _ := cles.SealedKTSAAD(uid, ktsGen)
	kts, err := a.in.cfg.KMS.Decrypt(a.ctx, ports.KtsSeal, raw(b, "sealedKts"), aad)
	if err != nil {
		return nil, nil, registre.ErrUnavailable
	}
	auth, _ := cles.KTSAuth(kts, uid)
	c, err := cles.OpenCharter(auth, raw(ch, "charte"), raw(ch, "charteIv"), uid, contact, gen, ktsGen)
	if err != nil {
		clear(kts)
		return nil, nil, deny("charte")
	}
	return c, kts, nil
}

// transmission rend la base d'une transmission en cours (à terme, non
// annulée, non close) : inactivite d'abord, qui reste la base quand un décès
// est vérifié ensuite, puis deces ; "" sans transmission (API.md §2.15).
func transmission(reports []registre.Line, now time.Time) string {
	base := ""
	for _, s := range reports {
		if s.Doc["annuleLe"] == nil && s.Doc["closLe"] == nil && !now.Before(at(s.Doc, "transmissibleLe")) {
			if !isDeath(s) {
				return "inactivite"
			}
			base = "deces"
		}
	}
	return base
}

func (a *call) reports(uid string) ([]registre.Line, error) {
	return a.list(titulaire(uid)+"/signalements", maxReports)
}

// list lit une collection du registre, hors écriture.
func (a *call) list(coll string, max int) ([]registre.Line, error) {
	var r []registre.Line
	err := a.in.reg.Write(a.ctx, func(w *registre.Writer) (err error) {
		r, err = w.List(coll, "", "", max)
		return err
	})
	return r, err
}

// actif lit la ligne du titulaire : registre illisible ⇒ erreur ; ligne
// absente, rejetée ou supprime ⇒ ok faux.
func (a *call) actif(uid string) (registre.Line, bool, error) {
	l, err := a.in.reg.Read(a.ctx, titulaire(uid))
	return l, err == nil && l.State == registre.Valid && l.Doc["etat"] != "supprime", err
}

// ouvrir suit §2.14 ; {sessionId} seul est « Renvoyer le code » (§2.15).
func (a *call) ouvrir() (jcs.Object, error) {
	_, resend := a.body["sessionId"]
	_, hasCode := a.body["code"]
	_, hasLookup := a.body["lookup"]
	if resend == (hasCode || hasLookup) || hasCode != hasLookup {
		return nil, fail("message_refuse")
	}
	if resend {
		return a.renvoyer()
	}
	if err := a.limits(); err != nil {
		return nil, err
	}
	if t := a.in.strikes.get(a.ip, a.now, false); a.now.Before(t.until) {
		return nil, failAt("trop_d_essais", "reessayerLe", t.until)
	}
	if !a.in.borne.UpToDate(a.ctx) {
		return nil, fail("reessayer_plus_tard")
	}
	code, _ := cles.NormalizeCode(str(a.body, "code"))
	var who [2]string
	s, err := a.admit(code, str(a.body, "lookup"), &who)
	var d denied
	if !errors.As(err, &d) {
		if err != nil {
			if s != nil {
				clear(s.KTS)
			}
			return nil, err
		}
		return a.started(s)
	}
	a.in.log("code_refuse", who[0], d.motif)
	left := a.in.strikes.get(a.ip, a.now, true).left(a.now)
	if who[1] != "" && d.motif != "essais_rejetes" {
		t, err := a.strike(who[0], who[1], "Code")
		if err != nil {
			return nil, err
		}
		left = min(left, t.left(a.now))
	}
	return nil, &refusal{"code_refuse", jcs.Object{"essaisRestants": left}}
}

// essais lit le document d'essais d'un proche désigné par le code : rejeté
// ⇒ code refusé sans vérifier ; blocage en cours ⇒ trop_d_essais.
func (a *call) essais(uid, contact, kind string) (jcs.Object, error) {
	l, err := a.in.reg.Read(a.ctx, a.path(uid, "essais", contact))
	switch t := readTally(l.Doc, kind); {
	case err != nil:
		return nil, err
	case l.State == registre.Rejected:
		return nil, deny("essais_rejetes")
	case a.now.Before(t.until):
		return nil, failAt("trop_d_essais", "reessayerLe", t.until)
	}
	return l.Doc, nil
}

// admit vérifie le code (parcours « courrier » puis code d'accès) et crée la
// session ; who reçoit le proche que le code désigne.
func (a *call) admit(code, lookup string, who *[2]string) (*sessions.Session, error) {
	inv, err := a.in.reg.Read(a.ctx, "invitations/"+lookup)
	if err != nil {
		return nil, err
	}
	s := &sessions.Session{Channel: a.ch.Key, Opened: a.now}
	var b, ch jcs.Object
	var l registre.Line
	if inv.State == registre.Rejected {
		return nil, deny("invitation_rejetee")
	}
	if inv.State == registre.Valid {
		uid, contact := str(inv.Doc, "uid"), str(inv.Doc, "contactId")
		*who = [2]string{uid, contact}
		if _, err := a.essais(uid, contact, "Code"); err != nil {
			return nil, err
		}
		letter, err := a.in.reg.Read(a.ctx, a.path(uid, "lettres", contact))
		if err != nil {
			return nil, err
		}
		if letter.State != registre.Valid || str(letter.Doc, "invitationLookup") != lookup {
			return nil, deny("invitation_remplacee")
		}
		aad, _ := cles.InvitationAAD(uid, contact, lookup)
		sealed, _ := primitives.B64(str(letter.Doc, "codeInvitationScelle"))
		plain, err := a.in.cfg.KMS.Decrypt(a.ctx, ports.InvitationSeal, sealed, aad)
		if err != nil {
			return nil, registre.ErrUnavailable
		}
		match := primitives.Equal(plain, []byte(code)) && code != ""
		clear(plain)
		ok := false
		if l, ok, err = a.actif(uid); err != nil {
			return nil, err
		}
		reports, err := a.reports(uid)
		switch s.Base = transmission(reports, a.now); {
		case err != nil:
			return nil, err
		case !match:
			return nil, deny("code_faux")
		case !a.now.Before(at(inv.Doc, "expireLe")):
			return nil, deny("invitation_expiree")
		case !ok:
			return nil, deny("ligne")
		case s.Base == "":
			return nil, deny("sans_transmission")
		}
		if b, err = a.bundle("uid="+uid+"&contactId="+contact, bundleFields); err != nil || b == nil {
			return nil, or(err, deny("bundle_refuse"))
		}
		if b["uid"] != uid || b["contactId"] != contact {
			return nil, fail("passerelle_indisponible")
		}
		s.Courrier, s.Path, ch = true, "identite", b
		if letter.Doc["identiteVerifieeLe"] != nil {
			s.Path = "aucun"
		}
	} else {
		if b, err = a.bundle("lookup="+lookup, bundleFields); err != nil || b == nil {
			return nil, or(err, deny("bundle_refuse"))
		}
		*who = [2]string{str(b, "uid"), str(b, "contactId")}
		if _, err := a.essais(who[0], who[1], "Code"); err != nil {
			return nil, err
		}
		ok := false
		if l, ok, err = a.actif(who[0]); err != nil || !ok {
			return nil, or(err, deny("ligne"))
		}
		ch = b
	}
	s.UID, s.Contact = who[0], who[1]
	c, kts, err := a.charter(b, ch, l)
	if err != nil {
		return nil, err
	}
	s.KTS, s.KTSGen, s.Gen = kts, num(b, "ktsGeneration"), num(ch, "charteGeneration")
	s.Prenom, s.Nom, s.Naissance, s.Role, s.Titulaire = str(c, "prenom"), str(c, "nom"), str(c, "dateNaissance"), str(c, "role"), str(c, "titulaireAffiche")
	s.Phone, s.Email = str(c, "telephone"), str(c, "email")
	if !s.Courrier {
		reports, err := a.reports(s.UID)
		s.Base = transmission(reports, a.now)
		refused := sub(sub(l.Doc, "rolesRefuses"), s.Contact)
		if s.Role == "gardien" && refused != nil && num(refused, "generation") == s.Gen {
			s.Role = "proche"
		}
		switch {
		case err != nil:
			clear(kts)
			return nil, err
		case !cles.CheckCode(code, str(c, "codeVerifier")) || code == "":
			err = deny("code_faux")
		case b["isUnlocked"] != true:
			err = deny("verrouillee")
		case c["courrier"] == true:
			err = deny("courrier")
		case s.Role == "proche" && s.Base == "":
			err = deny("sans_transmission")
		}
		if s.Base == "" {
			s.Base = "vivant"
		}
		if s.Path = "email"; s.Phone != "" {
			s.Path = "sms"
		}
		if err != nil {
			clear(kts)
			return nil, err
		}
	}
	if r := a.in.count(a.ctx, jcs.Object{"route": a.name, "uid": s.UID, "contactId": s.Contact, "instanceId": a.in.ID}); r["admis"] != true {
		clear(kts)
		return nil, failAt("trop_d_appels", "reessayerLe", a.now.Add(sessions.Idle))
	}
	if s.Courrier { // première session ouverte par ce code (R-DECES-09)
		err = a.edit(a.path(s.UID, "lettres", s.Contact), func(l registre.Line) (jcs.Object, error) {
			if l.State != registre.Valid || l.Doc["invitationUtiliseeLe"] != nil {
				return nil, nil
			}
			l.Doc["invitationUtiliseeLe"] = stamp(a.now)
			return l.Doc, nil
		})
	}
	return s, err
}

func or(err, other error) error {
	if err != nil {
		return err
	}
	return other
}

// started envoie le dernier code (§2.14, étape 3), inscrit la session et
// rend sa réponse ; canal "aucun" délivre aussitôt le jeton.
func (a *call) started(s *sessions.Session) (jcs.Object, error) {
	rnd, err := a.in.cfg.Alea.Read(32)
	if err == nil {
		err = a.sendCode(s)
	}
	if err != nil {
		clear(s.KTS)
		return nil, err
	}
	s.ID = base64URL(rnd)
	if !a.in.sess.Add(s) {
		clear(s.KTS)
		return nil, failAt("trop_d_appels", "reessayerLe", a.now.Add(sessions.Idle))
	}
	out := jcs.Object{"sessionId": s.ID, "canal": s.Path, "masque": mask(s)}
	if s.Path == "aucun" {
		s.Lock()
		a.held = s
		tok, err := a.deliver(s)
		for k, v := range tok {
			out[k] = v
		}
		return out, err
	}
	return out, nil
}

// sendCode envoie le dernier code : par Twilio Verify au téléphone de la
// charte, sinon par email, code de 6 chiffres tiré par Sésame, le même tant
// qu'il est valable.
func (a *call) sendCode(s *sessions.Session) error {
	var err error
	switch s.Path {
	case "sms":
		err = a.in.cfg.Sms.SendCode(a.ctx, a.in.cfg.reglages.VerificationService, s.Phone)
	case "email":
		for !a.now.Before(s.OTPUntil) && err == nil {
			var r []byte // tirage sans biais : 4 294 000 000 est un multiple de 10⁶
			if r, err = a.in.cfg.Alea.Read(4); err == nil && binary.BigEndian.Uint32(r) < 4_294_000_000 {
				s.OTP, s.OTPUntil = fmt.Sprintf("%06d", binary.BigEndian.Uint32(r)%1_000_000), a.now.Add(otpLife)
			}
		}
		if err == nil {
			subject, text := a.in.render("dernier_code_email", "code", s.OTP)
			err = a.in.cfg.Email.Send(a.ctx, a.in.cfg.reglages.EmailExpediteur, s.Email, subject, text)
		}
	}
	if err != nil {
		return fail("fournisseur_indisponible")
	}
	return nil
}

// mask rend le masque du §2.14.
func mask(s *sessions.Session) any {
	switch r := []rune(s.Phone); {
	case s.Path == "email":
		at := strings.LastIndexByte(s.Email, '@')
		return string([]rune(s.Email)[:1]) + "•••••" + s.Email[at:]
	case s.Path != "sms":
		return nil
	case strings.HasPrefix(s.Phone, "+33") && len(r) == 12:
		return "+33 " + string(r[3]) + " •• •• •• " + string(r[10:])
	default:
		return string(r[:4]) + strings.Repeat("•", len(r)-6) + string(r[len(r)-2:])
	}
}

// session authentifie un appel sur une session (étape 2 du §6) : inconnue
// ou expirée ⇒ session_expiree ; ouverte par un autre canal ⇒ canal_invalide
// en clair. La session reste verrouillée jusqu'à la fin de l'appel ; sa
// limite d'appels suit (étape 3).
func (a *call) session() (*sessions.Session, error) {
	s := a.in.sess.Get(str(a.body, "sessionId"))
	if s != nil && s.Channel != a.ch.Key {
		return nil, fail("canal_invalide")
	}
	if s == nil {
		return nil, fail("session_expiree")
	}
	s.Lock()
	a.held = s
	if !a.in.sess.Touch(s, a.now) {
		return nil, fail("session_expiree")
	}
	if ok, retry := a.in.local.hit(a.name+"|session|"+s.ID, a.r.perSess, a.now); !ok {
		return nil, failAt("trop_d_appels", "reessayerLe", retry)
	}
	return s, a.limits()
}

func (a *call) closeSession(s *sessions.Session) {
	a.in.sess.Close(func(o *sessions.Session) bool { return o == s }, a.now, a.held)
}

// renvoyer renvoie le dernier code de la session par son canal (§2.15).
func (a *call) renvoyer() (jcs.Object, error) {
	s, err := a.session()
	switch {
	case err != nil:
		return nil, err
	case s.Verified || (s.Path != "sms" && s.Path != "email"):
		return nil, fail("message_refuse")
	case s.Resends >= maxResends:
		return nil, failAt("trop_d_essais", "reessayerLe", a.now.Add(sessions.Idle))
	}
	s.Resends++
	if err := a.sendCode(s); err != nil {
		return nil, err
	}
	return jcs.Object{"sessionId": s.ID, "canal": s.Path, "masque": mask(s)}, nil
}

// verifier vérifie le dernier code (§2.15).
func (a *call) verifier() (jcs.Object, error) {
	s, err := a.session()
	if err != nil {
		return nil, err
	}
	if !a.in.borne.UpToDate(a.ctx) {
		return nil, fail("reessayer_plus_tard")
	}
	if s.Verified || (s.Path != "sms" && s.Path != "email") {
		return nil, fail("message_refuse")
	}
	doc, err := a.essais(s.UID, s.Contact, "DernierCode")
	var d denied
	if errors.As(err, &d) {
		return nil, &refusal{"otp_refuse", jcs.Object{"essaisRestants": readTally(doc, "DernierCode").left(a.now)}}
	} else if err != nil {
		return nil, err
	}
	otp := str(a.body, "otp")
	good := s.Path == "email" && primitives.Equal([]byte(otp), []byte(s.OTP)) && a.now.Before(s.OTPUntil)
	if s.Path == "sms" {
		if good, err = a.in.cfg.Sms.CheckCode(a.ctx, a.in.cfg.reglages.VerificationService, s.Phone, otp); err != nil {
			return nil, fail("fournisseur_indisponible")
		}
	}
	if good {
		return a.deliver(s)
	}
	t, err := a.strike(s.UID, s.Contact, "DernierCode")
	if err != nil {
		return nil, err
	}
	return nil, &refusal{"otp_refuse", jcs.Object{"essaisRestants": t.left(a.now)}}
}

// token signe le jeton de session (API.md §1.6).
func (a *call) token(s *sessions.Session) (jcs.Object, error) {
	iat := a.now.Unix()
	jws, err := a.in.signJWS(a.ctx, true, jcs.Object{"iss": "sesame", "sub": s.Contact, "uid": s.UID, "scope": "destine",
		"base": s.Base, "role": s.Role, "iat": iat, "exp": iat + jetons.SessionDuration})
	return jcs.Object{"jeton": jws, "expireLe": stamp(time.Unix(iat+jetons.SessionDuration, 0))}, err
}

// deliver délivre le jeton (§2.15) : ligne relue (rejetée, absente ou
// supprime ⇒ session fermée, session_expiree), ouverture inscrite,
// ouvertureNotifiee, alerte au titulaire pour le contact de confiance de son
// vivant.
func (a *call) deliver(s *sessions.Session) (jcs.Object, error) {
	if _, ok, err := a.actif(s.UID); err != nil || !ok {
		if err == nil {
			a.closeSession(s)
		}
		return nil, or(err, fail("session_expiree"))
	}
	rnd, err := a.in.cfg.Alea.Read(16)
	if err != nil {
		return nil, err
	}
	id, opened := base64URL(rnd), stamp(a.now)
	err = a.in.reg.Write(a.ctx, func(w *registre.Writer) error {
		w.Put(a.path(s.UID, "ouvertures", id), jcs.Object{"contactId": s.Contact, "role": s.Role, "ouvertLe": opened, "instanceId": a.in.ID})
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.Verified, s.OTP, s.Opening, s.OuvertLe = true, "", id, opened
	out, err := a.token(s)
	if err != nil {
		return nil, err
	}
	a.in.notif.Notify(a.ctx, "ouvertureNotifiee", jcs.Object{"uid": s.UID, "contactId": s.Contact, "role": s.Role, "ouvertLe": opened, "navigateur": a.ua})
	if s.Role == "gardien" && s.Base == "vivant" {
		a.in.alert(a.ctx, s.UID, "alerte_confiance", registre.Masque(s.Contact), "date", parisTime(a.now))
	}
	out["scope"], out["base"], out["role"], out["prenom"], out["titulaireAffiche"] = "destine", s.Base, s.Role, s.Prenom, s.Titulaire
	return out, nil
}

// identite tient le document d'identité du proche (§2.16, §2.17).
func (a *call) identite(s *sessions.Session) (registre.Line, error) {
	l, err := a.in.reg.Read(a.ctx, a.path(s.UID, "identites", s.Contact))
	if err == nil && l.State == registre.Valid && a.now.Before(at(l.Doc, "bloqueJusquau")) {
		return l, failAt("identite_bloquee", "reessayerLe", at(l.Doc, "bloqueJusquau"))
	}
	return l, err
}

func (a *call) demarrer() (jcs.Object, error) {
	s, err := a.session()
	if err == nil {
		_, err = a.identite(s)
	}
	switch {
	case err != nil:
		return nil, err
	case s.Path != "identite" || s.Verified:
		return nil, fail("message_refuse")
	}
	id, secret, err := a.in.cfg.Identite.Create(a.ctx)
	if err != nil {
		return nil, fail("fournisseur_indisponible")
	}
	s.StripeID = id
	return jcs.Object{"clientSecret": secret}, nil
}

// nameKey normalise un nom pour la comparaison tolérante
// (docs/technique/sesame/protocoles.md, « Ouverture ») : NFD sans signes
// diacritiques, minuscules, œ, æ et ß développés, toute suite d'autres
// caractères que lettres et chiffres réduite à une espace.
func nameKey(s string) string {
	var b strings.Builder
	space := false
	for _, r := range norm.NFD.String(s) {
		r = unicode.ToLower(r)
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteString(map[rune]string{'œ': "oe", 'æ': "ae", 'ß': "ss"}[r])
			if r != 'œ' && r != 'æ' && r != 'ß' {
				b.WriteRune(r)
			}
		default:
			space = true
		}
	}
	return b.String()
}

// NamesMatch dit si l'identité lue sur la pièce est celle de la charte :
// nom égal, prénom égal à une suite de mots consécutifs des prénoms, date
// égale (R-PROCHE-18).
func NamesMatch(prenom, nom, naissance string, v ports.Verification) bool {
	p, first := nameKey(prenom), " "+nameKey(v.FirstName)+" "
	return p != "" && nameKey(nom) == nameKey(v.LastName) && strings.Contains(first, " "+p+" ") && naissance == v.BirthDate
}

func (a *call) identiteVerifier() (jcs.Object, error) {
	s, err := a.session()
	var l registre.Line
	if err == nil {
		l, err = a.identite(s)
	}
	switch {
	case err != nil:
		return nil, err
	case s.Path != "identite" || s.StripeID == "" || s.Verified:
		return nil, fail("message_refuse")
	case !a.in.borne.UpToDate(a.ctx):
		return nil, fail("reessayer_plus_tard")
	}
	v, err := a.in.cfg.Identite.Read(a.ctx, s.StripeID)
	switch {
	case err != nil:
		return nil, fail("fournisseur_indisponible")
	case v.Status != "verified" && v.Status != "requires_input":
		return nil, fail("identite_en_attente")
	}
	redacted := a.in.cfg.Identite.Redact(a.ctx, s.StripeID) == nil
	good := v.Status == "verified" && NamesMatch(s.Prenom, s.Nom, s.Naissance, v)
	var count int64
	err = a.edit(l.Path, func(cur registre.Line) (jcs.Object, error) {
		doc := cur.Doc
		if cur.State != registre.Valid {
			doc = jcs.Object{"essaisIdentite": 0, "redactionsEnAttente": jcs.Array{}}
		}
		if count = num(doc, "essaisIdentite"); good {
			count = 0
		} else if count = count%idMax + 1; count == idMax {
			doc["bloqueJusquau"] = stamp(a.now.Add(idBlock))
		}
		doc["essaisIdentite"] = count
		if !redacted {
			pend, _ := doc["redactionsEnAttente"].(jcs.Array)
			doc["redactionsEnAttente"] = append(pend, jcs.Object{"verificationId": s.StripeID, "depuisLe": stamp(a.now)})
		}
		return doc, nil
	})
	if err != nil {
		return nil, err
	}
	if !good {
		s.StripeID = ""
		a.in.notif.Notify(a.ctx, "identiteEchouee", jcs.Object{"uid": s.UID, "contactId": s.Contact, "essaisIdentite": count})
		if count == idMax {
			return nil, failAt("identite_bloquee", "reessayerLe", a.now.Add(idBlock))
		}
		return nil, fail("identite_non_confirmee")
	}
	out, err := a.deliver(s)
	if err == nil && s.Courrier {
		err = a.edit(a.path(s.UID, "lettres", s.Contact), func(l registre.Line) (jcs.Object, error) {
			if l.State == registre.Valid {
				l.Doc["identiteVerifieeLe"] = stamp(a.now)
			}
			return l.Doc, nil
		})
	}
	return out, err
}

func (a *call) bascule() (jcs.Object, error) {
	s, err := a.session()
	switch {
	case err != nil:
		return nil, err
	case s.Verified || s.Courrier || s.Nom == "" || s.Naissance == "" || s.Base == "vivant":
		return nil, fail("bascule_impossible")
	}
	s.Path, s.Switched = "identite", true
	return jcs.Object{"canal": "identite"}, nil
}

// ouvrirCles ouvre les enveloppes du proche sous la KTS de la session
// (§2.19), après avoir relu la ligne du titulaire.
func (a *call) ouvrirCles() (jcs.Object, error) {
	s, err := a.session()
	if err != nil {
		return nil, err
	}
	if !s.Verified {
		return nil, fail("message_refuse")
	}
	l, ok, err := a.actif(s.UID)
	var reports []registre.Line
	if err == nil {
		reports, err = a.reports(s.UID)
	}
	if err != nil {
		return nil, err
	}
	refused := sub(sub(l.Doc, "rolesRefuses"), s.Contact)
	want, present := sub(l.Doc, "chartes")[s.Contact].(float64)
	stale := !ok || (present && want != float64(s.Gen)) || (!present && s.Gen != 1) || num(l.Doc, "ktsGeneration") != s.KTSGen ||
		(s.Role == "gardien" && s.Base == "vivant" && refused != nil && num(refused, "generation") == s.Gen && !at(refused, "refuseLe").Before(s.Opened))
	for _, r := range reports {
		stale = stale || (!isDeath(r) && !at(r.Doc, "closLe").IsZero() && !at(r.Doc, "closLe").Before(s.Opened))
	}
	if stale {
		a.closeSession(s)
		return nil, fail("session_expiree")
	}
	if !a.in.borne.UpToDate(a.ctx) {
		return nil, fail("reessayer_plus_tard")
	}
	env, _ := cles.KTSEnv(s.KTS, s.UID)
	defer clear(env)
	keys, refusals := jcs.Array{}, jcs.Array{}
	for _, v := range a.body["enveloppes"].(jcs.Array) {
		e := v.(jcs.Object)
		d := cles.Envelope{UID: s.UID, Type: str(e, "type"), ContentID: str(e, "id"), ContactID: str(e, "contactId"), KTSGeneration: s.KTSGen}
		k, err := d.Open(env, raw(e, "enveloppe"), raw(e, "iv"))
		if err != nil || d.ContactID != s.Contact {
			refusals = append(refusals, jcs.Object{"type": d.Type, "id": d.ContentID, "erreur": "enveloppe_refusee"})
			continue
		}
		keys = append(keys, jcs.Object{"type": d.Type, "id": d.ContentID, "cle": b64(k)})
		clear(k)
	}
	out, err := a.token(s)
	out["cles"], out["refusees"] = keys, refusals
	return out, err
}

// fermer est idempotente : une session inconnue, déjà fermée ou expirée
// reçoit la même réponse (§2.20).
func (a *call) fermer() (jcs.Object, error) {
	s, err := a.session()
	if r, _ := err.(*refusal); r != nil && r.code == "session_expiree" {
		err = a.limits()
	} else if err == nil {
		a.closeSession(s)
	}
	return jcs.Object{"fermee": true}, err
}

// refuserRole inscrit le refus du rôle pour la génération de la charte de la
// session (§2.21), puis ferme les sessions de ce proche que tient la VM.
func (a *call) refuserRole() (jcs.Object, error) {
	s, err := a.session()
	switch {
	case err != nil:
		return nil, err
	case !s.Verified:
		return nil, fail("message_refuse")
	case s.Role != "gardien":
		return nil, fail("role_insuffisant")
	}
	err = a.edit(titulaire(s.UID), func(l registre.Line) (jcs.Object, error) {
		if l.State != registre.Valid || l.Doc["etat"] == "supprime" {
			return nil, fail("session_expiree")
		}
		refused := sub(l.Doc, "rolesRefuses")
		if refused == nil {
			refused = jcs.Object{}
			l.Doc["rolesRefuses"] = refused
		}
		refused[s.Contact] = jcs.Object{"generation": s.Gen, "refuseLe": stamp(a.now)}
		return l.Doc, nil
	})
	if err != nil {
		return nil, err
	}
	a.in.notif.Notify(a.ctx, "roleRefuse", jcs.Object{"uid": s.UID, "contactId": s.Contact})
	a.in.closeSessions(s.UID, s.Contact, a.now, s)
	return jcs.Object{"refuse": true}, nil
}
