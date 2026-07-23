package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	speccontext "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
)

// ackSession records a complete context receipt and binds authority, which is
// what `specd session ack` does. Mutable operations refuse until this has run.
func ackSession(t *testing.T, root, slug, taskID string, session core.DriverSession) core.DriverSession {
	t.Helper()
	spec, err := loadSpec(root, slug)
	if err != nil {
		t.Fatal(err)
	}
	config, _ := core.LoadConfig(configPaths(root), getenv())
	manifest, err := speccontext.BuildMachineManifest(root, slug, spec.Tasks, taskID, "context", "execute",
		contextBudget(root), core.BootstrapHandshake(config))
	if err != nil {
		t.Fatal(err)
	}
	required := speccontext.RequiredDigests(manifest)
	receipt, err := core.BuildContextReceipt(manifest.ManifestDigest, session.Driver, session.Driver, required, required, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.RecordContextReceipt(root, slug, session.ID, receipt, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated, err := core.BindAuthority(root, slug, session.ID, "authority-digest", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

// R2.2 live: with no session open, mutable operations proceed unbound. This is
// the graduated behaviour that keeps hand-driven specd usable.
func TestSessionBindingAbsentSessionIsUnenforced(t *testing.T) {
	root := diffScopeRepo(t)
	state := core.State{Slug: "demo", Status: core.StatusExecuting, Phase: core.PhaseExecute, Revision: 0}
	if err := enforceSessionBinding(root, "demo", "T1", state, nil, time.Now()); err != nil {
		t.Fatalf("unbound operation refused with no session open: %v", err)
	}
}

// R2.2 live: an open session makes every binding mandatory.
func TestSessionBindingOpenSessionRequiresBindings(t *testing.T) {
	root := diffScopeRepo(t)
	if _, err := core.OpenDriverSession(root, "demo", "host", "hs", gitHead(root), 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	state := core.State{Slug: "demo", Status: core.StatusExecuting, Phase: core.PhaseExecute, Revision: 0}

	err := enforceSessionBinding(root, "demo", "T1", state, nil, time.Now())
	if err == nil {
		t.Fatal("an open session accepted an operation carrying no bindings")
	}
	refusal, ok := core.AsRefusal(err)
	if !ok || refusal.Code != "BINDING_MISSING" {
		t.Fatalf("got %v, want BINDING_MISSING", err)
	}
	if !strings.Contains(refusal.RecoveryCommand, "session action") {
		t.Errorf("refusal does not name the command that mints bindings: %q", refusal.RecoveryCommand)
	}

	// R7.1: the attempt is observed.
	events, loadErr := core.LoadConformanceEvents(core.ConformancePath(root, "demo"))
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(events) == 0 {
		t.Fatal("no conformance event recorded for an unbound mutable attempt")
	}
}

// R3.2 live: authority is withheld until the required context is acknowledged.
func TestSessionBindingWithheldUntilContextAcknowledged(t *testing.T) {
	root := diffScopeRepo(t)
	session, err := core.OpenDriverSession(root, "demo", "host", "hs", gitHead(root), 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	state := core.State{Slug: "demo", Status: core.StatusExecuting, Phase: core.PhaseExecute, Revision: 0}
	flags := map[string]string{"session": session.ID, "nonce": "n1"}

	err = enforceSessionBinding(root, "demo", "T1", state, flags, time.Now())
	if err == nil {
		t.Fatal("mutable authority activated with no context acknowledged")
	}
	if !strings.Contains(err.Error(), "acknowledged no context") {
		t.Fatalf("got %v, want a context-acknowledgement refusal", err)
	}
}

// R2.4 live: the nonce is spent by a successful operation and refused on reuse.
func TestSessionBindingNonceSpentOnUse(t *testing.T) {
	root := diffScopeRepo(t)
	session, err := core.OpenDriverSession(root, "demo", "host", "hs", gitHead(root), 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	session = ackSession(t, root, "demo", "T1", session)
	state := core.State{Slug: "demo", Status: core.StatusExecuting, Phase: core.PhaseExecute, Revision: 0}
	flags := map[string]string{"session": session.ID, "nonce": "nonce-1"}

	if err := enforceSessionBinding(root, "demo", "T1", state, flags, time.Now()); err != nil {
		t.Fatalf("fully bound operation refused: %v", err)
	}
	err = enforceSessionBinding(root, "demo", "T1", state, flags, time.Now())
	if err == nil {
		t.Fatal("the same nonce was accepted twice")
	}
	if refusal, ok := core.AsRefusal(err); !ok || refusal.Code != "NONCE_REPLAYED" {
		t.Fatalf("got %v, want NONCE_REPLAYED", err)
	}

	// R7.1: the replay is observed as a distinct event kind.
	events, loadErr := core.LoadConformanceEvents(core.ConformancePath(root, "demo"))
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	found := false
	for _, event := range events {
		if event.Kind == core.ConformanceStaleActionReplayed {
			found = true
		}
	}
	if !found {
		t.Fatalf("replay not recorded as %s: %+v", core.ConformanceStaleActionReplayed, events)
	}
}

// A closed session returns the spec to the graduated path. Closing is the only
// way out, and it is a visible act rather than a hidden flag.
func TestSessionBindingCloseReturnsToUnenforced(t *testing.T) {
	root := diffScopeRepo(t)
	if _, err := core.OpenDriverSession(root, "demo", "host", "hs", gitHead(root), 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	state := core.State{Slug: "demo", Status: core.StatusExecuting, Phase: core.PhaseExecute, Revision: 0}
	if err := enforceSessionBinding(root, "demo", "T1", state, nil, time.Now()); err == nil {
		t.Fatal("open session did not enforce")
	}
	if err := core.CloseDriverSession(root, "demo"); err != nil {
		t.Fatal(err)
	}
	if err := enforceSessionBinding(root, "demo", "T1", state, nil, time.Now()); err != nil {
		t.Fatalf("closed session still enforced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".specd", "specs", "demo", "driver-session.json")); !os.IsNotExist(err) {
		t.Error("closed session left its file behind")
	}
}

func TestBoundCompletionTransaction(t *testing.T) {
	t.Run("scope_refusal_preserves_nonce_and_untracked_baseline", func(t *testing.T) {
		root := diffScopeRepo(t)
		if err := os.WriteFile(filepath.Join(root, "preexisting.txt"), []byte("scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		session, err := core.OpenDriverSession(root, "demo", "host", "hs", gitHead(root), 0, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		session = ackSession(t, root, "demo", "T1", session)
		state := core.State{Slug: "demo", Status: core.StatusExecuting, Phase: core.PhaseExecute, Revision: 0}
		flags := map[string]string{"session": session.ID, "nonce": "retry-me"}
		spec, err := loadSpec(root, "demo")
		if err != nil {
			t.Fatal(err)
		}
		task, _ := findTaskRow(spec.Tasks, "T1")

		if err := validateSessionBinding(root, "demo", "T1", state, flags, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := enforceDiffScope(root, "demo", "T1", task); err != nil {
			t.Fatalf("pre-existing untracked path refused: %v", err)
		}
		later := filepath.Join(root, "later.txt")
		if err := os.WriteFile(later, []byte("new\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := enforceDiffScope(root, "demo", "T1", task); err == nil {
			t.Fatal("later untracked path accepted")
		}
		loaded, err := core.LoadDriverSession(core.DriverSessionPath(root, "demo"))
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(loaded.SpentNonces, "retry-me") {
			t.Fatal("non-mutating scope refusal spent nonce")
		}
		if err := os.Remove(later); err != nil {
			t.Fatal(err)
		}
		if err := enforceSessionBinding(root, "demo", "T1", state, flags, time.Now()); err != nil {
			t.Fatalf("corrected retry with same nonce refused: %v", err)
		}
	})

	t.Run("exact_harness_marker_is_not_worker_scope", func(t *testing.T) {
		root := diffScopeRepo(t)
		session, err := core.OpenDriverSession(root, "demo", "host", "hs", gitHead(root), 0, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		_ = session
		state := core.State{
			SchemaVersion: core.StateSchemaVersion,
			Slug:          "demo",
			Mode:          core.ModeDefault,
			Status:        core.StatusExecuting,
			Phase:         core.PhaseExecute,
			Cycle:         1,
			Stage:         core.StageExecuting,
			Condition:     core.ConditionActive,
			TaskStatus:    map[string]core.TaskRunStatus{"T1": core.TaskComplete},
		}
		if err := core.SaveState(core.StatePath(root, "demo"), state); err != nil {
			t.Fatal(err)
		}
		tasksPath := filepath.Join(root, ".specd", "specs", "demo", "tasks.md")
		raw, err := os.ReadFile(tasksPath)
		if err != nil {
			t.Fatal(err)
		}
		marked, err := core.RewriteTaskStatusLine(raw, "T1", "✅")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tasksPath, marked, 0o644); err != nil {
			t.Fatal(err)
		}
		spec, err := loadSpec(root, "demo")
		if err != nil {
			t.Fatal(err)
		}
		task, _ := findTaskRow(spec.Tasks, "T1")
		if err := enforceDiffScope(root, "demo", "T1", task); err != nil {
			t.Fatalf("exact harness marker refused: %v", err)
		}
		if err := os.WriteFile(tasksPath, append(marked, []byte("\ndirect edit\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := enforceDiffScope(root, "demo", "T1", task); err == nil {
			t.Fatal("direct harness edit accepted")
		}
	})
}
