package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// completableDemo drives the demo spec to the point where T1 has a passing
// verify record and can be completed.
func completableDemo(t *testing.T) string {
	t.Helper()
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)
	return root
}

func TestTaskTelemetry(t *testing.T) {
	root := completableDemo(t)

	// Malformed telemetry on verify fails closed (R2), writing nothing extra.
	if err := Run(root, "verify", []string{"demo", "T1"}, map[string]string{"cost": "1,00"}); err == nil {
		t.Fatal("malformed --cost should fail closed")
	}

	// Verify with valid telemetry stores it verbatim on the evidence record (R1).
	if err := Run(root, "verify", []string{"demo", "T1"}, map[string]string{"tokens": "1200", "cost": "0.034", "duration-ms": "45000"}); err != nil {
		t.Fatalf("verify with telemetry: %v", err)
	}
	records, err := core.LoadEvidenceRecords(core.EvidencePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	var found *core.Annotations
	for _, r := range records {
		if r.Telemetry != nil {
			found = r.Telemetry
		}
	}
	if found == nil || found.Tokens != 1200 || found.Cost != "0.034" || found.DurationMs != 45000 {
		t.Fatalf("telemetry not stored verbatim: %+v", found)
	}

	// task complete without telemetry stays fully valid (R5).
	if err := Run(root, "task", []string{"complete", "demo", "T1"}, nil); err != nil {
		t.Fatalf("task complete without telemetry: %v", err)
	}
}

func TestReportMetrics(t *testing.T) {
	root := completableDemo(t)
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := Run(root, "task", []string{"complete", "demo", "T1"}, map[string]string{"tokens": "300", "cost": "0.3", "duration-ms": "30"}); err != nil {
		t.Fatalf("task complete with telemetry: %v", err)
	}

	out, err := captureStdout(t, func() error { return Run(root, "report", []string{"demo"}, map[string]string{"metrics": ""}) })
	if err != nil {
		t.Fatalf("report --metrics: %v", err)
	}
	for _, want := range []string{
		`specd_tasks_total{spec="demo"} 1`,
		`specd_cost_tokens_total{spec="demo"} 300`,
		`specd_cost_total{spec="demo"} 0.3`,
		`specd_task_telemetry_present{spec="demo",task="T1"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metrics missing %q:\n%s", want, out)
		}
	}
}
