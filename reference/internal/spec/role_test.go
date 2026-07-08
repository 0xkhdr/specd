package spec

import "testing"

// TestRoleAccessors exercises the per-role accessor surface so the role
// registry's fail-loud lookups (and their unknown-role paths) stay covered.
func TestRoleAccessors(t *testing.T) {
	// Known role: every accessor returns the registry value.
	if got := RoleBudgetTier("craftsman"); got != "focused" {
		t.Errorf("RoleBudgetTier(craftsman) = %q, want focused", got)
	}
	if got := RolePromptClass("craftsman"); got != "card" {
		t.Errorf("RolePromptClass(craftsman) = %q, want card", got)
	}
	if got := RoleFilePolicy("scout"); got != "no writes" {
		t.Errorf("RoleFilePolicy(scout) = %q, want \"no writes\"", got)
	}
	if aff := RolePhaseAffinity("architect"); len(aff) != 2 {
		t.Errorf("RolePhaseAffinity(architect) = %v, want 2 phases", aff)
	}

	// Unknown role: every accessor returns its zero value rather than panicking.
	if got := RoleBudgetTier("nope"); got != "" {
		t.Errorf("RoleBudgetTier(unknown) = %q, want empty", got)
	}
	if got := RolePromptClass("nope"); got != "" {
		t.Errorf("RolePromptClass(unknown) = %q, want empty", got)
	}
	if got := RoleFilePolicy("nope"); got != "" {
		t.Errorf("RoleFilePolicy(unknown) = %q, want empty", got)
	}
	if aff := RolePhaseAffinity("nope"); aff != nil {
		t.Errorf("RolePhaseAffinity(unknown) = %v, want nil", aff)
	}
}

// TestRolePhaseAffinityIsCopy guards that the returned slice cannot mutate the
// registry — a caller appending to it must not corrupt the shared RoleDef.
func TestRolePhaseAffinityIsCopy(t *testing.T) {
	got := RolePhaseAffinity("architect")
	got = append(got, PhaseReflect) // mutate the copy
	_ = got
	if again := RolePhaseAffinity("architect"); len(again) != 2 {
		t.Fatalf("RolePhaseAffinity mutated registry: got %v", again)
	}
}

func TestRoleAllowsPhase(t *testing.T) {
	if !RoleAllowsPhase("architect", PhasePlan) {
		t.Error("architect should allow plan phase")
	}
	if RoleAllowsPhase("architect", PhaseExecute) {
		t.Error("architect should NOT allow execute phase")
	}
	if RoleAllowsPhase("nope", PhasePlan) {
		t.Error("unknown role should allow no phase")
	}
}

func TestRoleToolSetAndAllows(t *testing.T) {
	set := RoleToolSet("scout")
	if set == nil || !set["specd_read"] {
		t.Fatalf("RoleToolSet(scout) missing specd_read: %v", set)
	}
	if set["specd_dispatch"] {
		t.Error("scout (readonly) should not allow specd_dispatch")
	}
	// Unknown role has no tools → nil set, and allows nothing.
	if RoleToolSet("nope") != nil {
		t.Error("RoleToolSet(unknown) should be nil")
	}
	if RoleAllowsTool("nope", "specd_read") {
		t.Error("unknown role should allow no tool")
	}
	if RoleAllowsTool("scout", "specd_dispatch") {
		t.Error("scout should not allow specd_dispatch")
	}
}

func TestRoleToolsIsCopy(t *testing.T) {
	tools := RoleTools("craftsman")
	if len(tools) == 0 {
		t.Fatal("RoleTools(craftsman) empty")
	}
	tools[0] = "MUTATED"
	if again := RoleTools("craftsman"); again[0] == "MUTATED" {
		t.Fatal("RoleTools returned a reference into the registry, not a copy")
	}
	if RoleTools("nope") != nil {
		t.Error("RoleTools(unknown) should be nil")
	}
}

func TestReadonlyRoleNames(t *testing.T) {
	got := ReadonlyRoleNames()
	want := map[string]bool{"scout": true, "researcher": true, "auditor": true, "architect": true}
	if len(got) != len(want) {
		t.Fatalf("ReadonlyRoleNames() = %v, want %d names", got, len(want))
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected readonly role %q", n)
		}
	}
}
