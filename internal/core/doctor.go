package core

import (
	"os"
	"path/filepath"
	"sort"
)

// Doctor inspects agent-driving prerequisites and writes nothing.
func Doctor(root, pinned string) []DriverFinding {
	var findings []DriverFinding
	for _, rel := range []string{".specd", ".specd/roles", ".specd/steering", ".specd/specs", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			findings = append(findings, DriverFinding{Code: "LAYOUT_MISSING", Severity: "error", Ref: rel, Message: "required agent-driving layout is missing", RecoveryAction: "run `specd init --repair`"})
		}
	}
	if changes, err := PlanManagedRepair(root); err == nil {
		for _, change := range changes {
			findings = append(findings, DriverFinding{Code: "MANAGED_GUIDANCE_DRIFT", Severity: "error", Ref: change.RelPath, Message: "managed guidance differs from this binary", RecoveryAction: "run `specd init --refresh` and re-bootstrap"})
		}
	}
	if _, err := ResolveSpec(root, "", pinned); err != nil {
		code := FindingCode(err)
		if code != "SPEC_REQUIRED" || pinned != "" {
			findings = append(findings, DriverFinding{Code: code, Severity: "error", Ref: pinned, Message: err.Error(), RecoveryAction: "choose one valid spec explicitly"})
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Ref < findings[j].Ref
	})
	return findings
}
