package core

// CostBrakeLevel is the deterministic enforcement state for host-reported
// session spend. Host costs are untrusted telemetry, but an operator-supplied
// limit still acts as a dispatch brake: warn at 80%, halt at 100%.
type CostBrakeLevel string

// The CostBrakeLevel values are the three enforcement states EvaluateCostBrake
// can return, in increasing severity.
const (
	CostBrakeNone CostBrakeLevel = "none"
	CostBrakeWarn CostBrakeLevel = "warn"
	CostBrakeHalt CostBrakeLevel = "halt"
)

// EvaluateCostBrake compares accumulated host-reported cost to the configured
// limit. A limit <= 0 disables the brake. Callers validate finite/non-negative
// policy values before invoking this helper.
func EvaluateCostBrake(accumulatedUSD, limitUSD float64) CostBrakeLevel {
	if limitUSD <= 0 {
		return CostBrakeNone
	}
	if accumulatedUSD >= limitUSD {
		return CostBrakeHalt
	}
	if accumulatedUSD >= limitUSD*0.8 {
		return CostBrakeWarn
	}
	return CostBrakeNone
}
