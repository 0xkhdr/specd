package core

import (
	"encoding/json"
	"fmt"
	"sort"
)

const TransitionPlanSchemaVersion = "1"

const (
	TransitionMutationNone          = "none"
	TransitionMutationAdvanceStatus = "advance_status"
)

// TransitionInput is a caller-resolved snapshot. It deliberately contains no
// root, path, clock, or callback, so planning cannot observe or mutate external
// state.
type TransitionInput struct {
	Current               Status
	Target                Status
	StateRevision         int64
	Actor                 OperationActor
	ActorID               string
	ActorAssurance        string
	AuthorityRequired     bool
	Authority             *AuthorityV1
	ArmedGates            []string
	Inputs                map[string]string
	ArtifactDigests       map[string]string
	ConfigDigest          string
	PolicyDigest          string
	TransportCapabilities []string
	RequiredTransport     []string
	Blockers              []TransitionBlocker
	Warnings              []TransitionBlocker
	Recoveries            []TransitionRecovery
	ReadinessChecked      bool
	MutationIntent        string
	Terminal              bool
	TerminalReason        string
	ExternallyConsumed    bool
}

// TransitionDigest identifies caller-inspected bytes without retaining them.
type TransitionDigest struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type TransitionAuthority struct {
	Required         bool   `json:"required"`
	Available        bool   `json:"available"`
	Digest           string `json:"digest"`
	ActorID          string `json:"actor_id"`
	WorkerID         string `json:"worker_id"`
	Role             string `json:"role"`
	Mode             string `json:"mode"`
	PolicyDigest     string `json:"policy_digest"`
	BaselineRevision string `json:"baseline_revision"`
}

type TransitionBlocker struct {
	Code    string `json:"code"`
	Gate    string `json:"gate,omitempty"`
	Entity  string `json:"entity,omitempty"`
	Message string `json:"message"`
}

type TransitionRecovery struct {
	BlockerCode string         `json:"blocker_code"`
	Operation   string         `json:"operation"`
	Actor       OperationActor `json:"actor"`
}

type TransitionPlan struct {
	SchemaVersion         string               `json:"schema_version"`
	PlanDigest            string               `json:"plan_digest"`
	Current               Status               `json:"current"`
	Target                Status               `json:"target"`
	Terminal              bool                 `json:"terminal"`
	TerminalReason        string               `json:"terminal_reason"`
	Actor                 OperationActor       `json:"actor"`
	ActorID               string               `json:"actor_id"`
	ActorAssurance        string               `json:"actor_assurance"`
	Authority             TransitionAuthority  `json:"authority"`
	ArmedGates            []string             `json:"armed_gates"`
	Inputs                []TransitionDigest   `json:"inputs"`
	ArtifactDigests       []TransitionDigest   `json:"artifact_digests"`
	ConfigDigest          string               `json:"config_digest"`
	PolicyDigest          string               `json:"policy_digest"`
	StateRevision         int64                `json:"state_revision"`
	TransportCapabilities []string             `json:"transport_capabilities"`
	RequiredTransport     []string             `json:"required_transport"`
	Blockers              []TransitionBlocker  `json:"blockers"`
	Warnings              []TransitionBlocker  `json:"warnings"`
	Recoveries            []TransitionRecovery `json:"recoveries"`
	ReadinessChecked      bool                 `json:"readiness_checked"`
	MutationIntent        string               `json:"mutation_intent"`
	StateChanged          bool                 `json:"state_changed"`
	ExternallyConsumed    bool                 `json:"externally_consumed"`
}

// BuildTransitionPlan returns a canonical, content-addressed projection of the
// supplied snapshot. All validation failures stay in the plan as blockers.
func BuildTransitionPlan(input TransitionInput) TransitionPlan {
	plan := TransitionPlan{
		SchemaVersion:         TransitionPlanSchemaVersion,
		Current:               input.Current,
		Target:                input.Target,
		Actor:                 input.Actor,
		ActorID:               input.ActorID,
		ActorAssurance:        input.ActorAssurance,
		ArmedGates:            sortedTransitionSet(input.ArmedGates),
		Inputs:                transitionDigests(input.Inputs),
		ArtifactDigests:       transitionDigests(input.ArtifactDigests),
		ConfigDigest:          input.ConfigDigest,
		PolicyDigest:          input.PolicyDigest,
		StateRevision:         input.StateRevision,
		TransportCapabilities: sortedTransitionSet(input.TransportCapabilities),
		RequiredTransport:     sortedTransitionSet(input.RequiredTransport),
		Blockers:              append([]TransitionBlocker{}, input.Blockers...),
		Warnings:              append([]TransitionBlocker{}, input.Warnings...),
		Recoveries:            append([]TransitionRecovery{}, input.Recoveries...),
		ReadinessChecked:      input.ReadinessChecked,
		MutationIntent:        input.MutationIntent,
		ExternallyConsumed:    input.ExternallyConsumed,
		Authority:             transitionAuthority(input.AuthorityRequired, input.Authority),
	}
	if plan.ActorID == "" && input.Authority != nil {
		plan.ActorID = input.Authority.ActorID
	}
	if plan.MutationIntent == "" {
		plan.MutationIntent = TransitionMutationAdvanceStatus
	}

	if !ValidStatus(plan.Current) {
		plan.addBlocker("CURRENT_STATUS_INVALID", "", "spec", fmt.Sprintf("current status %q is invalid", plan.Current))
	} else if input.Terminal || NextStatus(plan.Current) == "" {
		plan.Terminal = true
		plan.TerminalReason = input.TerminalReason
		if plan.TerminalReason == "" {
			plan.TerminalReason = fmt.Sprintf("status %q has no legal successor", plan.Current)
		}
		plan.Target = ""
		plan.MutationIntent = TransitionMutationNone
	} else if !ValidStatus(plan.Target) {
		plan.addBlocker("TARGET_INVALID", "", "spec", fmt.Sprintf("target status %q is invalid", plan.Target))
		plan.addRecovery("TARGET_INVALID", "status", ActorAgent)
	} else if !CanAdvanceStatus(plan.Current, plan.Target) {
		plan.addBlocker("TARGET_NOT_SUCCESSOR", "", "spec", fmt.Sprintf("target status %q is not the legal successor of %q", plan.Target, plan.Current))
		plan.addRecovery("TARGET_NOT_SUCCESSOR", "status", ActorAgent)
	}

	if plan.StateRevision < 0 {
		plan.addBlocker("STATE_REVISION_INVALID", "", "state", "state revision must not be negative")
	}
	if plan.Actor == "" {
		plan.addBlocker("ACTOR_REQUIRED", "", "actor", "actor class is required")
	}
	if plan.ActorAssurance == "" {
		plan.addBlocker("ACTOR_ASSURANCE_REQUIRED", "", "actor", "actor assurance is required")
	}
	if plan.ConfigDigest == "" {
		plan.addBlocker("CONFIG_DIGEST_MISSING", "", "config", "config digest is required")
	}
	if plan.PolicyDigest == "" {
		plan.addBlocker("POLICY_DIGEST_MISSING", "", "policy", "policy digest is required")
	}
	if plan.Authority.Required && (!plan.Authority.Available || plan.Authority.Digest == "") {
		plan.addBlocker("AUTHORITY_REQUIRED", "", "authority", "current authority is required for this transition")
		plan.addRecovery("AUTHORITY_REQUIRED", "context", ActorAgent)
	}
	for _, gate := range plan.ArmedGates {
		if gate == "" {
			plan.addBlocker("GATE_ID_REQUIRED", "", "gate", "armed gate id is required")
		}
	}
	for _, ref := range append(append([]TransitionDigest{}, plan.Inputs...), plan.ArtifactDigests...) {
		if ref.ID == "" || ref.Digest == "" {
			plan.addBlocker("INPUT_DIGEST_INVALID", "", ref.ID, "input identities and digests must be non-empty")
		}
	}
	available := make(map[string]bool, len(plan.TransportCapabilities))
	for _, capability := range plan.TransportCapabilities {
		available[capability] = true
	}
	for _, required := range plan.RequiredTransport {
		if !available[required] {
			plan.addBlocker("TRANSPORT_CAPABILITY_MISSING", "", required, fmt.Sprintf("transport capability %q is required", required))
		}
	}

	canonicalizeTransitionPlan(&plan)
	plan.PlanDigest = transitionPlanDigest(plan)
	return plan
}

func transitionAuthority(required bool, authority *AuthorityV1) TransitionAuthority {
	result := TransitionAuthority{Required: required}
	if authority == nil {
		return result
	}
	result.Available = true
	result.Digest = authority.Digest
	result.ActorID = authority.ActorID
	result.WorkerID = authority.WorkerID
	result.Role = authority.Role
	result.Mode = authority.Mode
	result.PolicyDigest = authority.PolicyDigest
	result.BaselineRevision = authority.BaselineRevision
	return result
}

func transitionDigests(values map[string]string) []TransitionDigest {
	result := make([]TransitionDigest, 0, len(values))
	for id, digest := range values {
		result = append(result, TransitionDigest{ID: id, Digest: digest})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func sortedTransitionSet(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	if len(result) == 0 {
		return result
	}
	n := 1
	for _, value := range result[1:] {
		if value != result[n-1] {
			result[n] = value
			n++
		}
	}
	return result[:n]
}

func (p *TransitionPlan) addBlocker(code, gate, entity, message string) {
	blocker := TransitionBlocker{Code: code, Gate: gate, Entity: entity, Message: message}
	for _, existing := range p.Blockers {
		if existing == blocker {
			return
		}
	}
	p.Blockers = append(p.Blockers, blocker)
}

func (p *TransitionPlan) addRecovery(code, operation string, actor OperationActor) {
	recovery := TransitionRecovery{BlockerCode: code, Operation: operation, Actor: actor}
	for _, existing := range p.Recoveries {
		if existing == recovery {
			return
		}
	}
	p.Recoveries = append(p.Recoveries, recovery)
}

func canonicalizeTransitionPlan(plan *TransitionPlan) {
	sort.Slice(plan.Blockers, func(i, j int) bool {
		a, b := plan.Blockers[i], plan.Blockers[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Gate != b.Gate {
			return a.Gate < b.Gate
		}
		if a.Entity != b.Entity {
			return a.Entity < b.Entity
		}
		return a.Message < b.Message
	})
	sort.Slice(plan.Warnings, func(i, j int) bool {
		a, b := plan.Warnings[i], plan.Warnings[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Gate != b.Gate {
			return a.Gate < b.Gate
		}
		if a.Entity != b.Entity {
			return a.Entity < b.Entity
		}
		return a.Message < b.Message
	})
	sort.Slice(plan.Recoveries, func(i, j int) bool {
		a, b := plan.Recoveries[i], plan.Recoveries[j]
		if a.BlockerCode != b.BlockerCode {
			return a.BlockerCode < b.BlockerCode
		}
		if a.Operation != b.Operation {
			return a.Operation < b.Operation
		}
		return a.Actor < b.Actor
	})
}

func transitionPlanDigest(plan TransitionPlan) string {
	plan.PlanDigest = ""
	raw, _ := json.Marshal(plan)
	return Digest(raw)
}
