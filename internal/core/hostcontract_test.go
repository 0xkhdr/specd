package core

import (
	"strings"
	"testing"
)

// R5.1 to R5.3: the reference adapter proves the contract is satisfiable. If a
// new required control is added without the reference host asserting it, this
// fails — which is the point.
func TestHostContractReferenceAdapterIsGoverned(t *testing.T) {
	conformance := EvaluateHostContract(ReferenceHostContract())
	if len(conformance.Unmet) != 0 {
		t.Fatalf("reference adapter does not meet the contract it defines: %v", conformance.Unmet)
	}
	if conformance.Assurance != AssuranceSandboxed {
		t.Fatalf("assurance = %q, want sandboxed", conformance.Assurance)
	}
	if !conformance.Governed {
		t.Fatal("fully conformant host is not reported as governed")
	}
}

// R5.4: a host declaring no sandbox support yields an advisory session, no
// matter how many other controls it asserts.
func TestHostContractNoSandboxIsAdvisory(t *testing.T) {
	contract := ReferenceHostContract()
	contract.Sandbox = false

	conformance := EvaluateHostContract(contract)
	if len(conformance.Unmet) != 0 {
		t.Fatalf("sandbox is a ceiling, not a required control: %v", conformance.Unmet)
	}
	if conformance.Assurance != AssuranceAdvisory {
		t.Fatalf("assurance = %q, want advisory without a sandbox", conformance.Assurance)
	}
	if conformance.Governed {
		t.Fatal("a host with no sandbox was presented as fully governed")
	}
}

// R5.4: each missing control drops the session to advisory and says which
// clause went unmet, so the label is actionable rather than only honest.
func TestHostContractEachMissingControlDowngrades(t *testing.T) {
	cases := []struct {
		name string
		drop func(*HostContract)
		ref  string
	}{
		{"mutable_tools_before_bootstrap", func(c *HostContract) { c.GatesMutableToolsUntilBootstrap = false }, "R5.1"},
		{"human_only_exposed", func(c *HostContract) { c.HidesHumanOnlyOperations = false }, "R5.2"},
		{"harness_paths_writable", func(c *HostContract) { c.DeniesHarnessOwnedPaths = false }, "R5.2"},
		{"expiry_unchecked", func(c *HostContract) { c.ChecksAuthorityExpiryAtInvocation = false }, "R5.3"},
		{"path_permission_not_derived", func(c *HostContract) { c.DerivesPathPermission = false }, "R5.3"},
		{"process_permission_not_derived", func(c *HostContract) { c.DerivesProcessPermission = false }, "R5.3"},
		{"network_permission_not_derived", func(c *HostContract) { c.DerivesNetworkPermission = false }, "R5.3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			contract := ReferenceHostContract()
			tc.drop(&contract)

			conformance := EvaluateHostContract(contract)
			if conformance.Assurance != AssuranceAdvisory {
				t.Fatalf("assurance = %q with a control missing, want advisory", conformance.Assurance)
			}
			if conformance.Governed {
				t.Fatal("host missing a control was presented as governed")
			}
			if len(conformance.Unmet) != 1 {
				t.Fatalf("unmet = %v, want exactly the dropped control", conformance.Unmet)
			}
			if !strings.HasPrefix(conformance.Unmet[0], tc.ref+":") {
				t.Errorf("unmet %q does not cite %s", conformance.Unmet[0], tc.ref)
			}
		})
	}
}

// A host that asserts nothing is the fail-safe: advisory, with every clause
// reported unmet.
func TestHostContractEmptyDeclarationIsAdvisory(t *testing.T) {
	conformance := EvaluateHostContract(HostContract{})
	if conformance.Assurance != AssuranceAdvisory {
		t.Fatalf("assurance = %q for a host asserting nothing, want advisory", conformance.Assurance)
	}
	if len(conformance.Unmet) != len(requiredControls) {
		t.Fatalf("unmet = %v, want every control", conformance.Unmet)
	}
	// Deterministic order, so two runs of a conformance report read alike.
	for i := 1; i < len(conformance.Unmet); i++ {
		if conformance.Unmet[i-1] > conformance.Unmet[i] {
			t.Fatalf("unmet controls are not in stable order: %v", conformance.Unmet)
		}
	}
}

// R5.2: the human-only set a host must hide comes from the palette, so a verb
// marked human-only later is covered without editing the contract.
func TestHostContractHumanOnlySetTracksPalette(t *testing.T) {
	hidden := HumanOnlyOperations()
	if len(hidden) == 0 {
		t.Fatal("no human-only operations reported; a host would hide nothing")
	}
	for _, name := range hidden {
		command, ok := CommandByName(name)
		if !ok {
			t.Fatalf("%q is not a palette command", name)
		}
		if !command.HumanOnly {
			t.Fatalf("%q is exposed as human-only but the palette disagrees", name)
		}
	}
	// approve is the operation an agent must never reach; if it ever leaves
	// this set the contract has stopped protecting the thing it exists for.
	found := false
	for _, name := range hidden {
		if name == "approve" {
			found = true
		}
	}
	if !found {
		t.Fatal("approve is absent from the human-only set")
	}
}

// R5.4 end to end: the level the contract resolves to is the level the MCP
// transport already reports, so the two surfaces cannot drift apart.
func TestHostContractAgreesWithTransportAssurance(t *testing.T) {
	for _, sandbox := range []bool{true, false} {
		contract := ReferenceHostContract()
		contract.Sandbox = sandbox

		fromContract := EvaluateHostContract(contract).Assurance
		fromTransport := AssuranceCeiling(HostCapabilities{Sandbox: sandbox})
		if fromContract != fromTransport {
			t.Fatalf("sandbox=%v: contract says %q, transport says %q", sandbox, fromContract, fromTransport)
		}
	}
}
