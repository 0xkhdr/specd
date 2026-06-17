package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// T4 / R2 + R3 regression closure. Asserts the core primitives that back the
// phase/gate state machine and evidence-gated task flips. Full CLI-side gate
// blocking (`specd approve`/`specd task` refusing while awaiting-approval) and
// the `--unverified` bypass live in internal/cmd and are owned by the
// regression-cli-cmd spec; here we lock the engine behaviors those depend on.

// R2.1: PhaseReadiness is the planning gate — advancement is blocked (non-empty
// problems) until the phase artifact passes, and permitted (empty) once it does.
func TestPhaseReadinessBlocksUntilClean(t *testing.T) {
	validReq := "## Requirement 1\n**User story:** As a user, I want X.\n\n**Acceptance criteria:**\n1. WHEN a thing happens THE SYSTEM SHALL do Y\n"
	empty := ""

	// requirements: missing/empty blocks; EARS-valid clears.
	if p := PhaseReadiness(StatusRequirements, &empty, nil, ParsedTasks{}); len(p) == 0 {
		t.Error("R2.1: empty requirements must block advancement")
	}
	if p := PhaseReadiness(StatusRequirements, &validReq, nil, ParsedTasks{}); len(p) != 0 {
		t.Errorf("R2.1: valid requirements must clear, got %v", p)
	}

	// design: missing design.md blocks.
	if p := PhaseReadiness(StatusDesign, &validReq, nil, ParsedTasks{}); len(p) == 0 {
		t.Error("R2.1: missing design must block advancement")
	}

	// tasks: a cyclic task graph blocks.
	cyclic := ParsedTasks{Tasks: []ParsedTask{
		{ID: "T1", Wave: 1, Meta: map[string]string{"depends": "T2"}},
		{ID: "T2", Wave: 1, Meta: map[string]string{"depends": "T1"}},
	}}
	p := PhaseReadiness(StatusTasks, &validReq, &validReq, cyclic)
	if len(p) == 0 {
		t.Error("R2.1: cyclic tasks must block advancement")
	}
}

// R2.1 (data invariant): the planning ratchet is forward-only and offers no
// transition out of post-approval statuses — the engine cannot skip a gate.
func TestPhaseAdvanceIsForwardOnlyRatchet(t *testing.T) {
	if PlanningAdvance[StatusRequirements].Status != StatusDesign {
		t.Error("requirements must advance to design")
	}
	if PlanningAdvance[StatusDesign].Status != StatusTasks {
		t.Error("design must advance to tasks")
	}
	if PlanningAdvance[StatusTasks].Status != StatusExecuting {
		t.Error("tasks must advance to executing")
	}
	for _, terminal := range []SpecStatus{StatusExecuting, StatusVerifying, StatusComplete} {
		if _, ok := PlanningAdvance[terminal]; ok {
			t.Errorf("status %s must have no planning advance (gate must intervene)", terminal)
		}
	}
}

// R2.2: a builder task flipped complete without evidence is rejected by the
// evidence gate; with evidence but no verified record it is still rejected.
func TestGateEvidenceRejectsUnproven(t *testing.T) {
	ev := "proof"
	noEvidence := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Role: "builder", Status: TaskComplete},
	}}
	if v, _ := GateEvidence(CheckCtx{State: noEvidence}); len(v) != 1 || v[0].Gate != "evidence" {
		t.Fatalf("R2.2: complete-without-evidence must be 1 evidence violation, got %v", v)
	}
	noVerified := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Role: "builder", Status: TaskComplete, Evidence: &ev},
	}}
	if v, _ := GateEvidence(CheckCtx{State: noVerified, Slug: "demo"}); len(v) != 1 {
		t.Fatalf("R2.2: evidence-but-unverified builder must be rejected, got %v", v)
	}
}

// R2.3: SaveState persists revision strictly monotonically across writes and
// never decreases or repeats.
func TestPhaseStateRevisionMonotonic(t *testing.T) {
	dir := t.TempDir()
	slug := "rev-spec"
	if err := os.MkdirAll(filepath.Join(dir, ".specd", "specs", slug), 0o755); err != nil {
		t.Fatal(err)
	}
	st := InitialState(slug, "Rev Spec")
	prev := st.Revision
	for i := 0; i < 5; i++ {
		if err := SaveState(dir, slug, &st); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		if st.Revision != prev+1 {
			t.Fatalf("revision not monotonic: got %d, want %d", st.Revision, prev+1)
		}
		prev = st.Revision
	}
}

// R2.4: custom gates run after the ordered core pipeline and in configured
// order — the gate listing order is a stable contract.
func TestCustomGatePipelineOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("custom gate uses POSIX sh")
	}
	baseV, _ := RunGates(minimalCtx(nil))
	two := []CustomGateCfg{
		{Name: "first", Command: `echo '{"violations":[{"location":"a","message":"1"}]}'`},
		{Name: "second", Command: `echo '{"violations":[{"location":"b","message":"2"}]}'`},
	}
	v, _ := RunGates(minimalCtx(two))
	if len(v) != len(baseV)+2 {
		t.Fatalf("R2.4: want %d violations, got %d", len(baseV)+2, len(v))
	}
	// Core findings precede custom; customs appear in config order.
	if v[len(v)-2].Gate != "custom:first" || v[len(v)-1].Gate != "custom:second" {
		t.Fatalf("R2.4: custom gate order wrong: %s then %s", v[len(v)-2].Gate, v[len(v)-1].Gate)
	}
}

// R3.1: a completed task's evidence and finish timestamp survive a
// save/load round-trip (the durable proof of a flip).
func TestTaskFlipPersistsEvidenceAndTimestamp(t *testing.T) {
	dir := t.TempDir()
	slug := "ev-spec"
	if err := os.MkdirAll(filepath.Join(dir, ".specd", "specs", slug), 0o755); err != nil {
		t.Fatal(err)
	}
	st := InitialState(slug, "Ev Spec")
	ev := "go test ./... → ok"
	ts := NowISO()
	st.Tasks = map[string]TaskState{
		"T1": {ID: "T1", Role: "builder", Status: TaskComplete, Evidence: &ev, FinishedAt: &ts},
	}
	if err := SaveState(dir, slug, &st); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(dir, slug)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Tasks["T1"]
	if got.Evidence == nil || *got.Evidence != ev {
		t.Errorf("R3.1: evidence not persisted: %v", got.Evidence)
	}
	if got.FinishedAt == nil || *got.FinishedAt != ts {
		t.Errorf("R3.1: finish timestamp not persisted: %v", got.FinishedAt)
	}
}

// R3.3: telemetry annotations (tokens, cost) are stored verbatim and only
// summed — never priced or computed. The roll-up of stored per-task values
// must equal the literal inputs.
func TestTaskTelemetryStoredNotComputed(t *testing.T) {
	st := &State{Spec: "tel", Tasks: map[string]TaskState{
		"T1": {ID: "T1", Wave: 1, Telemetry: &Telemetry{Tokens: 1000, Cost: "0.42"}},
		"T2": {ID: "T2", Wave: 1, Telemetry: &Telemetry{Tokens: 234, Cost: "0.58"}},
	}}
	roll := RollupTelemetry(st)
	// Tokens are summed verbatim, not derived from any pricing model.
	if roll.Tokens != 1234 {
		t.Errorf("R3.3: tokens = %d, want 1234 (verbatim sum)", roll.Tokens)
	}
	// Cost is the sum of the annotated strings parsed as-is (0.42 + 0.58).
	if roll.Cost != 1.0 || !roll.CostAnnotated {
		t.Errorf("R3.3: cost = %v annotated=%v, want 1.0/true", roll.Cost, roll.CostAnnotated)
	}
}
