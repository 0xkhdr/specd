package core

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestTypedRefusalShape(t *testing.T) {
	// Every code in the recovery table must produce a fully populated shape:
	// a blank field is exactly the gap that makes an agent improvise (R4.1).
	for code := range refusalRecovery {
		refusal := Refuse(code, "blocked")
		if refusal.Code == "" || refusal.Blocker == "" {
			t.Fatalf("%s: code=%q blocker=%q", code, refusal.Code, refusal.Blocker)
		}
		if refusal.ActorRequired == "" || refusal.RecoveryCommand == "" {
			t.Fatalf("%s: actor=%q recovery=%q", code, refusal.ActorRequired, refusal.RecoveryCommand)
		}
		switch refusal.ActorRequired {
		case RefusalActorAgent, RefusalActorHuman, RefusalActorOperator:
		default:
			t.Fatalf("%s: actor class=%q", code, refusal.ActorRequired)
		}
		// A refusal only a human or operator can clear is never retry-safe for
		// the agent that hit it.
		if refusal.ActorRequired != RefusalActorAgent && refusal.RetrySafe {
			t.Fatalf("%s: retry_safe with actor=%s", code, refusal.ActorRequired)
		}
	}
}

func TestTypedRefusalUnknownCodeStillStructured(t *testing.T) {
	refusal := Refuse("NOT_IN_TABLE", "blocked")
	if refusal.ActorRequired == "" || refusal.RecoveryCommand == "" {
		t.Fatalf("unknown code left an empty field: %#v", refusal)
	}
	if refusal.Code != "NOT_IN_TABLE" {
		t.Fatalf("code=%q", refusal.Code)
	}
}

func TestTypedRefusalBeforeAuthorityReportsNotConsumed(t *testing.T) {
	// A refusal raised before authority is issued consumed nothing, so a retry
	// does not need a fresh packet.
	refusal := Refuse("PHASE_INVALID", "phase is perceive")
	if refusal.AuthorityConsumed {
		t.Fatal("refusal before authority issue reports authority_consumed true")
	}
	if !refusal.RetrySafe {
		t.Fatal("agent-clearable refusal is not retry safe")
	}

	consumed := refusal.Consumed()
	if !consumed.AuthorityConsumed || consumed.RetrySafe {
		t.Fatalf("Consumed() = %#v", consumed)
	}
	// Consumed returns a copy; the original must not change underneath a caller.
	if refusal.AuthorityConsumed {
		t.Fatal("Consumed mutated the receiver")
	}
}

func TestTypedRefusalHumanOnlyIsNotAgentRetryable(t *testing.T) {
	refusal := Refuse("APPROVAL_REQUIRED", "gate design awaits approval")
	if refusal.ActorRequired != RefusalActorHuman {
		t.Fatalf("actor=%q", refusal.ActorRequired)
	}
	if refusal.RetrySafe {
		t.Fatal("approval refusal advertised as agent-retryable")
	}
}

func TestTypedRefusalWrappingKeepsSentinel(t *testing.T) {
	sentinel := errors.New("unknown command")
	err := error(Refuse("UNKNOWN_COMMAND", "unknown command \"nope\"").Wrapping(sentinel))
	if !errors.Is(err, sentinel) {
		t.Fatal("wrapped refusal lost its sentinel")
	}
	refusal, ok := AsRefusal(err)
	if !ok {
		t.Fatal("AsRefusal did not recover the shape")
	}
	if refusal.Code != "UNKNOWN_COMMAND" {
		t.Fatalf("code=%q", refusal.Code)
	}
}

func TestTypedRefusalSerializesEveryField(t *testing.T) {
	raw, err := json.Marshal(Refuse("EVIDENCE_MISSING", "no passing verify record"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	// R4.2: one shape on every machine refusal path, so every field is always
	// present — an absent key is indistinguishable from a false value.
	for _, field := range []string{"code", "blocker", "authority_consumed", "retry_safe", "actor_required", "recovery_command"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("refusal JSON omits %q: %s", field, raw)
		}
	}
}
