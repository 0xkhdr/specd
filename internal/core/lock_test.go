package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestGoIDNonZeroStableAndDistinct(t *testing.T) {
	// Non-zero and never the unheld sentinel: a degraded goID must not alias
	// "unheld" and bypass the lock.
	a1 := goID()
	if a1 == 0 || a1 == lockUnheld {
		t.Fatalf("goID() = %d, want non-zero and != lockUnheld", a1)
	}

	// Stable within a goroutine.
	if a2 := goID(); a2 != a1 {
		t.Errorf("goID() unstable within goroutine: %d then %d", a1, a2)
	}

	// Distinct across goroutines.
	other := make(chan int64, 1)
	go func() { other <- goID() }()
	if b := <-other; b == a1 {
		t.Errorf("goID() not distinct across goroutines: both %d", b)
	}
}

func TestWithSpecLockReleasesOnReturn(t *testing.T) {
	// Arrange
	root := specRoot(t, "s")

	// Act
	_, err := WithSpecLock[int](root, "s", func() (int, error) { return 1, nil })
	if err != nil {
		t.Fatalf("WithSpecLock: %v", err)
	}

	// Assert: lock file removed after the critical section.
	if _, err := os.Stat(lockFilePath(root, "s")); !os.IsNotExist(err) {
		t.Errorf("lock file still present after release: %v", err)
	}
}

func TestWithSpecLockReentrant(t *testing.T) {
	// Arrange
	root := specRoot(t, "s")

	// Act: nested acquisition of the same lock must not deadlock.
	got, err := WithSpecLock[int](root, "s", func() (int, error) {
		return WithSpecLock[int](root, "s", func() (int, error) {
			return 42, nil
		})
	})

	// Assert
	if err != nil {
		t.Fatalf("reentrant lock: %v", err)
	}
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	if _, err := os.Stat(lockFilePath(root, "s")); !os.IsNotExist(err) {
		t.Errorf("lock leaked after reentrant release: %v", err)
	}
}

func TestWithSpecLockTimesOutWhenHeld(t *testing.T) {
	// Arrange: a fresh (non-stale) lock held by a "different process".
	root := specRoot(t, "s")
	t.Setenv("SPECD_LOCK_TIMEOUT_MS", "60")
	t.Setenv("SPECD_LOCK_STALE_MS", "60000")
	held := lockFilePath(root, "s")
	if err := os.WriteFile(held, []byte(fmt.Sprintf("99999 %d\n", time.Now().UnixMilli())), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := WithSpecLock[int](root, "s", func() (int, error) { return 1, nil })

	// Assert: blocked acquisition surfaces a gate error within the timeout.
	if se, ok := IsSpecdError(err); !ok || se.Code != ExitGate {
		t.Errorf("err = %v, want gate error on lock timeout", err)
	}
}

func TestWithSpecLockReclaimsStaleLock(t *testing.T) {
	// Arrange: a lock whose timestamp is far older than the stale threshold.
	root := specRoot(t, "s")
	t.Setenv("SPECD_LOCK_STALE_MS", "50")
	t.Setenv("SPECD_LOCK_TIMEOUT_MS", "2000")
	old := time.Now().Add(-time.Hour).UnixMilli()
	if err := os.WriteFile(lockFilePath(root, "s"), []byte(fmt.Sprintf("88888 %d\n", old)), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act: a live process should reclaim the abandoned lock.
	got, err := WithSpecLock[int](root, "s", func() (int, error) { return 7, nil })

	// Assert
	if err != nil {
		t.Fatalf("stale reclaim failed: %v", err)
	}
	if got != 7 {
		t.Errorf("got %d, want 7", got)
	}
}

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
