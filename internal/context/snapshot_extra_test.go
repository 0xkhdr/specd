package contextpkg

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotValidationAndDiffEdges(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd", "steering"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd", "steering", "product.md"), []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd", "steering", "memory.md"), []byte("m"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := MissionContextManifest{Version: ManifestVersion, Items: []MissionContextItem{
		{Path: "a.md"},
		{Path: "a.md"}, // duplicate ignored
		{Path: "missing.md"},
		{Command: "specd status"},
	}}
	snap, err := BuildContextSnapshot(root, 2, "EXECUTE", "T1", manifest, time.Date(2026, 6, 27, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildContextSnapshot: %v", err)
	}
	if len(snap.LoadedFiles) != 1 || snap.LoadedFiles[0].Lines != [2]int{1, 2} {
		t.Fatalf("loaded files = %#v", snap.LoadedFiles)
	}
	if snap.SteeringDigest == "" || snap.MemoryDigest == "" {
		t.Fatalf("missing digests: %#v", snap)
	}
	if _, err := CanonicalSnapshotJSON(snap); err != nil {
		t.Fatalf("CanonicalSnapshotJSON: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd", "steering", "memory.md"), []byte("m2"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := DiffContextSnapshot(snap, root)
	if err != nil {
		t.Fatalf("DiffContextSnapshot: %v", err)
	}
	if len(diff.Changed) != 1 || diff.Changed[0] != "a.md" || !diff.MemoryChanged || diff.SteeringChanged {
		t.Fatalf("diff = %#v", diff)
	}

	bad := snap
	bad.LoadedFiles = []LoadedFile{{Path: "x", SHA256: "not-hex", Lines: [2]int{2, 1}}}
	if err := ValidateContextSnapshot(bad); err == nil {
		t.Fatal("ValidateContextSnapshot accepted invalid file")
	}
	bad = snap
	bad.Task = ""
	if err := ValidateContextSnapshot(bad); err == nil {
		t.Fatal("ValidateContextSnapshot accepted missing task")
	}
}
