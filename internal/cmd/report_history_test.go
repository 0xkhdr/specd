package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// newHistoryDemo builds a demo spec with a passing task T1 and a failing task
// T2, drives it into execution, records one pass, one fail, a completion, and a
// submission — the mix `report --history` and `--format prometheus` replay.
func newHistoryDemo(t *testing.T) string {
	t.Helper()
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)

	// A failing verify attempt in the evidence ledger (verify runs in a clean
	// pinned checkout, so we record the fail directly — the real, honest store).
	if err := core.AppendEvidence(core.EvidencePath(root, "demo"), core.EvidenceRecord{
		TaskID:   "T1",
		Command:  "false",
		ExitCode: 1,
		GitHead:  "0000000000000000000000000000000000000000",
	}); err != nil {
		t.Fatalf("seed failing evidence: %v", err)
	}
	// Then a real passing verify and completion.
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify T1: %v", err)
	}
	if err := Run(root, "task", []string{"complete", "demo", "T1"}, nil); err != nil {
		t.Fatalf("complete T1: %v", err)
	}

	// A submission ledger entry (spec 08), streamed to `cat`.
	writeProjectConfig(t, root, "submit:\n  command: cat\n")
	if _, err := captureStdout(t, func() error { return Run(root, "submit", []string{"demo"}, nil) }); err != nil {
		t.Fatalf("submit: %v", err)
	}
	return root
}

func TestReportHistoryReplaysAndIsDeterministic(t *testing.T) {
	root := newHistoryDemo(t)

	first, err := captureStdout(t, func() error { return Run(root, "report", []string{"demo"}, map[string]string{"history": ""}) })
	if err != nil {
		t.Fatalf("report --history: %v", err)
	}
	// Every event type is present in the replay.
	for _, want := range []string{"approval", "verify:pass", "verify:fail", "completion", "submission"} {
		if !strings.Contains(first, want) {
			t.Fatalf("history missing %q:\n%s", want, first)
		}
	}

	// R3: a second run is byte-identical.
	second, err := captureStdout(t, func() error { return Run(root, "report", []string{"demo"}, map[string]string{"history": ""}) })
	if err != nil {
		t.Fatalf("second report --history: %v", err)
	}
	if first != second {
		t.Fatalf("history not byte-identical across runs:\n--- 1 ---\n%s\n--- 2 ---\n%s", first, second)
	}
}

func TestReportHistoryJSONLineParses(t *testing.T) {
	root := newHistoryDemo(t)
	out, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"history": "", "json": ""})
	})
	if err != nil {
		t.Fatalf("report --history --json: %v", err)
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		var e core.HistoryEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("history JSON line %q does not parse: %v", line, err)
		}
		if e.Event == "" {
			t.Fatalf("history event missing event field: %q", line)
		}
	}
}

func TestReportPrometheusLintsAndCounts(t *testing.T) {
	root := newHistoryDemo(t)
	out, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"format": "prometheus"})
	})
	if err != nil {
		t.Fatalf("report --format prometheus: %v", err)
	}
	// Structural sanity: every sample line names a specd_ metric (the exhaustive
	// promtool-style lint lives in core's prometheus_test).
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "specd_") {
			t.Fatalf("unexpected metric line: %q", line)
		}
	}
	for _, want := range []string{
		`specd_tasks{spec="demo",status="complete"} 1`,
		`specd_verify_failures_total{spec="demo"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("prometheus missing %q:\n%s", want, out)
		}
	}
}

func TestReportRejectsUnknownFormat(t *testing.T) {
	root := newDemoSpec(t)
	if err := Run(root, "report", []string{"demo"}, map[string]string{"format": "html"}); err == nil {
		t.Fatal("unsupported --format must fail closed")
	}
}
