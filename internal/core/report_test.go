package core

import (
	"strings"
	"testing"
)

func TestReportModel(t *testing.T) {
	tasks := []TaskRow{
		{ID: "T1", Marker: "✅", Verify: "go test ./..."},
		{ID: "T2", Marker: "⬜"},
	}
	model := BuildReportModel("demo", tasks, nil, map[string]EvidenceRecord{
		"T1": {TaskID: "T1", EvidenceRef: "ledger:1"},
	})
	if model.Total != 2 || model.Complete != 1 || model.Pending != 1 {
		t.Fatalf("unexpected counts: %#v", model)
	}
	if model.Tasks[0].EvidenceRef != "ledger:1" {
		t.Fatalf("missing evidence ref: %#v", model.Tasks[0])
	}
}

func TestPRSummaryGolden(t *testing.T) {
	model := BuildReportModel("demo", []TaskRow{{ID: "T1", Marker: "✅", Verify: "go test ./..."}, {ID: "T2"}}, nil, nil)
	got := PRSummary(model)
	for _, want := range []string{"## specd report: demo", "- complete: 1/2", "| T1 | complete | go test ./... |"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q:\n%s", want, got)
		}
	}
}

func TestMetricsGolden(t *testing.T) {
	model := BuildReportModel("demo", []TaskRow{{ID: "T1", Marker: "✅"}, {ID: "T2"}}, nil, nil)
	got := RenderMetrics(model)
	for _, want := range []string{
		`specd_tasks_total{spec="demo"} 2`,
		`specd_tasks_complete{spec="demo"} 1`,
		`specd_tasks_pending{spec="demo"} 1`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("metrics missing %q:\n%s", want, got)
		}
	}
}

func TestNoLLMInRender(t *testing.T) {
	model := BuildReportModel("demo", []TaskRow{{ID: "T1"}}, nil, nil)
	renderers := []string{RenderStatus(model), PRSummary(model), RenderMetrics(model)}
	for _, rendered := range renderers {
		lower := strings.ToLower(rendered)
		if strings.Contains(lower, "llm") || strings.Contains(lower, "network") {
			t.Fatalf("renderer leaked non-deterministic path wording: %s", rendered)
		}
	}
}

func TestForbiddenTool(t *testing.T) {
	for _, name := range []string{"report", "decision", "memory"} {
		if !ForbiddenTool(name) {
			t.Fatalf("%s should be forbidden", name)
		}
	}
	if ForbiddenTool("check") {
		t.Fatal("check should be allowed")
	}
}
