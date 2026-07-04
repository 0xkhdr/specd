package orchestration

func EvaluateBrakes(snapshot Snapshot, limits DecisionLimits) Decision {
	if limits.MaxCost > 0 && snapshot.Cost > limits.MaxCost {
		return Decision{Action: ActionHalt, Reason: "cost limit exceeded"}
	}
	if !limits.Deadline.IsZero() && !snapshot.Now.Before(limits.Deadline) {
		return Decision{Action: ActionTimeout, Reason: "deadline reached"}
	}
	return Decision{}
}
