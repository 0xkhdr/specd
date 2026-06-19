package core

import (
	"strings"
	"testing"
	"time"
)

func TestOrchestrationEngineRecordsOneIdempotentDispatch(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("8", 32)
	cfg := DefaultConfig.Orchestration
	policy := validOrchestrationPolicy()
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	first, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.Decision.Action != OrchestrationDispatch || first.Decision.TaskID != "T1" {
		t.Fatalf("decision = %#v, want dispatch T1", first.Decision)
	}
	if first.Event == nil || first.Event.Type != ACPMessageMission || first.Event.Sequence != 1 {
		t.Fatalf("event = %#v, want first mission event", first.Event)
	}
	second, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.Event == nil || second.Event.MessageID != first.Event.MessageID || second.Event.Sequence != 1 {
		t.Fatalf("second step not idempotent: first=%#v second=%#v", first.Event, second.Event)
	}
}
