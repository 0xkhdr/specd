package core

import "testing"

func TestEvalPolicyValidatesScorerAndAggregatesDeterministically(t *testing.T) {
	policy := EvalPolicy{Scorer: ScorerMetadata{Kind: ScorerLM, Provider: "p", Model: "m", PromptDigest: "prompt", Sampling: "temperature=0"}, MinSamples: 2, Repetitions: 2, Aggregation: AggregationMean, Threshold: 0.8, CriticalCases: []string{"critical"}}
	if findings := ValidateEvalPolicy(policy); len(findings) != 0 {
		t.Fatalf("valid policy findings=%+v", findings)
	}
	result := AggregateEval(policy, []EvalSample{{CaseID: "critical", Score: .9}, {CaseID: "ordinary", Score: .7}})
	if result.Status != EvalPolicyInsufficient || result.Score != .8 {
		t.Fatalf("result=%+v", result)
	}
	result = AggregateEval(policy, []EvalSample{{CaseID: "critical", Score: .9}, {CaseID: "ordinary", Score: .9}, {CaseID: "critical", Score: .9}, {CaseID: "ordinary", Score: .9}})
	if result.Status != EvalPolicyPass || result.Score != .9 {
		t.Fatalf("pass result=%+v", result)
	}
}

func TestEvalPolicyRejectsCriticalFailureAndUnknownScorer(t *testing.T) {
	policy := EvalPolicy{Scorer: ScorerMetadata{Kind: ScorerKind("vendor")}, MinSamples: 1, Repetitions: 1, Aggregation: AggregationMean, Threshold: .8, CriticalCases: []string{"critical"}}
	if findings := ValidateEvalPolicy(policy); len(findings) == 0 || findings[0].Code != "EVAL_SCORER_UNKNOWN" {
		t.Fatalf("findings=%+v", findings)
	}
	policy.Scorer.Kind = ScorerCode
	result := AggregateEval(policy, []EvalSample{{CaseID: "critical", Score: .7}, {CaseID: "ordinary", Score: .9}})
	if result.Status != EvalPolicyFail {
		t.Fatalf("critical failure result=%+v", result)
	}
}
