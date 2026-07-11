package core

import "testing"

func TestQualityContractExactClassAndCheck(t *testing.T) {
	c, err := ParseQualityContract(TaskRow{ID: "T1", Verify: "go test ./...", Evidence: "test/unit, output_eval/rubric-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Required) != 2 {
		t.Fatalf("contract = %+v", c)
	}
	records := []EvidenceEnvelopeV1{{EvidenceClass: EvidenceTest, CheckID: "unit", Verdict: EvalPass}, {EvidenceClass: EvidenceReview, CheckID: "rubric-v1", Verdict: EvalPass}}
	missing := MissingQualityEvidence(c, records)
	if len(missing) != 1 || missing[0].EvidenceClass != EvidenceOutputEval {
		t.Fatalf("cross-class record satisfied requirement: %+v", missing)
	}
}

func TestQualityContractLegacyAndMalformed(t *testing.T) {
	c, err := ParseQualityContract(TaskRow{ID: "T1", Verify: "go test ./..."})
	if err != nil || len(c.Required) != 0 {
		t.Fatalf("legacy = %+v, %v", c, err)
	}
	for _, value := range []string{"vibes/x", "test", "test/"} {
		if _, err := ParseQualityContract(TaskRow{ID: "T1", Evidence: value}); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
