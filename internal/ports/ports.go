// Paquet ports : les interfaces que voit le code des décisions de Sésame
// (ARCHITECTURE.md §2.4 ; décision 0063). L'image y branche les adaptateurs
// réels (Confidential Space, KMS, Firestore, Cloud Storage, Secret Manager,
// Cloud Logging, fournisseurs), sesame-fake les doubles d'internal/factice.
// Chaque interface porte le seul usage qu'en font les décisions ; une erreur
// rendue par un port est une panne (fournisseur, réseau, permission), que la
// décision traduit en code du contrat (API.md §6).
package ports

import (
	"context"
	"errors"
	"time"
)

// ErrExists : objet déjà présent (création seule, ifGenerationMatch=0).
// ErrConflict : transaction du registre non validée après ses reprises.
var (
	ErrExists   = errors.New("objet_existant")
	ErrConflict = errors.New("transaction_en_conflit")
)

// Horloge est l'heure de Sésame (ARCHITECTURE.md §2.2, « Heure »).
type Horloge interface{ Now() time.Time }

// Alea rend n octets de la primitive d'aléa.
type Alea interface{ Read(n int) ([]byte, error) }

// Noms des clés KMS (ARCHITECTURE.md §4).
const (
	KtsSeal        = "kts-seal"
	AlerteSeal     = "alerte-seal"
	InvitationSeal = "invitation-seal"
	SessionSign    = "session-sign"
	RegistreSign   = "registre-sign"
)

// KMS scelle sous la version principale d'une clé symétrique et descelle
// sous toute version activée ; Sign signe par la version fixée par l'image et
// rend la signature DER ; PublicKeys rend, par kid, la clé publique (point
// non compressé) de chaque version activée lue au démarrage, et une erreur si
// une version est importée (ARCHITECTURE.md §4).
type KMS interface {
	Encrypt(ctx context.Context, key string, plain, aad []byte) ([]byte, error)
	Decrypt(ctx context.Context, key string, sealed, aad []byte) ([]byte, error)
	Sign(ctx context.Context, key string, msg []byte) (der []byte, err error)
	PublicKeys(ctx context.Context, key string) (map[string][]byte, error)
}

// Doc est un document du registre : Data est nil pour un document absent ;
// UpdateTime est l'update_time posé par la base, jamais par Sésame.
type Doc struct {
	Path       string
	Data       map[string]any
	UpdateTime time.Time
}

// Tx est une transaction du registre : les lectures avant les écritures,
// validées ensemble ou pas du tout.
type Tx interface {
	Get(path string) (Doc, error)
	// List rend les documents directs de la collection dont le champ texte
	// field est strictement supérieur à after (tous si field est vide) ;
	// plus de max documents est une erreur.
	List(collection, field, after string, max int) ([]Doc, error)
	Set(path string, data map[string]any)
}

// Base est le registre (Firestore du projet Sésame). Transaction exécute f
// et la rejoue en entier sur un conflit, un nombre borné de fois.
type Base interface {
	Transaction(ctx context.Context, f func(Tx) error) error
}

// Object est un objet de Cloud Storage et son timeCreated.
type Object struct {
	Name    string
	Created time.Time
	Content []byte
}

// Buckets (ARCHITECTURE.md §6.2, §8.2).
const (
	BucketAudit    = "audit"
	BucketInscrits = "inscrits"
)

// Stockage crée un objet sans écrasement possible (ErrExists), lit un objet
// (nil sans erreur s'il est absent) et liste un préfixe, contenus compris.
type Stockage interface {
	Create(ctx context.Context, bucket, name string, content []byte) error
	Read(ctx context.Context, bucket, name string) ([]byte, error)
	List(ctx context.Context, bucket, prefix string, max int) ([]Object, error)
}

// Secrets lit un secret de Secret Manager (clés des fournisseurs, document
// des textes).
type Secrets interface {
	Secret(ctx context.Context, name string) ([]byte, error)
}

// Event est une entrée de journal (ARCHITECTURE.md §7) : seulement ces
// champs, identifiants masqués ; jamais de clé, de code, de numéro, d'email,
// de nom ni de corps de requête.
type Event struct {
	Event, UIDMasque, ContactMasque, InstanceID, ImageDigest, Resultat, CodeErreur string
}

// Journal écrit les événements (Cloud Logging du projet Sésame).
type Journal interface{ Write(e Event) }

// Émetteurs des jetons que Sésame vérifie (API.md §1.4).
const (
	IssuerIDToken = "idToken"
	IssuerGoogle  = "google"
)

// Emetteurs rend les clés publiques RS256 d'un émetteur par kid, chacune en
// SubjectPublicKeyInfo DER.
type Emetteurs interface {
	Keys(ctx context.Context, issuer string) (map[string][]byte, error)
}

// Lanceur rend un jeton d'attestation PKI dont eat_nonce porte nonces.
type Lanceur interface {
	Token(ctx context.Context, nonces []string) (string, error)
}

// Passerelle envoie une requête à la passerelle du client (API.md §5) et rend
// le statut et le corps de sa réponse.
type Passerelle interface {
	Send(ctx context.Context, method, pathQuery string, headers map[string]string, body []byte) (int, []byte, error)
}

// Sms est Twilio : Verify pour le dernier code par SMS (service attesté),
// Messaging pour les alertes par SMS.
type Sms interface {
	SendCode(ctx context.Context, service, phone string) error
	CheckCode(ctx context.Context, service, phone, code string) (bool, error)
	SendText(ctx context.Context, from, phone, text string) error
}

// Email est l'API Mail Send de SendGrid (alertes et dernier code).
type Email interface {
	Send(ctx context.Context, from, to, subject, text string) error
}

// Verification est le verdict d'une session Stripe Identity.
type Verification struct {
	Status                         string // verified, requires_input, processing
	FirstName, LastName, BirthDate string
}

// Identite est Stripe Identity (pièce et selfie exigés).
type Identite interface {
	Create(ctx context.Context) (id, clientSecret string, err error)
	Read(ctx context.Context, id string) (Verification, error)
	Redact(ctx context.Context, id string) error
}

// Courrier est Pingen (lettre PDF d'une page).
type Courrier interface {
	Send(ctx context.Context, organisation string, pdf []byte) (documentID string, err error)
	Status(ctx context.Context, organisation, documentID string) (string, error)
}
