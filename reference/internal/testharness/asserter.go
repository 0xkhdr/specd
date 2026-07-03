package testharness

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// StateAsserter provides chainable assertions over a spec's persisted state.json.
// Every failing assertion calls t.Errorf (non-fatal) so a single test can report
// several discrepancies at once; load failures are fatal.
type StateAsserter struct {
	h     *Harness
	slug  string
	state *core.State
}

// State loads the spec's state.json and returns an asserter over it.
func (h *Harness) State(slug string) *StateAsserter {
	h.T.Helper()
	st, err := core.LoadState(h.Root, slug)
	if err != nil {
		h.T.Fatalf("State(%q): load: %v", slug, err)
	}
	if st == nil {
		h.T.Fatalf("State(%q): no state.json", slug)
	}
	return &StateAsserter{h: h, slug: slug, state: st}
}

// Raw exposes the loaded state for assertions not covered by the helpers.
func (a *StateAsserter) Raw() *core.State { return a.state }

func (a *StateAsserter) Status(want core.SpecStatus) *StateAsserter {
	a.h.T.Helper()
	if a.state.Status != want {
		a.h.T.Errorf("spec %q status = %q, want %q", a.slug, a.state.Status, want)
	}
	return a
}

func (a *StateAsserter) Phase(want core.Phase) *StateAsserter {
	a.h.T.Helper()
	if a.state.Phase != want {
		a.h.T.Errorf("spec %q phase = %q, want %q", a.slug, a.state.Phase, want)
	}
	return a
}

func (a *StateAsserter) Gate(want core.Gate) *StateAsserter {
	a.h.T.Helper()
	if a.state.Gate != want {
		a.h.T.Errorf("spec %q gate = %q, want %q", a.slug, a.state.Gate, want)
	}
	return a
}

func (a *StateAsserter) Turn(want int) *StateAsserter {
	a.h.T.Helper()
	if a.state.Turn != want {
		a.h.T.Errorf("spec %q turn = %d, want %d", a.slug, a.state.Turn, want)
	}
	return a
}

func (a *StateAsserter) TaskStatus(id string, want core.TaskStatus) *StateAsserter {
	a.h.T.Helper()
	ts, ok := a.state.Tasks[id]
	if !ok {
		a.h.T.Errorf("spec %q has no task %q", a.slug, id)
		return a
	}
	if ts.Status != want {
		a.h.T.Errorf("spec %q task %q status = %q, want %q", a.slug, id, ts.Status, want)
	}
	return a
}

// TaskEvidence asserts a task's recorded evidence contains substr.
func (a *StateAsserter) TaskEvidence(id, substr string) *StateAsserter {
	a.h.T.Helper()
	ts, ok := a.state.Tasks[id]
	if !ok {
		a.h.T.Errorf("spec %q has no task %q", a.slug, id)
		return a
	}
	if ts.Evidence == nil || !strings.Contains(*ts.Evidence, substr) {
		got := "<nil>"
		if ts.Evidence != nil {
			got = *ts.Evidence
		}
		a.h.T.Errorf("spec %q task %q evidence = %q, want to contain %q", a.slug, id, got, substr)
	}
	return a
}

// HasBlocker asserts a blocker is recorded for the given task id.
func (a *StateAsserter) HasBlocker(taskID string) *StateAsserter {
	a.h.T.Helper()
	for _, b := range a.state.Blockers {
		if b.Task == taskID {
			return a
		}
	}
	a.h.T.Errorf("spec %q: expected a blocker for task %q, none found", a.slug, taskID)
	return a
}

// NoBlockers asserts the blocker list is empty.
func (a *StateAsserter) NoBlockers() *StateAsserter {
	a.h.T.Helper()
	if len(a.state.Blockers) != 0 {
		a.h.T.Errorf("spec %q: expected no blockers, got %d", a.slug, len(a.state.Blockers))
	}
	return a
}

// AcceptanceStatus asserts the recorded status ("pass"/"fail") of a criterion
// key like "1.2".
func (a *StateAsserter) AcceptanceStatus(key, want string) *StateAsserter {
	a.h.T.Helper()
	rec, ok := a.state.Acceptance[key]
	if !ok {
		a.h.T.Errorf("spec %q: no acceptance record for %q", a.slug, key)
		return a
	}
	if rec.Status != want {
		a.h.T.Errorf("spec %q criterion %q status = %q, want %q", a.slug, key, rec.Status, want)
	}
	return a
}

// --- Filesystem assertions (Harness-level) ---

// AssertFileExists fails if the slash-relative path under the project root is
// missing.
func (h *Harness) AssertFileExists(rel string) {
	h.T.Helper()
	if _, err := os.Stat(h.Path(rel)); err != nil {
		h.T.Errorf("expected file %q to exist: %v", rel, err)
	}
}

// AssertFileAbsent fails if the path exists.
func (h *Harness) AssertFileAbsent(rel string) {
	h.T.Helper()
	if _, err := os.Stat(h.Path(rel)); err == nil {
		h.T.Errorf("expected file %q to be absent, but it exists", rel)
	}
}

// ReadFile returns the contents of a slash-relative path, failing on error.
func (h *Harness) ReadFile(rel string) string {
	h.T.Helper()
	b, err := os.ReadFile(h.Path(rel))
	if err != nil {
		h.T.Fatalf("ReadFile(%q): %v", rel, err)
	}
	return string(b)
}

// AssertFileContains fails if the file is missing or lacks substr.
func (h *Harness) AssertFileContains(rel, substr string) {
	h.T.Helper()
	if got := h.ReadFile(rel); !strings.Contains(got, substr) {
		h.T.Errorf("file %q does not contain %q\n--- contents ---\n%s", rel, substr, got)
	}
}

// SpecArtifact reads a spec artifact (e.g. "tasks.md") for assertions.
func (h *Harness) SpecArtifact(slug, name string) string {
	h.T.Helper()
	return h.ReadFile(filepath.ToSlash(filepath.Join(".specd", "specs", slug, name)))
}
