package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestSpecDiff exercises the read-only artifact diff over git history through
// the survivor `specd report --diff`: a clean build of the command, a real
// modification across two commits, a bad ref that fails closed without
// panicking, and a deterministic (sorted) result.
func TestSpecDiff(t *testing.T) {
	h := th.New(t)
	h.InitGit()
	h.RunExpect(core.ExitOK, "new", "alpha")
	h.GitCommitAll("add spec alpha")

	// Modify an artifact and commit again.
	reqPath := h.SpecPath("alpha", "requirements.md")
	body, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reqPath, append(body, []byte("\nedited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	h.GitCommitAll("edit alpha requirements")

	res := h.RunExpect(core.ExitOK, "report", "--diff", "alpha", "--from", "HEAD~1", "--to", "HEAD")
	if !strings.Contains(res.Stdout, "modified") || !strings.Contains(res.Stdout, "requirements.md") {
		t.Errorf("expected modified requirements.md, got:\n%s", res.Stdout)
	}

	// --from is required.
	h.RunExpect(core.ExitUsage, "report", "--diff", "alpha")

	// A bad ref fails closed (exit 1) rather than panicking.
	h.RunExpect(core.ExitGate, "report", "--diff", "alpha", "--from", "no-such-ref")

	// Determinism: identical invocation, identical output.
	a := h.RunExpect(core.ExitOK, "report", "--diff", "alpha", "--from", "HEAD~1", "--to", "HEAD")
	b := h.RunExpect(core.ExitOK, "report", "--diff", "alpha", "--from", "HEAD~1", "--to", "HEAD")
	if a.Stdout != b.Stdout {
		t.Errorf("diff output not deterministic:\n%q\nvs\n%q", a.Stdout, b.Stdout)
	}
}
