package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestReviewCmd(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)

	// R1: scaffold the report.
	if _, err := captureStdout(t, func() error { return Run(root, "review", []string{"demo"}, nil) }); err != nil {
		t.Fatalf("review: %v", err)
	}
	raw, err := os.ReadFile(core.ReviewReportPath(root, "demo"))
	if err != nil {
		t.Fatalf("report not scaffolded: %v", err)
	}
	if !strings.Contains(string(raw), "Review Report — demo") || !strings.Contains(string(raw), "### T1") {
		t.Fatalf("scaffold content wrong:\n%s", raw)
	}

	// R2: same-HEAD re-scaffold refuses without --force.
	if err := Run(root, "review", []string{"demo"}, nil); err == nil {
		t.Fatal("re-scaffold at same HEAD should refuse without --force")
	}
	if _, err := captureStdout(t, func() error {
		return Run(root, "review", []string{"demo"}, map[string]string{"force": ""})
	}); err != nil {
		t.Fatalf("--force re-scaffold should succeed: %v", err)
	}
}

// TestReviewRestampPreservesBody pins the command-side half of R5.1/R5.2: an
// existing report is never clobbered by a re-scaffold, whatever HEAD it names,
// and --restamp is the non-destructive way forward.
func TestReviewRestampPreservesBody(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)
	path := core.ReviewReportPath(root, "demo")

	if _, err := captureStdout(t, func() error { return Run(root, "review", []string{"demo"}, nil) }); err != nil {
		t.Fatalf("review: %v", err)
	}
	findings := "Audited the rounding path by hand."
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.Contains(line, "- **Verdict:**") && strings.Contains(line, "<") {
			lines[i] = "- **Verdict:** approve"
		}
		if strings.Contains(line, "<Required when the verdict") {
			lines[i] = findings
		}
	}
	filled := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}

	// Move HEAD so the filled report becomes stale.
	if err := os.WriteFile(filepath.Join(root, "moved.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "moved.txt"}, {"commit", "-m", "move head"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// existing-report + stale-head: a stale report still holds human findings, so
	// a bare re-scaffold must refuse rather than destroy them.
	err = Run(root, "review", []string{"demo"}, nil)
	if err == nil {
		t.Fatal("re-scaffold over a stale filled report should refuse")
	}
	if !strings.Contains(err.Error(), "--restamp") || !strings.Contains(err.Error(), "--force") {
		t.Errorf("refusal does not name both recoveries: %v", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != filled {
		t.Fatal("refused re-scaffold still modified the report")
	}

	// --restamp carries the body to the new HEAD.
	if _, err := captureStdout(t, func() error {
		return Run(root, "review", []string{"demo"}, map[string]string{"restamp": ""})
	}); err != nil {
		t.Fatalf("restamp: %v", err)
	}
	restamped, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restamped), findings) {
		t.Error("restamp dropped the human findings")
	}
	report, err := core.ParseReviewReport(string(restamped))
	if err != nil {
		t.Fatalf("restamped report does not parse: %v", err)
	}
	if report.Head != gitHead(root) {
		t.Errorf("restamped HEAD = %q, want current HEAD", report.Head)
	}

	// unknown-flag: a typo must be refused, not silently ignored. Checked on a
	// fresh root with no report, so an accepted flag would scaffold successfully
	// and the assertion cannot pass for the wrong reason.
	t.Run("unknown_flag_refused", func(t *testing.T) {
		fresh := newDemoSpec(t)
		gitInitRepo(t, fresh)
		advanceToExecuting(t, fresh)
		err := Run(fresh, "review", []string{"demo"}, map[string]string{"restmap": ""})
		if err == nil {
			t.Fatal("typo'd --restmap accepted; it would have overwritten the report")
		}
		if !strings.Contains(err.Error(), "restmap") {
			t.Errorf("refusal does not name the offending flag: %v", err)
		}
		if _, statErr := os.Stat(core.ReviewReportPath(fresh, "demo")); statErr == nil {
			t.Error("refused invocation still wrote a report")
		}
	})
}

func TestReviewRestamp(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)

	// Create initial review report with old HEAD
	if _, err := captureStdout(t, func() error { return Run(root, "review", []string{"demo"}, nil) }); err != nil {
		t.Fatalf("initial review scaffold: %v", err)
	}
	oldReport, err := os.ReadFile(core.ReviewReportPath(root, "demo"))
	if err != nil {
		t.Fatalf("read initial report: %v", err)
	}
	oldHeadLine := "- **Git HEAD:** " + gitHead(root)

	// Fill in the review with human findings
	// Find the verdict line and replace just the placeholder
	humanReport := string(oldReport)
	lines := strings.Split(humanReport, "\n")
	for i, line := range lines {
		if strings.Contains(line, "- **Verdict:**") && strings.Contains(line, "<") {
			lines[i] = "- **Verdict:** approve"
		}
		if strings.Contains(line, "<Required when the verdict") {
			lines[i] = "Reviewed logic carefully."
		}
	}
	humanReport = strings.Join(lines, "\n")
	humanReport = humanReport + "\nAdded edge case tests."
	if err := os.WriteFile(core.ReviewReportPath(root, "demo"), []byte(humanReport), 0o644); err != nil {
		t.Fatal(err)
	}

	// Make a new commit to get a different HEAD
	if err := os.WriteFile(filepath.Join(root, "new_file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "add", "new_file.txt")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", root, "commit", "-m", "test: new commit")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	newHead := gitHead(root)
	if oldHeadLine == "- **Git HEAD:** "+newHead {
		t.Fatal("test setup: git HEAD should have changed")
	}

	// Restamp should update the HEAD while preserving human findings
	if _, err := captureStdout(t, func() error {
		return Run(root, "review", []string{"demo"}, map[string]string{"restamp": ""})
	}); err != nil {
		t.Fatalf("restamp failed: %v", err)
	}

	restampedReport, err := os.ReadFile(core.ReviewReportPath(root, "demo"))
	if err != nil {
		t.Fatalf("read restamped report: %v", err)
	}

	restampedStr := string(restampedReport)

	// Should have new HEAD
	if !strings.Contains(restampedStr, "- **Git HEAD:** "+newHead) {
		t.Fatalf("new HEAD not in restamped report: %s", restampedStr)
	}

	// Should not have old HEAD
	if strings.Contains(restampedStr, oldHeadLine) {
		t.Fatalf("old HEAD still in restamped report: %s", restampedStr)
	}

	// Should preserve human findings
	if !strings.Contains(restampedStr, "Reviewed logic carefully") {
		t.Fatalf("human findings lost in restamp: %s", restampedStr)
	}
}

func TestReviewGateCompletion(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := Run(root, "complete-task", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("complete-task: %v", err)
	}
	// Gate on, no approve report ⇒ completion refused (R3/R5 fail closed).
	writeProjectConfig(t, root, "review:\n  required: true\n")
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("advance to verifying: %v", err)
	}
	if err := Run(root, "approve", []string{"demo"}, nil); err == nil {
		t.Fatal("completion should refuse without an approve review report")
	}

	// Write an approve report pinned to the current HEAD ⇒ completion allowed.
	head := gitHead(root)
	report := "# Review Report — demo\n\n- **Git HEAD:** " + head + "\n- **Reviewer:** auditor\n- **Verdict:** approve\n\n## Findings\n\nChecked.\n"
	if err := os.WriteFile(core.ReviewReportPath(root, "demo"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error { return Run(root, "approve", []string{"demo"}, nil) }); err != nil {
		t.Fatalf("completion with fresh approve report should succeed: %v", err)
	}
}
