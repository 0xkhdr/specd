package core

import (
	"path/filepath"
	"strings"
	"testing"
)

// reopenTasks is a two-task chain: T2 depends on T1, so reopening T1 must mark
// T2 an impacted descendant.
func reopenTasks() []TaskRow {
	return []TaskRow{
		{ID: "T1", Marker: "✅", Role: "craftsman", Files: "a.go", DeclaredFiles: []string{"a.go"}, Verify: "printf ok"},
		{ID: "T2", Marker: "✅", Role: "craftsman", Files: "b.go", DeclaredFiles: []string{"b.go"}, DependsOn: []string{"T1"}, Verify: "printf ok"},
	}
}

func reopenRequest() ReopenRequest {
	return ReopenRequest{
		TaskID:           "T1",
		ExpectedRevision: 4,
		Reason:           "rounding defect found after completion",
		ActorID:          "operator:alice",
		Baseline:         strings.Repeat("b", 40),
	}
}

// reopenSpec seeds a spec directory with state and an empty ledger, and returns
// its state path, event path, and evidence path.
func reopenSpec(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	baseline := InitialState("demo")
	if err := SaveState(statePath, baseline); err != nil {
		t.Fatal(err)
	}
	return statePath, filepath.Join(dir, "workflow-events.jsonl"), filepath.Join(dir, "evidence.jsonl")
}

// reopenCommit reopens taskID through the durable commit path and returns the
// receipt plan.
func reopenCommit(t *testing.T, statePath, eventPath string, req ReopenRequest, tasks []TaskRow, status map[string]TaskRunStatus) ReopenPlan {
	t.Helper()
	state, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil {
		t.Fatal(err)
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedRevision = state.Revision
	preview := PlanTaskReopen("demo", req, tasks, status, events, state.Revision)
	if !preview.Eligible {
		t.Fatalf("preview blocked: %+v", preview.Blockers)
	}
	plan, err := CommitTaskReopen(statePath, eventPath, "demo", req, tasks, status, preview)
	if err != nil {
		t.Fatalf("commit reopen: %v", err)
	}
	return plan
}

func TestTaskReopenAttemptBindingCreatesNextAttempt(t *testing.T) {
	statePath, eventPath, _ := reopenSpec(t)
	plan := reopenCommit(t, statePath, eventPath, reopenRequest(), reopenTasks(), nil)

	attempt := plan.Attempt
	switch {
	case attempt.Attempt != 2 || attempt.PriorAttempt != 1:
		t.Fatalf("attempt = %d (prior %d), want 2 after 1", attempt.Attempt, attempt.PriorAttempt)
	case attempt.PlanRevision != plan.NewRevision:
		t.Fatalf("plan revision = %d, want the attempt's new state revision %d", attempt.PlanRevision, plan.NewRevision)
	case attempt.Baseline != strings.Repeat("b", 40):
		t.Fatalf("baseline = %q, want the fresh current head", attempt.Baseline)
	case attempt.AuthorityDigest == "":
		t.Fatal("attempt carries no fresh authority digest")
	case attempt.Activity != ActivityPending || attempt.Readiness != ReadinessReady:
		t.Fatalf("activity/readiness = %s/%s, want pending/ready", attempt.Activity, attempt.Readiness)
	case len(attempt.ImpactedDescendants) != 1 || attempt.ImpactedDescendants[0] != "T2":
		t.Fatalf("impacted descendants = %v, want the dependent task T2", attempt.ImpactedDescendants)
	}

	events, err := ReadWorkflowEvents(eventPath)
	if err != nil || len(events) != 1 {
		t.Fatalf("ledger = %d events, err %v, want one appended attempt event", len(events), err)
	}
	if events[0].ID != plan.EventID || events[0].Transition != ReopenTaskTransitionPrefix+"T1" {
		t.Fatalf("event = %+v, want the linked task reopen transition", events[0])
	}
	// The prior attempt stays linked and readable from the durable ledger alone.
	replayed := CurrentTaskAttempt(events, "T1")
	if replayed.Attempt != 2 || replayed.PriorAttempt != 1 || replayed.PlanRevision != plan.NewRevision {
		t.Fatalf("replayed attempt = %+v, want the committed attempt", replayed)
	}
	// Reopen never rewrites the marker; the pending activity is a projected fact.
	states, err := ProjectTaskStates(reopenTasks(), nil, ReopenTaskFacts(events, nil))
	if err != nil || states[0].Activity != ActivityPending || states[0].Attempt != 2 {
		t.Fatalf("projected state = %+v, err %v, want pending at attempt 2", states[0], err)
	}
}

func TestTaskReopenAttemptBindingRejectsPriorAttemptEvidence(t *testing.T) {
	statePath, eventPath, evidencePath := reopenSpec(t)
	head := strings.Repeat("a", 40)
	record := EvidenceRecord{TaskID: "T1", Command: "printf ok", ExitCode: 0, GitHead: head}
	if err := AppendEvidence(evidencePath, record); err != nil {
		t.Fatal(err)
	}

	t.Run("first-attempt-evidence-completes", func(t *testing.T) {
		records, err := LoadEvidence(evidencePath)
		if err != nil {
			t.Fatal(err)
		}
		if !HasPassingEvidence(records, "T1") {
			t.Fatal("evidence written before attempt binding must still complete a task that was never reopened")
		}
		if _, err := CompleteTask([]byte("| T1 | scout | a.go | - | printf ok | ok |"), "T1", records); err != nil {
			t.Fatalf("complete: %v", err)
		}
	})

	reopenCommit(t, statePath, eventPath, reopenRequest(), reopenTasks(), nil)

	t.Run("prior-evidence", func(t *testing.T) {
		records, err := LoadEvidence(evidencePath)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := records["T1"]; ok {
			t.Fatal("attempt-1 evidence still counts for a reopened task")
		}
		if _, err := CompleteTask([]byte("| T1 | scout | a.go | - | printf ok | ok |"), "T1", records); err == nil ||
			!strings.Contains(err.Error(), "current attempt") {
			t.Fatalf("complete = %v, want a refusal naming the current attempt", err)
		}
		// Identical command, files, and git HEAD: only the attempt differs.
		full, err := LoadEvidenceRecords(evidencePath)
		if err != nil || len(full) != 1 || full[0].GitHead != head {
			t.Fatalf("history = %+v, err %v, want the prior attempt preserved verbatim", full, err)
		}
	})

	t.Run("current-attempt-evidence-completes", func(t *testing.T) {
		if err := AppendEvidence(evidencePath, record); err != nil {
			t.Fatal(err)
		}
		records, err := LoadEvidence(evidencePath)
		if err != nil {
			t.Fatal(err)
		}
		stamped, ok := records["T1"]
		if !ok || stamped.Attempt != 2 || stamped.PlanRevision == 0 {
			t.Fatalf("record = %+v, want evidence stamped with the current attempt", stamped)
		}
		if _, err := CompleteTask([]byte("| T1 | scout | a.go | - | printf ok | ok |"), "T1", records); err != nil {
			t.Fatalf("complete: %v", err)
		}
	})
}

func TestTaskReopenAttemptBindingRefusesUnboundedRepairScope(t *testing.T) {
	t.Run("cross-task-scope", func(t *testing.T) {
		req := reopenRequest()
		req.RepairPaths = []string{"a.go", "internal/other.go"}
		plan := PlanTaskReopen("demo", req, reopenTasks(), nil, nil, 4)
		if plan.Eligible {
			t.Fatal("repair outside the task's declared files must refuse without an amendment")
		}
		if !reopenBlocked(plan, "REOPEN_SCOPE_AMENDMENT_REQUIRED") {
			t.Fatalf("blockers = %+v, want a bounded scope amendment demand", plan.Blockers)
		}
		if err := plan.Refusal(); err == nil || !strings.Contains(err.Error(), "--scope internal/other.go") {
			t.Fatalf("refusal = %v, want the exact --scope recovery", err)
		}
	})

	t.Run("approved-amendment-admits-the-path", func(t *testing.T) {
		statePath, eventPath, _ := reopenSpec(t)
		req := reopenRequest()
		req.RepairPaths = []string{"a.go", "internal/other.go"}
		req.ScopeAmendment = []string{"internal/other.go"}
		plan := reopenCommit(t, statePath, eventPath, req, reopenTasks(), nil)
		if got := plan.Attempt.EffectiveScope([]string{"a.go"}); len(got) != 2 || got[0] != "a.go" || got[1] != "internal/other.go" {
			t.Fatalf("effective scope = %v, want the declared files plus the amendment", got)
		}
		if plan.Attempt.ScopeRevision != plan.NewRevision {
			t.Fatalf("scope revision = %d, want the amending revision %d", plan.Attempt.ScopeRevision, plan.NewRevision)
		}
		events, err := ReadWorkflowEvents(eventPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := CurrentTaskAttempt(events, "T1").Amendment; len(got) != 1 || got[0] != "internal/other.go" {
			t.Fatalf("durable amendment = %v, want the approved bound replayed from the ledger", got)
		}
	})

	t.Run("unbounded-amendment", func(t *testing.T) {
		req := reopenRequest()
		req.ScopeAmendment = []string{"../outside.go"}
		plan := PlanTaskReopen("demo", req, reopenTasks(), nil, nil, 4)
		if plan.Eligible || !reopenBlocked(plan, "REOPEN_SCOPE_INVALID") {
			t.Fatalf("blockers = %+v, want an escape refusal", plan.Blockers)
		}
	})
}

func TestTaskReopenAttemptBindingRefusesLiveLease(t *testing.T) {
	t.Run("live-lease", func(t *testing.T) {
		req := reopenRequest()
		req.Leases = []TaskLease{{LeaseID: "lease-7", TaskID: "T1", Holder: "worker-9"}}
		plan := PlanTaskReopen("demo", req, reopenTasks(), nil, nil, 4)
		if plan.Eligible || !reopenBlocked(plan, "REOPEN_LEASE_ACTIVE") {
			t.Fatalf("blockers = %+v, want a live-lease refusal", plan.Blockers)
		}
		if err := plan.Refusal(); err == nil || !strings.Contains(err.Error(), "--revoke-lease lease-7") {
			t.Fatalf("refusal = %v, want the exact revoke recovery", err)
		}
	})

	t.Run("authorized-revocation-commits", func(t *testing.T) {
		req := reopenRequest()
		req.Leases = []TaskLease{{LeaseID: "lease-7", TaskID: "T1", Holder: "worker-9"}}
		req.RevokeLease = "lease-7"
		plan := PlanTaskReopen("demo", req, reopenTasks(), nil, nil, 4)
		if !plan.Eligible {
			t.Fatalf("blockers = %+v, want an authorized revocation to pass", plan.Blockers)
		}
		if len(plan.LeaseActions) != 1 || plan.LeaseActions[0].Action != ImpactLeaseRevoke {
			t.Fatalf("lease actions = %+v, want one revocation the caller must apply", plan.LeaseActions)
		}
	})

	t.Run("own-lease-releases", func(t *testing.T) {
		req := reopenRequest()
		req.Leases = []TaskLease{{LeaseID: "lease-7", TaskID: "T1", Holder: req.ActorID}}
		plan := PlanTaskReopen("demo", req, reopenTasks(), nil, nil, 4)
		if !plan.Eligible {
			t.Fatalf("blockers = %+v, want the operator's own lease released in place", plan.Blockers)
		}
		if len(plan.LeaseActions) != 1 || plan.LeaseActions[0].Action != ImpactLeaseRelease {
			t.Fatalf("lease actions = %+v, want a release", plan.LeaseActions)
		}
	})
}

func TestTaskReopenAttemptBindingEligibility(t *testing.T) {
	pending := reopenTasks()
	pending[0].Marker = " "

	cases := map[string]struct {
		tasks   []TaskRow
		facts   map[string]TaskFacts
		blocked string
	}{
		"failed-task":    {tasks: pending, facts: map[string]TaskFacts{"T1": {Activity: ActivityFailed}}},
		"cancelled-task": {tasks: pending, facts: map[string]TaskFacts{"T1": {Activity: ActivityCancelled}}},
		"completed-task": {tasks: reopenTasks()},
		"pending-task":   {tasks: pending, blocked: "REOPEN_TASK_NOT_ELIGIBLE"},
		"in-progress":    {tasks: pending, facts: map[string]TaskFacts{"T1": {Activity: ActivityInProgress}}, blocked: "REOPEN_TASK_NOT_ELIGIBLE"},
		"unknown-task":   {tasks: []TaskRow{{ID: "T9", Marker: "✅", Verify: "printf ok"}}, blocked: "REOPEN_TASK_UNKNOWN"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := reopenRequest()
			req.Facts = tc.facts
			plan := PlanTaskReopen("demo", req, tc.tasks, nil, nil, 4)
			if tc.blocked == "" {
				if !plan.Eligible {
					t.Fatalf("blockers = %+v, want an eligible reopen", plan.Blockers)
				}
				return
			}
			if plan.Eligible || !reopenBlocked(plan, tc.blocked) {
				t.Fatalf("blockers = %+v, want %s", plan.Blockers, tc.blocked)
			}
		})
	}
}

func TestTaskReopenAttemptBindingRefusesStaleAndUnpinnedRequests(t *testing.T) {
	cases := map[string]struct {
		mutate func(*ReopenRequest)
		want   string
	}{
		"stale-revision":        {func(r *ReopenRequest) { r.ExpectedRevision = 3 }, "REOPEN_REVISION_STALE"},
		"missing-reason":        {func(r *ReopenRequest) { r.Reason = "  " }, "REOPEN_REASON_REQUIRED"},
		"missing-actor":         {func(r *ReopenRequest) { r.ActorID = "" }, "REOPEN_ACTOR_REQUIRED"},
		"unresolvable-baseline": {func(r *ReopenRequest) { r.Baseline = UnknownHead }, "REOPEN_BASELINE_UNRESOLVABLE"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := reopenRequest()
			tc.mutate(&req)
			plan := PlanTaskReopen("demo", req, reopenTasks(), nil, nil, 4)
			if plan.Eligible || !reopenBlocked(plan, tc.want) {
				t.Fatalf("blockers = %+v, want %s", plan.Blockers, tc.want)
			}
		})
	}

	t.Run("commit-refuses-drifted-preview", func(t *testing.T) {
		statePath, eventPath, _ := reopenSpec(t)
		req := reopenRequest()
		req.ExpectedRevision = 0
		preview := PlanTaskReopen("demo", req, reopenTasks(), nil, nil, 0)
		if !preview.Eligible {
			t.Fatalf("blockers = %+v, want an eligible preview", preview.Blockers)
		}
		// Land one attempt, then commit the now-stale preview.
		reopenCommit(t, statePath, eventPath, reopenRequest(), reopenTasks(), nil)
		if _, err := CommitTaskReopen(statePath, eventPath, "demo", req, reopenTasks(), nil, preview); err == nil {
			t.Fatal("a preview from a superseded revision must refuse")
		}
	})
}

func reopenBlocked(plan ReopenPlan, code string) bool {
	for _, blocker := range plan.Blockers {
		if blocker.Code == code {
			return true
		}
	}
	return false
}
