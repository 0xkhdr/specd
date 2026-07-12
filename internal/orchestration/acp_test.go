package orchestration

import (
	"path/filepath"
	"testing"
)

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
