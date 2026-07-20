package core

import (
	"errors"
	"fmt"
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
	Code              string `json:"code"`
	Blocker           string `json:"blocker"`
	AuthorityConsumed bool   `json:"authority_consumed"`
	RetrySafe         bool   `json:"retry_safe"`
	ActorRequired     string `json:"actor_required"`
	RecoveryCommand   string `json:"recovery_command"`

	// wrapped keeps the sentinel a call site already returned, so migrating a
	// path to a typed refusal never breaks an existing errors.Is check.
	wrapped error
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

// refusalRecovery maps a refusal code to the actor who can clear it and the
// exact command that does. A code absent from this table is still a valid
// refusal; it just carries no canned recovery, which is honest — inventing one
// is what R4.2 exists to prevent.
var refusalRecovery = map[string]Refusal{
	"UNKNOWN_COMMAND":     {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd help"},
	"PHASE_INVALID":       {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd status <slug> --guide"},
	"OPERATION_UNKNOWN":   {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd help <command>"},
	"FLAG_VALUE_INVALID":  {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd help <command>"},
	"HUMAN_ONLY":          {ActorRequired: RefusalActorHuman, RecoveryCommand: "ask a human to run the operation"},
	"AUTHORITY_DENIED":    {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd context <slug> <task> --json"},
	"EVIDENCE_MISSING":    {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd verify <slug> <task>"},
	"GATE_FAILED":         {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd check <slug>"},
	"APPROVAL_REQUIRED":   {ActorRequired: RefusalActorHuman, RecoveryCommand: "specd approve <slug>"},
	"SPEC_INVALID":        {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd status --json"},
	"REVISION_CONFLICT":   {ActorRequired: RefusalActorAgent, RecoveryCommand: "specd status <slug> --json"},
	"SANDBOX_UNAVAILABLE": {ActorRequired: RefusalActorOperator, RecoveryCommand: "declare sandbox support on the host"},
}

// Refuse builds a typed refusal. Recovery defaults come from the code table;
// AuthorityConsumed defaults to false because a refusal raised before the
// operation ran consumed nothing. Callers past that point set it explicitly
// with Consumed.
func Refuse(code, blocker string) Refusal {
	refusal := Refusal{Code: code, Blocker: blocker, RetrySafe: true}
	if known, ok := refusalRecovery[code]; ok {
		refusal.ActorRequired = known.ActorRequired
		refusal.RecoveryCommand = known.RecoveryCommand
	} else {
		refusal.ActorRequired = RefusalActorAgent
		refusal.RecoveryCommand = "specd status <slug> --guide"
	}
	// Only the actor named on the refusal can clear it, so a refusal an agent
	// cannot clear is never retry-safe for the agent that hit it.
	if refusal.ActorRequired != RefusalActorAgent {
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
	r.RetrySafe = false
	return r
}

// WithRecovery overrides the canned recovery for a call site that knows the
// exact command, e.g. one that can name the real slug instead of a placeholder.
func (r Refusal) WithRecovery(actor, command string) Refusal {
	r.ActorRequired = actor
	r.RecoveryCommand = command
	if actor != RefusalActorAgent {
		r.RetrySafe = false
	}
	return r
}
