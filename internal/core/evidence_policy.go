package core

import "strings"

type EvidencePolicyFinding struct{ Message string }

func BoundaryEvidenceFindings(design DesignDoc, tasks []TaskRow, production bool) []EvidencePolicyFinding {
	if !production || !declaresExternalBoundary(design) {
		return nil
	}
	integration, errorPath := false, false
	for _, task := range tasks {
		// Evidence tokens come from the canonical splitter (spec 05 R1.1), and the
		// integration intent is read from the check-id half of `class/check-id` so
		// the canonical spelling `test/integration-payments` counts — previously
		// only a bare legacy `integration` token did.
		evidence, _ := SplitTaskField(task.Evidence)
		for _, token := range evidence {
			class, check, ok := strings.Cut(token, "/")
			if !ok {
				check = class
			}
			if strings.Contains(strings.ToLower(check), "integration") {
				integration = true
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
