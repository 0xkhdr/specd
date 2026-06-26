package spec

import (
	"strings"
	"testing"
)

func TestPhaseForStatus(t *testing.T) {
	cases := map[SpecStatus]Phase{
		StatusRequirements: PhaseAnalyze,
		StatusDesign:       PhasePlan,
		StatusTasks:        PhasePlan,
		StatusExecuting:    PhaseExecute,
		StatusBlocked:      PhaseExecute,
		StatusVerifying:    PhaseVerify,
		StatusComplete:     PhaseReflect,
	}
	for status, want := range cases {
		if got := PhaseForStatus(status); got != want {
			t.Errorf("PhaseForStatus(%q) = %q, want %q", status, got, want)
		}
	}
	// Unknown status falls back to the analyze phase.
	if got := PhaseForStatus(SpecStatus("bogus")); got != PhaseAnalyze {
		t.Errorf("PhaseForStatus(bogus) = %q, want %q", got, PhaseAnalyze)
	}
}

func TestIsReadonlyRole(t *testing.T) {
	for _, r := range []string{"scout", "researcher", "reviewer", "architect", "investigator"} {
		if !IsReadonlyRole(r) {
			t.Errorf("IsReadonlyRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"builder", "tester", "documenter", "verifier", ""} {
		if IsReadonlyRole(r) {
			t.Errorf("IsReadonlyRole(%q) = true, want false", r)
		}
	}
}

func TestStatusAndPhaseConstants(t *testing.T) {
	// Lock the wire values: state.json and manifests are byte-stable on these.
	if StatusRequirements != "requirements" || StatusExecuting != "executing" || StatusComplete != "complete" {
		t.Fatal("SpecStatus const values drifted")
	}
	if PhaseAnalyze != "analyze" || PhaseExecute != "execute" || PhaseReflect != "reflect" {
		t.Fatal("Phase const values drifted")
	}
}

func TestRoleRegistryContracts(t *testing.T) {
	want := []string{"scout", "researcher", "reviewer", "architect", "builder", "tester", "documenter", "verifier", "investigator"}
	if got := RoleNames(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("RoleNames() = %v, want %v", got, want)
	}
	for _, name := range want {
		def, ok := RoleByName(name)
		if !ok {
			t.Fatalf("missing role %q", name)
		}
		if def.Name != name || def.RW == "" || def.BudgetTier == "" || len(def.PhaseAffinity) == 0 || len(def.Tools) == 0 || def.FilePolicy == "" || def.PromptClass == "" {
			t.Fatalf("incomplete role contract for %q: %+v", name, def)
		}
		if len(RoleTools(name)) != len(def.Tools) {
			t.Fatalf("RoleTools(%q) length mismatch: %v vs %v", name, RoleTools(name), def.Tools)
		}
	}
	if !RoleAllowsTool("verifier", "specd_state_read") {
		t.Fatal("verifier should allow specd_state_read")
	}
}
