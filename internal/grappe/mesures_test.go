//go:build mesures

package grappe

import (
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/pin"
)

// Mesures du lot (tests.md du client, « Grappe du PIN », mesures) : tas Go
// d'un nœud après N enregistrements du PIN, instantanés et journal compris.
// go test -tags mesures -run Mesure -v ./internal/grappe

func heap() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

func TestMesureMemoire(t *testing.T) {
	key, _ := pin.NewKey(make([]byte, 32))
	for _, c := range []struct{ uids, depots int }{{10_000, 10_000}, {100_000, 100_000}, {1_000_000, 1_000_000}, {1_000, 100_000}} {
		base := heap()
		start := time.Now()
		b, n := seul(t)
		for i := range c.depots {
			b.propose(n, entry{Op: &pin.Op{UID: "uid-" + strconv.Itoa(i%c.uids) + "-titulaire-de-test", Motif: "nouveau_pin", Key: key, At: t0}})
		}
		h := heap()
		first, _ := n.st.FirstIndex()
		last, _ := n.st.LastIndex()
		snap, _ := n.st.Snapshot()
		t.Logf("%d enregistrements, %d dépôts : tas %d Mo (%.1f %% de 1,38 Gio), journal %d entrées, instantané %d Mo, %s",
			len(n.s.records), c.depots, (h-base)>>20, 100*float64(h-base)/Container, last-first+1, len(snap.GetData())>>20, time.Since(start).Round(time.Second))
		runtime.KeepAlive(n)
	}
}
