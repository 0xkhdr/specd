package core

import (
	"encoding/json"
	"os"
	"os/exec"
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

	// assertSchemaValid loads the persisted state through the backend and asserts
	// the document is schema-valid. It is backend-agnostic — it validates the
	// marshaled State, not an on-disk file — so it holds for file, git, and the
	// database-backed backends alike (R4.1).
	assertSchemaValid := func(t *testing.T, root, slug string) {
		t.Helper()
		st, err := b.Load(root, slug)
		if err != nil {
			t.Fatalf("load for schema check: %v", err)
		}
		raw, err := json.Marshal(st)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
		viols, err := ValidateState(raw, SchemaVersionID)
		if err != nil {
			t.Fatalf("ValidateState: %v", err)
		}
		if len(viols) != 0 {
			t.Errorf("persisted state not schema-valid: %v", viols)
		}
	}

	// R1.2: a key reads back the exact state it was written with.
	t.Run("read_returns_what_was_written", func(t *testing.T) {
		root := newSpec("rw")
		got, err := b.Load(root, "rw")
		if err != nil || got == nil {
			t.Fatalf("load: got=%v err=%v", got, err)
		}
		if got.Spec != "rw" || got.Title != "rw" {
			t.Errorf("read-back mismatch: spec=%q title=%q", got.Spec, got.Title)
		}
		assertSchemaValid(t, root, "rw")
	})

	// R2.1 + R2.3: a write presenting the current revision commits and bumps the
	// revision, and the revision increases monotonically across serialized writes.
	t.Run("cas_commits_and_revision_is_monotone", func(t *testing.T) {
		root := newSpec("m")
		prev := -1
		for i := 0; i < 5; i++ {
			err := b.WithLock(root, "m", func() error {
				st, err := b.Load(root, "m")
				if err != nil {
					return err
				}
				if st.Revision <= prev {
					t.Errorf("revision not monotone: %d follows %d", st.Revision, prev)
				}
				prev = st.Revision
				st.Turn++
				return b.Save(root, "m", st)
			})
			if err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
		}
		final, err := b.Load(root, "m")
		if err != nil {
			t.Fatal(err)
		}
		// newSpec wrote revision 1; five more committed writes ⇒ revision 6.
		if final.Revision != 6 {
			t.Errorf("revision = %d after 1+5 writes, want 6", final.Revision)
		}
		assertSchemaValid(t, root, "m")
	})

	// R2.2: a write presenting a stale revision is rejected, never clobbering the
	// newer state.
	t.Run("stale_base_cas_fails", func(t *testing.T) {
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
		assertSchemaValid(t, root, "s")
	})

	t.Run("reentrant_lock_does_not_deadlock", func(t *testing.T) {
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

	// R3.3: the lock is released on normal completion, so a later acquirer is not
	// blocked by a previous holder that already returned.
	t.Run("lock_released_on_completion", func(t *testing.T) {
		root := newSpec("rel")
		if err := b.WithLock(root, "rel", func() error { return nil }); err != nil {
			t.Fatalf("first acquire: %v", err)
		}
		got := false
		if err := b.WithLock(root, "rel", func() error { got = true; return nil }); err != nil {
			t.Fatalf("second acquire blocked — lock not released: %v", err)
		}
		if !got {
			t.Error("second WithLock body did not run")
		}
	})

	t.Run("32_goroutine_serialization_has_no_lost_updates", func(t *testing.T) {
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
		assertSchemaValid(t, root, "c")
	})

	// R4.3: the git backend produces one replayable commit per committed write,
	// so the commit count tracks the monotone revision exactly.
	if b.Name() == "git" {
		t.Run("git_commits_once_per_revision", func(t *testing.T) {
			root := newSpec("g")
			const extra = 3
			for i := 0; i < extra; i++ {
				if err := b.WithLock(root, "g", func() error {
					st, err := b.Load(root, "g")
					if err != nil {
						return err
					}
					st.Turn++
					return b.Save(root, "g", st)
				}); err != nil {
					t.Fatalf("write %d: %v", i, err)
				}
			}
			final, _ := b.Load(root, "g")
			out, err := runGit(root, "rev-list", "--count", "HEAD")
			if err != nil {
				t.Fatalf("rev-list: %s", out)
			}
			count := 0
			for _, c := range out {
				if c >= '0' && c <= '9' {
					count = count*10 + int(c-'0')
				}
			}
			// newSpec committed revision 1, plus `extra` writes ⇒ revision == commits.
			if count != final.Revision {
				t.Errorf("git commits = %d, revision = %d — not one commit per write", count, final.Revision)
			}
		})
	}
}

// conformanceBackend is one row of the backend table: a ready backend, or a
// reason it is unavailable. An unavailable backend is skipped with that reason —
// never silently passed (R1.3).
type conformanceBackend struct {
	name string
	b    StateBackend
	skip string
}

// availableBackends resolves every backend the build knows about, attaching a
// skip reason where a tool (git) or a compiled-in driver + live service
// (postgres/redis) is absent. The default build compiles in neither postgres nor
// redis, so those rows skip closed unless a fork built with the matching tag runs
// the suite against a configured service.
func availableBackends() []conformanceBackend {
	rows := []conformanceBackend{{name: "file", b: DefaultBackend()}}

	if _, err := exec.LookPath("git"); err == nil {
		rows = append(rows, conformanceBackend{name: "git", b: GitBackend()})
	} else {
		rows = append(rows, conformanceBackend{name: "git", skip: "git not on PATH"})
	}

	rows = append(rows, optionalConformanceBackend("postgres", "SPECD_PG_DSN"))
	rows = append(rows, optionalConformanceBackend("redis", "SPECD_REDIS_ADDR"))
	return rows
}

// optionalConformanceBackend builds a table row for a driver-backed backend. It
// skips with a precise reason when the backend was not compiled in (no build
// tag) or when the service-locating env var is unset, so a missing service is
// always a visible skip and never a false pass.
func optionalConformanceBackend(name, envVar string) conformanceBackend {
	b, err := SelectBackend(name)
	if err != nil {
		return conformanceBackend{name: name, skip: "not compiled in (build -tags specd_" + name + ")"}
	}
	if os.Getenv(envVar) == "" {
		return conformanceBackend{name: name, skip: envVar + " unset — no service to test against"}
	}
	return conformanceBackend{name: name, b: b}
}

func TestBackendConformance(t *testing.T) {
	for _, row := range availableBackends() {
		row := row
		t.Run(row.name, func(t *testing.T) {
			if row.b == nil {
				t.Skipf("backend %q unavailable: %s", row.name, row.skip)
			}
			backendConformance(t, row.b)
		})
	}
}
