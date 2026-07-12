package orchestration

// EvaluateBrakes is the honest cost brake. It halts only on measured, reported
// telemetry: an absent measurement is unknown (never treated as zero), so an
// unset limit or unknown cost never fabricates a halt. A brake fired on
// untrusted (worker-reported) data is labelled as such (spec 07 R4.1-R4.3).
func EvaluateBrakes(snapshot Snapshot, limits DecisionLimits) Decision {
	if limits.RequireTelemetry {
		if !snapshot.TelemetryKnown {
			return Decision{Action: ActionHalt, Reason: "required telemetry unknown"}
		}
		if !snapshot.TelemetryTrusted {
			return Decision{Action: ActionHalt, Reason: "required telemetry untrusted: only worker-reported accounting hints present; supply a trusted (host/adapter/attested) source"}
		}
	}
	if limits.MaxCostMicros > 0 && snapshot.TelemetryKnown && snapshot.CostMicros > limits.MaxCostMicros {
		return Decision{Action: ActionHalt, Reason: brakeReason("cost limit exceeded", snapshot.TelemetryTrusted)}
	}
	if limits.MaxTokens > 0 && snapshot.TelemetryKnown && snapshot.Tokens > limits.MaxTokens {
		return Decision{Action: ActionHalt, Reason: brakeReason("token limit exceeded", snapshot.TelemetryTrusted)}
	}
	if !limits.Deadline.IsZero() && !snapshot.Now.Before(limits.Deadline) {
		return Decision{Action: ActionTimeout, Reason: "deadline reached"}
	}
	return Decision{}
}

func brakeReason(base string, trusted bool) string {
	if trusted {
		return base
	}
	return base + " (untrusted telemetry)"
}
