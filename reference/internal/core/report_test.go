package core

import (
	"os"
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
func TestReportRenderGoldenDeterministic(t *testing.T) {
	d := reportGoldenData()
	cases := map[string]string{
		"markdown":  RenderMarkdown(d),
		"html":      RenderHTML(d, 5),
		"prsummary": BuildPRSummary(d.State, []Violation{{Gate: "design", Location: "design.md", Message: "missing detail"}}, []Violation{{Gate: "tasks", Location: "tasks.md", Message: "low confidence"}}, []CommitLink{{SHA: "abcdef0123456789", Subject: "finish T1", Tasks: []string{"T1"}}}).Markdown(),
	}
	files := map[string]string{
		"markdown":  "testdata/report_markdown.golden",
		"html":      "testdata/report_html.golden",
		"prsummary": "testdata/prsummary.golden",
	}
	second := map[string]string{
		"markdown":  RenderMarkdown(d),
		"html":      RenderHTML(d, 5),
		"prsummary": BuildPRSummary(d.State, []Violation{{Gate: "design", Location: "design.md", Message: "missing detail"}}, []Violation{{Gate: "tasks", Location: "tasks.md", Message: "low confidence"}}, []CommitLink{{SHA: "abcdef0123456789", Subject: "finish T1", Tasks: []string{"T1"}}}).Markdown(),
	}
	for name, got := range cases {
		if got != second[name] {
			t.Fatalf("%s output not deterministic across identical renders", name)
		}
		wantBytes, err := os.ReadFile(files[name])
		if err != nil {
			t.Fatalf("read %s golden: %v", name, err)
		}
		want := strings.ReplaceAll(string(wantBytes), "\r\n", "\n")
		if got != want {
			t.Fatalf("%s golden mismatch\nwant:\n%s\ngot:\n%s", name, want, got)
		}
	}
}

func TestReportHTMLGoldenEscapesUserContent(t *testing.T) {
	html := RenderHTML(reportGoldenData(), 5)
	for _, want := range []string{`<!doctype html>`, `Report &lt;Demo&gt;`, `&lt;script&gt;alert(1)&lt;/script&gt;`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing escaped content %q:\n%s", want, html)
		}
	}
	for _, bad := range []string{`<title>Report <Demo>`, `<script>alert(1)</script>`} {
		if strings.Contains(html, bad) {
			t.Fatalf("html contains unescaped user content %q:\n%s", bad, html)
		}
	}
}

func reportGoldenData() ReportData {
	ptr := func(s string) *string { return &s }
	return ReportData{
		State: &State{
			Spec: "report-demo", Title: "Report <Demo>", Status: StatusExecuting,
			Phase: PhaseExecute, Turn: 7, ExecutionMode: ModeOrchestrated, ModeOrigin: OriginUser,
			Tasks: map[string]TaskState{
				"T2": {ID: "T2", Title: "second", Role: "validator", Wave: 2, Status: TaskPending, Telemetry: &Telemetry{DurationMs: 500, Tokens: 25}},
				"T1": {ID: "T1", Title: "first", Role: "craftsman", Wave: 1, Status: TaskComplete, Verification: &VerificationRecord{Command: "go test", Verified: true, Coverage: "90.0%", ChangedFiles: []string{"internal/core/report.go"}, Sandbox: "bwrap"}, Telemetry: &Telemetry{DurationMs: 1500, VerifyDurationMs: 250, Retries: 1, Tokens: 100, Cost: "0.50"}},
			},
			Acceptance: map[string]CriterionRecord{"1.1": {Requirement: 1, Criterion: 1, Status: "pass", Evidence: "golden evidence", RanAt: "2026-01-01T00:00:00Z"}},
			Blockers:   []Blocker{{Task: "T2", Reason: "needs fixture", Since: "2026-01-01T00:00:00Z"}},
		},
		Requirements: ptr("# Requirements\n\n## Introduction\nStable report summary.\n\n## Acceptance Criteria\n- THE SYSTEM SHALL report."),
		Design:       ptr("# Design\n\n## Overview\nRender stable documents."),
		Tasks:        ptr("- [x] T1 first\n- [ ] T2 second"),
		Decisions:    ptr("# Decisions\n\n- ADR-1 keep deterministic order"),
		Memory:       ptr("# Memory\n\nEscaping sample: <script>alert(1)</script>"),
		MidReqs:      ptr("# Mid\n\nNo mid-requirements."),
	}
}

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
