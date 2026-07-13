package core

import (
	"strings"
	"testing"
)

func TestRecurringDefine(t *testing.T) {
	good := RecurringCheckV1{SchemaVersion: 1, ID: "api-health", Command: "go test ./...", Cadence: "0 3 * * *", Trigger: "schedule"}
	if err := good.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	for _, bad := range []RecurringCheckV1{{ID: "x", Command: "go test ./..."}, {ID: "x", Command: "curl https://example.com", Cadence: "daily"}, {ID: "bad id", Command: "go test ./...", Cadence: "daily"}} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("Validate(%+v) = nil", bad)
		}
	}
}

func TestRecurringAppendOnly(t *testing.T) {
	root := t.TempDir()
	pass := RecurringResultV1{SchemaVersion: 1, CheckID: "api-health", GitHead: strings.Repeat("a", 40), ReleaseID: "r1", ConfigID: "prod-v1", Verdict: RecurringPass, ObservedAt: "2026-01-01T00:00:00Z"}
	fail := pass
	fail.GitHead = strings.Repeat("b", 40)
	fail.Verdict = RecurringFail
	fail.ObservedAt = "2026-01-02T00:00:00Z"
	if err := RecordRecurringResult(root, "demo", pass); err != nil {
		t.Fatal(err)
	}
	if err := RecordRecurringResult(root, "demo", fail); err != nil {
		t.Fatal(err)
	}
	records, err := LoadRecurringResults(RecurringResultsPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Verdict != RecurringPass || records[1].GitHead != fail.GitHead {
		t.Fatalf("records = %+v", records)
	}
}

func TestRecurringSuccessorOptIn(t *testing.T) {
	result := RecurringResultV1{CheckID: "api-health", GitHead: strings.Repeat("b", 40), ReleaseID: "r2", ConfigID: "prod", Verdict: RecurringFail, ObservedAt: "2026-01-02T00:00:00Z"}
	if plan, err := PlanRecurringSuccessor("source", "repair", result, false); err != nil || plan != nil {
		t.Fatalf("disabled = %+v, %v", plan, err)
	}
	plan, err := PlanRecurringSuccessor("source", "repair", result, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Link.Kind != LinkKindMaintains || plan.Provenance.SourceType != SourcePolicy || plan.Provenance.SourceRef == "" {
		t.Fatalf("plan = %+v", plan)
	}
	pass := result
	pass.Verdict = RecurringPass
	if _, err := PlanRecurringSuccessor("source", "repair", pass, true); err == nil {
		t.Fatal("passing result accepted")
	}
}
