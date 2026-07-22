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
