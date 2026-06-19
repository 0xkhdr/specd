package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReplaySessionTimelineIncludesDecisionReason(t *testing.T) {
	events := []ACPEnvelope{
		{
			Sequence:  2,
			CreatedAt: "2026-01-01T00:00:02Z",
			Type:      ACPMessageProgress,
			Task:      "T1",
			Payload:   json.RawMessage(`{"percent":50,"message":"half done"}`),
		},
		{
			Sequence:  1,
			CreatedAt: "2026-01-01T00:00:01Z",
			Type:      ACPMessageMission,
			Spec:      "auth",
			Task:      "T1",
			Payload:   json.RawMessage(`{"role":"builder"}`),
			Decision: &OrchestrationDecision{
				Action:     OrchestrationDispatch,
				Spec:       "auth",
				TaskID:     "T1",
				Reason:     "frontier task ready",
				Escalation: OrchestrationEscalation{Code: EscalationNone},
			},
		},
	}

	timeline := ReplaySessionTimeline(events)
	if len(timeline) != 2 {
		t.Fatalf("timeline len = %d, want 2", len(timeline))
	}
	if timeline[0].Sequence != 1 {
		t.Fatalf("first sequence = %d, want 1", timeline[0].Sequence)
	}
	if timeline[0].Action != string(OrchestrationDispatch) {
		t.Fatalf("action = %q", timeline[0].Action)
	}
	if timeline[0].Reason != "frontier task ready" {
		t.Fatalf("reason = %q", timeline[0].Reason)
	}
	if timeline[1].Detail != "50% half done" {
		t.Fatalf("detail = %q", timeline[1].Detail)
	}
}

func TestExplainCurrentSessionDecisionPrefersLatestDecision(t *testing.T) {
	events := []ACPEnvelope{
		{
			Sequence: 1,
			Type:     ACPMessageMission,
			Payload:  json.RawMessage(`{"role":"builder"}`),
			Decision: &OrchestrationDecision{Action: OrchestrationDispatch, Reason: "dispatch T1"},
		},
		{
			Sequence: 2,
			Type:     ACPMessageProgress,
			Payload:  json.RawMessage(`{"message":"working"}`),
		},
		{
			Sequence: 3,
			Type:     ACPMessageBlocker,
			Payload:  json.RawMessage(`{"blocker":"needs approval"}`),
			Decision: &OrchestrationDecision{Action: OrchestrationEscalate, Reason: "worker blocked", Escalation: OrchestrationEscalation{Code: EscalationHumanIntervention}},
		},
	}

	event, ok := ExplainCurrentSessionDecision(events)
	if !ok {
		t.Fatal("ExplainCurrentSessionDecision ok = false")
	}
	if event.Action != string(OrchestrationEscalate) {
		t.Fatalf("action = %q", event.Action)
	}
	if event.Escalation != string(EscalationHumanIntervention) {
		t.Fatalf("escalation = %q", event.Escalation)
	}
	line := FormatSessionTimelineEvent(event)
	for _, want := range []string{"escalation=human-intervention", "reason=worker blocked", "detail=needs approval"} {
		if !strings.Contains(line, want) {
			t.Fatalf("formatted event %q missing %q", line, want)
		}
	}
}
