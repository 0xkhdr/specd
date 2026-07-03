package testharness_test

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// buildSpec authors a minimal gate-clean spec and returns the harness.
func buildSpec(t *testing.T) *th.Harness {
	t.Helper()
	h := th.New(t)
	h.Spec("demo").
		Req("Core", "story", "THE SYSTEM SHALL do the thing.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Status: core.TaskComplete}).
		Status(core.StatusVerifying).
		Gate(core.GateNone).
		Turn(3).
		Build()
	return h
}

func TestStateAsserterChainsFields(t *testing.T) {
	h := buildSpec(t)
	h.State("demo").
		Status(core.StatusVerifying).
		Phase(core.PhaseForStatus(core.StatusVerifying)).
		Gate(core.GateNone).
		Turn(3).
		TaskStatus("T1", core.TaskComplete).
		NoBlockers()
}

func TestStateAsserterRawExposesState(t *testing.T) {
	h := buildSpec(t)
	if raw := h.State("demo").Raw(); raw == nil || raw.Spec != "demo" {
		t.Errorf("Raw() = %+v, want state for slug demo", raw)
	}
}

func TestStateAsserterTaskEvidence(t *testing.T) {
	h := buildSpec(t)
	// A seeded-complete task records verification evidence.
	h.State("demo").TaskEvidence("T1", "")
}

func TestStateAsserterBlockersAndAcceptance(t *testing.T) {
	h := buildSpec(t)

	// Arrange: seed a blocker and an acceptance record into the persisted state.
	st, err := core.LoadState(h.Root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	st.Blockers = append(st.Blockers, core.Blocker{Task: "T1", Reason: "stuck", Since: "2026-01-02T03:04:05Z"})
	if st.Acceptance == nil {
		st.Acceptance = map[string]core.CriterionRecord{}
	}
	st.Acceptance["1.1"] = core.CriterionRecord{Requirement: 1, Criterion: 1, Status: "pass"}
	if err := core.SaveState(h.Root, "demo", st); err != nil {
		t.Fatal(err)
	}

	// Act/Assert
	h.State("demo").
		HasBlocker("T1").
		AcceptanceStatus("1.1", "pass")
}

func TestStateAsserterPhaseOverride(t *testing.T) {
	h := th.New(t)
	h.Spec("ph").
		Req("Core", "story", "THE SYSTEM SHALL do the thing.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true"}).
		Status(core.StatusTasks).
		Phase(core.PhasePerceive).
		Build()
	h.State("ph").Phase(core.PhasePerceive)
}
