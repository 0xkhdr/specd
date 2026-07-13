package core

import (
	"strings"
	"testing"
)

func TestPreventiveEvidence(t *testing.T) {
	complete := IncidentPreventionV1{Kind: PreventionRegressionTest, Owner: "payments-sre", EvidenceRef: "verify://payments/T12", WhyCaughtRef: "decision://payments/ADR-7"}
	if err := ValidatePreventiveEvidence(complete, true); err != nil {
		t.Fatalf("complete preventive evidence rejected: %v", err)
	}
	for name, record := range map[string]IncidentPreventionV1{
		"missing-evidence":  {Kind: PreventionRegressionTest, Owner: "payments-sre", WhyCaughtRef: "decision://payments/ADR-7"},
		"missing-rationale": {Kind: PreventionEval, Owner: "payments-sre", EvidenceRef: "eval://payments/regression"},
		"missing-owner":     {Kind: PreventionEval, EvidenceRef: "eval://payments/regression", WhyCaughtRef: "decision://payments/ADR-7"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidatePreventiveEvidence(record, true); err == nil {
				t.Fatal("required prevention accepted incomplete record")
			}
			if err := ValidatePreventiveEvidence(record, false); err != nil {
				t.Fatalf("default profile lost backward compatibility: %v", err)
			}
		})
	}
	if err := ValidatePreventiveEvidence(IncidentPreventionV1{Kind: "script"}, false); err == nil {
		t.Fatal("unknown prevention kind accepted when profile is unarmed")
	}

	root := t.TempDir()
	if err := RecordIncidentPrevention(root, "payments-recovery", complete); err != nil {
		t.Fatal(err)
	}
	second := complete
	second.EvidenceRef = "eval://payments/recheck"
	if err := RecordIncidentPrevention(root, "payments-recovery", second); err != nil {
		t.Fatal(err)
	}
	records, err := LoadIncidentPrevention(IncidentPreventionPath(root, "payments-recovery"))
	if err != nil || len(records) != 2 || records[0].EvidenceRef != complete.EvidenceRef || records[1].EvidenceRef != second.EvidenceRef {
		t.Fatalf("append-only prevention history = %+v, err=%v", records, err)
	}
}

func TestIncidentOriginalImmutable(t *testing.T) {
	seed := IncidentSeed{SourceSpec: "checkout", ReleaseID: "rel-7", DeploymentID: "dep-4", CriterionID: "availability", EvidenceRefs: []string{"obs://health/42"}}
	plan, err := PlanIncidentSuccessor("checkout-recovery", seed)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Link.From != "checkout-recovery" || plan.Link.To != "checkout" || plan.Link.Kind != LinkKindRegresses {
		t.Fatalf("typed successor link = %+v", plan.Link)
	}
	if len(plan.Provenance.PriorLinks) != 1 || plan.Provenance.PriorLinks[0].To != "checkout" || plan.Provenance.PriorLinks[0].Kind != LinkKindRegresses {
		t.Fatalf("provenance link = %+v", plan.Provenance.PriorLinks)
	}
}

func TestIncidentRedaction(t *testing.T) {
	base := IncidentSeed{SourceSpec: "checkout", ReleaseID: "rel-7", DeploymentID: "dep-4", CriterionID: "availability", EvidenceRefs: []string{"obs://health/42"}}
	if err := ValidateIncidentSeed(base); err != nil {
		t.Fatal(err)
	}
	bad := []IncidentSeed{
		{SourceSpec: base.SourceSpec, ReleaseID: strings.Repeat("x", 129), DeploymentID: base.DeploymentID, CriterionID: base.CriterionID, EvidenceRefs: base.EvidenceRefs},
		{SourceSpec: base.SourceSpec, ReleaseID: base.ReleaseID, DeploymentID: base.DeploymentID, CriterionID: base.CriterionID, EvidenceRefs: []string{"https://user:secret@example.test/raw"}},
		{SourceSpec: base.SourceSpec, ReleaseID: base.ReleaseID, DeploymentID: base.DeploymentID, CriterionID: base.CriterionID, EvidenceRefs: []string{"obs://health/42?token=secret"}},
	}
	for _, seed := range bad {
		if err := ValidateIncidentSeed(seed); err == nil {
			t.Fatalf("unsafe incident seed accepted: %+v", seed)
		}
	}
}
