package gates

import (
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// TestCoverageRefusalNamesRefsColumnAndRemedies pins spec R5.1: the coverage
// refusal states matching is done against the tasks.md `refs` column, lists
// every uncovered id, and names both remedies (add the id to `refs`, or mark
// the task `kind: deferred`). Matching semantics stay unchanged.
func TestCoverageRefusalNamesRefsColumnAndRemedies(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Refs: []string{"R1.1"}}}
	ctx := CheckCtx{
		ApproveTarget: string(core.StatusExecuting),
		Tasks:         tasks,
		CoverageGaps:  []string{"R2", "R2.1"},
	}
	findings := coverageGate(ctx)
	if !HasErrors(findings) {
		t.Fatalf("coverage gaps not refused: %+v", findings)
	}
	message := findings[0].Message
	for _, want := range []string{"tasks.md `refs` column", "R2", "R2.1", "add each id to an implementing task's `refs` column", "`kind: deferred`"} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal %q missing %q", message, want)
		}
	}
}

// TestCoverageGateArmingPerTarget pins spec R5.2: the tasks-phase approval
// runs the same coverage analysis at warning severity (approval proceeds, the
// gap is reported early), the executing transition keeps its blocking error
// severity, and every other target — or an untraced/gapless table — stays
// silent.
func TestCoverageGateArmingPerTarget(t *testing.T) {
	traced := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Refs: []string{"R1.1"}}}

	tasksFindings := coverageGate(CheckCtx{ApproveTarget: string(core.StatusTasks), Tasks: traced, CoverageGaps: []string{"R2"}})
	if len(tasksFindings) != 1 || tasksFindings[0].Severity != Warn || !strings.Contains(tasksFindings[0].Message, "R2") {
		t.Fatalf("tasks-phase coverage advisory wrong: %+v", tasksFindings)
	}
	if HasErrors(tasksFindings) {
		t.Fatalf("tasks-phase coverage advisory must not block approval: %+v", tasksFindings)
	}
	execFindings := coverageGate(CheckCtx{ApproveTarget: string(core.StatusExecuting), Tasks: traced, CoverageGaps: []string{"R2"}})
	if !HasErrors(execFindings) {
		t.Fatalf("executing transition no longer blocks on coverage gaps: %+v", execFindings)
	}
	if f := coverageGate(CheckCtx{ApproveTarget: "design", Tasks: traced, CoverageGaps: []string{"R2"}}); len(f) != 0 {
		t.Fatalf("gate armed outside tasks/executing: %+v", f)
	}
	untraced := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}}
	if f := coverageGate(CheckCtx{ApproveTarget: string(core.StatusExecuting), Tasks: untraced, CoverageGaps: []string{"R2"}}); len(f) != 0 {
		t.Fatalf("gate armed without task trace: %+v", f)
	}
	if f := coverageGate(CheckCtx{ApproveTarget: string(core.StatusExecuting), Tasks: traced}); len(f) != 0 {
		t.Fatalf("gate fired without gaps: %+v", f)
	}
}

// TestApprovalRequestIntegrationStaleDigest pins spec 03 R5.3/R5.4: the
// approval gate projects the immutable request identity and refuses an open
// request whose pinned inputs drifted or whose expiry passed, naming the only
// legal continuation — a new or superseding request. An approved request whose
// inputs drifted is reported without blocking; a terminal one is silent.
func TestApprovalRequestIntegrationStaleDigest(t *testing.T) {
	pinned := core.ApprovalPins{ArtifactDigest: "art1", StateRevision: 7, PlanDigest: "plan1", ConfigDigest: "cfg1"}
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour).Format(time.RFC3339)
	past := now.Add(-time.Hour).Format(time.RFC3339)
	request := func(id string, transition core.ApprovalTransition, expires string) core.ApprovalRequestRecord {
		return core.ApprovalRequestRecord{ID: id, Transition: transition, EntityKind: core.ApprovalEntitySpec, EntityID: "demo", Pins: pinned, ExpiresAt: expires}
	}
	requests := []core.ApprovalRequestRecord{
		request("approve:design", core.ApprovalRequested, future),
		request("approve:expired", core.ApprovalRequested, past),
		request("approve:requirements", core.ApprovalRequested, future),
		request("approve:requirements", core.ApprovalApproved, future),
		request("approve:tasks", core.ApprovalRequested, future),
		request("approve:tasks", core.ApprovalWithdrawn, future),
	}
	drifted := pinned
	drifted.ArtifactDigest = "art2"
	drifted.PlanDigest = "plan2"
	current := map[string]core.ApprovalPins{
		"approve:design":       drifted,
		"approve:expired":      pinned,
		"approve:requirements": drifted,
		"approve:tasks":        drifted,
	}

	states := ApprovalRequestStates(requests, current, now)
	if len(states) != 4 {
		t.Fatalf("projected %d requests, want one per id: %+v", len(states), states)
	}
	if states[0].ID != "approve:design" || states[0].State != core.ApprovalRequested || states[0].Entity != "spec:demo" {
		t.Fatalf("identity projection lost the request identity: %+v", states[0])
	}
	if states[0].Pins != pinned {
		t.Fatalf("projection rewrote the pinned identities: %+v", states[0].Pins)
	}
	if len(states[0].Drift) != 2 || states[0].Drift[0] != "artifact digest" || states[0].Drift[1] != "transition-plan digest" {
		t.Fatalf("drift = %v, want artifact and transition-plan digest", states[0].Drift)
	}
	if states[2].State != core.ApprovalApproved {
		t.Fatalf("newest transition not projected: %+v", states[2])
	}
	if !states[1].Expired {
		t.Fatalf("expired request not marked: %+v", states[1])
	}

	findings := ApprovalRequestFindings(states)
	if !HasErrors(findings) {
		t.Fatalf("stale open request not refused: %+v", findings)
	}
	if len(findings) != 3 {
		t.Fatalf("findings = %+v, want stale + expired errors and one approved warning", findings)
	}
	if findings[0].Severity != Error || !strings.Contains(findings[0].Message, "approve:design") ||
		!strings.Contains(findings[0].Message, "artifact digest") ||
		!strings.Contains(findings[0].Message, "new or superseding request") {
		t.Fatalf("stale refusal = %+v", findings[0])
	}
	if findings[1].Severity != Error || !strings.Contains(findings[1].Message, "expired at "+past) {
		t.Fatalf("expiry refusal = %+v", findings[1])
	}
	if findings[2].Severity != Warn || !strings.Contains(findings[2].Message, "approve:requirements") {
		t.Fatalf("approved-drift advisory = %+v", findings[2])
	}

	currentStates := ApprovalRequestStates(requests[:1], map[string]core.ApprovalPins{"approve:design": pinned}, now)
	if f := ApprovalRequestFindings(currentStates); len(f) != 0 {
		t.Fatalf("current request refused: %+v", f)
	}
}
