package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	contextpkg "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// enableContextSnapshot turns on the context-snapshot gate for a CLI test,
// leaving every other field to LoadConfig defaults via partial merge.
func enableContextSnapshot(t *testing.T, root string) {
	t.Helper()
	raw := []byte("orchestration:\n  enabled: true\n  resilience:\n    contextSnapshotEnabled: true\n")
	if err := os.WriteFile(filepath.Join(root, ".specd", "config.yml"), raw, 0o644); err != nil {
		t.Fatalf("write context-snapshot config: %v", err)
	}
}

// TestContextSnapshotGatedOff: with the feature off, --snapshot is a no-op error
// and writes nothing (R2, Req 4).
func TestContextSnapshotGatedOff(t *testing.T) {
	h := th.New(t)
	slug := execSpec(h, th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}})
	out := filepath.Join(h.Root, "snap.json")

	h.RunExpect(core.ExitUsage, "context", slug, "--snapshot", "--out", out)
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("snapshot written while feature disabled (stat err=%v)", err)
	}
}

// TestContextSnapshotEmitAndDelta (R2, T7): emit a snapshot, modify exactly one
// loaded file, and assert the diff reports that file changed and the rest
// reference — re-contextualization is O(changed files).
func TestContextSnapshotEmitAndDelta(t *testing.T) {
	h := th.New(t)
	slug := execSpec(h, th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}})
	enableContextSnapshot(t, h.Root)
	out := filepath.Join(h.Root, "snap.json")

	h.RunExpect(core.ExitOK, "context", slug, "--snapshot", "--out", out)

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snap contextpkg.ContextSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if err := contextpkg.ValidateContextSnapshot(snap); err != nil {
		t.Fatalf("emitted snapshot invalid: %v", err)
	}
	if len(snap.LoadedFiles) < 2 {
		t.Fatalf("loaded files = %d, want >= 2 (spec artifacts)", len(snap.LoadedFiles))
	}

	// Clean diff: nothing changed.
	if diff, err := contextpkg.DiffContextSnapshot(snap, h.Root); err != nil {
		t.Fatal(err)
	} else if len(diff.Changed) != 0 {
		t.Fatalf("clean diff changed = %v, want none", diff.Changed)
	}

	// Modify exactly one loaded file.
	target := snap.LoadedFiles[0].Path
	if err := os.WriteFile(filepath.Join(h.Root, target), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := contextpkg.DiffContextSnapshot(snap, h.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Changed) != 1 || diff.Changed[0] != target {
		t.Fatalf("changed = %v, want exactly [%s]", diff.Changed, target)
	}
	if len(diff.Unchanged) != len(snap.LoadedFiles)-1 {
		t.Fatalf("unchanged = %d, want %d", len(diff.Unchanged), len(snap.LoadedFiles)-1)
	}
}
