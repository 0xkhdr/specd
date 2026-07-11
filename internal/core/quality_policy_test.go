package core

import "testing"

func TestQualityPolicyComposition(t *testing.T) {
	policy := QualityPolicy{
		TaskID: "T1",
		Required: []EvidenceRequirement{
			{EvidenceClass: EvidenceOutputEval, CheckID: "rubric"},
			{EvidenceClass: EvidenceTest, CheckID: "verify"},
		},
	}
	records := []EvidenceEnvelopeV1{
		{TaskID: "T1", EvidenceClass: EvidenceTest, CheckID: "verify", Verdict: EvalPass, SubjectRevision: "head"},
		{TaskID: "T1", EvidenceClass: EvidenceOutputEval, CheckID: "rubric", Verdict: EvalPass, SubjectRevision: "old"},
	}
	result := EvaluateQualityPolicy(policy, records, FreshnessSubject{Revision: "head"})
	if len(result.Missing) != 0 || len(result.Stale) != 1 || result.Stale[0].CheckID != "rubric" {
		t.Fatalf("result = %+v", result)
	}
}

func TestQualityPolicyStableValidation(t *testing.T) {
	policy := QualityPolicy{TaskID: "T1", Required: []EvidenceRequirement{
		{EvidenceClass: EvidenceTest, CheckID: "z"},
		{EvidenceClass: EvidenceTest, CheckID: "z"},
		{EvidenceClass: EvidenceClass("mystery"), CheckID: "a"},
	}}
	findings := ValidateQualityPolicy(policy, nil)
	if len(findings) != 2 || findings[0].Code > findings[1].Code {
		t.Fatalf("findings = %+v", findings)
	}
}
