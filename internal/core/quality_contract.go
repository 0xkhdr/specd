package core

import (
	"fmt"
	"sort"
	"strings"
)

type EvidenceRequirement struct {
	EvidenceClass EvidenceClass `json:"evidence_class"`
	CheckID       string        `json:"check_id"`
}
type QualityContract struct {
	TaskID   string                `json:"task_id"`
	Verify   string                `json:"verify,omitempty"`
	Required []EvidenceRequirement `json:"required"`
	Checks   []string              `json:"checks,omitempty"`
}

// qualityDeclarationHint makes every quality-declaration parse error
// self-documenting (spec R1.3): the same text reaches complete-task, check,
// and approval, so an agent can repair the cell without further lookup.
func qualityDeclarationHint() string {
	classes := []string{string(EvidenceTest), string(EvidenceOutputEval), string(EvidenceTrajectoryEval), string(EvidenceReview)}
	return fmt.Sprintf("valid classes: %s; expected format: class/check-id (example: test/unit-auth)", strings.Join(classes, ", "))
}

func ParseQualityContract(task TaskRow) (QualityContract, error) {
	c := QualityContract{TaskID: task.ID, Verify: task.Verify, Checks: splitCanonical(task.Checks)}
	seen := map[string]bool{}
	for _, raw := range splitCanonical(task.Evidence) {
		class, check, ok := strings.Cut(raw, "/")
		if !ok || check == "" {
			return QualityContract{}, fmt.Errorf("QUALITY_DECLARATION_INVALID: %q must be class/check-id (%s)", raw, qualityDeclarationHint())
		}
		req := EvidenceRequirement{EvidenceClass: EvidenceClass(class), CheckID: check}
		switch req.EvidenceClass {
		case EvidenceTest, EvidenceOutputEval, EvidenceTrajectoryEval, EvidenceReview:
		default:
			return QualityContract{}, fmt.Errorf("QUALITY_CLASS_UNKNOWN: %q (%s)", class, qualityDeclarationHint())
		}
		key := class + "/" + check
		if seen[key] {
			return QualityContract{}, fmt.Errorf("QUALITY_DECLARATION_DUPLICATE: %s", key)
		}
		seen[key] = true
		c.Required = append(c.Required, req)
	}
	sort.Slice(c.Required, func(i, j int) bool {
		if c.Required[i].EvidenceClass != c.Required[j].EvidenceClass {
			return c.Required[i].EvidenceClass < c.Required[j].EvidenceClass
		}
		return c.Required[i].CheckID < c.Required[j].CheckID
	})
	sort.Strings(c.Checks)
	return c, nil
}

// IntegrationEquivalentClasses are the evidence classes that count as
// integration evidence at an external boundary (spec R7.2): a trajectory eval
// exercises the wired system end to end, so it is integration-equivalent to a
// check id that names integration.
var IntegrationEquivalentClasses = map[EvidenceClass]bool{EvidenceTrajectoryEval: true}

// EvidenceSatisfiesIntegration reports whether one parsed evidence requirement
// counts as integration evidence at a boundary: its class is
// integration-equivalent, or its check id names integration (spec R7.2). It
// reads the SAME parsed requirement ParseQualityContract produces, so the
// boundary-evidence gate and the quality-declaration gate can never disagree
// about a cell (spec R7.1).
func EvidenceSatisfiesIntegration(req EvidenceRequirement) bool {
	return IntegrationEquivalentClasses[req.EvidenceClass] || strings.Contains(strings.ToLower(req.CheckID), "integration")
}

// IntegrationEvidenceForms names the exact evidence forms that satisfy an
// integration boundary, so a refusal always carries a nameable remedy (spec
// R7.2/R7.3).
func IntegrationEvidenceForms() string {
	return `a trajectory_eval evidence class, or an evidence check id containing "integration" (e.g. test/integration-payments)`
}

func MissingQualityEvidence(c QualityContract, records []EvidenceEnvelopeV1) []EvidenceRequirement {
	passed := map[string]bool{}
	for _, record := range records {
		if record.Verdict == EvalPass {
			passed[string(record.EvidenceClass)+"/"+record.CheckID] = true
		}
	}
	var missing []EvidenceRequirement
	for _, req := range c.Required {
		if !passed[string(req.EvidenceClass)+"/"+req.CheckID] {
			missing = append(missing, req)
		}
	}
	return missing
}
