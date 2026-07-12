package core

import "testing"

func TestFreshnessAffectedAndUnrelated(t *testing.T) {
	amendment := Amendment{ChangeID: "chg", AffectedIDs: []string{"R2"}, Rationale: "change", RequiredRechecks: []string{"design"}}
	report := EvaluateFreshness([]FreshnessRecord{{Key: "approval:design", Kind: "design", DependsOn: []string{"R2"}}, {Key: "approval:requirements", Kind: "requirements", DependsOn: []string{"R1"}}}, []Amendment{amendment})
	if len(report.Stale) != 1 || report.Stale[0] != "approval:design" || len(report.Current) != 1 || report.Current[0] != "approval:requirements" {
		t.Fatalf("freshness report = %+v", report)
	}
}

func TestFreshnessDigestChange(t *testing.T) {
	a := Amendment{ChangeID: "chg", AffectedIDs: []string{"R1"}, Rationale: "change", BeforeDigests: map[string]string{"R1": "old"}, AfterDigests: map[string]string{"R1": "new"}, RequiredRechecks: []string{"requirements"}}
	if RecordIsFresh(FreshnessRecord{Key: "approval:requirements", Kind: "requirements", SourceDigest: "old", DependsOn: []string{"R1"}}, []Amendment{a}) {
		t.Fatal("record with changed source digest marked current")
	}
}

func TestFreshnessNewerApprovalIsCurrent(t *testing.T) {
	a := Amendment{ChangeID: "chg", AffectedIDs: []string{"design"}, Rationale: "change", RequiredRechecks: []string{"design"}, RecordedRevision: 2}
	if !RecordIsFresh(FreshnessRecord{Key: "approval:design", Kind: "design", Revision: 3}, []Amendment{a}) {
		t.Fatal("newer re-approval remained stale")
	}
}
