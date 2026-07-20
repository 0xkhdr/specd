package core

// AssuranceLevel names how much a machine-readable response is actually worth.
// A governed-looking session and a governed session read identically to an
// agent unless the difference is stated, so every machine surface carries this
// (spec agent-protocol-clarity R3.1).
//
// The order is a lattice, not a label set: advisory < gated < sandboxed.
type AssuranceLevel string

const (
	// AssuranceAdvisory: findings are reported, nothing is contained. The fail-safe.
	AssuranceAdvisory AssuranceLevel = "advisory"
	// AssuranceGated: harness gates block the transition, but execution is not isolated.
	AssuranceGated AssuranceLevel = "gated"
	// AssuranceSandboxed: gated, and the host isolates execution.
	AssuranceSandboxed AssuranceLevel = "sandboxed"
)

// assuranceRank orders the lattice. Absent from this map means unrecognized,
// which is deliberately indistinguishable from advisory (rank 0).
var assuranceRank = map[AssuranceLevel]int{
	AssuranceAdvisory:  0,
	AssuranceGated:     1,
	AssuranceSandboxed: 2,
}

// ParseAssuranceLevel reads a stored or declared level. An unknown value fails
// safe to advisory: a level nobody recognizes is a level nobody is enforcing,
// and guessing upward would let a typo advertise containment that does not
// exist (R3.2).
func ParseAssuranceLevel(value string) AssuranceLevel {
	level := AssuranceLevel(value)
	if _, ok := assuranceRank[level]; !ok {
		return AssuranceAdvisory
	}
	return level
}

// AssuranceCeiling is the highest level the host's declared capabilities can
// honestly support. A host that advertises no sandbox cannot present a session
// as fully governed no matter what the session claims (R3.2).
func AssuranceCeiling(host HostCapabilities) AssuranceLevel {
	if !host.Sandbox {
		return AssuranceAdvisory
	}
	return AssuranceSandboxed
}

// AssuranceFor resolves the level to report: the declared level capped by what
// the host can back. It only ever lowers — no host capability, stored value, or
// prose can raise a session above its ceiling.
func AssuranceFor(host HostCapabilities, declared string) AssuranceLevel {
	level := ParseAssuranceLevel(declared)
	if ceiling := AssuranceCeiling(host); assuranceRank[ceiling] < assuranceRank[level] {
		return ceiling
	}
	return level
}
