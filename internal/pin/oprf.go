// Paquet pin : la part de Sésame du protocole de PIN (ARCHITECTURE.md §9.1 et
// §9.2 ; décision 0017). Ici, l'évaluation OPRF (RFC 9497, mode OPRF, suite
// ristretto255-SHA512) de l'élément aveuglé reçu par la clé cleOprf du
// titulaire, par github.com/cloudflare/circl (décisions 0016 et 0067). Aucune
// E/S.
package pin

import (
	"github.com/cloudflare/circl/oprf"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/primitives"
)

// Suite est la seule suite admise (§9.1).
var Suite = oprf.SuiteRistretto255

// Evaluate rend BlindEvaluate(cleOprf, elementAveugle), 32 octets : cleOprf
// est un scalaire de 32 octets, elementAveugle un élément ristretto255
// encodé (32 octets), refusé s'il est hors forme ou l'identité.
func Evaluate(cleOprf, element []byte) ([]byte, error) {
	key := new(oprf.PrivateKey)
	if len(cleOprf) != 32 || key.UnmarshalBinary(Suite, cleOprf) != nil {
		return nil, primitives.ErrKey
	}
	e := Suite.Group().NewElement()
	if len(element) != 32 || e.UnmarshalBinary(element) != nil || e.IsIdentity() {
		return nil, primitives.ErrInvalid
	}
	eval, err := oprf.NewServer(Suite, key).Evaluate(&oprf.EvaluationRequest{Elements: []oprf.Blinded{e}})
	if err != nil || len(eval.Elements) != 1 {
		return nil, primitives.ErrInvalid
	}
	out, err := eval.Elements[0].MarshalBinaryCompress()
	if err != nil {
		return nil, primitives.ErrInvalid
	}
	return out, nil
}
