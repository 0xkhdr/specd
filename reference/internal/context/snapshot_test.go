package contextpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// snapshotRoot lays out a minimal project with two loaded files, a steering set,
// and memory.md, and returns the root plus a manifest naming the two files.
func snapshotRoot(t *testing.T) (string, MissionContextManifest) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a\n\nfunc A() {}\n")
	writeFile(t, filepath.Join(root, "dir/b.go"), "package dir\n")
	writeFile(t, filepath.Join(root, ".specd/steering/tech.md"), "# tech\n")
	writeFile(t, filepath.Join(root, ".specd/steering/structure.md"), "# structure\n")
	writeFile(t, filepath.Join(root, ".specd/steering/memory.md"), "# memory\n- fact\n")
	manifest := MissionContextManifest{
		Version: ManifestVersion,
		Items: []MissionContextItem{
			{Path: "a.go"},
			{Path: "dir/b.go"},
			{Path: "missing.go"}, // not on disk — must be skipped
		},
	}
	return root, manifest
}

func TestContextSnapshotRoundTripsCanonical(t *testing.T) {
	root, manifest := snapshotRoot(t)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	snap, err := BuildContextSnapshot(root, 7, "executing", "T1", manifest, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.LoadedFiles) != 2 {
		t.Fatalf("loaded files = %d, want 2 (missing.go skipped)", len(snap.LoadedFiles))
	}
	if snap.SteeringDigest == "" || snap.MemoryDigest == "" {
		t.Fatalf("digests empty: %#v", snap)
	}

	raw, err := CanonicalSnapshotJSON(snap)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CanonicalSnapshotJSON(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, again) {
		t.Fatal("canonical JSON is not stable across calls")
	}
	if raw[len(raw)-1] != '\n' {
		t.Fatal("canonical JSON missing trailing newline")
	}
}

func TestContextSnapshotValidationRejectsMalformed(t *testing.T) {
	bad := ContextSnapshot{
		Version:     ManifestVersion,
		Task:        "T1",
		LoadedFiles: []LoadedFile{{Path: "a.go", SHA256: "nothex", Lines: [2]int{1, 2}}},
	}
	if err := ValidateContextSnapshot(bad); err == nil {
		t.Fatal("invalid sha256 accepted")
	}
	bad.LoadedFiles[0].SHA256 = ""
	bad.Task = ""
	if err := ValidateContextSnapshot(bad); err == nil {
		t.Fatal("missing task accepted")
	}
}

func TestDigestsStableUntilBytesChange(t *testing.T) {
	root, _ := snapshotRoot(t)

	steer1, err := steeringDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	mem1, err := memoryDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := steeringDigest(root); again != steer1 {
		t.Fatal("steering digest unstable for unchanged inputs")
	}

	// memory.md change must move only the memory digest, not steering.
	writeFile(t, filepath.Join(root, ".specd/steering/memory.md"), "# memory\n- new fact\n")
	mem2, _ := memoryDigest(root)
	steer2, _ := steeringDigest(root)
	if mem2 == mem1 {
		t.Fatal("memory digest did not change after edit")
	}
	if steer2 != steer1 {
		t.Fatal("steering digest changed when only memory.md changed")
	}

	// A steering edit moves the steering digest.
	writeFile(t, filepath.Join(root, ".specd/steering/tech.md"), "# tech v2\n")
	if steer3, _ := steeringDigest(root); steer3 == steer2 {
		t.Fatal("steering digest did not change after steering edit")
	}
}

func TestDiffContextSnapshotIsMinimal(t *testing.T) {
	root, manifest := snapshotRoot(t)
	snap, err := BuildContextSnapshot(root, 1, "executing", "T1", manifest, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// No change: both files reference, nothing reloads.
	diff, err := DiffContextSnapshot(snap, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Changed) != 0 || len(diff.Unchanged) != 2 || diff.SteeringChanged || diff.MemoryChanged {
		t.Fatalf("clean diff = %#v, want all unchanged", diff)
	}

	// Edit exactly one loaded file + memory.md.
	writeFile(t, filepath.Join(root, "a.go"), "package a\n\nfunc A() { _ = 1 }\n")
	writeFile(t, filepath.Join(root, ".specd/steering/memory.md"), "# memory\n- changed\n")
	diff, err = DiffContextSnapshot(snap, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Changed) != 1 || diff.Changed[0] != "a.go" {
		t.Fatalf("changed = %v, want [a.go]", diff.Changed)
	}
	if len(diff.Unchanged) != 1 || diff.Unchanged[0] != "dir/b.go" {
		t.Fatalf("unchanged = %v, want [dir/b.go]", diff.Unchanged)
	}
	if !diff.MemoryChanged || diff.SteeringChanged {
		t.Fatalf("digest flags = (steering %t, memory %t), want (false,true)", diff.SteeringChanged, diff.MemoryChanged)
	}
}
