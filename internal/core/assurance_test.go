package core

import "testing"

// TestAssuranceLevel pins R3.1/R3.2: every machine surface can name a level, and
// the level only ever moves down. An unrecognized value is not an error to
// report later — it is advisory now, because the alternative is advertising
// containment nobody is providing.
func TestAssuranceLevelParseDegrades(t *testing.T) {
	for _, value := range []string{"", "governed", "SANDBOXED", "sandboxed ", "enforced", "advisory"} {
		if got := ParseAssuranceLevel(value); got != AssuranceAdvisory {
			t.Errorf("ParseAssuranceLevel(%q) = %q, want advisory", value, got)
		}
	}
	for _, level := range []AssuranceLevel{AssuranceAdvisory, AssuranceGated, AssuranceSandboxed} {
		if got := ParseAssuranceLevel(string(level)); got != level {
			t.Errorf("ParseAssuranceLevel(%q) = %q, want round-trip", level, got)
		}
	}
}

func TestAssuranceLevelCeiling(t *testing.T) {
	if got := AssuranceCeiling(HostCapabilities{}); got != AssuranceAdvisory {
		t.Errorf("no sandbox: ceiling = %q, want advisory", got)
	}
	// Every other capability is irrelevant: only containment raises the ceiling.
	if got := AssuranceCeiling(HostCapabilities{ContextLoading: true, Telemetry: true, Eval: true, A2A: true}); got != AssuranceAdvisory {
		t.Errorf("no sandbox but everything else: ceiling = %q, want advisory", got)
	}
	if got := AssuranceCeiling(HostCapabilities{Sandbox: true}); got != AssuranceSandboxed {
		t.Errorf("sandbox: ceiling = %q, want sandboxed", got)
	}
}

func TestAssuranceLevelNeverUpgrades(t *testing.T) {
	// Declaring more than the host backs is capped, not honored.
	if got := AssuranceFor(HostCapabilities{}, "sandboxed"); got != AssuranceAdvisory {
		t.Errorf("declared sandboxed on unsandboxed host = %q, want advisory", got)
	}
	if got := AssuranceFor(HostCapabilities{}, "gated"); got != AssuranceAdvisory {
		t.Errorf("declared gated on unsandboxed host = %q, want advisory", got)
	}
	// Declaring less than the host backs is honored: the ceiling caps, it never lifts.
	if got := AssuranceFor(HostCapabilities{Sandbox: true}, "gated"); got != AssuranceGated {
		t.Errorf("declared gated on sandboxed host = %q, want gated", got)
	}
	// An unknown declaration is advisory even where the host could back more.
	if got := AssuranceFor(HostCapabilities{Sandbox: true}, "fully-governed"); got != AssuranceAdvisory {
		t.Errorf("unknown declaration on sandboxed host = %q, want advisory", got)
	}
}
