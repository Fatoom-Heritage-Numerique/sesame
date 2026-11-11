// Commande sesame : le binaire de l'image de Sésame (ARCHITECTURE.md §2.4).
//
// Le cœur de Sésame (attestation, canal, KMS, registre, protocoles, grappe)
// n'est pas encore écrit : il arrive à l'étape 3 de l'ordre de construction
// (README.md, « État du projet »), avec les adaptateurs réels. Ce binaire
// existe dès l'étape 2 pour que la construction reproductible, le budget de
// code, le contrôle du lanceur factice et celui des dépendances portent sur
// ce que l'image lie vraiment (RELEASE.md §8). Il ne sert aucune route, ne
// lit aucune variable et n'ouvre aucune clé : il dit qu'il n'est pas Sésame
// et s'arrête en échec.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "sesame : le cœur de Sésame n'est pas encore écrit (README.md, « État du projet ») ; ce binaire ne sert rien.")
	os.Exit(1)
}
