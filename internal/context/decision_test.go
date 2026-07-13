package context

import (
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestActiveDecisionsOnly(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	records := []core.DecisionV1{
		{ID: "active", Status: core.GovernanceAccepted, ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)},
		{ID: "expired", Status: core.GovernanceAccepted, ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)},
		{ID: "old", Status: core.GovernanceSuperseded},
	}
	got := activeDecisionItems(records, now)
	if len(got) != 1 || got[0].TaskID != "active" || got[0].Kind != "decision" {
		t.Fatalf("items = %+v", got)
	}
}
