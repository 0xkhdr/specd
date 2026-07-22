package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const ImpactPlanSchemaVersion = "1"

// Repair operations previewed by BuildImpactPlan.
const (
	ImpactOperationUndo   = "undo"
	ImpactOperationReopen = "reopen"
)

// Exactly one classification is assigned to every previewed entity (R1.2).
const (
	ImpactCurrent    = "current"
	ImpactStale      = "stale"
	ImpactReopened   = "reopened"
	ImpactRetained   = "retained"
	ImpactSuperseded = "superseded"
	ImpactCancelled  = "cancelled"
	ImpactForbidden  = "forbidden"
)

// Lease dispositions required before a repair transaction may commit.
const (
	ImpactLeaseRelease = "release"
	ImpactLeaseRevoke  = "revoke"
)

// ErrImpactStale reports that a previewed plan no longer describes current
// state, so the repair transaction must refuse and re-preview (R1.4).
var ErrImpactStale = errors.New("impact preview is stale")

// ImpactConsumption names a record that consumed an entity. External marks
// consumption that cannot be repaired in place (release, deployment, archive,
// externally accepted submission).
type ImpactConsumption struct {
	Record   string `json:"record"`
	Kind     string `json:"kind"`
	External bool   `json:"external"`
}

// ImpactCandidate is a caller-resolved entity snapshot. Like TransitionInput it
// carries no root, path, clock, or callback, so planning cannot observe or
// mutate external state.
type ImpactCandidate struct {
	Kind             string
	ID               string
	Spec             string
	Version          string
	State            string
	DependsOn        []string
	Consumptions     []ImpactConsumption
	SnapshotRequired bool
	LeaseHolder      string
	Unavailable      bool
	Retain           bool
	Cancel           bool
	SupersededBy     string
}

// ImpactInput is the complete caller-resolved snapshot a preview plans over.
type ImpactInput struct {
	Operation             string
	RequestedKind         string
	RequestedID           string
	RequestedSpec         string
	RequestedVersion      string
	ExpectedStateRevision int64
	Actor                 OperationActor
	ActorID               string
	Candidates            []ImpactCandidate
}

// ImpactEntity is one classified entity of the preview.
type ImpactEntity struct {
	Kind           string              `json:"kind"`
	ID             string              `json:"id"`
	Spec           string              `json:"spec"`
	Version        string              `json:"version"`
	State          string              `json:"state"`
	CrossSpec      bool                `json:"cross_spec"`
	Classification string              `json:"classification"`
	Reason         string              `json:"reason"`
	Consumptions   []ImpactConsumption `json:"consumptions,omitempty"`
	SuccessorRoute string              `json:"successor_route,omitempty"`
	Snapshot       bool                `json:"snapshot"`
	Choices        []string            `json:"choices,omitempty"`
}

// ImpactLeaseAction names the disposition a live mission/lease needs before the
// repair transaction may commit.
type ImpactLeaseAction struct {
	EntityID string `json:"entity_id"`
	Holder   string `json:"holder"`
	Action   string `json:"action"`
}

type ImpactPlan struct {
	SchemaVersion         string              `json:"schema_version"`
	ImpactDigest          string              `json:"impact_digest"`
	Operation             string              `json:"operation"`
	RequestedKind         string              `json:"requested_kind"`
	RequestedID           string              `json:"requested_id"`
	RequestedSpec         string              `json:"requested_spec"`
	RequestedVersion      string              `json:"requested_version"`
	ExpectedStateRevision int64               `json:"expected_state_revision"`
	Actor                 OperationActor      `json:"actor"`
	ActorID               string              `json:"actor_id"`
	AuthorityRequired     bool                `json:"authority_required"`
	Gates                 []string            `json:"gates"`
	Snapshots             []string            `json:"snapshots"`
	LeaseActions          []ImpactLeaseAction `json:"lease_actions"`
	Entities              []ImpactEntity      `json:"entities"`
	Consumptions          []ImpactConsumption `json:"consumptions"`
	Choices               []string            `json:"choices"`
	Blockers              []TransitionBlocker `json:"blockers"`
}

// ImpactRef is the stable identity of a candidate; DependsOn entries use it.
func ImpactRef(kind, spec, id string) string { return kind + "|" + spec + "|" + id }

// BuildImpactPlan returns a canonical, content-addressed preview of every
// entity reachable from the requested entity (R1.1-R1.3). It is pure: all
// validation failures stay in the plan as blockers.
func BuildImpactPlan(input ImpactInput) ImpactPlan {
	plan := ImpactPlan{
		SchemaVersion:         ImpactPlanSchemaVersion,
		Operation:             input.Operation,
		RequestedKind:         input.RequestedKind,
		RequestedID:           input.RequestedID,
		RequestedSpec:         input.RequestedSpec,
		RequestedVersion:      input.RequestedVersion,
		ExpectedStateRevision: input.ExpectedStateRevision,
		Actor:                 input.Actor,
		ActorID:               input.ActorID,
		Gates:                 []string{"impact"},
		Snapshots:             []string{},
		LeaseActions:          []ImpactLeaseAction{},
		Entities:              []ImpactEntity{},
		Consumptions:          []ImpactConsumption{},
		Choices:               []string{},
		Blockers:              []TransitionBlocker{},
	}

	switch input.Operation {
	case ImpactOperationUndo, ImpactOperationReopen:
	default:
		plan.addBlocker("IMPACT_OPERATION_INVALID", "impact", "operation", fmt.Sprintf("operation %q is not one of undo, reopen", input.Operation))
	}
	if input.RequestedKind == "" || input.RequestedID == "" || input.RequestedSpec == "" {
		plan.addBlocker("IMPACT_REQUEST_INCOMPLETE", "impact", "request", "requested entity kind, id, and owning spec are required")
	}
	if input.ExpectedStateRevision < 0 {
		plan.addBlocker("IMPACT_REVISION_INVALID", "impact", "state", "expected state revision must not be negative")
	}
	if input.Actor == "" {
		plan.addBlocker("IMPACT_ACTOR_REQUIRED", "impact", "actor", "actor class is required")
	}

	candidates := make(map[string]ImpactCandidate, len(input.Candidates))
	for _, candidate := range input.Candidates {
		key := ImpactRef(candidate.Kind, candidate.Spec, candidate.ID)
		if _, seen := candidates[key]; seen {
			plan.addBlocker("IMPACT_CANDIDATE_DUPLICATE", "impact", candidate.ID, fmt.Sprintf("candidate %q is declared more than once", key))
			continue
		}
		candidates[key] = candidate
	}

	rootKey := ImpactRef(input.RequestedKind, input.RequestedSpec, input.RequestedID)
	if _, ok := candidates[rootKey]; !ok {
		plan.addBlocker("IMPACT_REQUEST_UNKNOWN", "impact", input.RequestedID, fmt.Sprintf("requested entity %q is not among the supplied candidates", rootKey))
	}

	reachable, cyclic := impactReach(&plan, candidates, rootKey)

	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	consumed := map[ImpactConsumption]bool{}
	for _, key := range keys {
		entity := classifyImpact(candidates[key], key == rootKey, reachable[key], cyclic[key], input)
		plan.Entities = append(plan.Entities, entity)
		for _, consumption := range entity.Consumptions {
			if !consumed[consumption] {
				consumed[consumption] = true
				plan.Consumptions = append(plan.Consumptions, consumption)
			}
		}
		if entity.Snapshot {
			plan.Snapshots = append(plan.Snapshots, entity.ID)
		}
		if holder := candidates[key].LeaseHolder; holder != "" && entity.Classification != ImpactCurrent {
			action := ImpactLeaseRevoke
			if holder == input.ActorID {
				action = ImpactLeaseRelease
			} else {
				plan.AuthorityRequired = true
			}
			plan.LeaseActions = append(plan.LeaseActions, ImpactLeaseAction{EntityID: entity.ID, Holder: holder, Action: action})
		}
		switch entity.Classification {
		case ImpactStale:
			plan.Gates = append(plan.Gates, "evidence")
			plan.Choices = append(plan.Choices, entity.Choices...)
		case ImpactRetained:
			plan.Gates = append(plan.Gates, "approval", "evidence")
			plan.AuthorityRequired = true
		case ImpactForbidden:
			plan.AuthorityRequired = true
		}
	}
	if len(plan.Snapshots) > 0 {
		plan.Gates = append(plan.Gates, "snapshot")
	}
	if len(plan.LeaseActions) > 0 {
		plan.Gates = append(plan.Gates, "lease")
	}

	plan.Gates = sortedTransitionSet(plan.Gates)
	plan.Choices = sortedTransitionSet(plan.Choices)
	sort.Slice(plan.Consumptions, func(i, j int) bool {
		a, b := plan.Consumptions[i], plan.Consumptions[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Record < b.Record
	})
	sort.Slice(plan.LeaseActions, func(i, j int) bool { return plan.LeaseActions[i].EntityID < plan.LeaseActions[j].EntityID })
	sort.Strings(plan.Snapshots)
	sortImpactBlockers(plan.Blockers)

	plan.ImpactDigest = impactPlanDigest(plan)
	return plan
}

// impactReach walks depends-on edges backwards from the requested entity, so
// every entity that transitively depends on it is previewed. Visited tracking
// bounds a malformed or cyclic graph; cycle members and dangling edges are
// reported so classification can stay conservative.
func impactReach(plan *ImpactPlan, candidates map[string]ImpactCandidate, root string) (map[string]bool, map[string]bool) {
	reachable := map[string]bool{}
	cyclic := map[string]bool{}
	dependents := map[string][]string{}
	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, dep := range sortedTransitionSet(candidates[key].DependsOn) {
			if _, ok := candidates[dep]; !ok {
				plan.addBlocker("IMPACT_DEPENDENCY_MISSING", "impact", candidates[key].ID, fmt.Sprintf("entity %q depends on unknown entity %q", key, dep))
				cyclic[key] = true
				continue
			}
			dependents[dep] = append(dependents[dep], key)
		}
	}
	if _, ok := candidates[root]; !ok {
		return reachable, cyclic
	}
	queue := []string{root}
	reachable[root] = true
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		for _, dependent := range dependents[key] {
			if !reachable[dependent] {
				reachable[dependent] = true
				queue = append(queue, dependent)
			}
		}
	}
	for _, key := range impactCycleMembers(candidates, reachable) {
		cyclic[key] = true
		plan.addBlocker("IMPACT_GRAPH_CYCLIC", "impact", candidates[key].ID, fmt.Sprintf("entity %q participates in a depends-on cycle", key))
	}
	return reachable, cyclic
}

// impactCycleMembers returns every reachable key that is not removable by
// repeated leaf elimination, i.e. every key on or feeding into a cycle.
func impactCycleMembers(candidates map[string]ImpactCandidate, reachable map[string]bool) []string {
	remaining := map[string]bool{}
	for key := range reachable {
		remaining[key] = true
	}
	for progress := true; progress; {
		progress = false
		for key := range remaining {
			live := false
			for _, dep := range candidates[key].DependsOn {
				if remaining[dep] && dep != key {
					live = true
					break
				}
				if dep == key {
					live = true
					break
				}
			}
			if !live {
				delete(remaining, key)
				progress = true
			}
		}
	}
	members := make([]string, 0, len(remaining))
	for key := range remaining {
		members = append(members, key)
	}
	sort.Strings(members)
	return members
}

func classifyImpact(candidate ImpactCandidate, requested, reachable, cyclic bool, input ImpactInput) ImpactEntity {
	entity := ImpactEntity{
		Kind:      candidate.Kind,
		ID:        candidate.ID,
		Spec:      candidate.Spec,
		Version:   candidate.Version,
		State:     candidate.State,
		CrossSpec: candidate.Spec != input.RequestedSpec,
		Snapshot:  candidate.SnapshotRequired,
	}
	entity.Consumptions = append([]ImpactConsumption{}, candidate.Consumptions...)
	sort.Slice(entity.Consumptions, func(i, j int) bool {
		a, b := entity.Consumptions[i], entity.Consumptions[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Record < b.Record
	})

	for _, consumption := range entity.Consumptions {
		if !consumption.External {
			continue
		}
		entity.Classification = ImpactForbidden
		entity.Reason = fmt.Sprintf("immutable %s record %q consumed this entity; in-place %s is forbidden", consumption.Kind, consumption.Record, input.Operation)
		entity.SuccessorRoute = fmt.Sprintf("link a successor %s to %q instead of repairing it in place", candidate.Kind, candidate.ID)
		entity.Choices = []string{"successor"}
		entity.Snapshot = false
		return entity
	}

	// A stale entity's choices are exactly the resolutions that clear staleness
	// (R5.2); the two must never drift apart, so there is one list.
	staleChoices := DescendantResolutions
	switch {
	case candidate.Cancel:
		entity.Classification = ImpactCancelled
		entity.Reason = "caller cancelled this entity; acceptance coverage must be reassigned"
	case candidate.SupersededBy != "":
		entity.Classification = ImpactSuperseded
		entity.Reason = fmt.Sprintf("superseded by %q; acceptance coverage must be reassigned", candidate.SupersededBy)
	case candidate.Retain:
		entity.Classification = ImpactRetained
		entity.Reason = "explicitly retained; retention needs impact approval plus fresh evidence"
	case candidate.Unavailable:
		entity.Classification = ImpactStale
		entity.Reason = "entity is unavailable, so impact cannot be proved unchanged"
		entity.Choices = staleChoices
	case cyclic:
		entity.Classification = ImpactStale
		entity.Reason = "malformed depends-on graph, so impact cannot be proved unchanged"
		entity.Choices = staleChoices
	case requested:
		entity.Classification = ImpactReopened
		entity.Reason = fmt.Sprintf("requested %s target", input.Operation)
	case reachable:
		entity.Classification = ImpactStale
		entity.Reason = fmt.Sprintf("depends transitively on the %s target", input.Operation)
		entity.Choices = staleChoices
	default:
		entity.Classification = ImpactCurrent
		entity.Reason = fmt.Sprintf("not reachable from the %s target", input.Operation)
		entity.Snapshot = false
	}
	return entity
}

// GuardImpactCommit refuses a repair commit whose preview no longer matches
// current state, naming the fresh-preview recovery (R1.4).
func GuardImpactCommit(preview ImpactPlan, currentStateRevision int64, fresh ImpactPlan) error {
	if preview.ExpectedStateRevision != currentStateRevision {
		return fmt.Errorf("%w: previewed state revision %d but current is %d; re-run the impact preview and commit with --expect-revision %d",
			ErrImpactStale, preview.ExpectedStateRevision, currentStateRevision, currentStateRevision)
	}
	if preview.ImpactDigest != fresh.ImpactDigest {
		return fmt.Errorf("%w: impact digest moved from %s to %s; re-run the impact preview before commit",
			ErrImpactStale, preview.ImpactDigest, fresh.ImpactDigest)
	}
	return nil
}

func (p *ImpactPlan) addBlocker(code, gate, entity, message string) {
	blocker := TransitionBlocker{Code: code, Gate: gate, Entity: entity, Message: message}
	for _, existing := range p.Blockers {
		if existing == blocker {
			return
		}
	}
	p.Blockers = append(p.Blockers, blocker)
}

func sortImpactBlockers(blockers []TransitionBlocker) {
	sort.Slice(blockers, func(i, j int) bool {
		a, b := blockers[i], blockers[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Entity != b.Entity {
			return a.Entity < b.Entity
		}
		return a.Message < b.Message
	})
}

func impactPlanDigest(plan ImpactPlan) string {
	plan.ImpactDigest = ""
	raw, _ := json.Marshal(plan)
	return Digest(raw)
}
