package orchestration

import (
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// TestSenseCostFromAcceptedTelemetryOnly pins R4.1: a snapshot's cost is
// populated only from accepted telemetry folded by AccrueTelemetry — never a
// value a test can hand-set in a way production cannot reproduce. Absent
// telemetry stays unknown (never zero-filled); a worker report is known but
// untrusted; a single unknown component poisons the whole total.
func TestSenseCostFromAcceptedTelemetryOnly(t *testing.T) {
	now := time.Unix(1, 0).UTC()

	empty := Sense(core.State{}, nil, nil, AccrueTelemetry(nil), now)
	if empty.TelemetryKnown || empty.CostMicros != 0 || empty.TelemetryTrusted {
		t.Fatalf("no telemetry must be unknown, not zero: %#v", empty)
	}

	trusted := AccrueTelemetry([]ACPEvent{{Kind: ACPKindReport, Observation: &ObservationV1{Version: "1", Known: true, Source: "attested", Unit: "micro-usd", CostMicros: 50, Tokens: 5}}})
	snap := Sense(core.State{}, nil, nil, trusted, now)
	if !snap.TelemetryKnown || !snap.TelemetryTrusted || snap.CostMicros != 50 || snap.Tokens != 5 {
		t.Fatalf("trusted telemetry snapshot = %#v", snap)
	}

	hint := AccrueTelemetry([]ACPEvent{{Kind: ACPKindReport, Observation: &ObservationV1{Version: "1", Known: true, Source: "worker", Unit: "micro-usd", CostMicros: 7}}})
	if !hint.Known || hint.Trusted || hint.CostMicros != 7 {
		t.Fatalf("worker telemetry must be known-but-untrusted: %#v", hint)
	}

	poisoned := AccrueTelemetry([]ACPEvent{
		{Kind: ACPKindReport, Observation: &ObservationV1{Version: "1", Known: true, Source: "attested", Unit: "micro-usd", CostMicros: 9}},
		{Kind: ACPKindReport, Observation: &ObservationV1{Version: "1", Known: false}},
	})
	if poisoned.Known || poisoned.CostMicros != 0 {
		t.Fatalf("an unknown component must leave the total unknown: %#v", poisoned)
	}

	// Non-report events never contribute cost.
	ignored := AccrueTelemetry([]ACPEvent{{Kind: ACPKindDispatch}})
	if ignored.Known {
		t.Fatalf("non-report events must not populate cost: %#v", ignored)
	}
}
