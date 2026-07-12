package core

import "sort"

// HostCapabilities describes optional host features. False is explicit: the
// negotiation result still reports every capability, so omission cannot hide
// a downgrade or refusal.
type HostCapabilities struct {
	ContextLoading bool `json:"context_loading"`
	Sandbox        bool `json:"sandbox"`
	Telemetry      bool `json:"telemetry"`
	Eval           bool `json:"eval"`
	A2A            bool `json:"a2a"`
}

type CapabilityStatus string

const (
	CapabilitySupported  CapabilityStatus = "supported"
	CapabilityDowngraded CapabilityStatus = "downgraded"
	CapabilityRefused    CapabilityStatus = "refused"
)

type CapabilityResult struct {
	Capability string           `json:"capability"`
	Status     CapabilityStatus `json:"status"`
	Reason     string           `json:"reason"`
	Recovery   string           `json:"recovery_action"`
}

type CapabilityReport struct {
	Results []CapabilityResult `json:"results"`
}

// NegotiateHostCapabilities applies deterministic policy before host actions.
// Sandbox is safety-critical and therefore refuses mutable execution without
// it; remaining optional features downgrade to the offline/local path.
func NegotiateHostCapabilities(host HostCapabilities) CapabilityReport {
	values := map[string]bool{
		"a2a": host.A2A, "context_loading": host.ContextLoading, "eval": host.Eval,
		"sandbox": host.Sandbox, "telemetry": host.Telemetry,
	}
	results := make([]CapabilityResult, 0, len(values))
	for _, name := range []string{"a2a", "context_loading", "eval", "sandbox", "telemetry"} {
		result := CapabilityResult{Capability: name, Status: CapabilitySupported, Reason: "host declared support"}
		if !values[name] {
			result.Status = CapabilityDowngraded
			result.Reason = "host did not declare capability"
			result.Recovery = "use the deterministic local fallback"
			if name == "sandbox" {
				result.Status = CapabilityRefused
				result.Reason = "sandbox is required before mutable execution"
				result.Recovery = "declare sandbox support or use read-only operations"
			}
		}
		results = append(results, result)
	}
	return CapabilityReport{Results: results}
}

func CapabilityNames() []string {
	names := []string{"a2a", "context_loading", "eval", "sandbox", "telemetry"}
	sort.Strings(names)
	return names
}
