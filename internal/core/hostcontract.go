package core

import "sort"

// HostContract is what a host asserts about the controls it actually enforces
// (R5.1 to R5.3).
//
// Every field is a control specd cannot implement itself. specd is a CLI: it
// owns the declaration, the gates, and the evidence, but it does not sit in
// front of the agent's file writes, subprocesses, or network calls. The host
// does. So this type is not a feature list — it is the boundary where specd
// stops being able to enforce anything and has to start trusting, and the whole
// point of recording it is that the trust becomes visible in the assurance
// level rather than assumed silently.
//
// A field left false is not a failure. It is an honest host saying "I do not do
// this", which lowers assurance rather than refusing the session (R5.4).
type HostContract struct {
	// R5.1: mutable tools are unavailable to the agent until bootstrap
	// completes. A host that exposes them earlier lets an agent act before it
	// has an authority packet to act under.
	GatesMutableToolsUntilBootstrap bool `json:"gates_mutable_tools_until_bootstrap"`

	// R5.2: human-only operations are absent from the agent's tool surface, and
	// harness-owned files are denied to ordinary editors. Absent, not merely
	// refused on call: a tool an agent can see is a tool it will try.
	HidesHumanOnlyOperations bool `json:"hides_human_only_operations"`
	DeniesHarnessOwnedPaths  bool `json:"denies_harness_owned_paths"`

	// R5.3: authority is checked at invocation time, not at issue time, and the
	// permission decision is derived from the packet rather than from host
	// configuration. An expiry checked only when the packet is minted is not an
	// expiry.
	ChecksAuthorityExpiryAtInvocation bool `json:"checks_authority_expiry_at_invocation"`
	DerivesPathPermission             bool `json:"derives_path_permission"`
	DerivesProcessPermission          bool `json:"derives_process_permission"`
	DerivesNetworkPermission          bool `json:"derives_network_permission"`

	// Sandbox reports whether the host isolates execution. It is the ceiling on
	// assurance: without it, no set of the controls above can make a session
	// fully governed, because nothing contains a process that ignores them.
	Sandbox bool `json:"sandbox"`
}

// HostConformance is the result of evaluating a declaration: the level the
// session may honestly be presented at, and every control the host did not
// assert.
type HostConformance struct {
	Assurance AssuranceLevel `json:"assurance"`

	// Unmet names each missing control in a stable order. It is the reason the
	// assurance is what it is, so an operator can see what to fix rather than
	// only that something is wrong.
	Unmet []string `json:"unmet"`

	// Governed is true only at the top of the lattice. A host reading this
	// field alone still cannot overstate its own containment.
	Governed bool `json:"governed"`
}

// requiredControls maps each R5 control to the requirement it satisfies, so an
// unmet entry cites the clause rather than restating the field name.
var requiredControls = []struct {
	name string
	ref  string
	get  func(HostContract) bool
}{
	{"gates_mutable_tools_until_bootstrap", "R5.1", func(c HostContract) bool { return c.GatesMutableToolsUntilBootstrap }},
	{"hides_human_only_operations", "R5.2", func(c HostContract) bool { return c.HidesHumanOnlyOperations }},
	{"denies_harness_owned_paths", "R5.2", func(c HostContract) bool { return c.DeniesHarnessOwnedPaths }},
	{"checks_authority_expiry_at_invocation", "R5.3", func(c HostContract) bool { return c.ChecksAuthorityExpiryAtInvocation }},
	{"derives_path_permission", "R5.3", func(c HostContract) bool { return c.DerivesPathPermission }},
	{"derives_process_permission", "R5.3", func(c HostContract) bool { return c.DerivesProcessPermission }},
	{"derives_network_permission", "R5.3", func(c HostContract) bool { return c.DerivesNetworkPermission }},
}

// EvaluateHostContract resolves the assurance level a host may be presented at
// (R5.4).
//
// It only ever lowers. The declared level is the best the asserted controls
// could support, and AssuranceFor caps that by what the host's sandbox
// capability can actually back — so neither an over-confident declaration nor a
// future edit to this function can raise a session above its ceiling.
func EvaluateHostContract(contract HostContract) HostConformance {
	conformance := HostConformance{Unmet: []string{}}
	for _, control := range requiredControls {
		if !control.get(contract) {
			conformance.Unmet = append(conformance.Unmet, control.ref+":"+control.name)
		}
	}
	sort.Strings(conformance.Unmet)

	// A host missing any control enforces nothing specd can rely on, so it
	// declares no better than advisory regardless of its sandbox.
	declared := string(AssuranceSandboxed)
	if len(conformance.Unmet) > 0 {
		declared = string(AssuranceAdvisory)
	}
	conformance.Assurance = AssuranceFor(HostCapabilities{Sandbox: contract.Sandbox}, declared)
	conformance.Governed = conformance.Assurance == AssuranceSandboxed
	return conformance
}

// ReferenceHostContract is the one adapter the spec ships: a declaration that
// satisfies every clause of R5.
//
// Its job is to prove the contract is satisfiable rather than aspirational. A
// contract no host can meet is a contract that will be ignored, so this is the
// existence proof — and the conformance test asserts it evaluates as governed,
// which fails the moment a new required control is added without a
// corresponding host capability.
func ReferenceHostContract() HostContract {
	return HostContract{
		GatesMutableToolsUntilBootstrap:   true,
		HidesHumanOnlyOperations:          true,
		DeniesHarnessOwnedPaths:           true,
		ChecksAuthorityExpiryAtInvocation: true,
		DerivesPathPermission:             true,
		DerivesProcessPermission:          true,
		DerivesNetworkPermission:          true,
		Sandbox:                           true,
	}
}

// HumanOnlyOperations is the set a conformant host must not expose to an agent
// (R5.2). Derived from the palette rather than restated, so a verb marked
// human-only later is covered without editing this file.
func HumanOnlyOperations() []string {
	var names []string
	for _, command := range Commands {
		if command.HumanOnly {
			names = append(names, command.Name)
		}
	}
	sort.Strings(names)
	return names
}
