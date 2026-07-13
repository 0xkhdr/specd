package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEventGoldenOffline(t *testing.T) {
	e := EventV1{SchemaVersion: EventSchemaV1, EventID: "evt-1", RunID: "run-1", SpanID: "span-1", ParentSpanID: "span-0", SpecID: "demo", TaskID: "T1", Attempt: 2, Kind: SpanVerify, Status: "pass", GitHead: strings.Repeat("a", 40), TelemetrySource: TelemetrySourceWorker, EvidenceRef: "artifacts/verify.json"}
	got, err := RenderEventsJSON([]EventV1{e})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":"event/v1","event_id":"evt-1","run_id":"run-1","span_id":"span-1","parent_span_id":"span-0","spec_id":"demo","task_id":"T1","attempt":2,"kind":"verify","status":"pass","git_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","telemetry_source":"worker","evidence_ref":"artifacts/verify.json"}` + "\n"
	if got != want {
		t.Fatalf("golden mismatch\nwant %s\n got %s", want, got)
	}
	if strings.Contains(got, "prompt") || strings.Contains(got, "response") {
		t.Fatalf("content leaked: %s", got)
	}
}

func TestEventRejectsUnknownVersionAndKind(t *testing.T) {
	base := EventV1{SchemaVersion: EventSchemaV1, EventID: "e", SpecID: "s", Kind: SpanTool}
	bad := base
	bad.SchemaVersion = "event/v2"
	if _, err := RenderEventsJSON([]EventV1{bad}); err == nil {
		t.Fatal("unknown version accepted")
	}
	bad = base
	bad.Kind = "critical-unknown"
	if _, err := RenderEventsJSON([]EventV1{bad}); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestEventAdapterRoundTripPreservesCorrelationAndPrivacy(t *testing.T) {
	in := EventV1{SchemaVersion: EventSchemaV1, EventID: "evt", RunID: "run", SpanID: "span", ParentSpanID: "parent", SpecID: "spec", TaskID: "T1", Kind: SpanContext, Status: "ok", TelemetrySource: TelemetrySourceAdapter, AttestationRef: "sha256:abc", EvidenceRef: "ledger:1", Redactions: []string{"prompt", "source_content"}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out EventV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if err := out.Validate(); err != nil {
		t.Fatal(err)
	}
	if out.RunID != in.RunID || out.SpanID != in.SpanID || out.ParentSpanID != in.ParentSpanID || out.AttestationRef != in.AttestationRef || out.EvidenceRef != in.EvidenceRef || strings.Join(out.Redactions, ",") != strings.Join(in.Redactions, ",") {
		t.Fatalf("round trip lost fields: %#v", out)
	}
	for _, forbidden := range []string{"raw_content", "chain_of_thought", "/home/"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("privacy leak %q", forbidden)
		}
	}
}
