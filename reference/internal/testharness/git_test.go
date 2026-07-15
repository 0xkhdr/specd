package testharness_test

import (
	"os"
	"path/filepath"
	"testing"

	th "github.com/0xkhdr/specd/internal/testharness"
)

func TestInitGitGivesHead(t *testing.T) {
	h := th.New(t)
	h.InitGit()
	if head := h.GitHead(); len(head) < 7 {
		t.Errorf("GitHead after InitGit = %q, want a commit hash", head)
	}
}

func TestGitCommitAllAdvancesHead(t *testing.T) {
	h := th.New(t)
	h.InitGit()
	before := h.GitHead()

	// Arrange: a new tracked file.
	if err := os.WriteFile(filepath.Join(h.Root, "note.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	after := h.GitCommitAll("add note")

	// Assert
	if after == before {
		t.Error("GitCommitAll did not advance HEAD")
	}
	if after != h.GitHead() {
		t.Errorf("GitCommitAll returned %q, GitHead = %q", after, h.GitHead())
	}
}
