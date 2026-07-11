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
	Required []EvidenceRequirement `json:"required"`
	Checks   []string              `json:"checks,omitempty"`
}

func ParseQualityContract(task TaskRow) (QualityContract, error) {
	c := QualityContract{TaskID: task.ID, Checks: splitTaskList(task.Checks)}
	seen := map[string]bool{}
	for _, raw := range splitTaskList(task.Evidence) {
		class, check, ok := strings.Cut(raw, "/")
		if !ok || check == "" {
			return QualityContract{}, fmt.Errorf("QUALITY_DECLARATION_INVALID: %q must be class/check-id", raw)
		}
		req := EvidenceRequirement{EvidenceClass: EvidenceClass(class), CheckID: check}
		switch req.EvidenceClass {
		case EvidenceTest, EvidenceOutputEval, EvidenceTrajectoryEval, EvidenceReview:
		default:
			return QualityContract{}, fmt.Errorf("QUALITY_CLASS_UNKNOWN: %q", class)
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
