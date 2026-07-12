package orchestration

import (
	"strings"
	"testing"
)

func TestTelemetryObservationValidationAndRedaction(t *testing.T) {
	obs := ObservationV1{Version: "1", Known: true, Source: "worker", Unit: "micro-usd", CostMicros: 12, Tokens: 34, Route: &RouteFact{Class: "reviewed", Provider: " provider ", Model: " model ", Reason: strings.Repeat("x", 300)}}
	got, err := NormalizeObservation(obs)
	if err != nil {
		t.Fatal(err)
	}
	if got.Route.Provider != "provider" || got.Route.Model != "model" || len(got.Route.Reason) != MaxObservationText {
		t.Fatalf("normalized observation = %#v", got)
	}
	for _, bad := range []ObservationV1{
		{Version: "2"},
		{Version: "1", Known: false, CostMicros: 1},
		{Version: "1", Known: true, Source: "", Unit: "micro-usd"},
		{Version: "1", Known: true, Source: "worker", Unit: "usd", CostMicros: 1},
		{Version: "1", Known: true, Source: "worker", Unit: "micro-usd", Route: &RouteFact{Provider: "secret=abc"}},
	} {
		if _, err := NormalizeObservation(bad); err == nil {
			t.Fatalf("accepted %#v", bad)
		}
	}
}

func TestTelemetryFactsNotCompletionProof(t *testing.T) {
	event := ACPEvent{Kind: ACPKindReport, TaskID: "T1", Observation: &ObservationV1{Version: "1", Known: true, Source: "worker", Unit: "micro-usd", CostMicros: 1}}
	if event.VerifyRef != "" {
		t.Fatal("observation created verify reference")
	}
	if _, err := NormalizeObservation(*event.Observation); err != nil {
		t.Fatal(err)
	}
}
