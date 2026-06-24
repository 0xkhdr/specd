package spec

import "testing"

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
	for _, r := range ReadonlyRoles {
		if !IsReadonlyRole(r) {
			t.Errorf("IsReadonlyRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"builder", "verifier", ""} {
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
