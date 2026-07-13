package core

import (
	"sort"
)

type ScorerKind string

const (
	ScorerCode      ScorerKind = "code"
	ScorerHuman     ScorerKind = "human"
	ScorerHeuristic ScorerKind = "heuristic"
	ScorerLM        ScorerKind = "lm"
)

type ScorerMetadata struct {
	Kind         ScorerKind `json:"kind"`
	Provider     string     `json:"provider,omitempty"`
	Model        string     `json:"model,omitempty"`
	PromptDigest string     `json:"prompt_digest,omitempty"`
	Sampling     string     `json:"sampling,omitempty"`
}

type Aggregation string

const (
	AggregationMean Aggregation = "mean"
	AggregationMin  Aggregation = "min"
)

type EvalPolicy struct {
	Scorer        ScorerMetadata `json:"scorer"`
	MinSamples    int            `json:"min_samples"`
	Repetitions   int            `json:"repetitions"`
	Aggregation   Aggregation    `json:"aggregation"`
	Threshold     float64        `json:"threshold"`
	CriticalCases []string       `json:"critical_cases,omitempty"`
}

type EvalSample struct {
	CaseID string  `json:"case_id"`
	Score  float64 `json:"score"`
}

type EvalPolicyStatus string

const (
	EvalPolicyPass         EvalPolicyStatus = "pass"
	EvalPolicyFail         EvalPolicyStatus = "fail"
	EvalPolicyInsufficient EvalPolicyStatus = "insufficient"
)

type EvalAggregation struct {
	Status EvalPolicyStatus `json:"status"`
	Score  float64          `json:"score"`
}

type EvalPolicyFinding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ValidateEvalPolicy(p EvalPolicy) []EvalPolicyFinding {
	findings := []EvalPolicyFinding{}
	switch p.Scorer.Kind {
	case ScorerCode, ScorerHuman, ScorerHeuristic, ScorerLM:
	default:
		findings = append(findings, EvalPolicyFinding{"EVAL_SCORER_UNKNOWN", "unknown scorer kind"})
	}
	if p.Scorer.Kind == ScorerLM && (p.Scorer.Provider == "" || p.Scorer.Model == "" || p.Scorer.PromptDigest == "") {
		findings = append(findings, EvalPolicyFinding{"EVAL_LM_METADATA_REQUIRED", "LM scorer requires provider, model, and prompt digest"})
	}
	if p.MinSamples < 1 || p.Repetitions < 1 {
		findings = append(findings, EvalPolicyFinding{"EVAL_SAMPLE_POLICY_INVALID", "min_samples and repetitions must be positive"})
	}
	if p.Aggregation != AggregationMean && p.Aggregation != AggregationMin {
		findings = append(findings, EvalPolicyFinding{"EVAL_AGGREGATION_UNKNOWN", "unknown aggregation"})
	}
	if p.Threshold < 0 || p.Threshold > 1 {
		findings = append(findings, EvalPolicyFinding{"EVAL_THRESHOLD_INVALID", "threshold must be between 0 and 1"})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Code < findings[j].Code })
	return findings
}

// AggregateEval consumes fixed imported samples only. It never scores or calls
// an adapter; insufficient samples and critical failures cannot pass.
func AggregateEval(p EvalPolicy, samples []EvalSample) EvalAggregation {
	if len(samples) == 0 {
		return EvalAggregation{Status: EvalPolicyInsufficient}
	}
	critical := map[string]bool{}
	for _, id := range p.CriticalCases {
		critical[id] = true
	}
	total := 0.0
	min := 1.0
	for _, s := range samples {
		if critical[s.CaseID] && s.Score < p.Threshold {
			return EvalAggregation{Status: EvalPolicyFail, Score: s.Score}
		}
		total += s.Score
		if s.Score < min {
			min = s.Score
		}
	}
	score := total / float64(len(samples))
	if p.Aggregation == AggregationMin {
		score = min
	}
	if len(samples) < p.MinSamples*p.Repetitions {
		return EvalAggregation{Status: EvalPolicyInsufficient, Score: score}
	}
	status := EvalPolicyFail
	if score >= p.Threshold {
		status = EvalPolicyPass
	}
	return EvalAggregation{Status: status, Score: score}
}
