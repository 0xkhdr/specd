package core

import "time"

// ActorClass is who the harness believes is driving one invocation (R1.1).
//
// The classes are not a permission model — they are a claim about origin. What
// makes a class mean anything is where it came from, which is why an
// ActorContext always carries its attestation next to its class.
type ActorClass string

const (
	ActorClassOperator ActorClass = "operator"
	ActorClassAgent    ActorClass = "agent"
	ActorClassService  ActorClass = "service"
	// ActorClassUnknown is the fail-safe. Legacy records, unattested
	// transports, and anything the harness only heard about from the
	// environment land here (R1.3, R6.1).
	ActorClassUnknown ActorClass = "unknown"
)

// ActorAttestation names where the class came from. Only ActorAttestationHost
// is evidence; every other source is display provenance that can be typed by
// anyone who can set an environment variable or open a terminal.
type ActorAttestation string

const (
	// ActorAttestationHost: a configured host declared the actor alongside a
	// host contract specd can evaluate.
	ActorAttestationHost ActorAttestation = "host"
	// ActorAttestationEnvironment: an environment variable such as SPECD_ACTOR.
	ActorAttestationEnvironment ActorAttestation = "environment"
	// ActorAttestationOSUser: the OS username of the calling process.
	ActorAttestationOSUser ActorAttestation = "os_user"
	// ActorAttestationTTY: the process has a terminal attached.
	ActorAttestationTTY ActorAttestation = "tty"
	// ActorAttestationRequest: the caller supplied the class in the request
	// payload (MCP arguments, repository prose, task text).
	ActorAttestationRequest ActorAttestation = "request"
	// ActorAttestationNone: nothing was supplied at all.
	ActorAttestationNone ActorAttestation = "none"
)

// ActorClaim is what a caller asserts before the harness decides what to
// believe. Class is a plain string because it arrives from outside: an
// unrecognized value degrades to unknown rather than failing the invocation.
type ActorClaim struct {
	Class       string
	Subject     string
	Transport   RouteTransport
	Attestation ActorAttestation
	// ExpiresAt bounds a host attestation. Zero means the host set no bound;
	// a past instant makes the attestation worthless (R5.3).
	ExpiresAt time.Time
}

// ActorContext is the resolved actor for one invocation, carried unchanged
// across CLI, MCP, and controller transports (R1.4).
type ActorContext struct {
	Class       ActorClass       `json:"class"`
	Subject     string           `json:"subject,omitempty"`
	Transport   RouteTransport   `json:"transport,omitempty"`
	Attestation ActorAttestation `json:"attestation"`
	Assurance   AssuranceLevel   `json:"assurance"`
	// Governed is true only when a conformant host attested the class. It is
	// the single bit that separates enforcement from provenance.
	Governed  bool      `json:"governed"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// ParseActorClass reads a stored or supplied class. Anything unrecognized —
// including the empty string an approval record written before this field
// existed carries — is unknown (R6.1). Old records are never reinterpreted as
// human proof.
func ParseActorClass(value string) ActorClass {
	switch ActorClass(value) {
	case ActorClassOperator, ActorClassAgent, ActorClassService:
		return ActorClass(value)
	default:
		return ActorClassUnknown
	}
}

// ResolveActorContext decides what the harness will believe about a claim.
//
// It only ever lowers. A class survives resolution only when a host attested it
// *and* that host's contract evaluates as governed — because a host that does
// not contain the agent cannot vouch for who the agent is (R5.3). Every other
// route (OS username, TTY, SPECD_ACTOR, MCP arguments, repository prose) keeps
// the subject as display provenance and reports unknown at advisory assurance,
// so no amount of text can widen an actor into an operator (R1.3, R1.4).
func ResolveActorContext(claim ActorClaim, contract HostContract, now time.Time) ActorContext {
	actor := ActorContext{
		Class:       ActorClassUnknown,
		Subject:     claim.Subject,
		Transport:   claim.Transport,
		Attestation: claim.Attestation,
		Assurance:   AssuranceAdvisory,
		ExpiresAt:   claim.ExpiresAt,
	}
	if actor.Attestation == "" {
		actor.Attestation = ActorAttestationNone
	}
	if actor.Attestation != ActorAttestationHost {
		return actor
	}
	if !claim.ExpiresAt.IsZero() && !now.Before(claim.ExpiresAt) {
		return actor
	}
	conformance := EvaluateHostContract(contract)
	if !conformance.Governed {
		return actor
	}
	actor.Class = ParseActorClass(claim.Class)
	if actor.Class == ActorClassUnknown {
		return actor
	}
	actor.Assurance, actor.Governed = conformance.Assurance, true
	return actor
}

// HumanProof reports whether the context is evidence of a human operator rather
// than a display name. Only a governed host attestation qualifies; nothing else
// may be presented as human intent (R1.3).
func (a ActorContext) HumanProof() bool {
	return a.Governed && a.Class == ActorClassOperator
}

// AuthorizeActorOperation refuses a governed non-operator actor invoking an
// operation reserved for a human or operator, before the handler runs (R1.1,
// R1.2). The refusal names the required actor and the legal handoff.
//
// An ungoverned actor is *not* refused here. Its class is unknown by
// construction, and refusing on an unknown would turn every unattested host —
// which is every host today — into a broken one while proving nothing. Unknown
// stays advisory and visible; the gates and the human-only palette still hold.
func AuthorizeActorOperation(actor ActorContext, operation Operation) error {
	if !operatorOnlyOperation(operation) || !actor.Governed || actor.Class == ActorClassOperator {
		return nil
	}
	return Refusef("HUMAN_ONLY", "operation %s requires a %s actor; governed actor class is %s",
		operation.ID, operation.Actor, actor.Class).
		WithRecovery(RefusalActorHuman, operation.Usage).
		WithContext("actor", string(actor.Class), string(operation.Actor))
}

// operatorOnlyOperation reports whether the palette reserves the operation for
// a human or operator. Derived from operation metadata, so a verb reclassified
// later is enforced without editing this file.
func operatorOnlyOperation(operation Operation) bool {
	return operation.Actor == ActorHuman || operation.Actor == ActorOperator
}
