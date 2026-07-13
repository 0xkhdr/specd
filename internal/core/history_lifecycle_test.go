package core

import (
	"strings"
	"testing"
	"time"
)

func TestDecisionLifecycle(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	old := DecisionV1{ID: "D-1", Status: GovernanceAccepted, Owner: "platform-team", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339), ReviewAt: now.Add(time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(2 * time.Hour).Format(time.RFC3339), AffectedInvariants: []string{"V1"}}
	next := DecisionV1{ID: "D-2", Status: GovernanceAccepted, Owner: "platform-team", CreatedAt: now.Format(time.RFC3339), ReviewAt: now.Add(time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(3 * time.Hour).Format(time.RFC3339), Supersedes: "D-1", AffectedInvariants: []string{"V1"}}
	records, err := AppendDecision(nil, old)
	if err != nil {
		t.Fatal(err)
	}
	records, err = AppendDecision(records, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Status != GovernanceAccepted || records[1].Supersedes != "D-1" || EffectiveDecisionStatus(records, "D-1") != GovernanceSuperseded {
		t.Fatalf("supersession = %+v", records)
	}
	if _, err := AppendDecision(records, next); err == nil {
		t.Fatal("duplicate id accepted")
	}
	chain := DecisionChain(records, "D-2")
	if len(chain) != 2 || chain[0].ID != "D-1" || chain[1].ID != "D-2" {
		t.Fatalf("chain = %+v", chain)
	}
}

func TestOwnerNotAgent(t *testing.T) {
	for _, owner := range []string{"agent", "craftsman", "worker:42", "model/gpt"} {
		if err := ValidateGovernanceOwner(owner); err == nil || !strings.Contains(err.Error(), "human or team") {
			t.Fatalf("owner %q accepted: %v", owner, err)
		}
	}
	if err := ValidateGovernanceOwner("platform-team"); err != nil {
		t.Fatal(err)
	}
}
