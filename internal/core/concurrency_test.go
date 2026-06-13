package core

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestConcurrentSaveStateIsSerialized hammers a single spec from many
// goroutines, each taking the spec lock and committing one increment. The
// advisory lock + CAS must serialize every write: exactly N commits land, the
// revision advances by exactly N, and no update is lost or duplicated.
//
// Run under `go test -race` for the strongest signal.
func TestConcurrentSaveStateIsSerialized(t *testing.T) {
	root := specRoot(t, "s")
	st := InitialState("s", "S")
	if err := SaveState(root, "s", &st); err != nil {
		t.Fatal(err)
	}

	const workers = 32
	var committed int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, _ = WithSpecLock[int](root, "s", func() (int, error) {
				cur, err := LoadState(root, "s")
				if err != nil {
					return 0, err
				}
				cur.Turn++
				if err := SaveState(root, "s", cur); err != nil {
					return 0, err
				}
				atomic.AddInt64(&committed, 1)
				return 0, nil
			})
		}()
	}
	wg.Wait()

	// Assert: every worker's increment survived (no lost updates).
	final, err := LoadState(root, "s")
	if err != nil {
		t.Fatalf("final load: %v", err)
	}
	if committed != workers {
		t.Errorf("committed = %d, want %d", committed, workers)
	}
	if final.Turn != workers {
		t.Errorf("final Turn = %d, want %d (lost or duplicated updates)", final.Turn, workers)
	}
}
