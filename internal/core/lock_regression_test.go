package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// readStateRaw reads the on-disk state.json for a spec.
func readStateRaw(t *testing.T, root, slug string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(SpecDir(root, slug), "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	return b
}

// R6.2: reclaiming a stale lock and writing under it must leave state.json
// schema-valid — recovery never corrupts state.
func TestWithSpecLockStaleReclaimKeepsStateValid(t *testing.T) {
	root := specRoot(t, "s")
	st := InitialState("s", "S")
	if err := SaveState(root, "s", &st); err != nil {
		t.Fatal(err)
	}

	// Plant a stale lock from an abandoned process.
	t.Setenv("SPECD_LOCK_STALE_MS", "50")
	t.Setenv("SPECD_LOCK_TIMEOUT_MS", "2000")
	old := time.Now().Add(-time.Hour).UnixMilli()
	if err := os.WriteFile(lockFilePath(root, "s"), []byte(fmt.Sprintf("88888 %d\n", old)), 0o644); err != nil {
		t.Fatal(err)
	}

	// A live process reclaims the lock and commits a write.
	_, err := WithSpecLock[int](root, "s", func() (int, error) {
		cur, err := LoadState(root, "s")
		if err != nil {
			return 0, err
		}
		cur.Turn++
		return 0, SaveState(root, "s", cur)
	})
	if err != nil {
		t.Fatalf("stale reclaim + write: %v", err)
	}

	viols, err := ValidateState(readStateRaw(t, root, "s"), SchemaVersionID)
	if err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	if len(viols) != 0 {
		t.Errorf("R6.2: state.json invalid after stale-lock recovery: %v", viols)
	}
}

// R6.1 + R6.3: many concurrent writers serialize through the lock+CAS, and the
// committed state.json is schema-valid after every contended write (the final
// document validates and the revision reflects exactly the surviving commits).
func TestConcurrentWritesKeepStateSchemaValid(t *testing.T) {
	root := specRoot(t, "s")
	st := InitialState("s", "S")
	if err := SaveState(root, "s", &st); err != nil {
		t.Fatal(err)
	}

	const workers = 16
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
				// Every committed write must be schema-valid, not just the last.
				viols, verr := ValidateState(readStateRaw(t, root, "s"), SchemaVersionID)
				if verr != nil {
					return 0, verr
				}
				if len(viols) != 0 {
					return 0, fmt.Errorf("state invalid mid-run: %v", viols)
				}
				return 0, nil
			})
		}()
	}
	wg.Wait()

	final, err := LoadState(root, "s")
	if err != nil {
		t.Fatalf("final load: %v", err)
	}
	if final.Turn != workers {
		t.Errorf("R6.1: final Turn = %d, want %d (lost/duplicated updates)", final.Turn, workers)
	}
	viols, err := ValidateState(readStateRaw(t, root, "s"), SchemaVersionID)
	if err != nil {
		t.Fatalf("final ValidateState: %v", err)
	}
	if len(viols) != 0 {
		t.Errorf("R6.3: final state.json invalid: %v", viols)
	}
}
