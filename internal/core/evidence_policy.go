package core

import "strings"

type EvidencePolicyFinding struct{ Message string }

func BoundaryEvidenceFindings(design DesignDoc, tasks []TaskRow, production bool) []EvidencePolicyFinding {
	if !production || !declaresExternalBoundary(design) {
		return nil
	}
	integration, errorPath := false, false
	for _, task := range tasks {
		for _, class := range strings.FieldsFunc(task.Evidence, func(r rune) bool { return r == ',' || r == ';' }) {
			if strings.EqualFold(strings.TrimSpace(class), "integration") {
				integration = true
			}
		}
		checks := strings.ToLower(task.Checks)
		if strings.Contains(checks, "error") || strings.Contains(checks, "failure") || strings.Contains(checks, "negative") {
			errorPath = true
		}
	}
	var findings []EvidencePolicyFinding
	if !integration {
		findings = append(findings, EvidencePolicyFinding{Message: "external boundary lacks integration evidence planning"})
	}
	if !errorPath {
		findings = append(findings, EvidencePolicyFinding{Message: "external boundary lacks error-path negative check planning"})
	}
	return findings
}

func declaresExternalBoundary(design DesignDoc) bool {
	for name, value := range design.Fields {
		if name != "boundaries" && name != "interfaces" && name != "integration" && name != "failure" {
			continue
		}
		lower := strings.ToLower(value)
		for _, marker := range []string{"external", "integration", "network", "api", "http", "rpc", "database", "webhook", "third-party"} {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}
	return false
}
