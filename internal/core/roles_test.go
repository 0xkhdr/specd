package core

import "testing"

func TestRoleCapabilityContract(t *testing.T) {
	for _, role := range KnownRoles() {
		capability, ok := RoleCapabilityFor(role)
		if !ok {
			t.Fatalf("role %s has no capability contract", role)
		}
		if len(capability.Effects) == 0 {
			t.Fatalf("role %s declares no effects", role)
		}
		if !RoleHasEffect(role, RoleEffectWorkspaceRead) {
			t.Fatalf("role %s cannot read the workspace", role)
		}
		if capability.HumanAuthority {
			t.Fatalf("role %s claims human authority", role)
		}
		if capability.NetworkPolicy != "deny" {
			t.Fatalf("role %s network policy=%q", role, capability.NetworkPolicy)
		}
	}

	// Only the craftsman writes, and only the craftsman completes.
	for _, role := range KnownRoles() {
		want := role == "craftsman"
		if got := RoleHasEffect(role, RoleEffectWorkspaceWrite); got != want {
			t.Fatalf("role %s workspace-write=%t", role, got)
		}
		if got := IsWriteRole(role); got != want {
			t.Fatalf("IsWriteRole(%s)=%t", role, got)
		}
		capability, _ := RoleCapabilityFor(role)
		if capability.CompletionAuthority != want {
			t.Fatalf("role %s completion authority=%t", role, capability.CompletionAuthority)
		}
	}

	// The validator writes evidence, so it is not read-only (R2.2).
	if !RoleHasEffect("validator", RoleEffectHarnessEvidenceWrite) {
		t.Fatal("validator does not declare harness-evidence-write")
	}
	if RoleHasEffect("scout", RoleEffectHarnessEvidenceWrite) || RoleHasEffect("auditor", RoleEffectHarnessEvidenceWrite) {
		t.Fatal("a read-only role declares harness-evidence-write")
	}

	// No role may reach outside the workspace.
	for _, role := range KnownRoles() {
		if RoleHasEffect(role, RoleEffectExternalWrite) {
			t.Fatalf("role %s declares external-write", role)
		}
	}
}

func TestRoleCapabilityUnknownDefaultsToEmptySet(t *testing.T) {
	capability, ok := RoleCapabilityFor("unknown")
	if ok {
		t.Fatal("unknown role reported a contract")
	}
	if len(capability.Effects) != 0 {
		t.Fatalf("unknown role effects=%v", capability.Effects)
	}
	if len(capability.AllowedOperations) != 0 {
		t.Fatalf("unknown role operations=%v", capability.AllowedOperations)
	}
	if capability.CompletionAuthority || capability.HumanAuthority {
		t.Fatalf("unknown role carries authority: %#v", capability)
	}
	if IsWriteRole("unknown") {
		t.Fatal("unknown role defaulted to workspace-write")
	}
	if RoleAllowsOperation("unknown", "verify") {
		t.Fatal("unknown role allows verify")
	}
	// Default deny reaches known roles too: no contract inherits an operation.
	if RoleAllowsOperation("scout", "complete-task") {
		t.Fatal("scout allows complete-task")
	}
	if !RoleAllowsOperation("craftsman", "complete-task") {
		t.Fatal("craftsman denied complete-task")
	}
}

func TestRolePromptDedup(t *testing.T) {
	prompt := RolePrompt("validator")
	got := DedupRolePrompts([]string{prompt, prompt})
	if len(got) != 1 || got[0] != prompt {
		t.Fatalf("DedupRolePrompts = %#v", got)
	}
}

func TestRolePromptUnknownNeverCraftsman(t *testing.T) {
	if got := RolePrompt("unknown"); got != "role:invalid\n" {
		t.Fatalf("unknown role prompt=%q", got)
	}
	if got := KnownRoles(); len(got) != 4 {
		t.Fatalf("known roles=%v", got)
	}
}
