package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestQualityGateDefaultParityAndComposition(t *testing.T) {
	task := core.TaskRow{ID: "T1", Role: "craftsman", Verify: "go test ./..."}
	if findings := QualityGate().Run(CheckCtx{Tasks: []core.TaskRow{task}}); len(findings) != 0 {
		t.Fatalf("default gate = %+v", findings)
	}

	ctx := CheckCtx{
		Tasks:                 []core.TaskRow{task},
		QualityPolicyRequired: true,
		QualityPolicies: map[string]core.QualityPolicy{
			"T1": {TaskID: "T1", Required: []core.EvidenceRequirement{{EvidenceClass: core.EvidenceTest, CheckID: "verify"}}},
		},
		QualitySubject: core.FreshnessSubject{Revision: "head"},
	}
	findings := QualityGate().Run(ctx)
	if !HasErrors(findings) || !strings.Contains(findings[0].Message, "QUALITY_EVIDENCE_MISSING") {
		t.Fatalf("missing evidence = %+v", findings)
	}
	ctx.Evals = []core.EvidenceEnvelopeV1{{TaskID: "T1", EvidenceClass: core.EvidenceTest, CheckID: "verify", Verdict: core.EvalPass, SubjectRevision: "old"}}
	findings = QualityGate().Run(ctx)
	if !HasErrors(findings) || !strings.Contains(findings[0].Message, "QUALITY_EVIDENCE_STALE") {
		t.Fatalf("stale evidence = %+v", findings)
	}
}

func TestQualityGateCriteria(t *testing.T) {
	ctx := CheckCtx{
		QualityPolicyRequired: true,
		KnownCriteria:         map[string]bool{"R5.1": true},
		QualityPolicies: map[string]core.QualityPolicy{
			"T1": {TaskID: "T1", Criteria: []core.AcceptanceCriterion{{ID: "R5.1", Critical: true}}},
		},
	}
	findings := QualityGate().Run(ctx)
	if !HasErrors(findings) || !strings.Contains(findings[0].Message, "CRITERION_UNCOVERED") {
		t.Fatalf("criteria findings = %+v", findings)
	}
}

func TestQualityGateVerifyStrengthAndReadOnlyException(t *testing.T) {
	ctx := CheckCtx{
		QualityPolicyRequired: true,
		Tasks: []core.TaskRow{
			{ID: "T1", Role: "craftsman", Risk: "critical", Verify: "go build ./..."},
			{ID: "T2", Role: "scout", Risk: "critical", Verify: "printf ok"},
		},
		QualityPolicies: map[string]core.QualityPolicy{
			"T1": {TaskID: "T1"},
			"T2": {TaskID: "T2"},
		},
	}
	findings := QualityGate().Run(ctx)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "VERIFY_COMPILE_ONLY") {
		t.Fatalf("findings = %+v", findings)
	}

	ctx.Tasks[0].Verify = "printf ok"
	findings = QualityGate().Run(ctx)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "VERIFY_TRIVIAL") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestQualityRegistryCompositionOrder(t *testing.T) {
	names := CoreRegistryWith(QualityGate()).Names()
	if names[len(names)-1] != "quality" {
		t.Fatalf("names = %v", names)
	}
}
