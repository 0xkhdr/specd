package core

import (
	"strings"
	"testing"
)

// staleAfterReopen reopens T1 in a fresh spec and returns the ledger paths plus
// the projected staleness of its completed descendant T2.
func staleAfterReopen(t *testing.T) (string, string, string, []StaleDescendant) {
	t.Helper()
	statePath, eventPath, evidencePath := reopenSpec(t)
	reopenCommit(t, statePath, eventPath, reopenRequest(), reopenTasks(), nil)
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	return statePath, eventPath, evidencePath, StaleDescendants(events)
}

// resolutionRequest is a well-formed resolution of the stale descendant T2; each
// test weakens exactly the proof it is about.
func resolutionRequest(t *testing.T, eventPath, resolution string) DescendantResolutionRequest {
	t.Helper()
	state, err := LoadState(strings.Replace(eventPath, "workflow-events.jsonl", "state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	return DescendantResolutionRequest{
		TaskID:           "T2",
		Resolution:       resolution,
		Reason:           "descendant reviewed after the T1 repair",
		ActorID:          "operator:alice",
		ExpectedRevision: state.Revision,
		CurrentHead:      strings.Repeat("b", 40),
		Attempt:          CurrentTaskAttempt(events, "T2"),
	}
}

func blockerCodes(blockers []TransitionBlocker) string {
	codes := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		codes = append(codes, blocker.Code)
	}
	return strings.Join(codes, ",")
}

func TestStaleDescendantResolutionMarksCompletedDescendantsStale(t *testing.T) {
	_, eventPath, _, stale := staleAfterReopen(t)
	if len(stale) != 1 || stale[0].TaskID != "T2" || stale[0].Parent != "T1" {
		t.Fatalf("stale = %+v, want the completed descendant T2 of the T1 reopen", stale)
	}
	t.Run("stale-until-explicitly-resolved", func(t *testing.T) {
		if !stale[0].Unresolved() || strings.Join(stale[0].Choices, ",") != strings.Join(DescendantResolutions, ",") {
			t.Fatalf("descendant = %+v, want unresolved with every allowed resolution offered", stale[0])
		}
		if stale[0].StaleSinceRevision == 0 {
			t.Fatal("staleness carries no revision, so readiness cannot be proved from current revisions")
		}
	})
	t.Run("marker-not-reset-to-pending", func(t *testing.T) {
		events, err := ReadWorkflowEvents(eventPath)
		if err != nil {
			t.Fatal(err)
		}
		states, err := ProjectTaskStates(reopenTasks(), nil, ReopenTaskFacts(events, nil))
		if err != nil {
			t.Fatal(err)
		}
		for _, state := range states {
			if state.ID == "T2" && state.Activity != ActivityCompleted {
				t.Fatalf("descendant activity = %s, want completed+stale rather than a silent reset", state.Activity)
			}
		}
		if row := reopenTasks()[1]; row.Marker != "✅" {
			t.Fatalf("descendant marker = %q, want the tasks.md marker left byte-stable", row.Marker)
		}
	})
}

func TestStaleDescendantResolutionRejectsDigestOnlyRetain(t *testing.T) {
	_, eventPath, evidencePath, stale := staleAfterReopen(t)
	req := resolutionRequest(t, eventPath, DescendantRetain)
	req.ApprovalRef = "ar-1"
	req.DigestUnchanged = true

	t.Run("digest-equality-is-not-proof", func(t *testing.T) {
		plan := PlanDescendantResolution("demo", req, stale, req.ExpectedRevision)
		if plan.Eligible || blockerCodes(plan.Blockers) != "DESCENDANT_DIGEST_ONLY" {
			t.Fatalf("plan = %v %s, want a refusal naming digest-only retention", plan.Eligible, blockerCodes(plan.Blockers))
		}
	})
	t.Run("approval-alone-is-not-proof", func(t *testing.T) {
		approved := req
		approved.DigestUnchanged = false
		plan := PlanDescendantResolution("demo", approved, stale, approved.ExpectedRevision)
		if plan.Eligible || blockerCodes(plan.Blockers) != "DESCENDANT_EVIDENCE_STALE" {
			t.Fatalf("plan = %v %s, want fresh evidence required beside the approval", plan.Eligible, blockerCodes(plan.Blockers))
		}
	})
	t.Run("evidence-alone-is-not-approval", func(t *testing.T) {
		fresh := req
		fresh.ApprovalRef = ""
		fresh.DigestUnchanged = false
		fresh.Evidence, fresh.HasEvidence = passingEvidence(t, evidencePath, fresh.CurrentHead), true
		plan := PlanDescendantResolution("demo", fresh, stale, fresh.ExpectedRevision)
		if plan.Eligible || blockerCodes(plan.Blockers) != "DESCENDANT_RETAIN_UNAPPROVED" {
			t.Fatalf("plan = %v %s, want explicit impact approval required", plan.Eligible, blockerCodes(plan.Blockers))
		}
	})
	t.Run("approval-plus-fresh-evidence-retains", func(t *testing.T) {
		fresh := req
		fresh.DigestUnchanged = false
		fresh.Evidence, fresh.HasEvidence = passingEvidence(t, evidencePath, fresh.CurrentHead), true
		plan := PlanDescendantResolution("demo", fresh, stale, fresh.ExpectedRevision)
		if !plan.Eligible {
			t.Fatalf("plan blocked: %s", blockerCodes(plan.Blockers))
		}
	})
}

func TestStaleDescendantResolutionRequiresCriterionReassignment(t *testing.T) {
	_, eventPath, _, stale := staleAfterReopen(t)
	req := resolutionRequest(t, eventPath, DescendantCancel)
	req.Criteria = []string{"R2.1", "R2.2"}

	t.Run("cancel-without-coverage-refuses", func(t *testing.T) {
		plan := PlanDescendantResolution("demo", req, stale, req.ExpectedRevision)
		if plan.Eligible || blockerCodes(plan.Blockers) != "DESCENDANT_COVERAGE_UNASSIGNED" {
			t.Fatalf("plan = %v %s, want cancelled acceptance coverage refused", plan.Eligible, blockerCodes(plan.Blockers))
		}
		if !strings.Contains(plan.Blockers[0].Message, "R2.1, R2.2") {
			t.Fatalf("refusal %q does not name the uncovered criteria", plan.Blockers[0].Message)
		}
	})
	t.Run("partial-reassignment-refuses", func(t *testing.T) {
		partial := req
		partial.Reassignments = []CriterionReassignment{{Criterion: "R2.1", From: "T2", To: "T3"}}
		plan := PlanDescendantResolution("demo", partial, stale, partial.ExpectedRevision)
		if plan.Eligible || !strings.Contains(plan.Blockers[0].Message, "R2.2") {
			t.Fatalf("plan = %v %+v, want the still-uncovered criterion refused", plan.Eligible, plan.Blockers)
		}
	})
	t.Run("reassignment-to-self-covers-nothing", func(t *testing.T) {
		self := req
		self.Reassignments = []CriterionReassignment{{Criterion: "R2.1", From: "T2", To: "T2"}, {Criterion: "R2.2", From: "T2", To: "T2"}}
		if plan := PlanDescendantResolution("demo", self, stale, self.ExpectedRevision); plan.Eligible {
			t.Fatal("reassigning coverage back to the cancelled task must not satisfy acceptance")
		}
	})
	t.Run("supersede-needs-a-successor", func(t *testing.T) {
		supersede := req
		supersede.Resolution = DescendantSupersede
		supersede.Reassignments = []CriterionReassignment{{Criterion: "R2.1", From: "T2", To: "T3"}, {Criterion: "R2.2", From: "T2", To: "T3"}}
		plan := PlanDescendantResolution("demo", supersede, stale, supersede.ExpectedRevision)
		if plan.Eligible || blockerCodes(plan.Blockers) != "DESCENDANT_SUCCESSOR_REQUIRED" {
			t.Fatalf("plan = %v %s, want the superseding task required", plan.Eligible, blockerCodes(plan.Blockers))
		}
		supersede.Successor = "T3"
		if plan := PlanDescendantResolution("demo", supersede, stale, supersede.ExpectedRevision); !plan.Eligible {
			t.Fatalf("plan blocked: %s", blockerCodes(plan.Blockers))
		}
	})
}

func TestStaleDescendantResolutionRevalidatesReadOnlyTask(t *testing.T) {
	statePath, eventPath, evidencePath, stale := staleAfterReopen(t)
	req := resolutionRequest(t, eventPath, DescendantRevalidate)

	t.Run("read-only-task-is-not-exempt", func(t *testing.T) {
		plan := PlanDescendantResolution("demo", req, stale, req.ExpectedRevision)
		if plan.Eligible || blockerCodes(plan.Blockers) != "DESCENDANT_EVIDENCE_STALE" {
			t.Fatalf("plan = %v %s, want a trivially-verified task still to need fresh evidence", plan.Eligible, blockerCodes(plan.Blockers))
		}
	})
	t.Run("prior-revision-evidence-refuses", func(t *testing.T) {
		prior := req
		prior.Evidence, prior.HasEvidence = passingEvidence(t, evidencePath, strings.Repeat("a", 40)), true
		if plan := PlanDescendantResolution("demo", prior, stale, prior.ExpectedRevision); plan.Eligible {
			t.Fatal("evidence pinned to a prior revision must not prove the descendant current")
		}
	})
	t.Run("fresh-trivial-verify-resolves", func(t *testing.T) {
		fresh := req
		fresh.Evidence, fresh.HasEvidence = passingEvidence(t, evidencePath, fresh.CurrentHead), true
		preview := PlanDescendantResolution("demo", fresh, stale, fresh.ExpectedRevision)
		if !preview.Eligible {
			t.Fatalf("preview blocked: %s", blockerCodes(preview.Blockers))
		}
		plan, err := CommitDescendantResolution(statePath, eventPath, "demo", fresh, preview)
		if err != nil {
			t.Fatalf("commit resolution: %v", err)
		}
		events, err := ReadWorkflowEvents(eventPath)
		if err != nil {
			t.Fatal(err)
		}
		replayed := StaleDescendants(events)
		if len(replayed) != 1 || replayed[0].Resolution != DescendantRevalidate || replayed[0].ResolvedRevision != plan.NewRevision {
			t.Fatalf("replayed = %+v, want the committed revalidation, replay-equivalent", replayed)
		}
		if attempt := CurrentTaskAttempt(events, "T2"); attempt.Attempt != 1 {
			t.Fatalf("descendant attempt = %d, want revalidation not to mint an attempt", attempt.Attempt)
		}
		if plan := PlanDescendantResolution("demo", fresh, replayed, plan.NewRevision); plan.Eligible {
			t.Fatal("a resolved descendant must not be resolvable twice")
		}
	})
}

func TestStaleDescendantResolutionBlocksParentReadiness(t *testing.T) {
	statePath, eventPath, _, stale := staleAfterReopen(t)
	blockers := StaleDescendantBlockers(stale)
	if len(blockers) != 1 || blockers[0].Entity != "T1" || blockers[0].Code != "DESCENDANT_STALE_UNRESOLVED" {
		t.Fatalf("blockers = %+v, want the parent T1 blocked by its unresolved descendant", blockers)
	}

	t.Run("reopen-route-is-not-a-resolution-record", func(t *testing.T) {
		req := resolutionRequest(t, eventPath, DescendantReopen)
		plan := PlanDescendantResolution("demo", req, stale, req.ExpectedRevision)
		if plan.Eligible || blockerCodes(plan.Blockers) != "DESCENDANT_REOPEN_ROUTE" {
			t.Fatalf("plan = %v %s, want the reopen route named instead", plan.Eligible, blockerCodes(plan.Blockers))
		}
	})
	t.Run("reopening-the-descendant-resolves-it", func(t *testing.T) {
		req := reopenRequest()
		req.TaskID = "T2"
		reopenCommit(t, statePath, eventPath, req, reopenTasks(), nil)
		events, err := ReadWorkflowEvents(eventPath)
		if err != nil {
			t.Fatal(err)
		}
		resolved := StaleDescendants(events)
		if len(resolved) != 1 || resolved[0].Resolution != DescendantReopen {
			t.Fatalf("resolved = %+v, want the descendant resolved by its own reopen", resolved)
		}
		if got := StaleDescendantBlockers(resolved); len(got) != 0 {
			t.Fatalf("blockers = %+v, want parent readiness unblocked once every descendant is resolved", got)
		}
	})
}

// TestStaleDescendantResolutionRecursOnLaterReopen walks a T0-T1-T2 chain: a
// resolved descendant goes stale again when a nearer ancestor is reopened, and
// its second resolution appends rather than replacing the first.
func TestStaleDescendantResolutionRecursOnLaterReopen(t *testing.T) {
	chain := []TaskRow{
		{ID: "T0", Marker: "✅", Role: "craftsman", Files: "z.go", DeclaredFiles: []string{"z.go"}, Verify: "printf ok"},
		{ID: "T1", Marker: "✅", Role: "craftsman", Files: "a.go", DeclaredFiles: []string{"a.go"}, DependsOn: []string{"T0"}, Verify: "printf ok"},
		{ID: "T2", Marker: "✅", Role: "craftsman", Files: "b.go", DeclaredFiles: []string{"b.go"}, DependsOn: []string{"T1"}, Verify: "printf ok"},
	}
	statePath, eventPath, evidencePath := reopenSpec(t)
	root := reopenRequest()
	root.TaskID = "T0"
	reopenCommit(t, statePath, eventPath, root, chain, nil)

	resolve := func(t *testing.T) DescendantResolutionPlan {
		t.Helper()
		events, err := ReadWorkflowEvents(eventPath)
		if err != nil {
			t.Fatal(err)
		}
		req := resolutionRequest(t, eventPath, DescendantRevalidate)
		req.Attempt = CurrentTaskAttempt(events, "T2")
		req.Evidence, req.HasEvidence = passingEvidence(t, evidencePath, req.CurrentHead), true
		preview := PlanDescendantResolution("demo", req, StaleDescendants(events), req.ExpectedRevision)
		if !preview.Eligible {
			t.Fatalf("preview blocked: %s", blockerCodes(preview.Blockers))
		}
		plan, err := CommitDescendantResolution(statePath, eventPath, "demo", req, preview)
		if err != nil {
			t.Fatalf("commit resolution: %v", err)
		}
		return plan
	}

	first := resolve(t)
	if first.Sequence != 0 {
		t.Fatalf("first resolution sequence = %d, want 0", first.Sequence)
	}
	// T1 is still completed, so the nearer ancestor is reopenable and its
	// reopen makes the already-resolved T2 stale again.
	mid := reopenRequest()
	reopenCommit(t, statePath, eventPath, mid, chain, nil)
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := StaleDescendants(events)
	if len(stale) != 2 {
		t.Fatalf("stale = %+v, want both T1 and T2 tracked", stale)
	}
	for _, entry := range stale {
		if entry.TaskID == "T2" && (!entry.Unresolved() || entry.Parent != "T1") {
			t.Fatalf("descendant = %+v, want T2 stale again under its nearer parent T1", entry)
		}
	}
	if second := resolve(t); second.Sequence != 1 {
		t.Fatalf("second resolution sequence = %d, want the resolution chain to advance", second.Sequence)
	}
	if events, err := ReadWorkflowEvents(eventPath); err != nil || len(events) != 4 {
		t.Fatalf("ledger = %d events, err %v, want every reopen and resolution preserved", len(events), err)
	}
}

// passingEvidence records a passing verify for the read-only descendant T2 at
// head and returns the attempt-current record the resolution is judged against.
func passingEvidence(t *testing.T, evidencePath, head string) EvidenceRecord {
	t.Helper()
	if err := AppendEvidence(evidencePath, EvidenceRecord{TaskID: "T2", Command: "printf ok", ExitCode: 0, GitHead: head}); err != nil {
		t.Fatal(err)
	}
	records, err := LoadEvidence(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	return records["T2"]
}
