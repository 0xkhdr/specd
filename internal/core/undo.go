package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const UndoPlanSchemaVersion = "1"

// UndoCompensationPrefix marks the transition name of a compensation event. It
// is also an irreversible prefix: a compensation is itself repair history and
// is never undone in place (R2.2).
const UndoCompensationPrefix = "undo.compensate."

// undoIrreversiblePrefixes are transition-name prefixes that can never be
// compensated in place. Externally observable work (submission, release,
// deployment, archive), consumed completion, the schema baseline, and prior
// compensations are successor-only.
var undoIrreversiblePrefixes = []string{
	"archive", "complete", "deploy", "release", "state.migrate", "submit", "undo.",
}

// ReversibleWorkflowTransition reports whether a transition may be compensated.
func ReversibleWorkflowTransition(transition string) bool {
	name := strings.ToLower(strings.TrimSpace(transition))
	if name == "" {
		return false
	}
	for _, prefix := range undoIrreversiblePrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

// UndoGuard is one consumption check evaluated against the undo target. Every
// declared guard is reported whether it passed or failed, so the compensation
// event records which guards were checked (R6.1).
type UndoGuard struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// undoGuardClasses maps a consumption kind to the guard it fails. Anything
// unrecognised falls to consumption.other and still blocks: unknown consumption
// is never assumed harmless.
var undoGuardClasses = []struct {
	Name  string
	Kinds []string
}{
	{"consumption.evidence", []string{"completion", "evidence", "verification"}},
	{"consumption.external", []string{"adapter", "archive", "deployment", "release", "submission"}},
	{"consumption.delegation", []string{"delegation", "lease", "mission"}},
	{"consumption.other", nil},
}

func undoGuardFor(consumption ImpactConsumption) string {
	if consumption.External {
		return "consumption.external"
	}
	kind := strings.ToLower(strings.TrimSpace(consumption.Kind))
	for _, class := range undoGuardClasses {
		for _, known := range class.Kinds {
			if kind == known {
				return class.Name
			}
		}
	}
	return "consumption.other"
}

// UndoRequest is the caller-resolved undo intent. TargetEventID always names
// the latest ledger event: there is no arbitrary event-id route, the field
// exists so the not-latest refusal is provable rather than assumed.
type UndoRequest struct {
	TargetEventID    string
	ExpectedRevision int64
	Reason           string
	Consumptions     []ImpactConsumption
}

// UndoPlan is the deterministic eligibility preview for one undo. It is also
// the commit receipt: CommitUndo returns the plan it actually committed.
type UndoPlan struct {
	SchemaVersion    string              `json:"schema_version"`
	Eligible         bool                `json:"eligible"`
	Slug             string              `json:"slug"`
	TargetEventID    string              `json:"target_event_id"`
	TargetTransition string              `json:"target_transition,omitempty"`
	PriorRevision    int64               `json:"prior_revision"`
	CurrentRevision  int64               `json:"current_revision"`
	NewRevision      int64               `json:"new_revision"`
	CompensationID   string              `json:"compensation_event_id,omitempty"`
	Guards           []UndoGuard         `json:"guards"`
	ImpactedEntities []string            `json:"impacted_entities"`
	ImpactDigest     string              `json:"impact_digest,omitempty"`
	Impact           ImpactPlan          `json:"impact"`
	Blockers         []TransitionBlocker `json:"blockers"`
}

// PlanUndo is pure: it classifies the requested target against the caller's
// ledger snapshot and returns every blocker instead of raising the first one.
func PlanUndo(req UndoRequest, events []WorkflowEventV1, currentRevision int64) UndoPlan {
	plan := UndoPlan{
		SchemaVersion:    UndoPlanSchemaVersion,
		TargetEventID:    req.TargetEventID,
		CurrentRevision:  currentRevision,
		NewRevision:      currentRevision + 1,
		Guards:           []UndoGuard{},
		ImpactedEntities: []string{},
		Blockers:         []TransitionBlocker{},
	}
	if strings.TrimSpace(req.Reason) == "" {
		plan.addBlocker("UNDO_REASON_REQUIRED", "reason", "undo requires a reason; re-run with --reason <text>")
	}
	if len(events) == 0 {
		plan.addBlocker("UNDO_LEDGER_EMPTY", "ledger", "this spec has no workflow events, so there is nothing to compensate")
		return plan
	}

	index := -1
	for i, event := range events {
		if event.ID == req.TargetEventID {
			index = i
			break
		}
	}
	if index < 0 {
		plan.addBlocker("UNDO_TARGET_UNKNOWN", req.TargetEventID, fmt.Sprintf("event %q is not in this spec's workflow ledger", req.TargetEventID))
		return plan
	}
	target := events[index]
	plan.Slug = target.Projection.Slug
	plan.TargetTransition = target.Transition
	plan.PriorRevision = target.ExpectedRevision
	plan.ImpactedEntities = sortedTransitionSet(append([]string{"workflow_event:" + target.ID}, target.ImpactedEntities...))

	// No child event: only the latest event is compensable, and a later event
	// is itself proof that something consumed the target (R2.2).
	child := ""
	if index != len(events)-1 {
		child = events[index+1].ID
	}
	childDetail := ""
	if child != "" {
		childDetail = "later event " + child
	}
	plan.Guards = append(plan.Guards, UndoGuard{Name: "child-event", Passed: child == "", Detail: childDetail})
	if child != "" {
		plan.addBlocker("UNDO_NOT_LATEST", target.ID, fmt.Sprintf(
			"event %q is not the latest workflow event: %q already followed it; undo only compensates the latest event, reopen the affected work instead",
			target.ID, child))
	}

	reversible := ReversibleWorkflowTransition(target.Transition)
	plan.Guards = append(plan.Guards, UndoGuard{Name: "reversible-transition", Passed: reversible, Detail: target.Transition})
	if !reversible {
		plan.addBlocker("UNDO_IRREVERSIBLE", target.ID, fmt.Sprintf(
			"transition %q is not reversible in place; link a successor instead of compensating it", target.Transition))
	}

	// The compensation projects the predecessor's projection, so a baseline
	// event with no predecessor has no prior effective state to restore.
	priorState := index > 0
	plan.Guards = append(plan.Guards, UndoGuard{Name: "prior-projection", Passed: priorState, Detail: "predecessor event required"})
	if !priorState {
		plan.addBlocker("UNDO_NO_PRIOR_STATE", target.ID, fmt.Sprintf(
			"event %q is the ledger baseline; there is no prior projection to restore", target.ID))
	}

	fresh := req.ExpectedRevision == currentRevision
	plan.Guards = append(plan.Guards, UndoGuard{Name: "state-revision", Passed: fresh, Detail: fmt.Sprintf("expected %d, current %d", req.ExpectedRevision, currentRevision)})
	if !fresh {
		plan.addBlocker("UNDO_REVISION_STALE", target.ID, fmt.Sprintf(
			"expected state revision %d but current is %d; re-preview and re-run with --expect-revision %d",
			req.ExpectedRevision, currentRevision, currentRevision))
	}

	plan.appendConsumptionGuards(req.Consumptions, target)

	plan.Impact = BuildImpactPlan(ImpactInput{
		Operation:             ImpactOperationUndo,
		RequestedKind:         "workflow_event",
		RequestedID:           target.ID,
		RequestedSpec:         plan.Slug,
		RequestedVersion:      fmt.Sprintf("%d", target.AfterEntityVersion),
		ExpectedStateRevision: req.ExpectedRevision,
		Actor:                 ActorOperator,
		ActorID:               target.Actor,
		Candidates: []ImpactCandidate{{
			Kind: "workflow_event", ID: target.ID, Spec: plan.Slug,
			Version: fmt.Sprintf("%d", target.AfterEntityVersion),
			State:   target.Transition, Consumptions: req.Consumptions,
		}},
	})
	plan.ImpactDigest = plan.Impact.ImpactDigest
	for _, blocker := range plan.Impact.Blockers {
		plan.addBlocker(blocker.Code, blocker.Entity, blocker.Message)
	}

	plan.Eligible = len(plan.Blockers) == 0
	return plan
}

// appendConsumptionGuards reports every declared consumption class, so the
// record proves which guards ran rather than only which ones failed.
func (p *UndoPlan) appendConsumptionGuards(consumptions []ImpactConsumption, target WorkflowEventV1) {
	failed := map[string][]string{}
	for _, consumption := range consumptions {
		guard := undoGuardFor(consumption)
		failed[guard] = append(failed[guard], consumption.Kind+" "+consumption.Record)
	}
	for _, class := range undoGuardClasses {
		records := sortedTransitionSet(failed[class.Name])
		guard := UndoGuard{Name: class.Name, Passed: len(records) == 0, Detail: strings.Join(records, ", ")}
		p.Guards = append(p.Guards, guard)
		if guard.Passed {
			continue
		}
		if class.Name == "consumption.external" {
			p.addBlocker("UNDO_CONSUMED_EXTERNALLY", target.ID, fmt.Sprintf(
				"immutable external record(s) %s consumed event %q; in-place undo is forbidden, link a successor spec instead",
				guard.Detail, target.ID))
			continue
		}
		p.addBlocker("UNDO_CONSUMED", target.ID, fmt.Sprintf(
			"record(s) %s consumed event %q; resolve or reopen that work before compensating the event", guard.Detail, target.ID))
	}
}

func (p *UndoPlan) addBlocker(code, entity, message string) {
	blocker := TransitionBlocker{Code: code, Gate: "undo", Entity: entity, Message: message}
	for _, existing := range p.Blockers {
		if existing == blocker {
			return
		}
	}
	p.Blockers = append(p.Blockers, blocker)
	sort.SliceStable(p.Blockers, func(i, j int) bool { return p.Blockers[i].Code < p.Blockers[j].Code })
}

// Refusal renders the plan's first blocker as the operator-actionable refusal.
func (p UndoPlan) Refusal(slug string) error {
	if len(p.Blockers) == 0 {
		return nil
	}
	blocker := p.Blockers[0]
	return Refusef(blocker.Code, "%s", blocker.Message).
		WithRecovery(RefusalActorOperator, fmt.Sprintf("specd undo %s --reason <text> --expect-revision %d", slug, p.CurrentRevision)).
		WithContext(blocker.Entity, "undo target", "latest unconsumed reversible event")
}

// BuildUndoCompensation returns the append-only compensation event for an
// eligible plan. It never rewrites the target: the target's own projection
// stays in the ledger and the prior effective state is re-projected at a higher
// revision (R2.2, R6.1).
func BuildUndoCompensation(plan UndoPlan, req UndoRequest, events []WorkflowEventV1) (WorkflowEventV1, error) {
	if !plan.Eligible {
		return WorkflowEventV1{}, plan.Refusal(plan.Slug)
	}
	index := -1
	for i, event := range events {
		if event.ID == plan.TargetEventID {
			index = i
			break
		}
	}
	if index < 1 {
		return WorkflowEventV1{}, fmt.Errorf("undo target %q has no predecessor event", plan.TargetEventID)
	}
	target, prior := events[index], events[index-1]

	actor := recordActor()
	inputs := map[string]string{
		"undone_event": target.ID,
		"impact_plan":  plan.ImpactDigest,
	}
	for _, guard := range plan.Guards {
		inputs["guard."+guard.Name] = Digest([]byte(fmt.Sprintf("%s|%t|%s", guard.Name, guard.Passed, guard.Detail)))
	}
	return NewWorkflowEvent(WorkflowEventV1{
		EntityKind:          target.EntityKind,
		EntityID:            target.EntityID,
		BeforeEntityVersion: target.AfterEntityVersion,
		AfterEntityVersion:  target.AfterEntityVersion + 1,
		ExpectedRevision:    plan.CurrentRevision,
		Transition:          UndoCompensationPrefix + target.Transition,
		Actor:               actor,
		AuthorityDigest:     Digest([]byte("undo|" + actor + "|" + target.ID + "|" + req.Reason)),
		Reason:              req.Reason,
		InputDigests:        inputs,
		ImpactedEntities:    plan.ImpactedEntities,
		Timestamp:           Clock().Format(time.RFC3339),
		Projection:          prior.Projection,
	})
}

// CommitUndo reloads durable state (finishing any half-committed transition),
// re-plans against it, refuses on drift from the preview, and appends the
// compensation under the same event-first CAS as any other transition. Nothing
// is ever deleted or decremented; a refusal mutates nothing (R2.1-R2.3, R6.2).
func CommitUndo(statePath, eventPath string, req UndoRequest, preview UndoPlan) (UndoPlan, error) {
	state, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil {
		return UndoPlan{}, err
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return UndoPlan{}, err
	}
	fresh := PlanUndo(req, events, state.Revision)
	if preview.ImpactDigest != "" {
		if err := GuardImpactCommit(preview.Impact, state.Revision, fresh.Impact); err != nil {
			return fresh, err
		}
	}
	if !fresh.Eligible {
		return fresh, fresh.Refusal(fresh.Slug)
	}
	event, err := BuildUndoCompensation(fresh, req, events)
	if err != nil {
		return fresh, err
	}
	if err := CommitWorkflowTransition(TransitionCommit{StatePath: statePath, EventPath: eventPath, Event: event}); err != nil {
		return fresh, err
	}
	fresh.CompensationID = event.ID
	fresh.NewRevision = event.ResultingRevision
	return fresh, nil
}
