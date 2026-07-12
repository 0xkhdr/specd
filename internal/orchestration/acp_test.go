package orchestration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestACPTelemetryEnvelope pins the W1 fail-closed rules on ACP telemetry
// (spec 07 R1.2). Legacy worker telemetry (bare cost) is grandfathered; a
// malformed canonical envelope is rejected on both append and decode.
func TestACPTelemetryEnvelope(t *testing.T) {
	dir := t.TempDir()

	legacy := filepath.Join(dir, "legacy.jsonl")
	if err := AppendACP(legacy, ACPEvent{Kind: ACPKindReport, TaskID: "T1",
		Telemetry: &core.Annotations{Cost: "0.01"}}); err != nil {
		t.Fatalf("legacy telemetry rejected: %v", err)
	}

	if err := AppendACP(filepath.Join(dir, "bad.jsonl"), ACPEvent{Kind: ACPKindReport, TaskID: "T1",
		Telemetry: &core.Annotations{EnvelopeVersion: "v1", Source: "worker", Cost: "0.02"}}); err == nil {
		t.Fatal("canonical cost-without-currency accepted on append")
	}

	decode := filepath.Join(dir, "decode.jsonl")
	os.WriteFile(decode, []byte(`{"kind":"report","task_id":"T1","telemetry":{"envelope_version":"v2"}}`+"\n"), 0o644)
	if _, err := ReadACP(decode); err == nil {
		t.Fatal("unknown envelope version accepted on decode")
	}
}

func TestACPReportPinsTraceDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acp.jsonl")
	events := []ObservableEvent{
		{RunID: "r1", EventID: "a", Seq: 1, Tool: "read", Time: "t", Actor: "w", Paths: []string{"b.go"}},
		{RunID: "r1", EventID: "b", Seq: 2, Tool: "edit", Time: "t", Actor: "w", Paths: []string{"a.go", "b.go"}},
	}
	if err := AppendACP(path, ACPEvent{Kind: ACPKindReport, TaskID: "T1", TraceDigest: TraceDigest(events)}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadACP(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TraceDigest != TraceDigest(events) {
		t.Fatalf("trace digest not round-tripped: %+v", got)
	}
}

func TestACPRejectsInvalidObservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acp.jsonl")
	err := AppendACP(path, ACPEvent{Kind: ACPKindReport, Observation: &ObservationV1{Version: "1", Known: true, Source: "worker", Unit: "usd", CostMicros: 1}})
	if err == nil {
		t.Fatal("invalid observation appended")
	}
}

func TestHarnessAffectedPathsIsDerivedNotClaimed(t *testing.T) {
	events := []ObservableEvent{
		{RunID: "r1", EventID: "a", Seq: 1, Tool: "read", Time: "t", Actor: "w", Paths: []string{"b.go"}},
		{RunID: "r1", EventID: "b", Seq: 2, Tool: "edit", Time: "t", Actor: "w", Paths: []string{"a.go", "b.go"}},
	}
	got := HarnessAffectedPaths(events)
	if len(got) != 2 || got[0] != "a.go" || got[1] != "b.go" {
		t.Fatalf("harness paths = %v (want sorted unique a.go,b.go)", got)
	}
}
