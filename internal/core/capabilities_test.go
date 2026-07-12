package core

import "testing"

func TestCapabilitiesDeterministicPolicy(t *testing.T) {
	full := NegotiateHostCapabilities(HostCapabilities{ContextLoading: true, Sandbox: true, Telemetry: true, Eval: true, A2A: true})
	for _, result := range full.Results {
		if result.Status != CapabilitySupported || result.Recovery != "" {
			t.Fatalf("full capability result = %+v", result)
		}
	}
	limited := NegotiateHostCapabilities(HostCapabilities{})
	if len(limited.Results) != 5 {
		t.Fatalf("capability omission silently hidden: %+v", limited)
	}
	for _, result := range limited.Results {
		want := CapabilityDowngraded
		if result.Capability == "sandbox" {
			want = CapabilityRefused
		}
		if result.Status != want || result.Reason == "" || result.Recovery == "" {
			t.Fatalf("limited capability result = %+v, want %s", result, want)
		}
	}
}

func TestCapabilityNamesStable(t *testing.T) {
	got := CapabilityNames()
	want := []string{"a2a", "context_loading", "eval", "sandbox", "telemetry"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("names = %v, want %v", got, want)
		}
	}
}
