package core

import (
	"testing"
	"time"
)

func TestException(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	e := ExceptionV1{ID: "E-1", Status: GovernanceAccepted, Owner: "security-team", CreatedAt: now.Format(time.RFC3339), ReviewAt: now.Add(time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(2 * time.Hour).Format(time.RFC3339), Blocking: true, AffectedInvariants: []string{"SEC-1"}}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	e.Status = "open"
	if err := e.Validate(); err == nil {
		t.Fatal("unknown status accepted")
	}
}
