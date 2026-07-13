package core

import (
	"reflect"
	"testing"
	"time"
)

func TestDriftProjection(t *testing.T) {
	asOf := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	invariants := []DriftInvariantV1{
		{ID: "INV-2", Path: "config/prod.json", EvidenceTask: "T2", Severity: DriftSeverityCritical},
		{ID: "INV-1", Path: "internal/auth.go", EvidenceTask: "T1", Severity: DriftSeverityHigh},
	}
	decisions := []DecisionV1{{ID: "D-1", Status: GovernanceAccepted, Owner: "platform", CreatedAt: "2026-01-01T00:00:00Z", ReviewAt: "2026-06-01T00:00:00Z", ExpiresAt: "2027-01-01T00:00:00Z", AffectedInvariants: []string{"INV-1"}}}
	evidence := []EvidenceRecord{
		{TaskID: "T1", ExitCode: 0, GitHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{TaskID: "T1", ExitCode: 1, GitHead: "cccccccccccccccccccccccccccccccccccccccc"},
		{TaskID: "T2", ExitCode: 0, GitHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}

	got := ProjectDrift(invariants, decisions, evidence, asOf, "payments")
	want := []DriftFinding{
		{Source: "decision:D-1/invariant:INV-1", Path: "internal/auth.go", Severity: DriftSeverityHigh, Status: Drifted, LastPassingHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SuggestedCommand: "specd new payments-drift"},
		{Source: "invariant:INV-2", Path: "config/prod.json", Severity: DriftSeverityCritical, Status: DriftHolds, LastPassingHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SuggestedCommand: "specd new payments-drift"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProjectDrift() = %#v, want %#v", got, want)
	}
	// Projection must not mutate caller-owned ordering or records.
	if invariants[0].ID != "INV-2" || decisions[0].AffectedInvariants[0] != "INV-1" {
		t.Fatal("ProjectDrift mutated input")
	}
}

func TestDriftNotEvaluable(t *testing.T) {
	got := ProjectDrift([]DriftInvariantV1{{ID: "INV-1", Path: "missing", EvidenceTask: "T1", Severity: DriftSeverityUnknown}}, nil, nil, time.Time{}, "demo")
	if len(got) != 1 || got[0].Status != DriftNotEvaluable || got[0].LastPassingHead != "unknown" {
		t.Fatalf("missing evidence = %#v, want not-evaluable", got)
	}
	if got := ProjectDrift(nil, nil, nil, time.Time{}, "demo"); len(got) != 1 || got[0].Status != DriftNone {
		t.Fatalf("no declarations = %#v, want none", got)
	}
}

func TestDriftRejectsInvalidDeclaration(t *testing.T) {
	for _, tc := range []DriftInvariantV1{
		{ID: "INV", Path: "../secret", EvidenceTask: "T1", Severity: DriftSeverityHigh},
		{ID: "INV", Path: "ok", EvidenceTask: "T1", Severity: "urgent"},
	} {
		if err := tc.Validate(); err == nil {
			t.Fatalf("Validate(%#v) succeeded", tc)
		}
	}
}
