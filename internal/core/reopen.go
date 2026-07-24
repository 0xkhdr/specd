package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
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
const ScopeAmendTransitionPrefix = "scope.amend."

// reopenEntityKind is the workflow-event entity kind a task attempt advances.
// BeforeEntityVersion/AfterEntityVersion are the prior and new attempt numbers,
// so the attempt chain is the event chain — there is no second counter to drift.
const reopenEntityKind = "task"
const scopeEntityKind = "scope"

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
		if event.EntityKind == scopeEntityKind && event.EntityID == taskID &&
			event.Transition == ScopeAmendTransitionPrefix+taskID {
			for _, entity := range event.ImpactedEntities {
				if strings.HasPrefix(entity, ReopenScopeEntityPrefix) {
					attempt.Amendment = corescope.Amend(attempt.Amendment,
						[]string{strings.TrimPrefix(entity, ReopenScopeEntityPrefix)})
				}
			}
			attempt.ScopeRevision = event.ResultingRevision
			attempt.AuthorityDigest = event.AuthorityDigest
			continue
		}
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

type ScopeAmendRequest struct {
	TaskID, Path, Reason, ActorID, GitHead string
	ExpectedRevision                       int64
}

type ScopeAmendPlan struct {
	SchemaVersion   string              `json:"schema_version"`
	Slug            string              `json:"slug"`
	TaskID          string              `json:"task_id"`
	Path            string              `json:"path"`
	CurrentRevision int64               `json:"current_revision"`
	NewRevision     int64               `json:"new_revision"`
	AuthorityDigest string              `json:"authority_digest,omitempty"`
	EventID         string              `json:"event_id,omitempty"`
	Eligible        bool                `json:"eligible"`
	Blockers        []TransitionBlocker `json:"blockers"`
}

func (p *ScopeAmendPlan) addBlocker(code, entity, message string) {
	p.Blockers = append(p.Blockers, TransitionBlocker{Code: code, Entity: entity, Message: message})
}

func (p ScopeAmendPlan) Refusal() error {
	if p.Eligible {
		return nil
	}
	if len(p.Blockers) == 0 {
		return Refusef("SCOPE_AMEND_REFUSED", "scope amendment refused")
	}
	return Refusef(p.Blockers[0].Code, "%s", p.Blockers[0].Message)
}

func PlanScopeAmend(slug string, req ScopeAmendRequest, tasks []TaskRow, status map[string]TaskRunStatus, currentRevision int64) ScopeAmendPlan {
	plan := ScopeAmendPlan{SchemaVersion: ReopenPlanSchemaVersion, Slug: slug, TaskID: req.TaskID,
		CurrentRevision: currentRevision, NewRevision: currentRevision + 1}
	if strings.TrimSpace(req.Reason) == "" {
		plan.addBlocker("SCOPE_AMEND_REASON_REQUIRED", req.TaskID, "scope amendment requires --reason <text>")
	}
	if strings.TrimSpace(req.ActorID) == "" {
		plan.addBlocker("SCOPE_AMEND_ACTOR_REQUIRED", req.TaskID, "scope amendment requires an accountable actor; set SPECD_ACTOR")
	}
	if req.ExpectedRevision != currentRevision {
		plan.addBlocker("SCOPE_AMEND_REVISION_STALE", req.TaskID, fmt.Sprintf(
			"expected state revision %d but current is %d; re-run with --expect-revision %d",
			req.ExpectedRevision, currentRevision, currentRevision))
	}
	normalized, err := corescope.NormalizeAll([]string{req.Path})
	if err != nil || len(normalized) != 1 {
		plan.addBlocker("SCOPE_AMEND_PATH_INVALID", req.Path, fmt.Sprintf("scope path must be one workspace-relative path: %v", err))
	} else {
		plan.Path = normalized[0]
	}
	var row *TaskRow
	for i := range tasks {
		if tasks[i].ID == req.TaskID {
			row = &tasks[i]
			break
		}
	}
	if row == nil {
		plan.addBlocker("SCOPE_AMEND_TASK_UNKNOWN", req.TaskID, fmt.Sprintf("task %q is not in this spec's tasks.md", req.TaskID))
	} else {
		if status[req.TaskID] != TaskRunning {
			plan.addBlocker("SCOPE_AMEND_TASK_NOT_RUNNING", req.TaskID, fmt.Sprintf("task %s is %s; scope may be amended only while it is running", req.TaskID, status[req.TaskID]))
		}
		if slices.Contains(row.DeclaredFiles, plan.Path) {
			plan.addBlocker("SCOPE_AMEND_ALREADY_DECLARED", req.TaskID, fmt.Sprintf("path %s is already declared by task %s", plan.Path, req.TaskID))
		}
	}
	plan.AuthorityDigest = Digest([]byte(strings.Join([]string{"scope-amend", req.ActorID, req.TaskID, plan.Path, req.Reason}, "|")))
	plan.Eligible = len(plan.Blockers) == 0
	return plan
}

func BuildScopeAmendEvent(plan ScopeAmendPlan, req ScopeAmendRequest, projection State, priorVersion int64) (WorkflowEventV1, error) {
	if !plan.Eligible {
		return WorkflowEventV1{}, plan.Refusal()
	}
	return NewWorkflowEvent(WorkflowEventV1{
		EntityKind: scopeEntityKind, EntityID: plan.TaskID,
		BeforeEntityVersion: priorVersion, AfterEntityVersion: priorVersion + 1,
		ExpectedRevision: plan.CurrentRevision, Transition: ScopeAmendTransitionPrefix + plan.TaskID,
		Actor: req.ActorID, AuthorityDigest: plan.AuthorityDigest, Reason: req.Reason,
		InputDigests:     map[string]string{"path": Digest([]byte(plan.Path))},
		ImpactedEntities: []string{ReopenTaskEntityPrefix + plan.TaskID, ReopenScopeEntityPrefix + plan.Path},
		GitHead:          req.GitHead, Timestamp: Clock().Format(time.RFC3339), Projection: projection,
	})
}

func CommitScopeAmend(tasksPath, statePath, eventPath, slug string, req ScopeAmendRequest, tasks []TaskRow, status map[string]TaskRunStatus, preview ScopeAmendPlan) (ScopeAmendPlan, error) {
	state, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil {
		return ScopeAmendPlan{}, err
	}
	fresh := PlanScopeAmend(slug, req, tasks, status, state.Revision)
	if !fresh.Eligible {
		return fresh, fresh.Refusal()
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return fresh, err
	}
	var version int64
	for _, event := range events {
		if event.EntityKind == scopeEntityKind && event.EntityID == req.TaskID && event.AfterEntityVersion > version {
			version = event.AfterEntityVersion
		}
	}
	event, err := BuildScopeAmendEvent(fresh, req, state, version)
	if err != nil {
		return fresh, err
	}
	original, err := os.ReadFile(tasksPath)
	if err != nil {
		return fresh, err
	}
	updated, err := RewriteTaskDeclaredPath(original, req.TaskID, fresh.Path)
	if err != nil {
		return fresh, err
	}
	artifact := TransitionArtifact{Path: tasksPath, Before: string(original), After: string(updated)}
	event, err = bindTransitionArtifact(event, artifact)
	if err != nil {
		return fresh, err
	}
	if err := CommitWorkflowTransition(TransitionCommit{
		StatePath: statePath, EventPath: eventPath, Event: event,
		Artifact: &artifact,
	}); err != nil {
		return fresh, err
	}
	fresh.EventID, fresh.NewRevision = event.ID, event.ResultingRevision
	return fresh, nil
}

func RewriteTaskDeclaredPath(raw []byte, id, added string) ([]byte, error) {
	lines := bytes.SplitAfter(raw, []byte{'\n'})
	fileIndex := -1
	changed := false
	out := make([]byte, 0, len(raw)+len(added)+2)
	for _, line := range lines {
		cells := parsePipeRow(strings.TrimRight(string(line), "\n"))
		if cells != nil && fileIndex < 0 {
			for i, cell := range cells {
				if strings.EqualFold(strings.TrimSpace(cell), "files") {
					fileIndex = i
				}
			}
		}
		if cells == nil || fileIndex < 0 || fileIndex >= len(cells) {
			out = append(out, line...)
			continue
		}
		_, taskID := splitMarkedTaskID(cells[0])
		if taskID != id {
			out = append(out, line...)
			continue
		}
		if changed {
			return nil, fmt.Errorf("duplicate task id %q", id)
		}
		files, err := normalizeDeclaredFiles(cells[fileIndex])
		if err != nil {
			return nil, err
		}
		cells[fileIndex] = strings.Join(corescope.Amend(files, []string{added}), ", ")
		newLine := "| " + strings.Join(cells, " | ") + " |"
		if bytes.HasSuffix(line, []byte{'\n'}) {
			newLine += "\n"
		}
		out = append(out, newLine...)
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return out, nil
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

// Artifact and spec reopen transitions (R4.1, R4.2). The artifact name or the
// slug follows the prefix, so a ledger scan reads as `reopen.artifact.design`.
const (
	ReopenArtifactTransitionPrefix = "reopen.artifact."
	ReopenSpecTransitionPrefix     = "reopen.spec."
	ReopenArtifactEntityKind       = "artifact"
	ReopenSpecEntityKind           = "spec"
)

// Impacted-entity prefixes of an artifact or spec reopen event: the artifact
// revisions the transaction opened and the approval requests it invalidated.
const (
	ReopenRevisionEntityPrefix = "revision:"
	ReopenApprovalEntityPrefix = "approval_request:"
)

// CurrentArtifactVersion projects an artifact's current draft version from the
// ledger. An artifact that was never reopened is version 1; every reopen event
// names the version it opened in its impacted entities, so the ledger is the
// only counter (R4.1).
func CurrentArtifactVersion(events []WorkflowEventV1, artifact string) int {
	version := 1
	prefix := ReopenRevisionEntityPrefix + artifact + "@"
	for _, event := range events {
		for _, entity := range event.ImpactedEntities {
			if !strings.HasPrefix(entity, prefix) {
				continue
			}
			if n, err := strconv.Atoi(strings.TrimPrefix(entity, prefix)); err == nil && n > version {
				version = n
			}
		}
	}
	return version
}

// ArtifactVersions is CurrentArtifactVersion for every reopenable artifact that
// has actually been reopened; unlisted artifacts are on version 1.
func ArtifactVersions(events []WorkflowEventV1) map[string]int {
	versions := map[string]int{}
	for _, artifact := range ReopenableArtifacts {
		if version := CurrentArtifactVersion(events, artifact); version > 1 {
			versions[artifact] = version
		}
	}
	return versions
}

// ArtifactReopenRequest is the caller-resolved artifact or spec reopen intent.
// An empty Artifact reopens the whole spec into a new lifecycle cycle (R4.2).
type ArtifactReopenRequest struct {
	Artifact         string
	ExpectedRevision int64
	Reason           string
	ActorID          string
	GitHead          string
	// Digests are the current artifact bytes digests the caller read, keyed by
	// artifact name. Planning stays pure; commit re-snapshots and refuses when
	// the bytes moved since the preview.
	Digests map[string]string
	// Consumptions are the durable records that already consumed this work.
	// External ones (release, deployment, archive) make in-place reopen
	// forbidden and successor-only (R4.3).
	Consumptions []ImpactConsumption
}

// ArtifactRevision is one artifact's move from its prior revision to a fresh
// draft version with its own identity (R4.1).
type ArtifactRevision struct {
	Artifact     string `json:"artifact"`
	PriorVersion int    `json:"prior_version"`
	Version      int    `json:"version"`
	PriorDigest  string `json:"prior_digest"`
	VersionID    string `json:"version_id"`
	SnapshotPath string `json:"snapshot_path"`
}

// ArtifactReopenPlan is the deterministic preview of one artifact or spec
// reopen, and the receipt CommitArtifactReopen returns.
type ArtifactReopenPlan struct {
	SchemaVersion        string              `json:"schema_version"`
	Eligible             bool                `json:"eligible"`
	Slug                 string              `json:"slug"`
	Kind                 string              `json:"kind"`
	Artifact             string              `json:"artifact,omitempty"`
	PriorCycle           int                 `json:"prior_cycle"`
	Cycle                int                 `json:"cycle"`
	Revisions            []ArtifactRevision  `json:"revisions"`
	InvalidatedApprovals []string            `json:"invalidated_approvals"`
	CurrentRevision      int64               `json:"current_revision"`
	NewRevision          int64               `json:"new_revision"`
	EventID              string              `json:"event_id,omitempty"`
	SuccessorRoute       string              `json:"successor_route,omitempty"`
	ImpactDigest         string              `json:"impact_digest,omitempty"`
	Impact               ImpactPlan          `json:"impact"`
	Blockers             []TransitionBlocker `json:"blockers"`
}

// PlanArtifactReopen is pure: it classifies the request against the caller's
// state and ledger snapshot and returns every blocker instead of raising the
// first one.
func PlanArtifactReopen(slug string, req ArtifactReopenRequest, state State, events []WorkflowEventV1) ArtifactReopenPlan {
	cycle := state.Cycle
	if cycle < 1 {
		cycle = 1
	}
	plan := ArtifactReopenPlan{
		SchemaVersion:        ReopenPlanSchemaVersion,
		Slug:                 slug,
		Kind:                 ReopenSpecEntityKind,
		Artifact:             req.Artifact,
		PriorCycle:           cycle,
		Cycle:                cycle + 1,
		Revisions:            []ArtifactRevision{},
		InvalidatedApprovals: []string{},
		CurrentRevision:      state.Revision,
		NewRevision:          state.Revision + 1,
		Blockers:             []TransitionBlocker{},
	}
	targets := ReopenableArtifacts
	if req.Artifact != "" {
		// An artifact reopen stays inside the current cycle: only the spec
		// lifecycle itself starts a new one (R4.2).
		plan.Kind, plan.Cycle, targets = ReopenArtifactEntityKind, cycle, []string{req.Artifact}
	}
	if strings.TrimSpace(req.Reason) == "" {
		plan.addBlocker("REOPEN_REASON_REQUIRED", "reason", "reopen requires a reason; re-run with --reason <text>")
	}
	if strings.TrimSpace(req.ActorID) == "" {
		plan.addBlocker("REOPEN_ACTOR_REQUIRED", "actor", "reopen requires an accountable actor; set SPECD_ACTOR")
	}
	if req.ExpectedRevision != state.Revision {
		plan.addBlocker("REOPEN_REVISION_STALE", plan.target(), fmt.Sprintf(
			"expected state revision %d but current is %d; re-preview and re-run with --expect-revision %d",
			req.ExpectedRevision, state.Revision, state.Revision))
	}
	if req.Artifact != "" && !ReopenableArtifact(req.Artifact) {
		plan.addBlocker("REOPEN_ARTIFACT_UNKNOWN", req.Artifact, fmt.Sprintf(
			"%q is not a spec artifact; reopen one of %s", req.Artifact, strings.Join(ReopenableArtifacts, ", ")))
		targets = nil
	}

	for _, artifact := range targets {
		digest := req.Digests[artifact]
		if !hexDigest(digest) {
			plan.addBlocker("REOPEN_ARTIFACT_UNREADABLE", artifact, fmt.Sprintf(
				"cannot read the current bytes of %s.md; a revision cannot be preserved for an artifact that is missing or unreadable", artifact))
			continue
		}
		version := CurrentArtifactVersion(events, artifact) + 1
		plan.Revisions = append(plan.Revisions, ArtifactRevision{
			Artifact:     artifact,
			PriorVersion: version - 1,
			Version:      version,
			PriorDigest:  digest,
			SnapshotPath: RevisionSnapshotRelPath(artifact, digest),
			// The new draft's identity is minted here: it is distinct from the
			// preserved revision even while the bytes are still identical.
			VersionID: Digest([]byte(strings.Join([]string{
				slug, artifact, strconv.Itoa(version), digest, req.ActorID, req.Reason,
			}, "|"))),
		})
	}

	requests, err := state.ApprovalRequests()
	if err != nil {
		plan.addBlocker("REOPEN_APPROVALS_INVALID", plan.target(), fmt.Sprintf("cannot project approval requests: %v", err))
	}
	plan.InvalidatedApprovals = invalidatedApprovals(requests, req.Artifact)

	plan.Impact = buildArtifactReopenImpact(slug, req, state, events)
	plan.ImpactDigest = plan.Impact.ImpactDigest
	for _, blocker := range plan.Impact.Blockers {
		plan.addBlocker(blocker.Code, blocker.Entity, blocker.Message)
	}
	for _, entity := range plan.Impact.Entities {
		if entity.Classification != ImpactForbidden {
			continue
		}
		plan.SuccessorRoute = entity.SuccessorRoute
		plan.addBlocker("REOPEN_CONSUMED_EXTERNALLY", entity.ID, fmt.Sprintf("%s; %s", entity.Reason, entity.SuccessorRoute))
	}
	for _, consumption := range plan.Impact.Consumptions {
		if consumption.External {
			continue
		}
		plan.addBlocker("REOPEN_CONSUMED", plan.target(), fmt.Sprintf(
			"%s record %s consumed this work; withdraw or revoke it before reopening, or link a successor instead",
			consumption.Kind, consumption.Record))
	}

	plan.Eligible = len(plan.Blockers) == 0
	return plan
}

// target is the entity a blocker is reported against: the artifact for an
// artifact reopen, the spec itself otherwise.
func (p ArtifactReopenPlan) target() string {
	if p.Artifact != "" {
		return p.Artifact
	}
	return p.Slug
}

// Command is the exact invocation that produced (or re-previews) this plan.
func (p ArtifactReopenPlan) Command() string {
	if p.Artifact != "" {
		return fmt.Sprintf("specd reopen %s artifact %s --reason <text> --expect-revision %d", p.Slug, p.Artifact, p.CurrentRevision)
	}
	return fmt.Sprintf("specd reopen %s spec --reason <text> --expect-revision %d", p.Slug, p.CurrentRevision)
}

// Refusal renders the plan's first blocker as the operator-actionable refusal.
func (p ArtifactReopenPlan) Refusal() error {
	if len(p.Blockers) == 0 {
		return nil
	}
	blocker := p.Blockers[0]
	return Refusef(blocker.Code, "%s", blocker.Message).
		WithRecovery(RefusalActorOperator, p.Command()).
		WithContext(blocker.Entity, "reopen target", "eligible unreleased artifact or spec")
}

// invalidatedApprovals names every still-open approval request the reopen
// invalidates: the requests for that artifact's gate, or — when the whole spec
// starts a new cycle — every open request (R4.1, R4.2).
func invalidatedApprovals(requests []ApprovalRequestRecord, artifact string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, rec := range requests {
		if seen[rec.ID] || !ApprovalRequestPending(requests, rec.ID) {
			continue
		}
		if artifact != "" && rec.EntityVersion != artifact && !(rec.EntityKind == ApprovalEntityArtifact && rec.EntityID == artifact) {
			continue
		}
		seen[rec.ID] = true
		ids = append(ids, rec.ID)
	}
	sort.Strings(ids)
	return ids
}

// buildArtifactReopenImpact previews the reopened artifact plus the artifacts
// derived from it — design depends on requirements, tasks on design — so a
// requirements reopen names the downstream drafts it makes stale through the
// same shared preview undo and task reopen use (R1, R4.1).
func buildArtifactReopenImpact(slug string, req ArtifactReopenRequest, state State, events []WorkflowEventV1) ImpactPlan {
	input := ImpactInput{
		Operation:             ImpactOperationReopen,
		RequestedKind:         ReopenSpecEntityKind,
		RequestedID:           slug,
		RequestedSpec:         slug,
		RequestedVersion:      strconv.Itoa(state.Cycle),
		ExpectedStateRevision: req.ExpectedRevision,
		Actor:                 ActorOperator,
		ActorID:               req.ActorID,
	}
	if req.Artifact == "" {
		input.Candidates = []ImpactCandidate{{
			Kind: ReopenSpecEntityKind, ID: slug, Spec: slug,
			Version: strconv.Itoa(state.Cycle), State: string(state.Stage),
			Consumptions: req.Consumptions, SnapshotRequired: true,
		}}
		return BuildImpactPlan(input)
	}
	input.RequestedKind = ReopenArtifactEntityKind
	input.RequestedID = req.Artifact
	input.RequestedVersion = strconv.Itoa(CurrentArtifactVersion(events, req.Artifact))
	prior := ""
	for _, artifact := range ReopenableArtifacts {
		candidate := ImpactCandidate{
			Kind: ReopenArtifactEntityKind, ID: artifact, Spec: slug,
			Version: strconv.Itoa(CurrentArtifactVersion(events, artifact)),
			State:   string(state.Stage),
		}
		if prior != "" {
			candidate.DependsOn = []string{ImpactRef(ReopenArtifactEntityKind, slug, prior)}
		}
		if artifact == req.Artifact {
			candidate.Consumptions = req.Consumptions
			candidate.SnapshotRequired = true
		}
		input.Candidates = append(input.Candidates, candidate)
		prior = artifact
	}
	return BuildImpactPlan(input)
}

func (p *ArtifactReopenPlan) addBlocker(code, entity, message string) {
	blocker := TransitionBlocker{Code: code, Gate: "reopen", Entity: entity, Message: message}
	for _, existing := range p.Blockers {
		if existing == blocker {
			return
		}
	}
	p.Blockers = append(p.Blockers, blocker)
	sort.SliceStable(p.Blockers, func(i, j int) bool { return p.Blockers[i].Code < p.Blockers[j].Code })
}

// ArtifactReopenProjection is the state an eligible plan commits: the new draft
// stage (or the new cycle), with every invalidated approval request closed by an
// appended expiry transition. Nothing is deleted — the prior cycle stays fully
// reportable in its records, events, and revision snapshots (R4.2).
func ArtifactReopenProjection(plan ArtifactReopenPlan, req ArtifactReopenRequest, state State) (State, error) {
	next := state
	next.Records = maps.Clone(state.Records)
	next.TaskStatus = maps.Clone(state.TaskStatus)
	if next.Records == nil {
		next.Records = map[string]json.RawMessage{}
	}
	requests, err := ReadApprovalRequests(next.Records)
	if err != nil {
		return State{}, err
	}
	for _, id := range plan.InvalidatedApprovals {
		rec := ApprovalRequestRecord{ID: id, Transition: ApprovalExpired, Reason: "invalidated by " + plan.Transition()}
		rec = StampApprovalRequest(rec, req.GitHead)
		key, planned, err := PlanApprovalRequest(requests, rec)
		if err != nil {
			return State{}, err
		}
		raw, err := json.Marshal(planned)
		if err != nil {
			return State{}, err
		}
		next.Records[key] = raw
		requests = append(requests, planned)
	}
	next.CurrentRequest = ""
	next.Cycle = plan.Cycle
	next.Stage = StageRequirements
	if plan.Artifact != "" {
		next.Stage = Stage(plan.Artifact)
	}
	next.Condition = ConditionActive
	next.Status = ProjectStatus(StageCondition{Stage: next.Stage, Condition: next.Condition})
	next.Phase = PhaseForStatus(next.Status)
	if plan.Artifact == "" {
		for _, gate := range []Status{
			StatusRequirements, StatusDesign, StatusTasks,
			StatusExecuting, StatusVerifying, StatusComplete,
		} {
			key := "approval:" + string(gate)
			if raw, ok := next.Records[key]; ok {
				next.Records[fmt.Sprintf("%s:cycle:%d", key, plan.PriorCycle)] = raw
				delete(next.Records, key)
			}
		}
		for id := range next.TaskStatus {
			next.TaskStatus[id] = TaskPending
		}
	}
	return next, next.Validate()
}

// Transition is the ledger transition name this reopen appends.
func (p ArtifactReopenPlan) Transition() string {
	if p.Artifact != "" {
		return ReopenArtifactTransitionPrefix + p.Artifact
	}
	return ReopenSpecTransitionPrefix + p.Slug
}

// BuildArtifactReopenEvent returns the append-only event that opens the new
// draft version or lifecycle cycle. The prior version stays in the ledger and is
// linked by BeforeEntityVersion (R4.1, R4.2).
func BuildArtifactReopenEvent(plan ArtifactReopenPlan, req ArtifactReopenRequest, state State) (WorkflowEventV1, error) {
	if !plan.Eligible {
		return WorkflowEventV1{}, plan.Refusal()
	}
	projection, err := ArtifactReopenProjection(plan, req, state)
	if err != nil {
		return WorkflowEventV1{}, err
	}
	inputs := map[string]string{"impact_plan": plan.ImpactDigest}
	impacted := make([]string, 0, len(plan.Revisions)+len(plan.InvalidatedApprovals))
	for _, revision := range plan.Revisions {
		inputs["revision."+revision.Artifact] = revision.PriorDigest
		impacted = append(impacted, fmt.Sprintf("%s%s@%d", ReopenRevisionEntityPrefix, revision.Artifact, revision.Version))
	}
	for _, id := range plan.InvalidatedApprovals {
		impacted = append(impacted, ReopenApprovalEntityPrefix+id)
	}
	entityKind, entityID := ReopenSpecEntityKind, plan.Slug
	before, after := int64(plan.PriorCycle), int64(plan.Cycle)
	if plan.Artifact != "" {
		entityKind, entityID = ReopenArtifactEntityKind, plan.Artifact
		before, after = int64(plan.Revisions[0].PriorVersion), int64(plan.Revisions[0].Version)
	}
	return NewWorkflowEvent(WorkflowEventV1{
		EntityKind:          entityKind,
		EntityID:            entityID,
		BeforeEntityVersion: before,
		AfterEntityVersion:  after,
		ExpectedRevision:    plan.CurrentRevision,
		Transition:          plan.Transition(),
		Actor:               req.ActorID,
		AuthorityDigest: Digest([]byte(strings.Join([]string{
			"reopen", req.ActorID, plan.Transition(), strconv.Itoa(plan.Cycle), req.Reason,
		}, "|"))),
		Reason:           req.Reason,
		InputDigests:     inputs,
		ImpactedEntities: sortedTransitionSet(impacted),
		GitHead:          req.GitHead,
		Timestamp:        Clock().Format(time.RFC3339),
		Projection:       projection,
	})
}

// CommitArtifactReopen reloads durable state, re-plans against it, refuses on
// drift from the preview, preserves every artifact revision as a snapshot, and
// only then appends the transition. A snapshot failure mutates nothing: no
// event is appended and no state is written (R4.4).
func CommitArtifactReopen(root, slug string, req ArtifactReopenRequest, preview ArtifactReopenPlan) (ArtifactReopenPlan, error) {
	statePath, eventPath := StatePath(root, slug), WorkflowEventPath(root, slug)
	state, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil {
		return ArtifactReopenPlan{}, err
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return ArtifactReopenPlan{}, err
	}
	fresh := PlanArtifactReopen(slug, req, state, events)
	if preview.ImpactDigest != "" {
		if err := GuardImpactCommit(preview.Impact, state.Revision, fresh.Impact); err != nil {
			return fresh, err
		}
	}
	if !fresh.Eligible {
		return fresh, fresh.Refusal()
	}
	for _, revision := range fresh.Revisions {
		snapshot, err := SnapshotArtifactRevision(root, slug, revision.Artifact)
		if err != nil {
			return fresh, err
		}
		if snapshot.Digest != revision.PriorDigest {
			return fresh, Refusef("REOPEN_ARTIFACT_MOVED",
				"%s.md changed from %s to %s after the preview; re-run %s",
				revision.Artifact, revision.PriorDigest, snapshot.Digest, fresh.Command())
		}
	}
	var tasksPath string
	var taskIDs []string
	if fresh.Artifact == "" {
		var pathErr error
		tasksPath, pathErr = SpecArtifactPath(root, slug, "tasks")
		if pathErr != nil {
			return fresh, pathErr
		}
		raw, readErr := os.ReadFile(tasksPath)
		if readErr != nil {
			return fresh, readErr
		}
		doc, parseErr := ParseTasksMd(raw)
		if parseErr != nil {
			return fresh, parseErr
		}
		state.TaskStatus = maps.Clone(state.TaskStatus)
		if state.TaskStatus == nil {
			state.TaskStatus = map[string]TaskRunStatus{}
		}
		for _, task := range doc.Tasks {
			taskIDs = append(taskIDs, task.ID)
			state.TaskStatus[task.ID] = TaskPending
		}
	}
	event, err := BuildArtifactReopenEvent(fresh, req, state)
	if err != nil {
		return fresh, err
	}
	commit := TransitionCommit{StatePath: statePath, EventPath: eventPath, Event: event}
	if fresh.Artifact == "" {
		event, err = commitReopenMarkers(tasksPath, taskIDs, commit)
	} else {
		err = CommitWorkflowTransition(commit)
	}
	if err != nil {
		return fresh, err
	}
	fresh.EventID = event.ID
	fresh.NewRevision = event.ResultingRevision
	return fresh, nil
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
	projection.TaskStatus = maps.Clone(projection.TaskStatus)
	if projection.TaskStatus == nil {
		projection.TaskStatus = map[string]TaskRunStatus{}
	}
	projection.TaskStatus[plan.TaskID] = TaskPending
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
func CommitTaskReopen(tasksPath, statePath, eventPath, slug string, req ReopenRequest, tasks []TaskRow, status map[string]TaskRunStatus, preview ReopenPlan) (ReopenPlan, error) {
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
	event, err = commitReopenMarkers(tasksPath, []string{fresh.TaskID}, TransitionCommit{StatePath: statePath, EventPath: eventPath, Event: event})
	if err != nil {
		return fresh, err
	}
	fresh.EventID = event.ID
	fresh.NewRevision = event.ResultingRevision
	return fresh, nil
}

// commitReopenMarkers applies the byte-stable pending markers and the
// event/state CAS as one locked command transaction. A refused commit restores
// the original task bytes.
func commitReopenMarkers(tasksPath string, taskIDs []string, commit TransitionCommit) (WorkflowEventV1, error) {
	original, err := os.ReadFile(tasksPath)
	if err != nil {
		return WorkflowEventV1{}, err
	}
	updated := original
	for _, id := range taskIDs {
		updated, err = RewriteTaskStatusLine(updated, id, "")
		if err != nil {
			return WorkflowEventV1{}, err
		}
	}
	if string(updated) != string(original) {
		commit.Artifact = &TransitionArtifact{Path: tasksPath, Before: string(original), After: string(updated)}
		commit.Event, err = bindTransitionArtifact(commit.Event, *commit.Artifact)
		if err != nil {
			return WorkflowEventV1{}, err
		}
	}
	return commit.Event, CommitWorkflowTransition(commit)
}
