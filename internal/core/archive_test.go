package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchivePreservesHashes(t *testing.T) {
	root := t.TempDir()
	spec := filepath.Join(SpecdDir(root), "specs", "old")
	if err := os.MkdirAll(spec, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spec, "requirements.md"), []byte("immutable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := ArchiveSpec(root, ArchiveRequest{SpecID: "old", SuccessorID: "new", Owner: "platform", EvidenceRef: "evidence:release-1"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Files[0].SHA256 != Digest([]byte("immutable\n")) {
		t.Fatalf("hash=%s", record.Files[0].SHA256)
	}
	if _, err := os.Stat(filepath.Join(SpecdDir(root), "archive", "specs", "old", "requirements.md")); err != nil {
		t.Fatal(err)
	}
	if got := ListSpecs(root); len(got) != 0 {
		t.Fatalf("archived spec active: %v", got)
	}
	again, err := ArchiveSpec(root, ArchiveRequest{SpecID: "old", SuccessorID: "new", Owner: "platform", EvidenceRef: "evidence:release-1"})
	if err != nil || again.ManifestHash != record.ManifestHash {
		t.Fatalf("replay=%+v err=%v", again, err)
	}
}
