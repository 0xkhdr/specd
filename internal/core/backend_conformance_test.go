package core

import (
	"os"
	"sync"
	"testing"
)

// backendConformance is the storage contract every StateBackend must satisfy. It
// is parameterized over the backend under test so the same suite can be run
// against the file backend today and remote backends later, guaranteeing none
// weakens the lock + CAS + atomicity spine.
func backendConformance(t *testing.T, b StateBackend) {
	newSpec := func(slug string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
			t.Fatal(err)
		}
		st := InitialState(slug, slug)
		if err := b.WithLock(root, slug, func() error { return b.Save(root, slug, &st) }); err != nil {
			t.Fatal(err)
		}
		return root
	}

	t.Run("stale-base CAS fails", func(t *testing.T) {
		root := newSpec("s")
		stale, _ := b.Load(root, "s") // revision 1
		// A concurrent writer advances the on-disk revision.
		fresh, _ := b.Load(root, "s")
		if err := b.WithLock(root, "s", func() error { return b.Save(root, "s", fresh) }); err != nil {
			t.Fatalf("fresh save: %v", err)
		}
		// The stale handle must be rejected, not clobber the newer state.
		err := b.WithLock(root, "s", func() error { return b.Save(root, "s", stale) })
		if err == nil {
			t.Fatal("stale-base Save = nil, want CAS conflict error")
		}
	})

	t.Run("reentrant lock does not deadlock", func(t *testing.T) {
		root := newSpec("r")
		inner := false
		err := b.WithLock(root, "r", func() error {
			return b.WithLock(root, "r", func() error {
				inner = true
				return nil
			})
		})
		if err != nil || !inner {
			t.Fatalf("reentrant WithLock: err=%v inner=%v", err, inner)
		}
	})

	t.Run("32-goroutine serialization has no lost updates", func(t *testing.T) {
		root := newSpec("c")
		const n = 32
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				// Each writer load-modifies-saves under the lock; serialization +
				// CAS must make every increment land exactly once.
				for {
					err := b.WithLock(root, "c", func() error {
						st, err := b.Load(root, "c")
						if err != nil {
							return err
						}
						st.Turn++
						return b.Save(root, "c", st)
					})
					if err == nil {
						return
					}
				}
			}()
		}
		wg.Wait()

		final, err := b.Load(root, "c")
		if err != nil {
			t.Fatal(err)
		}
		if final.Turn != n {
			t.Errorf("Turn = %d after %d writers, want %d (lost updates)", final.Turn, n, n)
		}
	})
}

func TestBackendConformance(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		backendConformance(t, DefaultBackend())
	})
}
