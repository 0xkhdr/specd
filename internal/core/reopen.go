package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corescope "github.com/0xkhdr/specd/internal/core/scope"
)

const ReopenPlanSchemaVersion = "1"

// ReopenTaskTransitionPrefix marks a task-reopen workflow event; the task id
// follows it, so a ledger scan reads as `reopen.task.T7`.
const ReopenTaskTransitionPrefix = "reopen.task."

// reopenEntityKind is the workflow-event entity kind a task attempt advances.
// BeforeEntityVersion/AfterEntityVersion are the prior and new attempt numbers,
// so the attempt chain is the event chain — there is no second counter to drift.
const reopenEntityKind = "task"

// Impacted-entity prefixes of a reopen event. InputDigests admits only 32-byte
// hex digests, so the bounded scope amendment travels in ImpactedEntities,
// where it stays durable, replayable, and readable.
const (
	ReopenTaskEntityPrefix  = "task:"
	ReopenScopeEntityPrefix = "scope:"
)

// ReopenableActivities are the terminal activities an operator may reopen
// (R3.1). Pending and in-progress work is not reopened — it is still open.
var ReopenableActivities = []TaskActivity{ActivityCompleted, ActivityFailed, ActivityCancelled}

// TaskAttempt is the identity every piece of a task's work binds to: evidence,
// scope, and authority are valid only for the attempt they were minted under
// (R3.1, R3.2). Attempt 1 is implicit — a task that was never reopened has no
// reopen event, which is why the zero value of every field reads as "first
// attempt, task-declared scope, no prior attempt".
type TaskAttempt struct {
	TaskID              string       `json:"task_id"`
	Attempt             int          `json:"attempt"`
	PriorAttempt        int          `json:"prior_attempt,omitempty"`
	PlanRevision        int64        `json:"plan_revision"`
	ScopeRevision       int64        `json:"scope_revision"`
	Baseline            string       `json:"baseline,omitempty"`
	AuthorityDigest     string       `json:"authority_digest,omitempty"`
	Amendment           []string     `json:"scope_amendment,omitempty"`
	ImpactedDescendants []string     `json:"impacted_descendants,omitempty"`
	Activity            TaskActivity `json:"activity,omitempty"`
	Readiness           Readiness    `json:"readiness,omitempty"`
}

// EffectiveScope is the attempt's bounded write scope: the task's declared
// files plus the amendment approved inside the reopen transaction (R3.3).
func (a TaskAttempt) EffectiveScope(declared []string) []string {
	return corescope.Amend(declared, a.Amendment)
}

// CurrentTaskAttempt projects a task's current attempt from the spec's workflow
// ledger. A task with no reopen event is on attempt 1 with plan and scope
// revision 0, which is exactly what an evidence record written before attempt
// binding decodes to — old evidence keeps completing tasks that were never
// reopened (R3.2).
func CurrentTaskAttempt(events []WorkflowEventV1, taskID string) TaskAttempt {
	attempt := TaskAttempt{TaskID: taskID, Attempt: 1, Activity: ActivityPending}
	scopeDigest := reopenScopeDigest(nil)
	for _, event := range events {
		if !isTaskReopenEvent(event, taskID) {
			continue
		}
		attempt.Attempt = int(event.AfterEntityVersion)
		attempt.PriorAttempt = int(event.BeforeEntityVersion)
		attempt.PlanRevision = event.ResultingRevision
		attempt.Baseline = event.GitHead
		attempt.AuthorityDigest = event.AuthorityDigest
		attempt.Amendment, attempt.ImpactedDescendants = reopenEntities(event)
		// The scope revision moves only when the amendment itself moves, so a
		// re-reopen that keeps the same bounds does not invalidate scope-bound
		// evidence for no reason.
		if digest := event.InputDigests["scope"]; digest != scopeDigest {
			attempt.ScopeRevision = event.ResultingRevision
			scopeDigest = digest
		}
	}
	return attempt
}

// TaskAttempts maps every reopened task to its current attempt. Tasks absent
// from the map are on attempt 1 (CurrentTaskAttempt).
func TaskAttempts(events []WorkflowEventV1) map[string]TaskAttempt {
	attempts := map[string]TaskAttempt{}
	for _, event := range events {
		if !isTaskReopenEvent(event, event.EntityID) {
			continue
		}
		attempts[event.EntityID] = CurrentTaskAttempt(events, event.EntityID)
	}
	return attempts
}

// ReopenTaskFacts projects reopened tasks back to pending activity at their
// current attempt. The tasks.md marker is deliberately not rewritten: activity
// no marker can express is carried by facts, so tasks.md stays byte-stable.
func ReopenTaskFacts(events []WorkflowEventV1, base map[string]TaskFacts) map[string]TaskFacts {
	facts := make(map[string]TaskFacts, len(base))
	for id, fact := range base {
		facts[id] = fact
	}
	for id, attempt := range TaskAttempts(events) {
		fact := facts[id]
		fact.Activity = ActivityPending
		fact.Attempt = attempt.Attempt
		facts[id] = fact
	}
	return facts
}

func isTaskReopenEvent(event WorkflowEventV1, taskID string) bool {
	return event.EntityKind == reopenEntityKind && event.EntityID == taskID &&
		strings.HasPrefix(event.Transition, ReopenTaskTransitionPrefix)
}

// reopenEntities splits a reopen event's impacted entities back into the
// approved scope amendment and the impacted descendant tasks.
func reopenEntities(event WorkflowEventV1) (amendment, descendants []string) {
	for _, entity := range event.ImpactedEntities {
		switch {
		case strings.HasPrefix(entity, ReopenScopeEntityPrefix):
			amendment = append(amendment, strings.TrimPrefix(entity, ReopenScopeEntityPrefix))
		case strings.HasPrefix(entity, ReopenTaskEntityPrefix):
			id := strings.TrimPrefix(entity, ReopenTaskEntityPrefix)
			if id != event.EntityID {
				descendants = append(descendants, id)
			}
		}
	}
	return amendment, descendants
}

func reopenScopeDigest(amendment []string) string {
	return Digest([]byte(strings.Join(amendment, "\n")))
}

// TaskLease is a caller-resolved live claim on a task. Planning stays pure: the
// caller reads the session, the plan only classifies what it is handed.
type TaskLease struct {
	LeaseID string
	TaskID  string
	Holder  string
}

// ReopenRequest is the caller-resolved reopen intent.
type ReopenRequest struct {
	TaskID           string
	ExpectedRevision int64
	Reason           string
	ActorID          string
	// Baseline is the current git HEAD the new attempt is pinned to. An
	// unresolvable baseline refuses: an attempt with no subject revision can
	// never carry evidence that completes it.
	Baseline string
	// ScopeAmendment is the bounded repair scope approved inside this
	// transaction, beyond the task's declared files (R3.3).
	ScopeAmendment []string
	// RepairPaths are paths the pending repair already touches, normally the
	// worktree diff against the baseline. Any path outside the task's declared
	// files plus the amendment refuses.
	RepairPaths []string
	// Leases are the live claims on this spec's tasks; RevokeLease is the one
	// lease the operator explicitly authorized revoking (R3.4).
	Leases      []TaskLease
	RevokeLease string
	// Facts are the caller's existing per-task facts (clarifications, manual
	// waits, prior dispositions) that eligibility and readiness project over.
	Facts map[string]TaskFacts
}

// ReopenPlan is the deterministic preview of one task reopen, and the receipt
// CommitTaskReopen returns for the attempt it actually created.
type ReopenPlan struct {
	SchemaVersion   string              `json:"schema_version"`
	Eligible        bool                `json:"eligible"`
	Slug            string              `json:"slug"`
	TaskID          string              `json:"task_id"`
	PriorActivity   TaskActivity        `json:"prior_activity,omitempty"`
	Attempt         TaskAttempt         `json:"attempt"`
	CurrentRevision int64               `json:"current_revision"`
	NewRevision     int64               `json:"new_revision"`
	EventID         string              `json:"event_id,omitempty"`
	LeaseActions    []ImpactLeaseAction `json:"lease_actions"`
	ImpactDigest    string              `json:"impact_digest,omitempty"`
	Impact          ImpactPlan          `json:"impact"`
	Blockers        []TransitionBlocker `json:"blockers"`
}

// PlanTaskReopen is pure: it classifies the request against the caller's task,
// ledger, and lease snapshot and returns every blocker instead of raising the
// first one.
func PlanTaskReopen(slug string, req ReopenRequest, tasks []TaskRow, status map[string]TaskRunStatus, events []WorkflowEventV1, currentRevision int64) ReopenPlan {
	plan := ReopenPlan{
		SchemaVersion:   ReopenPlanSchemaVersion,
		Slug:            slug,
		TaskID:          req.TaskID,
		CurrentRevision: currentRevision,
		NewRevision:     currentRevision + 1,
		LeaseActions:    []ImpactLeaseAction{},
		Blockers:        []TransitionBlocker{},
	}
	if strings.TrimSpace(req.Reason) == "" {
		plan.addBlocker("REOPEN_REASON_REQUIRED", "reason", "reopen requires a reason; re-run with --reason <text>")
	}
	if strings.TrimSpace(req.ActorID) == "" {
		plan.addBlocker("REOPEN_ACTOR_REQUIRED", "actor", "reopen requires an accountable actor; set SPECD_ACTOR")
	}
	if !HeadPinned(req.Baseline) {
		plan.addBlocker("REOPEN_BASELINE_UNRESOLVABLE", req.TaskID, fmt.Sprintf(
			"cannot pin a fresh baseline for task %s (git_head %q); reopen inside a repository with a resolvable HEAD", req.TaskID, req.Baseline))
	}
	if req.ExpectedRevision != currentRevision {
		plan.addBlocker("REOPEN_REVISION_STALE", req.TaskID, fmt.Sprintf(
			"expected state revision %d but current is %d; re-preview and re-run with --expect-revision %d",
			req.ExpectedRevision, currentRevision, currentRevision))
	}

	var row *TaskRow
	for i := range tasks {
		if tasks[i].ID == req.TaskID {
			row = &tasks[i]
		}
	}
	if row == nil {
		plan.addBlocker("REOPEN_TASK_UNKNOWN", req.TaskID, fmt.Sprintf("task %q is not in this spec's tasks.md", req.TaskID))
		return plan
	}

	facts := ReopenTaskFacts(events, req.Facts)
	states, err := ProjectTaskStates(tasks, status, facts)
	if err != nil {
		plan.addBlocker("REOPEN_TASKS_INVALID", req.TaskID, fmt.Sprintf("cannot project task states: %v", err))
		return plan
	}
	activity := map[string]TaskActivity{}
	for _, state := range states {
		activity[state.ID] = state.Activity
	}
	plan.PriorActivity = activity[req.TaskID]
	if !reopenable(plan.PriorActivity) {
		plan.addBlocker("REOPEN_TASK_NOT_ELIGIBLE", req.TaskID, fmt.Sprintf(
			"task %s is %s; only a completed, failed, or cancelled task is reopened", req.TaskID, plan.PriorActivity))
	}

	amendment, scopeErr := corescope.NormalizeAll(req.ScopeAmendment)
	if scopeErr != nil {
		plan.addBlocker("REOPEN_SCOPE_INVALID", req.TaskID, fmt.Sprintf(
			"scope amendment is not bounded to the workspace: %v; pass workspace-relative paths to --scope", scopeErr))
	}
	effective := corescope.Amend(row.DeclaredFiles, amendment)
	if outside := corescope.Outside(req.RepairPaths, effective); len(outside) > 0 {
		plan.addBlocker("REOPEN_SCOPE_AMENDMENT_REQUIRED", req.TaskID, fmt.Sprintf(
			"repair touches %s outside the declared scope of task %s; approve a bounded amendment with --scope %s",
			strings.Join(outside, ", "), req.TaskID, strings.Join(outside, ",")))
	}

	prior := CurrentTaskAttempt(events, req.TaskID)
	attempt := TaskAttempt{
		TaskID:       req.TaskID,
		Attempt:      prior.Attempt + 1,
		PriorAttempt: prior.Attempt,
		PlanRevision: plan.NewRevision,
		Baseline:     req.Baseline,
		Amendment:    amendment,
		Activity:     ActivityPending,
	}
	attempt.ScopeRevision = prior.ScopeRevision
	if reopenScopeDigest(amendment) != reopenScopeDigest(prior.Amendment) {
		attempt.ScopeRevision = plan.NewRevision
	}
	attempt.AuthorityDigest = Digest([]byte(strings.Join([]string{
		"reopen", req.ActorID, req.TaskID, strconv.Itoa(attempt.Attempt), req.Reason,
	}, "|")))

	plan.Impact = buildReopenImpact(slug, req, tasks, activity, prior.Attempt)
	plan.ImpactDigest = plan.Impact.ImpactDigest
	for _, blocker := range plan.Impact.Blockers {
		plan.addBlocker(blocker.Code, blocker.Entity, blocker.Message)
	}
	for _, entity := range plan.Impact.Entities {
		if entity.Kind == reopenEntityKind && entity.Classification == ImpactStale {
			attempt.ImpactedDescendants = append(attempt.ImpactedDescendants, entity.ID)
		}
	}

	plan.LeaseActions = plan.Impact.LeaseActions
	for _, action := range plan.LeaseActions {
		if action.Action == ImpactLeaseRelease || action.EntityID == "" {
			continue
		}
		lease := leaseFor(req.Leases, action.EntityID)
		if req.RevokeLease == lease.LeaseID && lease.LeaseID != "" {
			continue
		}
		plan.addBlocker("REOPEN_LEASE_ACTIVE", action.EntityID, fmt.Sprintf(
			"lease %s held by %s still owns task %s; authorize the revocation inside this reopen with --revoke-lease %s, or wait for %s to release it",
			lease.LeaseID, action.Holder, action.EntityID, lease.LeaseID, action.Holder))
	}

	// Readiness is derived, never stored: project the tasks again with the new
	// attempt's pending activity in place (R3.1).
	next := make(map[string]TaskFacts, len(facts)+1)
	for id, fact := range facts {
		next[id] = fact
	}
	next[req.TaskID] = TaskFacts{Activity: ActivityPending, Attempt: attempt.Attempt, Waits: facts[req.TaskID].Waits}
	if projected, err := ProjectTaskStates(tasks, status, next); err == nil {
		for _, state := range projected {
			if state.ID == req.TaskID {
				attempt.Readiness = state.Readiness
			}
		}
	}

	plan.Attempt = attempt
	plan.Eligible = len(plan.Blockers) == 0
	return plan
}

// ReopenActor is the accountable identity a reopen is recorded under — the same
// identity every other durable record in this package resolves.
func ReopenActor() string { return recordActor() }

func reopenable(activity TaskActivity) bool {
	for _, eligible := range ReopenableActivities {
		if activity == eligible {
			return true
		}
	}
	return false
}

func leaseFor(leases []TaskLease, taskID string) TaskLease {
	for _, lease := range leases {
		if lease.TaskID == taskID {
			return lease
		}
	}
	return TaskLease{}
}

// buildReopenImpact previews the reopened task plus every task that transitively
// depends on it, so the descendants the new attempt invalidates are named by the
// same shared preview `specd undo` uses (R1, R3.1).
func buildReopenImpact(slug string, req ReopenRequest, tasks []TaskRow, activity map[string]TaskActivity, priorAttempt int) ImpactPlan {
	candidates := make([]ImpactCandidate, 0, len(tasks))
	for _, task := range tasks {
		deps := make([]string, 0, len(task.DependsOn))
		for _, dep := range task.DependsOn {
			deps = append(deps, ImpactRef(reopenEntityKind, slug, dep))
		}
		candidate := ImpactCandidate{
			Kind: reopenEntityKind, ID: task.ID, Spec: slug,
			Version: strconv.Itoa(1), State: string(activity[task.ID]), DependsOn: deps,
		}
		if task.ID == req.TaskID {
			candidate.Version = strconv.Itoa(priorAttempt)
		}
		if lease := leaseFor(req.Leases, task.ID); lease.LeaseID != "" {
			candidate.LeaseHolder = lease.Holder
		}
		candidates = append(candidates, candidate)
	}
	return BuildImpactPlan(ImpactInput{
		Operation:             ImpactOperationReopen,
		RequestedKind:         reopenEntityKind,
		RequestedID:           req.TaskID,
		RequestedSpec:         slug,
		RequestedVersion:      strconv.Itoa(priorAttempt),
		ExpectedStateRevision: req.ExpectedRevision,
		Actor:                 ActorOperator,
		ActorID:               req.ActorID,
		Candidates:            candidates,
	})
}

func (p *ReopenPlan) addBlocker(code, entity, message string) {
	blocker := TransitionBlocker{Code: code, Gate: "reopen", Entity: entity, Message: message}
	for _, existing := range p.Blockers {
		if existing == blocker {
			return
		}
	}
	p.Blockers = append(p.Blockers, blocker)
	sort.SliceStable(p.Blockers, func(i, j int) bool { return p.Blockers[i].Code < p.Blockers[j].Code })
}

// Refusal renders the plan's first blocker as the operator-actionable refusal.
func (p ReopenPlan) Refusal() error {
	if len(p.Blockers) == 0 {
		return nil
	}
	blocker := p.Blockers[0]
	return Refusef(blocker.Code, "%s", blocker.Message).
		WithRecovery(RefusalActorOperator, fmt.Sprintf(
			"specd reopen %s task %s --reason <text> --expect-revision %d", p.Slug, p.TaskID, p.CurrentRevision)).
		WithContext(blocker.Entity, "reopen target", "eligible unleased completed, failed, or cancelled task")
}

// BuildTaskReopenEvent returns the append-only event that opens the next
// attempt. Nothing is deleted: the prior attempt stays in the ledger and is
// linked by BeforeEntityVersion (R3.1).
func BuildTaskReopenEvent(plan ReopenPlan, req ReopenRequest, projection State) (WorkflowEventV1, error) {
	if !plan.Eligible {
		return WorkflowEventV1{}, plan.Refusal()
	}
	impacted := []string{ReopenTaskEntityPrefix + plan.TaskID}
	for _, id := range plan.Attempt.ImpactedDescendants {
		impacted = append(impacted, ReopenTaskEntityPrefix+id)
	}
	for _, path := range plan.Attempt.Amendment {
		impacted = append(impacted, ReopenScopeEntityPrefix+path)
	}
	return NewWorkflowEvent(WorkflowEventV1{
		EntityKind:          reopenEntityKind,
		EntityID:            plan.TaskID,
		BeforeEntityVersion: int64(plan.Attempt.PriorAttempt),
		AfterEntityVersion:  int64(plan.Attempt.Attempt),
		ExpectedRevision:    plan.CurrentRevision,
		Transition:          ReopenTaskTransitionPrefix + plan.TaskID,
		Actor:               req.ActorID,
		AuthorityDigest:     plan.Attempt.AuthorityDigest,
		Reason:              req.Reason,
		InputDigests: map[string]string{
			"impact_plan": plan.ImpactDigest,
			"scope":       reopenScopeDigest(plan.Attempt.Amendment),
		},
		ImpactedEntities: sortedTransitionSet(impacted),
		GitHead:          plan.Attempt.Baseline,
		Timestamp:        Clock().Format(time.RFC3339),
		Projection:       projection,
	})
}

// CommitTaskReopen reloads durable state, re-plans against it, refuses on drift
// from the preview, and appends the attempt event under the same event-first
// CAS as any other transition. A refusal mutates nothing.
func CommitTaskReopen(statePath, eventPath, slug string, req ReopenRequest, tasks []TaskRow, status map[string]TaskRunStatus, preview ReopenPlan) (ReopenPlan, error) {
	state, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil {
		return ReopenPlan{}, err
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return ReopenPlan{}, err
	}
	fresh := PlanTaskReopen(slug, req, tasks, status, events, state.Revision)
	if preview.ImpactDigest != "" {
		if err := GuardImpactCommit(preview.Impact, state.Revision, fresh.Impact); err != nil {
			return fresh, err
		}
	}
	if !fresh.Eligible {
		return fresh, fresh.Refusal()
	}
	event, err := BuildTaskReopenEvent(fresh, req, state)
	if err != nil {
		return fresh, err
	}
	if err := CommitWorkflowTransition(TransitionCommit{StatePath: statePath, EventPath: eventPath, Event: event}); err != nil {
		return fresh, err
	}
	fresh.EventID = event.ID
	fresh.NewRevision = event.ResultingRevision
	return fresh, nil
}
