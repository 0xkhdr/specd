package core

import (
	"strings"
	"testing"
)

// TestReportScope confirms the report surfaces per-task verify evidence —
// changed-file count and coverage — as data, never as a pass/fail floor.
func TestReportScope(t *testing.T) {
	st := &State{
		Spec: "demo", Tasks: map[string]TaskState{
			"T1": {ID: "T1", Status: TaskComplete, Verification: &VerificationRecord{
				Command: "go test", Verified: true, Coverage: "84%",
				ChangedFiles: []string{"a.go", "b.go"},
			}},
		},
	}
	md := RenderMarkdown(ReportData{State: st})
	if !strings.Contains(md, "Verification Evidence") {
		t.Error("missing Verification Evidence section")
	}
	if !strings.Contains(md, "84%") || !strings.Contains(md, "Changed files") {
		t.Errorf("coverage/changed-files not rendered:\n%s", md)
	}
}

// TestReportAcceptance confirms recorded acceptance criteria render in the report.
func TestReportAcceptance(t *testing.T) {
	st := &State{
		Spec:  "demo",
		Tasks: map[string]TaskState{},
		Acceptance: map[string]CriterionRecord{
			"1.1": {Requirement: 1, Criterion: 1, Status: "pass", Evidence: "manual proof", RanAt: "2026-01-01T00:00:00Z"},
		},
	}
	md := RenderMarkdown(ReportData{State: st})
	if !strings.Contains(md, "Acceptance Criteria") || !strings.Contains(md, "1.1") {
		t.Errorf("acceptance criteria not rendered:\n%s", md)
	}
}

// TestReportTelemetryRollup confirms the per-wave/per-spec telemetry roll-up
// renders when telemetry is present (and stays absent otherwise).
func TestReportTelemetryRollup(t *testing.T) {
	st := &State{
		Spec: "demo", Tasks: map[string]TaskState{
			"T1": {ID: "T1", Wave: 1, Status: TaskComplete, Telemetry: &Telemetry{DurationMs: 1000, Retries: 2}},
		},
	}
	md := RenderMarkdown(ReportData{State: st})
	if !strings.Contains(md, "Telemetry") {
		t.Errorf("telemetry roll-up not rendered:\n%s", md)
	}

	// No telemetry → no section.
	bare := RenderMarkdown(ReportData{State: &State{Spec: "x", Tasks: map[string]TaskState{"T1": {ID: "T1"}}}})
	if strings.Contains(bare, "⏱️") {
		t.Error("telemetry section should be absent when no task carries telemetry")
	}
}

// TestRenderHTMLLiveResponsive pins the browser-native dashboard contract: the
// rendered report carries a responsive viewport meta and a live-update
// EventSource client, and stays self-contained with no external asset fetches.
func TestRenderHTMLLiveResponsive(t *testing.T) {
	st := &State{Spec: "demo", Title: "Demo", Status: StatusExecuting, Tasks: map[string]TaskState{"T1": {ID: "T1"}}}
	html := RenderHTML(ReportData{State: st}, 0)

	for _, want := range []string{
		`<meta name="viewport"`,
		`new EventSource("/events")`,
		`/api/report?spec=`,
		`@media`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("RenderHTML output missing %q", want)
		}
	}
	// Self-contained: no external asset fetches.
	for _, bad := range []string{"http://", "https://", "//cdn", "src=\"//"} {
		if strings.Contains(html, bad) {
			t.Errorf("RenderHTML output has external reference %q", bad)
		}
	}
}
