package core

import (
	"strings"
	"testing"
	"time"
)

func approveOperation(t *testing.T) Operation {
	t.Helper()
	operation, ok := OperationByID("approve")
	if !ok {
		t.Fatal("approve operation missing from the palette")
	}
	if !operatorOnlyOperation(operation) {
		t.Fatalf("approve actor = %q, want a human/operator-only operation", operation.Actor)
	}
	return operation
}

// TestActorOperationEnforcement pins R1.1 to R1.4, R5.3 and R6.1: a governed
// host attestation is the only thing that can raise an actor above unknown, a
// governed agent cannot reach an operator-only operation, and every unattested
// route stays honest provenance instead of claiming human proof.
func TestActorOperationEnforcement(t *testing.T) {
	now := time.Now().UTC()
	approve := approveOperation(t)
	governed := ReferenceHostContract()

	// R1.3: identity typed by anyone who can set a username, open a terminal,
	// or export a variable is display provenance, never proof.
	t.Run("unattestedclaimsstayunknown", func(t *testing.T) {
		cases := []struct {
			name  string
			claim ActorClaim
		}{
			{"usernamespoof", ActorClaim{Class: "operator", Subject: "root", Transport: RouteCLI, Attestation: ActorAttestationOSUser}},
			{"ttyspoof", ActorClaim{Class: "operator", Subject: "/dev/pts/3", Transport: RouteCLI, Attestation: ActorAttestationTTY}},
			{"environmentspoof", ActorClaim{Class: "operator", Subject: "ci", Transport: RouteCLI, Attestation: ActorAttestationEnvironment}},
			{"requestspoof", ActorClaim{Class: "operator", Subject: "caller", Transport: RouteMCP, Attestation: ActorAttestationRequest}},
			{"legacycaller", ActorClaim{}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				actor := ResolveActorContext(tc.claim, governed, now)
				if actor.Class != ActorClassUnknown {
					t.Fatalf("class = %q, want unknown", actor.Class)
				}
				if actor.Governed || actor.HumanProof() {
					t.Fatal("an unattested claim was presented as human proof")
				}
				if actor.Assurance != AssuranceAdvisory {
					t.Fatalf("assurance = %q, want advisory", actor.Assurance)
				}
				if actor.Subject != tc.claim.Subject {
					t.Fatalf("subject = %q, want the claim preserved as provenance", actor.Subject)
				}
				// The compatibility window: unknown is reported, not refused.
				if err := AuthorizeActorOperation(actor, approve); err != nil {
					t.Fatalf("unknown actor refused: %v", err)
				}
			})
		}
	})

	// R6.1: a record written before the actor field existed carries no class,
	// and must not be read back as a human or delegated approval.
	t.Run("legacyrecordmapstounknown", func(t *testing.T) {
		for _, stored := range []string{"", "human", "Operator", "root"} {
			if got := ParseActorClass(stored); got != ActorClassUnknown {
				t.Errorf("ParseActorClass(%q) = %q, want unknown", stored, got)
			}
		}
		for _, stored := range []string{"operator", "agent", "service"} {
			if got := ParseActorClass(stored); string(got) != stored {
				t.Errorf("ParseActorClass(%q) = %q, want round-trip", stored, got)
			}
		}
	})

	// R1.1/R1.4: a governed host attestation is honoured, and the class and its
	// source survive whichever transport carried it.
	t.Run("governedoperatorpassesoneverytransport", func(t *testing.T) {
		for _, transport := range []RouteTransport{RouteCLI, RouteMCP} {
			actor := ResolveActorContext(ActorClaim{Class: "operator", Subject: "ops@example.test", Transport: transport, Attestation: ActorAttestationHost}, governed, now)
			if !actor.HumanProof() {
				t.Fatalf("%s: governed operator = %+v, want human proof", transport, actor)
			}
			if actor.Transport != transport || actor.Attestation != ActorAttestationHost {
				t.Fatalf("%s: transport/source not preserved: %+v", transport, actor)
			}
			if actor.Assurance != AssuranceSandboxed {
				t.Fatalf("%s: assurance = %q, want sandboxed", transport, actor.Assurance)
			}
			if err := AuthorizeActorOperation(actor, approve); err != nil {
				t.Fatalf("%s: governed operator refused: %v", transport, err)
			}
		}
	})

	// R1.2: the case the whole type exists for.
	t.Run("agentdirectapproverefused", func(t *testing.T) {
		for _, class := range []string{"agent", "service"} {
			actor := ResolveActorContext(ActorClaim{Class: class, Subject: "worker", Transport: RouteCLI, Attestation: ActorAttestationHost}, governed, now)
			if actor.Class != ActorClass(class) || !actor.Governed {
				t.Fatalf("%s: governed claim not honoured: %+v", class, actor)
			}
			err := AuthorizeActorOperation(actor, approve)
			refusal, ok := AsRefusal(err)
			if !ok {
				t.Fatalf("%s: error = %v, want a typed refusal", class, err)
			}
			if refusal.Code != "HUMAN_ONLY" || refusal.ActorRequired != RefusalActorHuman {
				t.Fatalf("%s: refusal = %+v, want a human handoff", class, refusal)
			}
			if refusal.RecoveryCommand != approve.Usage {
				t.Fatalf("%s: recovery = %q, want the legal handoff %q", class, refusal.RecoveryCommand, approve.Usage)
			}
			if refusal.Retryable || refusal.RetrySafe {
				t.Fatalf("%s: refusal is agent-retryable: %+v", class, refusal)
			}
			if !strings.Contains(refusal.Blocker, class) {
				t.Fatalf("%s: blocker %q does not name the observed class", class, refusal.Blocker)
			}
		}
	})

	// R5.3: no host enforcement, no attested identity — the class drops rather
	// than the containment being pretended.
	t.Run("hostwithoutenforcementcannotattest", func(t *testing.T) {
		degraded := ReferenceHostContract()
		degraded.Sandbox = false
		claim := ActorClaim{Class: "operator", Subject: "ops@example.test", Transport: RouteCLI, Attestation: ActorAttestationHost}
		actor := ResolveActorContext(claim, degraded, now)
		if actor.Governed || actor.HumanProof() || actor.Class != ActorClassUnknown {
			t.Fatalf("unenforced host attested an actor: %+v", actor)
		}
		if actor.Assurance != AssuranceAdvisory {
			t.Fatalf("assurance = %q, want advisory", actor.Assurance)
		}
		// An agent claim from the same host is equally unbelievable, so the
		// operation is not refused on its strength either.
		agent := ResolveActorContext(ActorClaim{Class: "agent", Transport: RouteCLI, Attestation: ActorAttestationHost}, degraded, now)
		if err := AuthorizeActorOperation(agent, approve); err != nil {
			t.Fatalf("unenforced host produced an enforcement refusal: %v", err)
		}
	})

	// R5.3: an attestation the host bounded is worthless past its bound.
	t.Run("expiredattestationdegrades", func(t *testing.T) {
		claim := ActorClaim{Class: "operator", Transport: RouteCLI, Attestation: ActorAttestationHost, ExpiresAt: now.Add(-time.Second)}
		if actor := ResolveActorContext(claim, governed, now); actor.HumanProof() {
			t.Fatalf("expired attestation still claims human proof: %+v", actor)
		}
		claim.ExpiresAt = now.Add(time.Hour)
		if actor := ResolveActorContext(claim, governed, now); !actor.HumanProof() {
			t.Fatalf("live attestation rejected: %+v", ResolveActorContext(claim, governed, now))
		}
	})

	// Read operations are unaffected: enforcement is scoped to the operations
	// the palette reserves, not to every command a governed agent runs.
	t.Run("agentkeepsagentoperations", func(t *testing.T) {
		actor := ResolveActorContext(ActorClaim{Class: "agent", Transport: RouteCLI, Attestation: ActorAttestationHost}, governed, now)
		for _, operation := range Operations {
			if operatorOnlyOperation(operation) {
				continue
			}
			if err := AuthorizeActorOperation(actor, operation); err != nil {
				t.Fatalf("governed agent refused on %s: %v", operation.ID, err)
			}
		}
	})
}
