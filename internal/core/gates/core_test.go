package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestTaskTraceGate(t *testing.T) {
	reqs := "### R1 — Title\n\n- R1.1: When x, the system shall y.\n"

	// Unknown requirement reference is always refused (spec 01 R3.1 safety).
	badRef := []core.TaskRow{{ID: "T1", Role: "craftsman", Refs: []string{"R9"}}}
	if f := taskTrace(CheckCtx{Tasks: badRef, RequirementsDoc: reqs}); !HasErrors(f) {
		t.Fatalf("unknown ref should refuse, got %+v", f)
	}

	// Unknown risk tier is always refused.
	badRisk := []core.TaskRow{{ID: "T1", Role: "craftsman", Refs: []string{"R1.1"}, Risk: "spicy"}}
	if f := taskTrace(CheckCtx{Tasks: badRisk, RequirementsDoc: reqs}); !HasErrors(f) {
		t.Fatalf("unknown risk tier should refuse, got %+v", f)
	}

	// Legacy task (no trace columns) passes under the default profile (R7.1).
	legacy := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}}
	if f := taskTrace(CheckCtx{Tasks: legacy, RequirementsDoc: reqs}); len(f) != 0 {
		t.Fatalf("legacy task should pass default profile, got %+v", f)
	}

	// Production planning profile demands the full contract (R3.1).
	if f := taskTrace(CheckCtx{Tasks: legacy, RequirementsDoc: reqs, TaskTraceRequired: true}); !HasErrors(f) {
		t.Fatalf("production profile should require the trace contract, got %+v", f)
	}

	// A fully-declared write task passes under the production profile.
	full := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...",
		Refs: []string{"R1.1"}, Kind: "feature", Risk: "high", Context: "design", Evidence: "unit", Checks: "empty"}}
	if f := taskTrace(CheckCtx{Tasks: full, RequirementsDoc: reqs, TaskTraceRequired: true}); len(f) != 0 {
		t.Fatalf("full contract should pass, got %+v", f)
	}
}

func TestRolesGate(t *testing.T) {
	if f := roles(CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}}}); HasErrors(f) {
		t.Fatalf("known role should pass, got %+v", f)
	}
	f := roles(CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Role: "wizard", Files: "a.go", Verify: "go test ./..."}}})
	if !HasErrors(f) {
		t.Fatal("unknown role should error")
	}
	if !strings.Contains(f[0].Message, "wizard") || !strings.Contains(f[0].Message, "T1") {
		t.Fatalf("finding should name task and role, got %+v", f)
	}
	if f := roles(CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Files: "a.go"}}}); !HasErrors(f) {
		t.Fatal("empty role should error")
	}
}

func TestVerifyGate(t *testing.T) {
	trivial := core.DefaultTrivialVerify
	// Write task (craftsman) with a trivial verify → rejected (spec 01 R4.2).
	f := verifyCommands(CheckCtx{
		Tasks:         []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "printf ok"}},
		TrivialVerify: trivial,
	})
	if !HasErrors(f) {
		t.Fatal("craftsman with trivial verify should error")
	}
	if !strings.Contains(f[0].Message, "T1") {
		t.Fatalf("finding should name the task, got %+v", f)
	}
	// Read-only task (scout) may retain a trivial verify.
	if f := verifyCommands(CheckCtx{
		Tasks:         []core.TaskRow{{ID: "T1", Role: "scout", Files: "a.go", Verify: "printf ok"}},
		TrivialVerify: trivial,
	}); HasErrors(f) {
		t.Fatalf("scout with trivial verify should pass, got %+v", f)
	}
	// Write task with a real verify passes.
	if f := verifyCommands(CheckCtx{
		Tasks:         []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}},
		TrivialVerify: trivial,
	}); HasErrors(f) {
		t.Fatalf("craftsman with real verify should pass, got %+v", f)
	}
	// Missing verify is still required regardless of trivial policy.
	if f := verifyCommands(CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go"}}}); !HasErrors(f) {
		t.Fatal("empty verify should error")
	}
}

func TestCoreGates(t *testing.T) {
	tasks := []core.TaskRow{
		{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."},
		{ID: "T2", Role: "craftsman", Files: "b.go", DependsOn: []string{"T1"}, Verify: "go test ./..."},
	}
	ctx := CheckCtx{
		Tasks:    tasks,
		Status:   map[string]core.TaskRunStatus{"T1": core.TaskComplete},
		Evidence: map[string]core.EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: "abc"}},
	}
	if findings := CoreRegistry().Run(ctx); HasErrors(findings) {
		t.Fatalf("valid tasks produced errors: %#v", findings)
	}

	ctx.Evidence = map[string]core.EvidenceRecord{}
	findings := CoreRegistry().Run(ctx)
	if !HasErrors(findings) {
		t.Fatalf("missing evidence should fail")
	}
}

func TestCoreGatesQualityFreshness(t *testing.T) {
	base := CheckCtx{
		Tasks:    []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}},
		Status:   map[string]core.TaskRunStatus{"T1": core.TaskComplete},
		Evidence: map[string]core.EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: "abc"}},
	}
	// No quality contract configured ⇒ gate stays silent (parity).
	if HasErrors(CoreRegistry().Run(base)) {
		t.Fatalf("empty quality contract produced errors")
	}

	base.QualityContracts = map[string]core.QualityContract{
		"T1": {TaskID: "T1", Required: []core.EvidenceRequirement{{EvidenceClass: core.EvidenceOutputEval, CheckID: "rubric"}}},
	}
	base.QualitySubject = core.FreshnessSubject{Revision: "abc"}

	// required eval absent ⇒ completed task flagged
	if !HasErrors(CoreRegistry().Run(base)) {
		t.Fatalf("missing required eval on complete task not flagged")
	}
	// stale eval ⇒ flagged
	base.Evals = []core.EvidenceEnvelopeV1{{EvidenceClass: core.EvidenceOutputEval, TaskID: "T1", CheckID: "rubric", Verdict: core.EvalPass, SubjectRevision: "old"}}
	if !HasErrors(CoreRegistry().Run(base)) {
		t.Fatalf("stale eval not flagged")
	}
	// fresh eval ⇒ clean
	base.Evals = []core.EvidenceEnvelopeV1{{EvidenceClass: core.EvidenceOutputEval, TaskID: "T1", CheckID: "rubric", Verdict: core.EvalPass, SubjectRevision: "abc"}}
	if HasErrors(CoreRegistry().Run(base)) {
		t.Fatalf("fresh eval still flagged")
	}
}
