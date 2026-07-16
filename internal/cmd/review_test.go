package cmd

import (
	"os"
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
