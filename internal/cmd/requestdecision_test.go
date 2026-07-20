package cmd

import (
	"encoding/json"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestRequestDecision pins R1.1/R1.2: an agent that hits a deviation has a route
// its own authority permits. The route records the request and nothing else —
// it is not agent-side approval wearing a different name, so it must advance no
// phase, complete no task, and write no evidence.
func TestRequestDecision(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")
	before, _ := core.LoadState(statePath)

	command, ok := core.CommandByName("request-decision")
	if !ok {
		t.Fatal("request-decision missing from the palette")
	}
	if command.HumanOnly {
		t.Fatal("request-decision is human-only, so no role may take it — that is the gap it exists to close")
	}

	if err := Run(root, "request-decision", []string{"demo"}, map[string]string{"text": "retry needs backoff", "scope": "design"}); err != nil {
		t.Fatalf("request-decision: %v", err)
	}

	state, _ := core.LoadState(statePath)
	var rec core.Record
	if err := json.Unmarshal(state.Records["decision-request:0"], &rec); err != nil {
		t.Fatalf("decision-request record: %v", err)
	}
	if rec.Text != "retry needs backoff" || rec.Scope != "design" {
		t.Fatalf("content not round-tripped: %+v", rec)
	}
	if rec.Timestamp == "" || rec.Actor == "" || rec.GitHead == "" {
		t.Fatalf("record not stamped: %+v", rec)
	}
	// A request is not an answer: it must not masquerade as an approval record.
	if _, isApproval := state.Records["approval:"+string(state.Phase)]; isApproval {
		t.Fatal("request-decision wrote an approval record")
	}
	if state.Phase != before.Phase || state.Status != before.Status {
		t.Fatalf("phase/status advanced: %q/%q -> %q/%q", before.Phase, before.Status, state.Phase, state.Status)
	}
	if len(state.TaskStatus) != len(before.TaskStatus) {
		t.Fatalf("task status mutated: %v -> %v", before.TaskStatus, state.TaskStatus)
	}
	evidence, err := core.LoadEvidence(core.EvidencePath(root, "demo"))
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	if len(evidence) != 0 {
		t.Fatalf("request-decision wrote evidence: %v", evidence)
	}

	// Same contract as decision/midreq: no content, no record.
	if err := Run(root, "request-decision", []string{"demo"}, map[string]string{"text": " "}); err == nil {
		t.Fatal("blank --text: want usage error, got nil")
	}
}
