package orchestration

type Authority struct {
	Enabled bool
}

type GateSeverity string

const (
	GateLow      GateSeverity = "low"
	GateMedium   GateSeverity = "medium"
	GateHigh     GateSeverity = "high"
	GateCritical GateSeverity = "critical"
)

func (authority Authority) CanDispatch() bool {
	return authority.Enabled
}

func (authority Authority) CanClearGate(severity GateSeverity) bool {
	if !authority.Enabled {
		return false
	}
	return severity != GateHigh && severity != GateCritical
}

func DecisionLimitsForAuthority(authority Authority, limits DecisionLimits) DecisionLimits {
	limits.AllowDispatch = authority.CanDispatch()
	return limits
}
