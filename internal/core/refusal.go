package core

import (
	"errors"
	"fmt"
	"strings"
)

// Refusal is the one shape every refusal path returns. An agent that hits a
// blocker must be able to act on it without improvising: the code is stable
// enough to branch on, RecoveryCommand is the exact next call, and
// ActorRequired names who can make that call (spec agent-protocol-clarity
// R4.1, R4.2).
//
// AuthorityConsumed reports whether the refused operation burned the authority
// packet it was issued. A refusal raised before authority is issued reports
// false, so a retry does not need a fresh packet.
type Refusal struct {
	Code               string              `json:"code"`
	Category           string              `json:"category"`
	Entity             string              `json:"entity"`
	Observed           string              `json:"observed"`
	Expected           string              `json:"expected"`
	InputDigests       map[string]string   `json:"input_digests"`
	StateChanged       bool                `json:"state_changed"`
	CheckpointID       string              `json:"checkpoint_id"`
	Retryable          bool                `json:"retryable"`
	ActorRequired      string              `json:"actor_required"`
	RecoveryOperations []RecoveryOperation `json:"recovery_operations"`
	Detail             string              `json:"detail"`

	// Compatibility fields retained for consumers of the first refusal shape.
	Blocker           string `json:"blocker"`
	AuthorityConsumed bool   `json:"authority_consumed"`
	RetrySafe         bool   `json:"retry_safe"`
	RecoveryCommand   string `json:"recovery_command"`

	// wrapped keeps the sentinel a call site already returned, so migrating a
	// path to a typed refusal never breaks an existing errors.Is check.
	wrapped error
}

// RecoveryOperation is an actor-legal next route. InPlace is false when the
// current entity cannot be repaired and the route is a successor or escalation.
type RecoveryOperation struct {
	Operation string `json:"operation"`
	Actor     string `json:"actor"`
	Command   string `json:"command"`
	InPlace   bool   `json:"in_place"`
}

// Wrapping returns a copy of the refusal that also satisfies errors.Is(err,
// sentinel), so a call site can adopt the typed shape without changing how
// callers classify it.
func (r Refusal) Wrapping(err error) Refusal {
	r.wrapped = err
	return r
}

func (r Refusal) Unwrap() error { return r.wrapped }

// Refusal actor classes. A refusal that only a human can clear says so, rather
// than leaving an agent to retry a command it will never be allowed to run.
const (
	RefusalActorAgent    = "agent"
	RefusalActorHuman    = "human"
	RefusalActorOperator = "operator"
)

func (r Refusal) Error() string { return r.Code + ": " + r.Blocker }

// AsRefusal extracts the structured refusal from err, if err carries one.
func AsRefusal(err error) (Refusal, bool) {
	var refusal Refusal
	if errors.As(err, &refusal) {
		return refusal, true
	}
	return Refusal{}, false
}

// refusalRecovery maps every constructed code to an actor-legal next route.
// Codes with a caller-specific recovery use the same explicit non-retryable
// escalation as the old fallback; the conformance test keeps that list closed.
var refusalRecovery = func() map[string]Refusal {
	recoveries := map[string]Refusal{
		"UNKNOWN_COMMAND":     refusalTemplate("usage", "command exists", RefusalActorAgent, "help", "specd help", true),
		"PHASE_INVALID":       refusalTemplate("lifecycle", "operation is legal in the current phase", RefusalActorAgent, "status", "specd status <slug> --guide", true),
		"OPERATION_UNKNOWN":   refusalTemplate("usage", "declared command operation", RefusalActorAgent, "help", "specd help <command>", true),
		"FLAG_VALUE_INVALID":  refusalTemplate("usage", "declared flag value", RefusalActorAgent, "help", "specd help <command>", true),
		"HUMAN_ONLY":          refusalTemplate("authority", "required human actor", RefusalActorHuman, "handoff", "ask a human to run the operation", false),
		"AUTHORITY_DENIED":    refusalTemplate("authority", "valid scoped authority", RefusalActorAgent, "context", "specd context <slug> <task> --json", true),
		"EVIDENCE_MISSING":    refusalTemplate("evidence", "current passing evidence", RefusalActorAgent, "verify.task", "specd verify <slug> <task>", true),
		"EVIDENCE_FAILING":    refusalTemplate("evidence", "passing evidence", RefusalActorAgent, "verify.task", "specd verify <slug> <task>", true),
		"EVIDENCE_STALE":      refusalTemplate("evidence", "evidence pinned to current HEAD", RefusalActorAgent, "verify.task", "specd verify <slug> <task>", true),
		"GATE_FAILED":         refusalTemplate("gate", "no blocking gate findings", RefusalActorAgent, "check", "specd check <slug>", true),
		"APPROVAL_REQUIRED":   refusalTemplate("authority", "human approval", RefusalActorHuman, "approve", "specd approve <slug>", true),
		"SPEC_INVALID":        refusalTemplate("usage", "valid spec identity", RefusalActorAgent, "status", "specd status --json", true),
		"REVISION_CONFLICT":   refusalTemplate("conflict", "current state revision", RefusalActorAgent, "status", "specd status <slug> --json", true),
		"SANDBOX_UNAVAILABLE": refusalTemplate("host", "declared verify sandbox", RefusalActorOperator, "host.configure", "declare sandbox support on the host", false),
		"DISPATCH_LEDGER_FAILED": refusalTemplate("orchestration", "checkpoint reconciled with dispatch ledger", RefusalActorOperator,
			"brain.resume", "specd brain resume <slug>", false),
		"SESSION_WRITE_FAILED": refusalTemplate("orchestration", "session reconciled with durable dispatch", RefusalActorOperator,
			"brain.resume", "specd brain resume <slug>", false),
		"MISSION_INVALID": refusalTemplate("input", "valid mission identity and authority envelope", RefusalActorOperator,
			"brain.status", "specd brain status <slug>", false),
		"WORKER_OUT_OF_SCOPE": refusalTemplate("scope", "a worker id the approved plan named for this task", RefusalActorOperator,
			"brain.status", "specd brain status <slug>", false),
		"NO_SUCCESSOR": refusalTemplate("lifecycle", "supported successor or escalation", RefusalActorAgent,
			"new", "specd new <successor>", false),
	}
	escalation := refusalTemplate("governance", "governed operation preconditions satisfied", RefusalActorAgent,
		"request-decision", "specd request-decision <slug> --text <reason>", false)
	escalation.Retryable, escalation.RetrySafe = false, false
	for _, code := range []string{
		"ARTIFACT_PATH_ABSOLUTE", "BASELINE_DRIFTED", "BASELINE_UNPINNED", "BASELINE_UNRESOLVABLE",
		"BINDING_MISSING", "BRAIN_ZERO_PROGRESS", "CLARIFICATION_OPEN", "CONFORMANCE_KIND_UNKNOWN",
		"CONFORMANCE_SLUG_REQUIRED", "FLAG_UNKNOWN", "GRANT_EXHAUSTED", "GRANT_EXPIRED",
		"GRANT_POLICY_STALE", "GRANT_PROHIBITED", "GRANT_REASON_REQUIRED", "GRANT_REPLAY",
		"GRANT_REVOKED", "GRANT_SCOPE", "GRANT_SECRET_INVALID", "GRANT_USE_UNRESERVED",
		"HANDSHAKE_MISMATCH", "LEASE_SESSION_CONFLICT", "LEASE_SESSION_MISMATCH", "MANAGED_TARGET_REQUIRED",
		"NONCE_REPLAYED", "OUTSIDE_SCOPE", "RECEIPT_INVALID", "RECEIPT_STALE",
		"REOPEN_ARTIFACT_MOVED", "REQUEST_MODE_CONFLICT", "REQUEST_MODE_INVALID", "ROUTE_DISPATCH_MISSING",
		"ROUTE_HANDOFF_REQUIRED", "SCOPE_AMEND_REFUSED", "SESSION_EXPIRED", "SESSION_INVALID", "SESSION_UNKNOWN",
		"SPEC_NOT_DRIVEABLE", "WRITE_SCOPE_CONFLICT",
	} {
		recoveries[code] = escalation
	}
	return recoveries
}()

func refusalTemplate(category, expected, actor, operation, command string, inPlace bool) Refusal {
	recovery := RecoveryOperation{Operation: operation, Actor: actor, Command: command, InPlace: inPlace}
	retryable := actor == RefusalActorAgent
	return Refusal{Category: category, Expected: expected, ActorRequired: actor, RecoveryCommand: command, RecoveryOperations: []RecoveryOperation{recovery}, Retryable: retryable, RetrySafe: retryable}
}

// Refuse builds a typed refusal. Recovery defaults come from the code table;
// AuthorityConsumed defaults to false because a refusal raised before the
// operation ran consumed nothing. Callers past that point set it explicitly
// with Consumed.
func Refuse(code, blocker string) Refusal {
	refusal := Refusal{
		Code:         code,
		Category:     "governance",
		Entity:       "operation",
		Observed:     blocker,
		Expected:     "governed operation preconditions satisfied",
		InputDigests: map[string]string{},
		Retryable:    true,
		Detail:       blocker,
		Blocker:      blocker,
		RetrySafe:    true,
	}
	if known, ok := refusalRecovery[code]; ok {
		refusal.Category = known.Category
		refusal.Expected = known.Expected
		refusal.ActorRequired = known.ActorRequired
		refusal.RecoveryCommand = known.RecoveryCommand
		refusal.RecoveryOperations = append([]RecoveryOperation{}, known.RecoveryOperations...)
		refusal.Retryable = known.Retryable
		refusal.RetrySafe = known.RetrySafe
	} else {
		refusal.ActorRequired = RefusalActorAgent
		refusal.RecoveryCommand = "specd request-decision <slug> --text <reason>"
		refusal.RecoveryOperations = []RecoveryOperation{{Operation: "request-decision", Actor: RefusalActorAgent, Command: refusal.RecoveryCommand, InPlace: false}}
		refusal.Retryable = false
		refusal.RetrySafe = false
	}
	// Only the actor named on the refusal can clear it, so a refusal an agent
	// cannot clear is never retry-safe for the agent that hit it.
	if refusal.ActorRequired != RefusalActorAgent {
		refusal.Retryable = false
		refusal.RetrySafe = false
	}
	return refusal
}

// Refusef is Refuse with a formatted blocker.
func Refusef(code, format string, args ...any) Refusal {
	return Refuse(code, fmt.Sprintf(format, args...))
}

// Consumed marks the refusal as having burned its authority packet: a retry
// needs a freshly issued one.
func (r Refusal) Consumed() Refusal {
	r.AuthorityConsumed = true
	r.Retryable = false
	r.RetrySafe = false
	return r
}

// WithRecovery overrides the canned recovery for a call site that knows the
// exact command, e.g. one that can name the real slug instead of a placeholder.
func (r Refusal) WithRecovery(actor, command string) Refusal {
	r.ActorRequired = actor
	r.RecoveryCommand = command
	r.RecoveryOperations = []RecoveryOperation{{Operation: recoveryOperation(command), Actor: actor, Command: command, InPlace: true}}
	r.Retryable, r.RetrySafe = actor == RefusalActorAgent, actor == RefusalActorAgent
	return r
}

// WithContext names the governed entity and the observed/expected comparison.
func (r Refusal) WithContext(entity, observed, expected string) Refusal {
	r.Entity, r.Observed, r.Expected = entity, observed, expected
	return r
}

// WithInput records only a digest of an inspected input; raw values and secrets
// never enter the refusal envelope.
func (r Refusal) WithInput(identity string, value []byte) Refusal {
	if r.InputDigests == nil {
		r.InputDigests = map[string]string{}
	}
	r.InputDigests[identity] = Digest(value)
	return r
}

// WithInputDigests adds caller-resolved input identities without retaining the
// source bytes. The map is copied so later caller mutation cannot alter an error.
func (r Refusal) WithInputDigests(inputs map[string]string) Refusal {
	if r.InputDigests == nil {
		r.InputDigests = map[string]string{}
	}
	for identity, digest := range inputs {
		r.InputDigests[identity] = digest
	}
	return r
}

// WithMutation reports durable effects left behind by a failed operation.
func (r Refusal) WithMutation(stateChanged bool, checkpointID string) Refusal {
	r.StateChanged, r.CheckpointID = stateChanged, checkpointID
	return r
}

// WithRetryable overrides whether the refused operation itself may be retried.
func (r Refusal) WithRetryable(retryable bool) Refusal {
	r.Retryable, r.RetrySafe = retryable, retryable
	return r
}

// WithSuccessor marks the recovery as a successor/escalation, not an in-place
// repair of the refused entity.
func (r Refusal) WithSuccessor(actor, operation, command string) Refusal {
	r.ActorRequired, r.RecoveryCommand = actor, command
	r.RecoveryOperations = []RecoveryOperation{{Operation: operation, Actor: actor, Command: command, InPlace: false}}
	return r.WithRetryable(false)
}

func recoveryOperation(command string) string {
	fields := strings.Fields(command)
	if len(fields) > 1 && fields[0] == "specd" {
		if operation, ok := ResolveOperation(fields[1], fields[2:], nil); ok {
			return operation.ID
		}
		return fields[1]
	}
	return "handoff"
}
