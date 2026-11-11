package attestation

import (
	"regexp"
	"slices"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/jcs"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Version est une ligne de versions.json (RELEASE.md §2).
type Version struct {
	Version, ImageDigest, EnvDigest string
	HWModels                        []string
	Tag, Commit, PublishedAt        string
	Status                          string // en_service | retiree
	RetiredAt                       *string
	Notes                           string
}

// Versions est la liste des versions et les clés publiques session-sign
// (points P-256 non compressés) par kid.
type Versions struct {
	Versions    []Version
	SessionKeys map[string][]byte
}

// ByDigest rend la version d'empreinte digest, quel que soit son statut.
func (l *Versions) ByDigest(digest string) *Version {
	for i := range l.Versions {
		if l.Versions[i].ImageDigest == digest {
			return &l.Versions[i]
		}
	}
	return nil
}

var (
	reDigest  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	reVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	reHW      = regexp.MustCompile(`^[A-Z0-9_]+$`)
	reCommit  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	reKid     = regexp.MustCompile(`^session-sign/[1-9][0-9]*$`)
)

// fields rend o s'il a exactement les clés données, sinon nil.
func fields(v any, keys ...string) jcs.Object {
	o := obj(v)
	if o == nil || len(o) != len(keys) {
		return nil
	}
	for _, k := range keys {
		if _, ok := o[k]; !ok {
			return nil
		}
	}
	return o
}

func text(v any, re *regexp.Regexp) (string, bool) {
	s, ok := v.(string)
	return s, ok && (re == nil || re.MatchString(s))
}

func timestamp(v any) bool {
	s, ok := v.(string)
	_, err := primitives.ReadTime(s)
	return ok && err == nil
}

// ReadVersions lit versions.json strictement : champ inconnu, absent ou hors
// forme, empreinte en double, statut et retireLe incohérents sont refusés
// (parametres_invalides).
func ReadVersions(content []byte) (*Versions, error) {
	bad := primitives.ErrInvalid
	v, err := jcs.Parse(content)
	root := fields(v, "clesSession", "v", "versions")
	if err != nil || root == nil || root["v"] != 1.0 {
		return nil, bad
	}
	list, ok := root["versions"].(jcs.Array)
	keys := obj(root["clesSession"])
	if !ok || keys == nil {
		return nil, bad
	}
	out := &Versions{SessionKeys: map[string][]byte{}}
	for _, e := range list {
		o := fields(e, "commit", "envDigest", "hwmodels", "imageDigest", "notes", "publieLe", "retireLe", "statut", "tag", "version")
		if o == nil {
			return nil, bad
		}
		var r Version
		var oks [8]bool
		r.Version, oks[0] = text(o["version"], reVersion)
		r.ImageDigest, oks[1] = text(o["imageDigest"], reDigest)
		r.EnvDigest, oks[2] = text(o["envDigest"], reDigest)
		r.Tag, oks[3] = text(o["tag"], nil)
		r.Commit, oks[4] = text(o["commit"], reCommit)
		r.PublishedAt, oks[5] = text(o["publieLe"], nil)
		r.Status, _ = text(o["statut"], nil)
		r.Notes, oks[6] = text(o["notes"], nil)
		hw, okHW := o["hwmodels"].(jcs.Array)
		oks[7] = okHW && len(hw) > 0 && r.Tag == "v"+r.Version && timestamp(o["publieLe"])
		for _, h := range hw {
			s, ok := text(h, reHW)
			oks[7] = oks[7] && ok
			r.HWModels = append(r.HWModels, s)
		}
		if retired, ok := o["retireLe"].(string); ok && timestamp(retired) {
			r.RetiredAt = &retired
		} else if o["retireLe"] != nil {
			return nil, bad
		}
		if slices.Contains(oks[:], false) || (r.Status != "en_service" && r.Status != "retiree") ||
			(r.Status == "retiree") != (r.RetiredAt != nil) || out.ByDigest(r.ImageDigest) != nil {
			return nil, bad
		}
		out.Versions = append(out.Versions, r)
	}
	for kid, e := range keys {
		o := fields(e, "cle", "depuis")
		s, _ := o["cle"].(string)
		k, err := primitives.B64(s)
		if !reKid.MatchString(kid) || o == nil || !timestamp(o["depuis"]) || err != nil || len(k) != 65 || k[0] != 4 {
			return nil, bad
		}
		out.SessionKeys[kid] = k
	}
	return out, nil
}
