package core

import (
	"strings"
	"testing"
)

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

func TestQualityContractMinimalAndMalformed(t *testing.T) {
	c, err := ParseQualityContract(TaskRow{ID: "T1", Verify: "go test ./..."})
	if err != nil || len(c.Required) != 0 {
		t.Fatalf("minimal = %+v, %v", c, err)
	}
	for _, value := range []string{"vibes/x", "test", "test/"} {
		_, err := ParseQualityContract(TaskRow{ID: "T1", Evidence: value})
		if err == nil {
			t.Fatalf("accepted %q", value)
		}
		// R1.3: every parse error is self-documenting — it lists the full class
		// enum and the class/check-id format with an example.
		for _, want := range []string{"test, output_eval, trajectory_eval, review", "class/check-id", "test/unit-auth"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error for %q missing %q: %v", value, want, err)
			}
		}
	}
}

func TestQualityContractCarriesVerifyCommand(t *testing.T) {
	c, err := ParseQualityContract(TaskRow{ID: "T1", Verify: "go test ./...", Evidence: "test/unit"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Verify != "go test ./..." {
		t.Fatalf("verify = %q", c.Verify)
	}
}
