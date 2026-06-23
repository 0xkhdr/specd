package core

import (
	"strings"
	"testing"
)

// report_cov_test.go fills the report.go branches the existing report_test.go
// leaves uncovered: every GetBadge status, ExtractSection hit/miss, the
// execSummary Introduction/Overview fallback chain, blockers, and the
// verification/telemetry formatting edge cells (empty coverage, sandbox,
// annotated cost).

func TestGetBadgeAllStatuses(t *testing.T) {
	cases := map[SpecStatus]string{
		StatusRequirements:  "Planning",
		StatusDesign:        "Planning",
		StatusTasks:         "Planning",
		StatusExecuting:     "Implementing",
		StatusVerifying:     "Verifying",
		StatusComplete:      "Complete",
		StatusBlocked:       "Blocked",
		SpecStatus("weird"): "Unknown",
	}
	for status, label := range cases {
		if got := GetBadge(status); got.Label != label {
			t.Errorf("GetBadge(%s).Label = %q, want %q", status, got.Label, label)
		}
	}
}

func TestExtractSectionHitMiss(t *testing.T) {
	md := "## Introduction\nhello world\n\n## Next\ntail\n"
	got := ExtractSection(&md, "Introduction")
	if got == nil || !strings.Contains(*got, "hello world") {
		t.Fatalf("introduction section = %v", got)
	}
	// Heading absent → nil.
	if ExtractSection(&md, "Nonexistent") != nil {
		t.Error("missing heading should yield nil")
	}
	// Empty body → nil.
	empty := "## Empty\n## Next\n"
	if ExtractSection(&empty, "Empty") != nil {
		t.Error("empty section body should yield nil")
	}
	// nil md → nil.
	if ExtractSection(nil, "x") != nil {
		t.Error("nil md should yield nil")
	}
}

func TestExecSummaryFallbackChain(t *testing.T) {
	// Introduction wins when present.
	intro := "## Introduction\nintro text\n"
	if s := execSummary(ReportData{State: &State{}, Requirements: &intro}); !strings.Contains(s, "intro text") {
		t.Errorf("intro summary = %q", s)
	}
	// Falls back to design Overview when no requirements intro.
	overview := "## Overview\ndesign overview\n"
	if s := execSummary(ReportData{State: &State{}, Design: &overview}); !strings.Contains(s, "design overview") {
		t.Errorf("overview summary = %q", s)
	}
	// Neither → placeholder.
	if s := execSummary(ReportData{State: &State{}}); !strings.Contains(s, "No summary provided") {
		t.Errorf("placeholder summary = %q", s)
	}
}

func TestReportFormattingEdges(t *testing.T) {
	cov := "" // empty coverage → "—"
	_ = cov
	st := &State{
		Spec:   "demo",
		Title:  "Demo",
		Status: StatusBlocked,
		Tasks: map[string]TaskState{
			"T1": {ID: "T1", Wave: 1, Status: TaskComplete, Verification: &VerificationRecord{
				Verified: false, Coverage: "", Sandbox: "bwrap", ChangedFiles: []string{"a.go"},
			}},
			"T2": {ID: "T2", Wave: 1, Status: TaskComplete, Telemetry: &Telemetry{
				DurationMs: 2500, VerifyDurationMs: 1200, Retries: 1, Tokens: 4096, Cost: "0.42",
			}},
		},
		Blockers: []Blocker{{Task: "T1", Reason: "stuck", Since: "now"}},
	}
	md := RenderMarkdown(ReportData{State: st})
	for _, want := range []string{
		"Blockers", "stuck",
		"sandbox: bwrap",
		"❌",    // unverified mark
		"4096", // tokens rendered
		"0.42", // annotated cost
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report missing %q:\n%s", want, md)
		}
	}

	// HTML render of the same data exercises esc + section HTML assembly.
	html := RenderHTML(ReportData{State: st}, 5)
	if !strings.Contains(html, "http-equiv=\"refresh\"") {
		t.Error("auto-refresh meta missing when seconds > 0")
	}
}

func TestFormatHelpers(t *testing.T) {
	if humanMs(0) != "—" || humanMs(500) != "500ms" || !strings.HasSuffix(humanMs(2500), "s") {
		t.Errorf("humanMs wrong: %q %q %q", humanMs(0), humanMs(500), humanMs(2500))
	}
	if tokensStr(0) != "—" || tokensStr(10) != "10" {
		t.Errorf("tokensStr wrong")
	}
	if costStr(1.5, false) != "—" || costStr(1.5, true) != "1.50" {
		t.Errorf("costStr wrong: %q %q", costStr(1.5, false), costStr(1.5, true))
	}
}
