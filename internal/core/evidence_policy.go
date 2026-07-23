package core

import (
	"fmt"
	"strings"
)

type EvidencePolicyFinding struct{ Message string }

func BoundaryEvidenceFindings(design DesignDoc, tasks []TaskRow, production bool) []EvidencePolicyFinding {
	boundary, ok := externalBoundary(design)
	if !production || !ok {
		return nil
	}
	integration, errorPath := false, false
	for _, task := range tasks {
		// R7.1: integration intent is read from the SAME parse the
		// quality-declaration gate uses (ParseQualityContract), so a cell one gate
		// rejects can never secretly satisfy the other. A malformed cell — which
		// quality-declaration refuses at the tasks gate — yields no integration
		// credit here rather than a divergent bare-token acceptance.
		if contract, err := ParseQualityContract(task); err == nil {
			for _, req := range contract.Required {
				if EvidenceSatisfiesIntegration(req) {
					integration = true
				}
			}
		}
		checks, _ := SplitTaskField(task.Checks)
		joined := strings.ToLower(strings.Join(checks, ","))
		if strings.Contains(joined, "error") || strings.Contains(joined, "failure") || strings.Contains(joined, "negative") {
			errorPath = true
		}
	}
	var findings []EvidencePolicyFinding
	if !integration {
		findings = append(findings, EvidencePolicyFinding{Message: fmt.Sprintf("external boundary %q lacks integration evidence; satisfy it with %s", boundary, IntegrationEvidenceForms())})
	}
	if !errorPath {
		findings = append(findings, EvidencePolicyFinding{Message: fmt.Sprintf("external boundary %q lacks error-path negative check planning; add a checks entry naming an error/failure/negative path", boundary)})
	}
	return findings
}

// externalBoundary returns a stable descriptor of the first external boundary
// the design declares, or ok=false when none does (spec R7.3: a refusal names
// the boundary it inspected). Field names are visited in sorted order and
// markers in declared order, so the descriptor is deterministic.
func externalBoundary(design DesignDoc) (string, bool) {
	for _, name := range []string{"boundaries", "failure", "integration", "interfaces"} {
		value, ok := design.Fields[name]
		if !ok {
			continue
		}
		lower := strings.ToLower(value)
		for _, marker := range []string{"external", "integration", "network", "api", "http", "rpc", "database", "webhook", "third-party"} {
			if strings.Contains(lower, marker) {
				return name + ":" + marker, true
			}
		}
	}
	return "", false
}
