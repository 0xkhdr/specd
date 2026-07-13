package gates

import (
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestGovernanceExpiry(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	if got := governanceGate(CheckCtx{}); len(got) != 0 {
		t.Fatalf("unconfigured findings = %+v", got)
	}
	ctx := CheckCtx{GovernanceRequired: true, GovernanceNow: now, RequiredDecisionIDs: []string{"D-1"}, Decisions: []core.DecisionV1{{ID: "D-1", Status: core.GovernanceProposed, Owner: "platform-team"}}, Exceptions: []core.ExceptionV1{{ID: "E-1", Status: core.GovernanceAccepted, Owner: "security-team", Blocking: true, ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339)}}}
	got := governanceGate(ctx)
	if len(got) != 2 {
		t.Fatalf("findings = %+v", got)
	}
	joined := got[0].Message + " " + got[1].Message
	if !strings.Contains(joined, "platform-team") || !strings.Contains(joined, "security-team") || !strings.Contains(joined, "review") {
		t.Fatalf("messages = %q", joined)
	}
}
