package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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

func TestScopeAmendRunningTaskAppendsPathAndAuditEvent(t *testing.T) {
	statePath, eventPath, _ := reopenSpec(t)
	tasksPath := filepath.Join(filepath.Dir(statePath), "tasks.md")
	raw := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| 🚧 T1 | craftsman | a.go | - | printf ok | R6.1 |\n"
	if err := AtomicWrite(tasksPath, raw); err != nil {
		t.Fatal(err)
	}
	tasks := []TaskRow{{ID: "T1", Marker: "🚧", Role: "craftsman", Files: "a.go", DeclaredFiles: []string{"a.go"}}}
	req := ScopeAmendRequest{TaskID: "T1", Path: "internal/new.go", Reason: "implementation discovered dependency",
		ActorID: "operator:alice", GitHead: strings.Repeat("a", 40)}
	state, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedRevision = state.Revision
	preview := PlanScopeAmend("demo", req, tasks, map[string]TaskRunStatus{"T1": TaskRunning}, state.Revision)
	plan, err := CommitScopeAmend(tasksPath, statePath, eventPath, "demo", req, tasks,
		map[string]TaskRunStatus{"T1": TaskRunning}, preview)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Eligible || plan.EventID == "" {
		t.Fatalf("plan = %+v", plan)
	}
	updated, err := os.ReadFile(tasksPath)
	if err != nil || !strings.Contains(string(updated), "a.go, internal/new.go") {
		t.Fatalf("tasks = %q, err %v", updated, err)
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil || len(events) != 1 || events[0].Transition != "scope.amend.T1" {
		t.Fatalf("events = %+v, err %v", events, err)
	}
	attempt := CurrentTaskAttempt(events, "T1")
	if attempt.Attempt != 1 || attempt.ScopeRevision != plan.NewRevision ||
		!slices.Contains(attempt.Amendment, "internal/new.go") {
		t.Fatalf("attempt = %+v", attempt)
	}
}

func TestScopeAmendRefusesUnsafePathAndNonRunningTask(t *testing.T) {
	tasks := []TaskRow{{ID: "T1", DeclaredFiles: []string{"a.go"}}}
	req := ScopeAmendRequest{TaskID: "T1", Path: "../escape", Reason: "x", ActorID: "operator:alice"}
	plan := PlanScopeAmend("demo", req, tasks, map[string]TaskRunStatus{"T1": TaskPending}, 0)
	if plan.Eligible || len(plan.Blockers) != 2 {
		t.Fatalf("plan = %+v, want path and running blockers", plan)
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
	var body strings.Builder
	body.WriteString("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n")
	for _, task := range tasks {
		deps := strings.Join(task.DependsOn, ",")
		if deps == "" {
			deps = "-"
		}
		body.WriteString("| " + strings.TrimSpace(task.Marker+" "+task.ID) + " | " + task.Role + " | " + task.Files + " | " + deps + " | " + task.Verify + " | ok |\n")
	}
	tasksPath := filepath.Join(filepath.Dir(statePath), "tasks.md")
	if err := AtomicWrite(tasksPath, body.String()); err != nil {
		t.Fatal(err)
	}
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
	plan, err := CommitTaskReopen(tasksPath, statePath, eventPath, "demo", req, tasks, status, preview)
	if err != nil {
		t.Fatalf("commit reopen: %v", err)
	}
	return plan
}

func TestReopenTaskResetCreatesNextAttempt(t *testing.T) {
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
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(statePath), "tasks.md"))
	if err != nil || strings.Contains(string(raw), "✅ T1") {
		t.Fatalf("tasks.md = %q, err %v, want T1 reset to pending", raw, err)
	}
	state, err := LoadState(statePath)
	if err != nil || state.TaskStatus["T1"] != TaskPending {
		t.Fatalf("state = %+v, err %v, want T1 reset to pending", state, err)
	}
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
		if _, err := CommitTaskReopen(filepath.Join(filepath.Dir(statePath), "tasks.md"), statePath, eventPath, "demo", req, reopenTasks(), nil, preview); err == nil {
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

// artifactReopenRoot seeds a spec whose three artifacts exist on disk, with a
// pending approval request for the design gate.
func artifactReopenRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, artifact := range ReopenableArtifacts {
		body := "# " + artifact + "\n"
		if artifact == "tasks" {
			body = "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| ✅ T1 | craftsman | a.go | - | printf ok | ok |\n| ✅ T2 | craftsman | b.go | T1 | printf ok | ok |\n"
		}
		if err := AtomicWrite(filepath.Join(SpecdDir(root), "specs", "demo", artifact+".md"), body); err != nil {
			t.Fatal(err)
		}
	}
	state := InitialState("demo")
	state.Stage, state.Status, state.Phase = StageDesign, StatusDesign, PhaseForStatus(StatusDesign)
	state.TaskStatus = map[string]TaskRunStatus{"T1": TaskComplete, "T2": TaskComplete}
	rec := StampApprovalRequest(ApprovalRequestRecord{
		ID: "approve:design", Transition: ApprovalRequested,
		EntityKind: ApprovalEntitySpec, EntityID: "demo", EntityVersion: "design",
		Pins:      ApprovalPins{ArtifactDigest: "a", PlanDigest: "p", ConfigDigest: "c"},
		Requester: "operator:alice", ExpiresAt: Clock().Add(time.Hour).Format(time.RFC3339),
	}, strings.Repeat("b", 40))
	key, planned, err := PlanApprovalRequest(nil, rec)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(planned)
	if err != nil {
		t.Fatal(err)
	}
	state.Records[key] = raw
	approval, err := json.Marshal(StampRecord(Record{Kind: "approval", Gate: "design"}, strings.Repeat("b", 40)))
	if err != nil {
		t.Fatal(err)
	}
	state.Records["approval:design"] = approval
	if err := SaveState(StatePath(root, "demo"), state); err != nil {
		t.Fatal(err)
	}
	return root
}

// artifactReopenRequest is a well-formed reopen of artifact ("" reopens the spec).
func artifactReopenRequest(t *testing.T, root, artifact string) ArtifactReopenRequest {
	t.Helper()
	digests := map[string]string{}
	for _, name := range ReopenableArtifacts {
		path, err := SpecArtifactPath(root, "demo", name)
		if err != nil {
			t.Fatal(err)
		}
		if raw, readErr := os.ReadFile(path); readErr == nil {
			digests[name] = Digest(raw)
		}
	}
	return ArtifactReopenRequest{
		Artifact: artifact,
		Reason:   "acceptance defect found before release",
		ActorID:  "operator:alice",
		GitHead:  strings.Repeat("b", 40),
		Digests:  digests,
	}
}

// artifactReopenCommit previews and commits req against the durable spec.
func artifactReopenCommit(t *testing.T, root string, req ArtifactReopenRequest) (ArtifactReopenPlan, error) {
	t.Helper()
	statePath, eventPath := StatePath(root, "demo"), WorkflowEventPath(root, "demo")
	state, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil {
		t.Fatal(err)
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedRevision = state.Revision
	preview := PlanArtifactReopen("demo", req, state, events)
	if !preview.Eligible {
		return preview, preview.Refusal()
	}
	return CommitArtifactReopen(root, "demo", req, preview)
}

// artifactReopenLedger is the spec's current state and event count.
func artifactReopenLedger(t *testing.T, root string) (State, int) {
	t.Helper()
	state, err := LoadState(StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	events, err := ReadWorkflowEvents(WorkflowEventPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	return state, len(events)
}

func TestArtifactSpecReopenCreatesDraftVersion(t *testing.T) {
	root := artifactReopenRoot(t)
	prior := Digest([]byte("# design\n"))
	plan, err := artifactReopenCommit(t, root, artifactReopenRequest(t, root, "design"))
	if err != nil {
		t.Fatalf("reopen design: %v", err)
	}
	if len(plan.Revisions) != 1 {
		t.Fatalf("revisions = %+v, want exactly the reopened artifact", plan.Revisions)
	}
	revision := plan.Revisions[0]
	switch {
	case revision.Version != 2 || revision.PriorVersion != 1:
		t.Fatalf("version = %d (prior %d), want 2 after 1", revision.Version, revision.PriorVersion)
	case revision.PriorDigest != prior:
		t.Fatalf("prior digest = %q, want the preserved bytes %q", revision.PriorDigest, prior)
	case revision.VersionID == "" || revision.VersionID == revision.PriorDigest:
		t.Fatalf("version id = %q, want a fresh identity distinct from the preserved digest", revision.VersionID)
	case plan.Kind != ReopenArtifactEntityKind || plan.Cycle != plan.PriorCycle:
		t.Fatalf("plan = %+v, want an artifact reopen inside the same cycle", plan)
	}

	// R4.1: the prior revision is preserved byte-for-byte under its digest.
	raw, err := os.ReadFile(filepath.Join(SpecdDir(root), "specs", "demo", revision.SnapshotPath))
	if err != nil || string(raw) != "# design\n" {
		t.Fatalf("snapshot = %q, err %v, want the prior revision preserved", raw, err)
	}

	state, count := artifactReopenLedger(t, root)
	if count != 1 || state.Revision != plan.NewRevision {
		t.Fatalf("ledger = %d events at revision %d, want one event at %d", count, state.Revision, plan.NewRevision)
	}
	if state.Stage != StageDesign || state.Condition != ConditionActive {
		t.Fatalf("stage/condition = %s/%s, want the new draft active at its own stage", state.Stage, state.Condition)
	}
	// R4.1: the open approval request for that artifact no longer holds.
	requests, err := state.ApprovalRequests()
	if err != nil || ApprovalRequestPending(requests, "approve:design") {
		t.Fatalf("requests = %+v, err %v, want approve:design invalidated", requests, err)
	}
	if len(requests) != 2 || requests[0].Transition != ApprovalRequested {
		t.Fatalf("requests = %+v, want the original transition retained beside the invalidation", requests)
	}
	if versions := ArtifactVersions(mustEvents(t, root)); versions["design"] != 2 {
		t.Fatalf("versions = %+v, want design on draft version 2", versions)
	}
}

func mustEvents(t *testing.T, root string) []WorkflowEventV1 {
	t.Helper()
	events, err := ReadWorkflowEvents(WorkflowEventPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestReopenSpecStartsNewCycle(t *testing.T) {
	root := artifactReopenRoot(t)
	plan, err := artifactReopenCommit(t, root, artifactReopenRequest(t, root, ""))
	if err != nil {
		t.Fatalf("reopen spec: %v", err)
	}
	if plan.Kind != ReopenSpecEntityKind || plan.Cycle != 2 || plan.PriorCycle != 1 {
		t.Fatalf("plan = %+v, want cycle 2 referencing cycle 1", plan)
	}
	if len(plan.Revisions) != len(ReopenableArtifacts) {
		t.Fatalf("revisions = %+v, want the complete prior cycle preserved", plan.Revisions)
	}
	for _, revision := range plan.Revisions {
		if _, err := os.Stat(filepath.Join(SpecdDir(root), "specs", "demo", revision.SnapshotPath)); err != nil {
			t.Fatalf("snapshot %s: %v", revision.SnapshotPath, err)
		}
	}
	state, count := artifactReopenLedger(t, root)
	if state.Cycle != 2 || state.Stage != StageRequirements || count != 1 {
		t.Fatalf("state = %+v (%d events), want a fresh cycle at requirements", state, count)
	}
	if state.TaskStatus["T1"] != TaskPending || state.TaskStatus["T2"] != TaskPending {
		t.Fatalf("task status = %+v, want every task reset to pending", state.TaskStatus)
	}
	raw, err := os.ReadFile(filepath.Join(SpecdDir(root), "specs", "demo", "tasks.md"))
	if err != nil || strings.Contains(string(raw), "✅ T1") || strings.Contains(string(raw), "✅ T2") {
		t.Fatalf("tasks.md = %q, err %v, want every marker reset to pending", raw, err)
	}
	// R4.2: the prior cycle stays fully reportable — its event, its projection,
	// and its approval history are all still on disk.
	events := mustEvents(t, root)
	if events[0].BeforeEntityVersion != 1 || events[0].AfterEntityVersion != 2 || events[0].Projection.Cycle != 2 {
		t.Fatalf("event = %+v, want the new cycle linked to the prior one", events[0])
	}
	if requests, err := state.ApprovalRequests(); err != nil || len(requests) != 2 {
		t.Fatalf("requests = %+v, err %v, want the prior cycle's request history retained", requests, err)
	}
	if _, ok := state.Records["approval:design:cycle:1"]; !ok {
		t.Fatalf("records = %+v, want the prior cycle approval retained", state.Records)
	}
	if _, ok := state.Records["approval:design"]; ok {
		t.Fatalf("records = %+v, want the current-cycle design approval cleared", state.Records)
	}
}

func TestArtifactSpecReopenRefusesConsumedWork(t *testing.T) {
	cases := map[string]struct {
		consumption ImpactConsumption
		want        string
	}{
		"released-work":  {ImpactConsumption{Record: "releases.jsonl", Kind: "release", External: true}, "REOPEN_CONSUMED_EXTERNALLY"},
		"deployed-work":  {ImpactConsumption{Record: "deployments.jsonl", Kind: "deployment", External: true}, "REOPEN_CONSUMED_EXTERNALLY"},
		"archived-work":  {ImpactConsumption{Record: "archive.json", Kind: "archive", External: true}, "REOPEN_CONSUMED_EXTERNALLY"},
		"submitted-work": {ImpactConsumption{Record: "submissions.jsonl", Kind: "submission"}, "REOPEN_CONSUMED"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := artifactReopenRoot(t)
			req := artifactReopenRequest(t, root, "design")
			req.Consumptions = []ImpactConsumption{tc.consumption}
			plan, err := artifactReopenCommit(t, root, req)
			if err == nil {
				t.Fatal("consumed work must refuse in-place reopen")
			}
			if !artifactReopenBlocked(plan, tc.want) {
				t.Fatalf("blockers = %+v, want %s", plan.Blockers, tc.want)
			}
			if tc.consumption.External && !strings.Contains(err.Error(), "link a successor") {
				t.Fatalf("refusal = %v, want the linked-successor route", err)
			}
			if !tc.consumption.External && !strings.Contains(err.Error(), "withdraw or revoke") {
				t.Fatalf("refusal = %v, want the withdrawal requirement", err)
			}
			state, count := artifactReopenLedger(t, root)
			if count != 0 || state.Revision != 0 {
				t.Fatalf("ledger = %d events at revision %d, want a refusal to mutate nothing", count, state.Revision)
			}
		})
	}
}

func TestArtifactSpecReopenSnapshotFailureMutatesNothing(t *testing.T) {
	root := artifactReopenRoot(t)
	req := artifactReopenRequest(t, root, "design")
	state, err := RecoverWorkflowState(StatePath(root, "demo"), WorkflowEventPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedRevision = state.Revision
	preview := PlanArtifactReopen("demo", req, state, nil)
	if !preview.Eligible {
		t.Fatalf("blockers = %+v, want an eligible preview", preview.Blockers)
	}

	t.Run("snapshot-failure", func(t *testing.T) {
		// The artifact disappears between preview and commit, so its prior
		// revision can no longer be preserved (R4.4).
		path, err := SpecArtifactPath(root, "demo", "design")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if _, err := CommitArtifactReopen(root, "demo", req, preview); err == nil {
			t.Fatal("a failed snapshot must refuse the whole transaction")
		}
		current, count := artifactReopenLedger(t, root)
		if count != 0 || current.Revision != state.Revision {
			t.Fatalf("ledger = %d events at revision %d, want no mutation at all", count, current.Revision)
		}
		if requests, err := current.ApprovalRequests(); err != nil || !ApprovalRequestPending(requests, "approve:design") {
			t.Fatalf("requests = %+v, err %v, want the approval request untouched", requests, err)
		}
	})

	t.Run("unreadable-artifact-refuses-preview", func(t *testing.T) {
		blocked := PlanArtifactReopen("demo", artifactReopenRequest(t, root, "design"), state, nil)
		if blocked.Eligible || !artifactReopenBlocked(blocked, "REOPEN_ARTIFACT_UNREADABLE") {
			t.Fatalf("blockers = %+v, want REOPEN_ARTIFACT_UNREADABLE", blocked.Blockers)
		}
	})
}

func TestArtifactSpecReopenRefusesMalformedRequests(t *testing.T) {
	root := artifactReopenRoot(t)
	state, err := LoadState(StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		mutate func(*ArtifactReopenRequest)
		want   string
	}{
		"missing-reason":   {func(r *ArtifactReopenRequest) { r.Reason = "  " }, "REOPEN_REASON_REQUIRED"},
		"missing-actor":    {func(r *ArtifactReopenRequest) { r.ActorID = "" }, "REOPEN_ACTOR_REQUIRED"},
		"stale-revision":   {func(r *ArtifactReopenRequest) { r.ExpectedRevision = 9 }, "REOPEN_REVISION_STALE"},
		"unknown-artifact": {func(r *ArtifactReopenRequest) { r.Artifact = "evidence" }, "REOPEN_ARTIFACT_UNKNOWN"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := artifactReopenRequest(t, root, "design")
			req.ExpectedRevision = state.Revision
			tc.mutate(&req)
			plan := PlanArtifactReopen("demo", req, state, nil)
			if plan.Eligible || !artifactReopenBlocked(plan, tc.want) {
				t.Fatalf("blockers = %+v, want %s", plan.Blockers, tc.want)
			}
		})
	}

	t.Run("commit-refuses-drifted-preview", func(t *testing.T) {
		fresh := artifactReopenRoot(t)
		req := artifactReopenRequest(t, fresh, "design")
		req.ExpectedRevision = 0
		preview := PlanArtifactReopen("demo", req, state, nil)
		if _, err := artifactReopenCommit(t, fresh, artifactReopenRequest(t, fresh, "design")); err != nil {
			t.Fatalf("first reopen: %v", err)
		}
		if _, err := CommitArtifactReopen(fresh, "demo", req, preview); err == nil {
			t.Fatal("a preview from a superseded revision must refuse")
		}
	})
}

func artifactReopenBlocked(plan ArtifactReopenPlan, code string) bool {
	for _, blocker := range plan.Blockers {
		if blocker.Code == code {
			return true
		}
	}
	return false
}
