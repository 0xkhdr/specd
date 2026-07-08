package core

import (
	"os"
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

// TestWithSpecLockReleasesOnPanic asserts that a panic inside fn still unwinds
// the release defer — the lock file is removed and ls.mu is freed — so the next
// acquirer is not deadlocked by an abandoned lock.
func TestWithSpecLockReleasesOnPanic(t *testing.T) {
	root := specRoot(t, "s")

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic to propagate out of WithSpecLock")
			}
		}()
		_, _ = WithSpecLock[int](root, "s", func() (int, error) {
			panic("boom inside critical section")
		})
	}()

	// Lock file must be gone.
	if _, err := os.Stat(lockFilePath(root, "s")); !os.IsNotExist(err) {
		t.Errorf("lock file leaked after panic: %v", err)
	}

	// A subsequent acquire must succeed immediately (mu not left locked).
	got, err := WithSpecLock[int](root, "s", func() (int, error) { return 9, nil })
	if err != nil {
		t.Fatalf("acquire after panic failed: %v", err)
	}
	if got != 9 {
		t.Errorf("got %d, want 9", got)
	}
}
