package core

import (
	"strings"
	"testing"
)

// TestReportMetricsLabelsAllowlisted pins the task-count metrics surface to the
// cardinality allowlist (spec 07 R5.1): specd_tasks_* series carry only a `spec`
// label — no per-run, per-path, or per-actor label ever widens it.
func TestReportMetricsLabelsAllowlisted(t *testing.T) {
	out := RenderMetrics(ReportModel{Slug: "demo", Total: 5, Complete: 2, Running: 1, Blocked: 1, Pending: 1})
	for _, name := range MetricLabelNames(out) {
		if !MetricLabelAllowlist[name] {
			t.Fatalf("report metric emits label %q outside allowlist:\n%s", name, out)
		}
	}
	if names := MetricLabelNames(out); len(names) != 1 || names[0] != "spec" {
		t.Fatalf("report metrics should carry only the spec label, got %v", names)
	}
	if strings.Contains(out, "task=") {
		t.Fatalf("task-count metrics must not carry an unbounded task label:\n%s", out)
	}
}
