package routes

import (
	"context"
	"strings"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/ports"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/vie"
)

// parisTime et parisDay écrivent une heure et une date de Paris dans les
// textes de Sésame (docs/technique/sesame/versions-et-alertes.md).
func parisTime(t time.Time) string { return t.In(paris).Format("02/01/2006 à 15 h 04") }
func parisDay(t time.Time) string  { return t.In(paris).Format("02/01/2006") }

// render rend l'objet (première ligne) et le texte du texte attesté name
// (ARCHITECTURE.md §2.3, « Textes attestés »), chaque {clé} de kv remplacée
// par sa valeur ; un texte d'une seule ligne est aussi son objet.
func (in *Instance) render(name string, kv ...string) (subject, text string) {
	kv = append([]string{}, kv...)
	for i := range kv {
		if i%2 == 0 {
			kv[i] = "{" + kv[i] + "}"
		}
	}
	text = strings.NewReplacer(kv...).Replace(in.cfg.reglages.Textes[name])
	subject, body, _ := strings.Cut(text, "\n")
	if body == "" {
		return subject, text
	}
	return subject, body
}

// alert envoie l'alerte name au titulaire uid (« Ce que Sésame envoie
// lui-même ») : par email à l'adresse de sa fiche d'alerte, ouverte par un
// seul descellement d'alerte-seal, et par SMS en plus si elle porte un
// téléphone ; un envoi refusé est réessayé 24 h, puis journalisé et publié
// dans la borne (alerte_refusee, ressource uidMasque/object). Sans fiche,
// rien ne part (LIM-08).
func (in *Instance) alert(ctx context.Context, uid, name, object string, kv ...string) {
	l, err := in.reg.Read(ctx, titulaire(uid))
	gen := num(l.Doc, "ficheGeneration")
	if err != nil || gen == 0 {
		return
	}
	sealed, _ := primitives.B64(str(l.Doc, "ficheScellee"))
	aad, _ := vie.AlertAAD(uid, gen)
	plain, err := in.cfg.KMS.Decrypt(ctx, ports.AlerteSeal, sealed, aad)
	f, err2 := vie.ReadAlert(plain, uid, gen)
	clear(plain)
	lost := func() {
		in.log("alerte_refusee", uid, name)
		in.borne.Signal("alerte_refusee", registre.Masque(uid)+"/"+object)
	}
	if err != nil || err2 != nil {
		lost()
		return
	}
	r := in.cfg.reglages
	subject, text := in.render(name+"_email", kv...)
	in.retry.Do(ctx, func(ctx context.Context) bool {
		return in.cfg.Email.Send(ctx, r.EmailExpediteur, f.Email, subject, text) == nil
	}, lost)
	if f.Telephone != nil {
		_, sms := in.render(strings.TrimSuffix(name, "_rappel")+"_sms", kv...)
		in.retry.Do(ctx, func(ctx context.Context) bool {
			return in.cfg.Sms.SendText(ctx, r.SmsExpediteur, *f.Telephone, sms) == nil
		}, lost)
	}
}
