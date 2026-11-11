// Paquet sessions : l'état des sessions du portail d'une instance (API.md
// §2.14, étape 4) : en mémoire de la VM seulement, lié au canal qui a ouvert
// la session, perdu à son redémarrage (410 instance_inconnue, §1.2). La KTS
// déchiffrée et les champs d'identité de la charte y restent jusqu'à la fin
// de la session, puis sont effacés. Les décisions (internal/routes) disent
// quand une session finit ; ce paquet garantit qu'elle ne finit qu'une fois.
package sessions

import (
	"sync"
	"time"
)

// Idle est la durée sans appel après laquelle une session expire (API.md
// §7) ; Max borne les sessions d'une instance (choix de ce dépôt ; au-delà,
// session/ouvrir répond trop_d_appels).
const (
	Idle = time.Hour
	Max  = 100_000
)

// Session est l'état d'une session. Path est le canal du second facteur :
// "sms", "email", "identite" ou "aucun" (§2.14). Base est la base du jeton
// (§2.15), fixée à l'ouverture ; Gen la génération de charte vérifiée.
type Session struct {
	sync.Mutex
	ID, Channel, UID, Contact, Role, Base, Path string
	Courrier, Switched, Verified                bool // invitation, bascule, jeton délivré
	KTS                                         []byte
	KTSGen, Gen                                 int64
	Prenom, Nom, Naissance, Titulaire           string
	Phone, Email, OTP                           string
	OTPUntil, Opened                            time.Time
	last                                        time.Time // sous le verrou du store
	Resends                                     int
	StripeID, Opening, OuvertLe                 string // ouverture inscrite (§2.15)
}

// Store tient les sessions d'une instance. End est appelé, hors du verrou du
// store, une seule fois par session finie (retirée du store une seule fois),
// après l'effacement de sa KTS.
type Store struct {
	mu   sync.Mutex
	byID map[string]*Session
	End  func(s *Session, now time.Time)
}

// Add inscrit une session ; faux si l'instance en tient déjà Max.
func (st *Store) Add(s *Session) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.byID == nil {
		st.byID = map[string]*Session{}
	}
	if len(st.byID) >= Max {
		return false
	}
	s.last = s.Opened
	st.byID[s.ID] = s
	return true
}

// Touch compte un appel sur la session à now et dit si elle était encore
// vivante (moins d'une heure depuis le dernier appel, §2.15) ; sinon elle
// est finie.
func (st *Store) Touch(s *Session, now time.Time) bool {
	st.mu.Lock()
	alive := now.Sub(s.last) < Idle
	if alive {
		s.last = now
	}
	st.mu.Unlock()
	if !alive {
		st.Close(func(o *Session) bool { return o == s }, now, s)
	}
	return alive
}

// Expire finit les sessions sans appel depuis une heure.
func (st *Store) Expire(now time.Time) {
	st.Close(func(s *Session) bool { return now.Sub(s.last) >= Idle }, now, nil)
}

// Get rend la session id, nil si elle est inconnue ou finie.
func (st *Store) Get(id string) *Session {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.byID[id]
}

// Count rend le nombre de sessions tenues pour ce proche de ce titulaire.
func (st *Store) Count(uid, contact string) int {
	n := 0
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, s := range st.byID {
		if s.UID == uid && s.Contact == contact {
			n++
		}
	}
	return n
}

// Close finit chaque session pour laquelle match est vrai : retirée, KTS
// effacée sous son verrou (sauf held, dont l'appelant tient déjà le verrou),
// puis End. match est appelé sous le verrou du store et ne lit que les
// champs fixés à la création (UID, Contact, Channel, Opened) et l'heure du
// dernier appel.
func (st *Store) Close(match func(s *Session) bool, now time.Time, held *Session) {
	var done []*Session
	st.mu.Lock()
	for id, s := range st.byID {
		if match(s) {
			delete(st.byID, id)
			done = append(done, s)
		}
	}
	st.mu.Unlock()
	for _, s := range done {
		if s != held {
			s.Lock()
		}
		clear(s.KTS)
		if st.End != nil {
			st.End(s, now)
		}
		if s != held {
			s.Unlock()
		}
	}
}
