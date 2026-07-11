package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// newCriterionSpec scaffolds a spec whose requirements declare R1 with two
// acceptance criteria (1.1, 1.2) and R2 with one (2.1), approved through the
// design gate so execution verbs and criterion recording are unlocked.
func newCriterionSpec(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := Run(root, "init", nil, nil); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := Run(root, "new", []string{"demo"}, nil); err != nil {
		t.Fatalf("new: %v", err)
	}
	dir := filepath.Join(root, ".specd", "specs", "demo")
	reqs := "# Requirements — demo\n\n" +
		"- **R1** When a user submits, the system shall respond.\n" +
		"  - When input is valid, the system shall accept it.\n" +
		"  - When input is invalid, the system shall reject it.\n" +
		"- **R2** When idle, the system shall wait.\n"
	design := "# Design — demo\n\n## Modules\nThe check module runs gates.\n\n## On-disk contracts\nstate.json holds status.\n\n## Invariants\nOutput is deterministic.\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.md"), []byte(reqs), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "design.md"), []byte(design), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTasks(t, root, "demo", "| T1 | scout | spec.md | - | true | ok |")
	if err := Run(root, "approve", []string{"demo", "requirements"}, nil); err != nil {
		t.Fatalf("approve requirements: %v", err)
	}
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err != nil {
		t.Fatalf("approve design: %v", err)
	}
	return root
}

func TestVerifyCriterion(t *testing.T) {
	root := newCriterionSpec(t)

	// Happy path: record a pass for a valid criterion.
	if err := Run(root, "verify", []string{"demo"}, map[string]string{"criterion": "1.2", "status": "pass", "evidence": "covered by T1"}); err != nil {
		t.Fatalf("verify --criterion: %v", err)
	}
	records, err := core.LoadCriteria(core.CriteriaPath(root, "demo"))
	if err != nil || len(records) != 1 || records[0].Criterion != "1.2" {
		t.Fatalf("criterion record not written: %v %+v", err, records)
	}

	// The criterion path must never create a task verify record (R7).
	if _, statErr := os.Stat(core.EvidencePath(root, "demo")); statErr == nil {
		t.Fatal("criterion path wrote a task evidence record")
	}

	// Unknown criterion id fails closed (exit 2, R2).
	if err := Run(root, "verify", []string{"demo"}, map[string]string{"criterion": "9.9", "status": "pass", "evidence": "x"}); err == nil {
		t.Fatal("unknown criterion should fail closed")
	}

	// Missing evidence fails closed.
	if err := Run(root, "verify", []string{"demo"}, map[string]string{"criterion": "1.1", "status": "pass"}); err == nil {
		t.Fatal("missing evidence should fail closed")
	}
}

func TestCriteriaRequiredCompletionGate(t *testing.T) {
	root := newCriterionSpec(t)
	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte("criteria.required: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, gate := range []string{"tasks", "executing", "verifying"} {
		if err := Run(root, "approve", []string{"demo", gate}, nil); err != nil {
			t.Fatalf("approve %s: %v", gate, err)
		}
	}

	// With criteria.required on and no passing records, completion is refused.
	if err := Run(root, "approve", []string{"demo", "complete"}, nil); err == nil {
		t.Fatal("completion should refuse while criteria unmet")
	}

	// Attest every criterion, then completion succeeds.
	for _, id := range []string{"1.1", "1.2", "2.1"} {
		if err := Run(root, "verify", []string{"demo"}, map[string]string{"criterion": id, "status": "pass", "evidence": "e"}); err != nil {
			t.Fatalf("record %s: %v", id, err)
		}
	}
	if err := Run(root, "approve", []string{"demo", "complete"}, nil); err != nil {
		t.Fatalf("completion should pass once all criteria met: %v", err)
	}
}

func TestStatusCriteria(t *testing.T) {
	root := newCriterionSpec(t)
	if err := Run(root, "verify", []string{"demo"}, map[string]string{"criterion": "1.1", "status": "pass", "evidence": "a"}); err != nil {
		t.Fatalf("record 1.1: %v", err)
	}

	out, err := captureStdout(t, func() error { return Run(root, "status", []string{"demo"}, nil) })
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	// R1 has 1 of 2 criteria passing; total 1/3.
	for _, want := range []string{"Acceptance criteria coverage", "R1  1/2", "R2  0/1", "1/3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status coverage missing %q:\n%s", want, out)
		}
	}
}

func TestReportCriteria(t *testing.T) {
	root := newCriterionSpec(t)
	if err := Run(root, "verify", []string{"demo"}, map[string]string{"criterion": "2.1", "status": "pass", "evidence": "b"}); err != nil {
		t.Fatalf("record 2.1: %v", err)
	}
	out, err := captureStdout(t, func() error { return Run(root, "report", []string{"demo"}, nil) })
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if !strings.Contains(out, "R2  1/1") || !strings.Contains(out, "1/3") {
		t.Fatalf("report coverage wrong:\n%s", out)
	}
}
