package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// gitInitRepo makes root a git repo with one commit so gitHead resolves to a
// real, pinnable HEAD — the resubmit idempotence guard (R5) keys on it.
func gitInitRepo(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// advanceToExecuting drives the demo spec through the approval gates into the
// execute phase, where the terminal submit verb is valid (R6).
func advanceToExecuting(t *testing.T, root string) {
	t.Helper()
	for _, gate := range []string{"requirements", "design", "executing"} {
		if err := Run(root, "approve", []string{"demo", gate}, nil); err != nil {
			t.Fatalf("approve %s: %v", gate, err)
		}
	}
}

func TestSubmitRefusesUntilComplete(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)

	// Out of phase: a spec still in requirements refuses submit (R6, exit 2).
	if err := Run(root, "submit", []string{"demo"}, nil); err == nil {
		t.Fatal("submit in perceive phase should fail closed")
	}

	advanceToExecuting(t, root)

	// In phase but T1 not complete: R1 enumerates the incomplete task.
	err := Run(root, "submit", []string{"demo"}, nil)
	if err == nil {
		t.Fatal("submit with incomplete task should refuse")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmitDryRunAndLedger(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)

	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := Run(root, "task", []string{"complete", "demo", "T1"}, nil); err != nil {
		t.Fatalf("task complete: %v", err)
	}

	// R3: no submit.command configured ⇒ print summary to stdout, exit 0.
	out, err := captureStdout(t, func() error { return Run(root, "submit", []string{"demo"}, nil) })
	if err != nil {
		t.Fatalf("dry-run submit: %v", err)
	}
	if !strings.Contains(out, "## specd report: demo") {
		t.Fatalf("dry-run did not print PR summary:\n%s", out)
	}
	// Dry-run records nothing.
	if recs, _ := core.LoadSubmissions(core.SubmissionsPath(root, "demo")); len(recs) != 0 {
		t.Fatalf("dry-run should not append a ledger record, got %d", len(recs))
	}

	// R2/R4: configure an operator command; the summary is streamed to it and a
	// submission record is appended.
	writeProjectConfig(t, root, "submit:\n  command: cat\n")
	out, err = captureStdout(t, func() error { return Run(root, "submit", []string{"demo"}, nil) })
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !strings.Contains(out, "## specd report: demo") {
		t.Fatalf("submit did not stream summary to command:\n%s", out)
	}
	recs, err := core.LoadSubmissions(core.SubmissionsPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 submission record, got %d", len(recs))
	}
	if recs[0].Exit != 0 || recs[0].GitHead == "" || recs[0].SummaryHash == "" {
		t.Fatalf("submission record incomplete: %+v", recs[0])
	}

	// R5: a second submit at the same HEAD refuses without --resubmit.
	if err := Run(root, "submit", []string{"demo"}, nil); err == nil {
		t.Fatal("same-HEAD resubmit should refuse")
	}
	if _, err := captureStdout(t, func() error {
		return Run(root, "submit", []string{"demo"}, map[string]string{"resubmit": ""})
	}); err != nil {
		t.Fatalf("--resubmit should succeed: %v", err)
	}
	if recs, _ := core.LoadSubmissions(core.SubmissionsPath(root, "demo")); len(recs) != 2 {
		t.Fatalf("want 2 records after resubmit, got %d", len(recs))
	}
}

func writeProjectConfig(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write project.yml: %v", err)
	}
}
