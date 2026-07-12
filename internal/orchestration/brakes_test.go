package orchestration

import (
	"strings"
	"testing"
	"time"
)

// TestBrakeDormantWhenMaxCostUnset proves the cost brake is honest: with no
// configured MaxCostMicros it never fires however high the accrued cost climbs,
// and a positive limit trips only against known telemetry.
func TestBrakeDormantWhenMaxCostUnset(t *testing.T) {
	snap := Snapshot{CostMicros: 1 << 30, TelemetryKnown: true, TelemetryTrusted: true, Now: time.Now()}
	if d := EvaluateBrakes(snap, DecisionLimits{MaxCostMicros: 0}); d.Action != "" {
		t.Fatalf("unset cost limit already halts: %+v", d)
	}
	// A positive limit still trips, proving the brake works when armed.
	if d := EvaluateBrakes(snap, DecisionLimits{MaxCostMicros: 10}); d.Action != ActionHalt {
		t.Fatalf("armed cost brake failed to halt: %+v", d)
	}
	// Unknown cost never fabricates a halt, even under a positive limit.
	if d := EvaluateBrakes(Snapshot{Now: time.Now()}, DecisionLimits{MaxCostMicros: 10}); d.Action != "" {
		t.Fatalf("unknown cost fabricated a halt: %+v", d)
	}
}

func TestBrakesUnknownAndExceededTelemetry(t *testing.T) {
	now := time.Unix(10, 0).UTC()
	unknown := EvaluateBrakes(Snapshot{Now: now}, DecisionLimits{RequireTelemetry: true})
	if unknown.Action != ActionHalt || unknown.Reason != "required telemetry unknown" {
		t.Fatalf("unknown brake = %#v", unknown)
	}
	cost := EvaluateBrakes(Snapshot{Now: now, TelemetryKnown: true, CostMicros: 11}, DecisionLimits{MaxCostMicros: 10})
	if cost.Action != ActionHalt {
		t.Fatalf("cost brake = %#v", cost)
	}
	tokens := EvaluateBrakes(Snapshot{Now: now, TelemetryKnown: true, Tokens: 101}, DecisionLimits{MaxTokens: 100})
	if tokens.Action != ActionHalt {
		t.Fatalf("token brake = %#v", tokens)
	}
}

// TestBrakeRequiresTrustedTelemetry pins R4.3: production requiring telemetry
// fails closed with one actionable message when it is missing or untrusted, and
// a brake fired on untrusted data is labelled as such while a trusted brake is
// not.
func TestBrakeRequiresTrustedTelemetry(t *testing.T) {
	now := time.Unix(10, 0).UTC()
	require := DecisionLimits{RequireTelemetry: true, MaxCostMicros: 100}

	missing := EvaluateBrakes(Snapshot{Now: now}, require)
	if missing.Action != ActionHalt || !strings.Contains(missing.Reason, "unknown") {
		t.Fatalf("missing telemetry must fail closed: %#v", missing)
	}
	untrustedRequired := EvaluateBrakes(Snapshot{Now: now, TelemetryKnown: true, CostMicros: 1}, require)
	if untrustedRequired.Action != ActionHalt || !strings.Contains(untrustedRequired.Reason, "untrusted") {
		t.Fatalf("untrusted required telemetry must fail closed: %#v", untrustedRequired)
	}

	overUntrusted := EvaluateBrakes(Snapshot{Now: now, TelemetryKnown: true, CostMicros: 200}, DecisionLimits{MaxCostMicros: 100})
	if overUntrusted.Action != ActionHalt || !strings.Contains(overUntrusted.Reason, "untrusted") {
		t.Fatalf("untrusted cost brake must be labelled: %#v", overUntrusted)
	}
	overTrusted := EvaluateBrakes(Snapshot{Now: now, TelemetryKnown: true, TelemetryTrusted: true, CostMicros: 200}, DecisionLimits{MaxCostMicros: 100})
	if overTrusted.Action != ActionHalt || strings.Contains(overTrusted.Reason, "untrusted") {
		t.Fatalf("trusted cost brake must not be labelled untrusted: %#v", overTrusted)
	}
}
