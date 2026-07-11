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

// TestHistoryTelemetryTokensAreConflated characterizes the W0 gap the later
// token-split work closes: worker-reported tokens are a single scalar with no
// input/output/cache breakdown. Proven on the aggregated telemetry report's
// JSON surface, which history rendering draws from.
func TestHistoryTelemetryTokensAreConflated(t *testing.T) {
	records := []EvidenceRecord{{TaskID: "T1", Telemetry: &Annotations{Tokens: 100}}}
	report := AggregateTelemetry(records, []string{"T1"})
	blob, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	js := string(blob)
	for _, absent := range []string{"input_tokens", "output_tokens", "cache_tokens"} {
		if strings.Contains(js, absent) {
			t.Fatalf("W0 gap closed early: token breakdown %q present: %s", absent, js)
		}
	}
	if !strings.Contains(js, `"tokens":100`) {
		t.Fatalf("conflated tokens scalar missing: %s", js)
	}
}
