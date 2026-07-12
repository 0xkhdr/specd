package orchestration

func EvaluateBrakes(snapshot Snapshot, limits DecisionLimits) Decision {
	if limits.RequireTelemetry && !snapshot.TelemetryKnown {
		return Decision{Action: ActionHalt, Reason: "required telemetry unknown"}
	}
	if limits.MaxCostMicros > 0 && snapshot.TelemetryKnown && snapshot.CostMicros > limits.MaxCostMicros {
		return Decision{Action: ActionHalt, Reason: "cost limit exceeded"}
	}
	if limits.MaxTokens > 0 && snapshot.TelemetryKnown && snapshot.Tokens > limits.MaxTokens {
		return Decision{Action: ActionHalt, Reason: "token limit exceeded"}
	}
	if limits.MaxCost > 0 && snapshot.Cost > limits.MaxCost {
		return Decision{Action: ActionHalt, Reason: "cost limit exceeded"}
	}
	if !limits.Deadline.IsZero() && !snapshot.Now.Before(limits.Deadline) {
		return Decision{Action: ActionTimeout, Reason: "deadline reached"}
	}
	return Decision{}
}
