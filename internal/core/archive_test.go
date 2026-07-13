package core

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func archiveGitRepo(t *testing.T, root string) string {
	t.Helper()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.test"}, {"config", "user.name", "Test"}, {"add", "."}, {"commit", "--allow-empty", "-m", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestArchivePreservesHashes(t *testing.T) {
	root := t.TempDir()
	spec := filepath.Join(SpecdDir(root), "specs", "old")
	if err := os.MkdirAll(spec, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spec, "requirements.md"), []byte("immutable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := InitialState("old")
	state.Status, state.Phase = StatusComplete, PhaseReflect
	if err := SaveState(StatePath(root, "old"), state); err != nil {
		t.Fatal(err)
	}
	head := archiveGitRepo(t, root)
	if err := AppendEvidence(EvidencePath(root, "old"), EvidenceRecord{TaskID: "T1", Command: "true", ExitCode: 0, GitHead: head}); err != nil {
		t.Fatal(err)
	}
	record, err := ArchiveSpec(root, ArchiveRequest{SpecID: "old", SuccessorID: "new", Owner: "platform", EvidenceRef: "artifact:release-1"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range record.Files {
		if file.Path == "requirements.md" && file.SHA256 == Digest([]byte("immutable\n")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("requirements hash missing: %+v", record.Files)
	}
	if _, err := os.Stat(filepath.Join(SpecdDir(root), "archive", "specs", "old", "requirements.md")); err != nil {
		t.Fatal(err)
	}
	if got := ListSpecs(root); len(got) != 0 {
		t.Fatalf("archived spec active: %v", got)
	}
	again, err := ArchiveSpec(root, ArchiveRequest{SpecID: "old", SuccessorID: "new", Owner: "platform", EvidenceRef: "artifact:release-1"})
	if err != nil || again.ManifestHash != record.ManifestHash {
		t.Fatalf("replay=%+v err=%v", again, err)
	}
}

func TestArchiveRejectsTraversalTamperAndRollsBack(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"../escape", "a/b", "."} {
		if _, err := ArchiveSpec(root, ArchiveRequest{SpecID: id, SuccessorID: "next", Owner: "team", EvidenceRef: "artifact:x"}); err == nil {
			t.Fatalf("accepted spec id %q", id)
		}
		if _, err := ArchiveSpec(root, ArchiveRequest{SpecID: "old", SuccessorID: id, Owner: "team", EvidenceRef: "artifact:x"}); err == nil {
			t.Fatalf("accepted successor id %q", id)
		}
	}
	spec := filepath.Join(SpecdDir(root), "specs", "old")
	if err := os.MkdirAll(spec, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spec, "requirements.md"), []byte("immutable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := InitialState("old")
	state.Status, state.Phase = StatusComplete, PhaseReflect
	if err := SaveState(StatePath(root, "old"), state); err != nil {
		t.Fatal(err)
	}
	head := archiveGitRepo(t, root)
	if err := AppendEvidence(EvidencePath(root, "old"), EvidenceRecord{TaskID: "T1", Command: "true", ExitCode: 0, GitHead: head}); err != nil {
		t.Fatal(err)
	}
	original := archiveAtomicWrite
	archiveAtomicWrite = func(string, string) error { return errors.New("injected manifest failure") }
	t.Cleanup(func() { archiveAtomicWrite = original })
	if _, err := ArchiveSpec(root, ArchiveRequest{SpecID: "old", SuccessorID: "next", Owner: "team", EvidenceRef: "artifact:x"}); err == nil {
		t.Fatal("injected failure accepted")
	}
	if _, err := os.Stat(spec); err != nil {
		t.Fatalf("source not rolled back: %v", err)
	}
	archiveAtomicWrite = original
	if _, err := ArchiveSpec(root, ArchiveRequest{SpecID: "old", SuccessorID: "next", Owner: "team", EvidenceRef: "artifact:x"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(SpecdDir(root), "archive", "specs", "old", "requirements.md")
	if err := os.WriteFile(path, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyArchive(root, "old"); err == nil {
		t.Fatal("tampered archive replay accepted")
	}
}
