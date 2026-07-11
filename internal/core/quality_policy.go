package core

import (
	"fmt"
	"sort"
	"strings"
)

// QualityPolicy is the deterministic, task-scoped proof contract evaluated by
// the quality gate. Required entries compose: every exact class/check pair
// must have fresh passing evidence; one class or check cannot substitute for
// another. Criteria and Checks add production coverage rules in R5.
type QualityPolicy struct {
	TaskID   string
	Required []EvidenceRequirement
	Checks   []QualityCheck
	Criteria []AcceptanceCriterion
}

// QualityPolicyFinding is stable machine-readable policy output.
type QualityPolicyFinding struct {
	Code    string
	TaskID  string
	Message string
}

// ValidateQualityPolicy rejects malformed proof composition. Findings sort by
// code then message so map/input iteration cannot alter gate output.
func ValidateQualityPolicy(policy QualityPolicy, knownChecks map[string]bool) []QualityPolicyFinding {
	seen := map[string]bool{}
	var findings []QualityPolicyFinding
	for _, req := range policy.Required {
		key := string(req.EvidenceClass) + "/" + req.CheckID
		switch req.EvidenceClass {
		case EvidenceTest, EvidenceOutputEval, EvidenceTrajectoryEval, EvidenceReview:
		default:
			findings = append(findings, qualityFinding("EVIDENCE_CLASS_UNKNOWN", policy.TaskID, fmt.Sprintf("unknown evidence class %q", req.EvidenceClass)))
		}
		if strings.TrimSpace(req.CheckID) == "" {
			findings = append(findings, qualityFinding("CHECK_ID_REQUIRED", policy.TaskID, "required evidence has empty check id"))
		} else if knownChecks != nil && !knownChecks[req.CheckID] {
			findings = append(findings, qualityFinding("CHECK_ID_UNKNOWN", policy.TaskID, fmt.Sprintf("unknown check %q", req.CheckID)))
		}
		if seen[key] {
			findings = append(findings, qualityFinding("EVIDENCE_REQUIREMENT_DUPLICATE", policy.TaskID, fmt.Sprintf("duplicate evidence requirement %s", key)))
		}
		seen[key] = true
	}
	sortQualityFindings(findings)
	return findings
}

// EvaluateQualityPolicy applies exact composition and freshness using the W2
// evidence predicate. It is pure and never invokes an adapter or scorer.
func EvaluateQualityPolicy(policy QualityPolicy, records []EvidenceEnvelopeV1, subject FreshnessSubject) QualityStatus {
	return EvaluateQuality(QualityContract{TaskID: policy.TaskID, Required: policy.Required}, records, subject)
}

func qualityFinding(code, taskID, detail string) QualityPolicyFinding {
	return QualityPolicyFinding{Code: code, TaskID: taskID, Message: code + ": " + detail}
}

func sortQualityFindings(findings []QualityPolicyFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		if findings[i].TaskID != findings[j].TaskID {
			return findings[i].TaskID < findings[j].TaskID
		}
		return findings[i].Message < findings[j].Message
	})
}
