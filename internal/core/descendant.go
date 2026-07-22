package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const DescendantPlanSchemaVersion = "1"

// DescendantEntityKind is the workflow-event entity kind a resolution advances.
// It is deliberately not the task kind: a resolution is not a new attempt, and
// CurrentTaskAttempt must keep reading the attempt chain from reopen events
// only (R5.1 — resolving staleness never resets a task).
const DescendantEntityKind = "descendant"

// DescendantResolveTransitionPrefix marks a resolution event; the resolution
// itself follows it, so a ledger scan reads as `resolve.descendant.revalidate`.
const DescendantResolveTransitionPrefix = "resolve.descendant."

// The only ways a stale descendant stops being stale (R5.2). Nothing else
// clears staleness — in particular, no elapsed time, no re-run of the parent,
// and no digest comparison (R5.3).
const (
	DescendantRevalidate = "revalidate"
	DescendantReopen     = "reopen"
	DescendantRetain     = "retain"
	DescendantSupersede  = "supersede"
	DescendantCancel     = "cancel"
)

// DescendantResolutions is the sorted allowed-resolution set, and the same set
// an impact preview offers a stale entity.
var DescendantResolutions = []string{
	DescendantCancel, DescendantReopen, DescendantRetain, DescendantRevalidate, DescendantSupersede,
}

// DescendantCriterionEntityPrefix carries one acceptance-coverage reassignment
// on a resolution event, as `criterion:<id>=<task>`.
const DescendantCriterionEntityPrefix = "criterion:"

// StaleDescendant is one completed task a reopen invalidated. It stays
// `completed + stale` — the tasks.md marker is never rewritten to pending and
// staleness is never cleared implicitly (R5.1).
type StaleDescendant struct {
	TaskID             string   `json:"task_id"`
	Parent             string   `json:"parent_task_id"`
	StaleSinceRevision int64    `json:"stale_since_revision"`
	Resolution         string   `json:"resolution,omitempty"`
	ResolvedRevision   int64    `json:"resolved_revision,omitempty"`
	Successor          string   `json:"successor,omitempty"`
	Resolutions        int      `json:"resolutions,omitempty"`
	Choices            []string `json:"choices,omitempty"`
}

// Unresolved reports whether this descendant still blocks its parent (R5.4).
func (d StaleDescendant) Unresolved() bool { return d.Resolution == "" }

// StaleDescendants projects every descendant a reopen made stale, and the
// resolution each one has (if any), from the append-only ledger. Pure: the same
// events always replay to the same staleness (R6.3).
func StaleDescendants(events []WorkflowEventV1) []StaleDescendant {
	index := map[string]*StaleDescendant{}
	for _, event := range events {
		switch {
		case isTaskReopenEvent(event, event.EntityID):
			// Reopening a stale descendant into a new attempt is itself one of
			// the legal resolutions (R5.2).
			if entry, ok := index[event.EntityID]; ok && entry.Unresolved() {
				entry.Resolution = DescendantReopen
				entry.ResolvedRevision = event.ResultingRevision
				entry.Resolutions++
				entry.Choices = nil
			}
			_, descendants := reopenEntities(event)
			for _, id := range descendants {
				entry, ok := index[id]
				if !ok {
					entry = &StaleDescendant{}
					index[id] = entry
				}
				// A later reopen makes an already-resolved descendant stale
				// again; the resolution count is carried so its event chain
				// keeps advancing by one.
				*entry = StaleDescendant{
					TaskID: id, Parent: event.EntityID,
					StaleSinceRevision: event.ResultingRevision,
					Resolutions:        entry.Resolutions,
					Choices:            DescendantResolutions,
				}
			}
		case event.EntityKind == DescendantEntityKind && strings.HasPrefix(event.Transition, DescendantResolveTransitionPrefix):
			entry, ok := index[event.EntityID]
			if !ok {
				continue
			}
			entry.Resolution = strings.TrimPrefix(event.Transition, DescendantResolveTransitionPrefix)
			entry.ResolvedRevision = event.ResultingRevision
			entry.Resolutions++
			entry.Choices = nil
			for _, impacted := range event.ImpactedEntities {
				if strings.HasPrefix(impacted, ReopenTaskEntityPrefix) {
					entry.Successor = strings.TrimPrefix(impacted, ReopenTaskEntityPrefix)
				}
			}
		}
	}
	stale := make([]StaleDescendant, 0, len(index))
	for _, entry := range index {
		stale = append(stale, *entry)
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].TaskID < stale[j].TaskID })
	return stale
}

// UnresolvedStaleDescendants keeps only the descendants that still need an
// explicit resolution.
func UnresolvedStaleDescendants(stale []StaleDescendant) []StaleDescendant {
	open := make([]StaleDescendant, 0, len(stale))
	for _, entry := range stale {
		if entry.Unresolved() {
			open = append(open, entry)
		}
	}
	return open
}

// StaleDescendantBlockers is the parent-readiness proof (R5.4): a parent is
// blocked while any of its descendants is unresolved, and readiness is stated
// against the revision the staleness was recorded at — never against a digest
// or a prior attempt.
func StaleDescendantBlockers(stale []StaleDescendant) []TransitionBlocker {
	var blockers []TransitionBlocker
	for _, entry := range UnresolvedStaleDescendants(stale) {
		blockers = append(blockers, TransitionBlocker{
			Code: "DESCENDANT_STALE_UNRESOLVED", Gate: "descendant", Entity: entry.Parent,
			Message: fmt.Sprintf("task %s is blocked: completed descendant %s has been stale since revision %d; resolve it with one of %s",
				entry.Parent, entry.TaskID, entry.StaleSinceRevision, strings.Join(DescendantResolutions, ", ")),
		})
	}
	return blockers
}

// DescendantResolutionRequest is the caller-resolved intent. Like every other
// planner in this package it carries no root, clock, or callback: evidence,
// approval, and coverage are resolved by the caller and judged here.
type DescendantResolutionRequest struct {
	TaskID           string
	Resolution       string
	Reason           string
	ActorID          string
	ExpectedRevision int64
	// CurrentHead is the git HEAD readiness is proved against; fresh evidence
	// must be pinned to it (R5.4).
	CurrentHead string
	// Attempt is the descendant's current attempt and Evidence its latest
	// attempt-current verify record.
	Attempt     TaskAttempt
	Evidence    EvidenceRecord
	HasEvidence bool
	// DigestUnchanged is the caller's byte comparison. It may narrow review but
	// never proves behavioural retention on its own (R5.3).
	DigestUnchanged bool
	// ApprovalRef is the approved impact-approval request that authorized a
	// retain; retention needs it *and* fresh evidence.
	ApprovalRef string
	// Successor is the task that takes over a superseded descendant.
	Successor string
	// Criteria are the acceptance criteria this descendant covers today; every
	// one of them needs a reassignment before it is superseded or cancelled.
	Criteria      []string
	Reassignments []CriterionReassignment
}

// DescendantResolutionPlan is the deterministic preview of one resolution, and
// the receipt CommitDescendantResolution returns.
type DescendantResolutionPlan struct {
	SchemaVersion   string                  `json:"schema_version"`
	Eligible        bool                    `json:"eligible"`
	Slug            string                  `json:"slug"`
	TaskID          string                  `json:"task_id"`
	Resolution      string                  `json:"resolution"`
	Parent          string                  `json:"parent_task_id,omitempty"`
	Sequence        int                     `json:"sequence"`
	CurrentRevision int64                   `json:"current_revision"`
	NewRevision     int64                   `json:"new_revision"`
	EventID         string                  `json:"event_id,omitempty"`
	Successor       string                  `json:"successor,omitempty"`
	Reassignments   []CriterionReassignment `json:"reassignments,omitempty"`
	Blockers        []TransitionBlocker     `json:"blockers"`
}

// PlanDescendantResolution is pure and returns every blocker rather than the
// first one. It is the single place the R5.2 resolution matrix is enforced.
func PlanDescendantResolution(slug string, req DescendantResolutionRequest, stale []StaleDescendant, currentRevision int64) DescendantResolutionPlan {
	plan := DescendantResolutionPlan{
		SchemaVersion:   DescendantPlanSchemaVersion,
		Slug:            slug,
		TaskID:          req.TaskID,
		Resolution:      req.Resolution,
		CurrentRevision: currentRevision,
		NewRevision:     currentRevision + 1,
		Reassignments:   req.Reassignments,
		Successor:       req.Successor,
		Blockers:        []TransitionBlocker{},
	}
	if strings.TrimSpace(req.Reason) == "" {
		plan.addBlocker("DESCENDANT_REASON_REQUIRED", req.TaskID, "resolving a stale descendant requires a reason; re-run with --reason <text>")
	}
	if strings.TrimSpace(req.ActorID) == "" {
		plan.addBlocker("DESCENDANT_ACTOR_REQUIRED", req.TaskID, "resolving a stale descendant requires an accountable actor; set SPECD_ACTOR")
	}
	if req.ExpectedRevision != currentRevision {
		plan.addBlocker("DESCENDANT_REVISION_STALE", req.TaskID, fmt.Sprintf(
			"expected state revision %d but current is %d; re-read status and re-run with --expect-revision %d",
			req.ExpectedRevision, currentRevision, currentRevision))
	}

	var entry *StaleDescendant
	for i := range stale {
		if stale[i].TaskID == req.TaskID {
			entry = &stale[i]
		}
	}
	switch {
	case entry == nil:
		plan.addBlocker("DESCENDANT_NOT_STALE", req.TaskID, fmt.Sprintf(
			"task %q is not a stale descendant of any reopen; there is nothing to resolve", req.TaskID))
		return plan
	case !entry.Unresolved():
		plan.addBlocker("DESCENDANT_ALREADY_RESOLVED", req.TaskID, fmt.Sprintf(
			"descendant %s was already resolved as %s at revision %d; reopen its parent again to make it stale",
			req.TaskID, entry.Resolution, entry.ResolvedRevision))
	}
	plan.Parent = entry.Parent
	plan.Sequence = entry.Resolutions

	switch req.Resolution {
	case DescendantRevalidate:
		plan.requireFreshEvidence(req)
	case DescendantRetain:
		if strings.TrimSpace(req.ApprovalRef) == "" {
			plan.addBlocker("DESCENDANT_RETAIN_UNAPPROVED", req.TaskID, fmt.Sprintf(
				"retaining descendant %s needs an approved impact approval request; approve one and retry", req.TaskID))
		}
		plan.requireFreshEvidence(req)
	case DescendantReopen:
		// Reopen is a real attempt transition, not a resolution record: routing
		// it here would mint a resolution without a new attempt behind it.
		plan.addBlocker("DESCENDANT_REOPEN_ROUTE", req.TaskID, fmt.Sprintf(
			"reopening descendant %s is done by the reopen route, which resolves the staleness itself: specd reopen %s task %s --reason <text> --expect-revision %d",
			req.TaskID, slug, req.TaskID, currentRevision))
	case DescendantSupersede:
		if strings.TrimSpace(req.Successor) == "" {
			plan.addBlocker("DESCENDANT_SUCCESSOR_REQUIRED", req.TaskID, fmt.Sprintf(
				"superseding descendant %s requires the task that supersedes it", req.TaskID))
		}
		plan.requireCoverage(req)
	case DescendantCancel:
		plan.requireCoverage(req)
	default:
		plan.addBlocker("DESCENDANT_RESOLUTION_INVALID", req.TaskID, fmt.Sprintf(
			"resolution %q is not one of %s", req.Resolution, strings.Join(DescendantResolutions, ", ")))
	}

	plan.Eligible = len(plan.Blockers) == 0
	return plan
}

// requireFreshEvidence is R5.2/R5.3: revalidation and retention both stand on a
// passing verify record recorded for the descendant's current attempt against
// the current HEAD. Digest equality is reported as the insufficient proof it is
// rather than silently ignored.
func (p *DescendantResolutionPlan) requireFreshEvidence(req DescendantResolutionRequest) {
	if req.HasEvidence && EvidenceProvesCurrent(req.Evidence, req.Attempt, req.CurrentHead) {
		return
	}
	if req.DigestUnchanged {
		p.addBlocker("DESCENDANT_DIGEST_ONLY", req.TaskID, fmt.Sprintf(
			"descendant %s has unchanged digests, which never proves its behaviour unchanged; record fresh passing evidence for attempt %d at HEAD %s: specd verify %s %s",
			req.TaskID, effectiveAttempt(req.Attempt), req.CurrentHead, p.Slug, req.TaskID))
		return
	}
	p.addBlocker("DESCENDANT_EVIDENCE_STALE", req.TaskID, fmt.Sprintf(
		"descendant %s has no passing evidence for attempt %d at HEAD %s; record it with: specd verify %s %s",
		req.TaskID, effectiveAttempt(req.Attempt), req.CurrentHead, p.Slug, req.TaskID))
}

// requireCoverage is R5.2's coverage half: a superseded or cancelled descendant
// must hand every acceptance criterion it covers to a live task, so disposing of
// work never drops acceptance.
func (p *DescendantResolutionPlan) requireCoverage(req DescendantResolutionRequest) {
	if uncovered := UncoveredCriteria(req.Criteria, req.TaskID, req.Reassignments); len(uncovered) > 0 {
		p.addBlocker("DESCENDANT_COVERAGE_UNASSIGNED", req.TaskID, fmt.Sprintf(
			"criterion/criteria %s are covered only by descendant %s; reassign each one to the task that covers it now",
			strings.Join(uncovered, ", "), req.TaskID))
	}
}

func effectiveAttempt(attempt TaskAttempt) int {
	if attempt.Attempt == 0 {
		return 1
	}
	return attempt.Attempt
}

func (p *DescendantResolutionPlan) addBlocker(code, entity, message string) {
	blocker := TransitionBlocker{Code: code, Gate: "descendant", Entity: entity, Message: message}
	for _, existing := range p.Blockers {
		if existing == blocker {
			return
		}
	}
	p.Blockers = append(p.Blockers, blocker)
	sort.SliceStable(p.Blockers, func(i, j int) bool { return p.Blockers[i].Code < p.Blockers[j].Code })
}

// Refusal renders the plan's first blocker as the operator-actionable refusal.
func (p DescendantResolutionPlan) Refusal() error {
	if len(p.Blockers) == 0 {
		return nil
	}
	blocker := p.Blockers[0]
	return Refusef(blocker.Code, "%s", blocker.Message).
		WithRecovery(RefusalActorOperator, fmt.Sprintf(
			"specd reopen %s descendant %s <%s> --reason <text> --expect-revision %d",
			p.Slug, p.TaskID, strings.Join(DescendantResolutions, "|"), p.CurrentRevision)).
		WithContext(blocker.Entity, "stale descendant", "explicitly resolved descendant")
}

// BuildDescendantResolutionEvent returns the append-only record of one
// resolution. Nothing is deleted or rewritten: the staleness stays in the
// ledger and this event is what replay reads it as resolved by (R6.3).
func BuildDescendantResolutionEvent(plan DescendantResolutionPlan, req DescendantResolutionRequest, projection State) (WorkflowEventV1, error) {
	if !plan.Eligible {
		return WorkflowEventV1{}, plan.Refusal()
	}
	impacted := []string{ReopenTaskEntityPrefix + plan.Parent}
	if plan.Successor != "" {
		impacted = []string{ReopenTaskEntityPrefix + plan.Successor}
	}
	for _, reassignment := range plan.Reassignments {
		impacted = append(impacted, DescendantCriterionEntityPrefix+reassignment.Criterion+"="+reassignment.To)
	}
	return NewWorkflowEvent(WorkflowEventV1{
		EntityKind:          DescendantEntityKind,
		EntityID:            plan.TaskID,
		BeforeEntityVersion: int64(plan.Sequence),
		AfterEntityVersion:  int64(plan.Sequence + 1),
		ExpectedRevision:    plan.CurrentRevision,
		Transition:          DescendantResolveTransitionPrefix + plan.Resolution,
		Actor:               req.ActorID,
		AuthorityDigest: Digest([]byte(strings.Join([]string{
			"resolve", req.ActorID, plan.TaskID, plan.Resolution, req.ApprovalRef, req.Reason,
		}, "|"))),
		Reason:           req.Reason,
		ImpactedEntities: sortedTransitionSet(impacted),
		GitHead:          req.CurrentHead,
		Timestamp:        Clock().Format(time.RFC3339),
		Projection:       projection,
	})
}

// CommitDescendantResolution reloads durable state, re-plans against it, and
// appends the resolution under the same event-first CAS as any other
// transition. A refusal mutates nothing.
func CommitDescendantResolution(statePath, eventPath, slug string, req DescendantResolutionRequest, preview DescendantResolutionPlan) (DescendantResolutionPlan, error) {
	state, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil {
		return DescendantResolutionPlan{}, err
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return DescendantResolutionPlan{}, err
	}
	fresh := PlanDescendantResolution(slug, req, StaleDescendants(events), state.Revision)
	if !fresh.Eligible {
		return fresh, fresh.Refusal()
	}
	event, err := BuildDescendantResolutionEvent(fresh, req, state)
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
