package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSortHistoryTieBreakIsDeterministic(t *testing.T) {
	// Three events share a timestamp; a fourth is absent one. Ordering must be
	// resolved by (SourceRank, Seq) so the output is byte-identical every run.
	ts := "2026-07-05T10:00:00Z"
	events := []HistoryEvent{
		{Timestamp: ts, Event: "verify:pass", SourceRank: HistorySourceVerify, Seq: 1},
		{Timestamp: ts, Event: "approval", SourceRank: HistorySourceApproval, Seq: 0},
		{Timestamp: ts, Event: "verify:fail", SourceRank: HistorySourceVerify, Seq: 0},
		{Timestamp: "", Event: "orphan", SourceRank: HistorySourceACP, Seq: 9},
	}

	first := RenderHistory("demo", events)
	// Re-render from a reshuffled copy: same total order, same bytes.
	shuffled := []HistoryEvent{events[3], events[0], events[2], events[1]}
	second := RenderHistory("demo", shuffled)
	if first != second {
		t.Fatalf("history render not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}

	lines := strings.Split(strings.TrimRight(first, "\n"), "\n")
	// Line 0 is the header; the empty-timestamp event sorts first, then the
	// shared-timestamp trio in (SourceRank, Seq) order.
	wantOrder := []string{"orphan", "approval", "verify:fail", "verify:pass"}
	for i, want := range wantOrder {
		if !strings.Contains(lines[i+1], want) {
			t.Fatalf("line %d = %q, want event %q", i+1, lines[i+1], want)
		}
	}
}

func TestRenderHistoryJSONLineParses(t *testing.T) {
	events := []HistoryEvent{
		{Timestamp: "2026-07-05T10:00:00Z", Actor: "alice", Event: "approval", Reference: "gate=design", GitHead: "abcdef1234567890"},
		{Timestamp: "2026-07-05T10:01:00Z", Actor: "bob", Event: "verify:pass", Reference: "task=T1"},
	}
	out, err := RenderHistoryJSON(events)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 JSON lines, got %d", len(lines))
	}
	var first HistoryEvent
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 0 not valid JSON: %v", err)
	}
	if first.Event != "approval" || first.Actor != "alice" {
		t.Fatalf("round-trip lost fields: %+v", first)
	}
}

// TestHistorySpanKindMapping pins the single event-name → span-kind mapping the
// trace exporter reuses (spec 07 R6.1). Trace-worthy activities classify to a
// closed-enum kind; bookkeeping events (decision, submission, ACP claim/report)
// are not spans and return false so the trace stays a metadata trajectory.
func TestHistorySpanKindMapping(t *testing.T) {
	cases := []struct {
		event string
		kind  SpanKind
		ok    bool
	}{
		{"approval", SpanApproval, true},
		{"verify:pass", SpanVerify, true},
		{"verify:fail", SpanVerify, true},
		{"completion", SpanEval, true},
		{"criterion:pass", SpanEval, true},
		{"acp:dispatch", SpanDispatch, true},
		{"acp:report", "", false},
		{"acp:claim", "", false},
		{"decision", "", false},
		{"midreq", "", false},
		{"submission", "", false},
	}
	for _, c := range cases {
		got, ok := (HistoryEvent{Event: c.event}).SpanKind()
		if ok != c.ok || got != c.kind {
			t.Fatalf("SpanKind(%q) = (%q, %v), want (%q, %v)", c.event, got, ok, c.kind, c.ok)
		}
	}
}

// TestHistoryTelemetryTokenCategoriesPreserved flips W0's characterization
// when W7 closes the gap: the aggregate total remains while provider-neutral token
// categories survive the aggregated JSON surface.
func TestHistoryTelemetryTokenCategoriesPreserved(t *testing.T) {
	records := []EvidenceRecord{{TaskID: "T1", Telemetry: &Annotations{Tokens: 100, InputTokens: 60, OutputTokens: 30, CachedTokens: 10}}}
	report := AggregateTelemetry(records, []string{"T1"})
	blob, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	js := string(blob)
	for _, want := range []string{`"tokens":100`, `"input_tokens":60`, `"output_tokens":30`, `"cached_tokens":10`} {
		if !strings.Contains(js, want) {
			t.Fatalf("token breakdown %q missing: %s", want, js)
		}
	}
}
