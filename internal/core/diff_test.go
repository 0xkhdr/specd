package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffDerivesTrackedAndUntracked(t *testing.T) {
	root := t.TempDir()
	runGit := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	runGit("init")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "base")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := DeriveDiff(root, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Paths) != 2 || d.Paths[0] != "a.go" || d.Paths[1] != "b.go" {
		t.Fatalf("paths=%v", d.Paths)
	}
}

func TestDiffExcludesHarnessMetadata(t *testing.T) {
	root := t.TempDir()
	runGit := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	runGit("init")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "base")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))
	if err := os.MkdirAll(filepath.Join(root, ".specd/specs/demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo/acp.jsonl"), []byte("event"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := DeriveDiff(root, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Paths) != 0 {
		t.Fatalf("harness metadata leaked into worker scope: %v", d.Paths)
	}
}
