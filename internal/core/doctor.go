package core

import (
	"os"
	"path/filepath"
	"sort"
)

var planManagedRepair = PlanManagedRepair

// DoctorResultV1 is the versioned, machine-readable diagnostic envelope.
// Findings is always encoded as an array, including for a healthy project.
type DoctorResultV1 struct {
	ProtocolVersion string          `json:"protocol_version"`
	Healthy         bool            `json:"healthy"`
	Findings        []DriverFinding `json:"findings"`
	NextAction      string          `json:"next_action"`
}

// Doctor inspects agent-driving prerequisites and writes nothing.
func Doctor(root, pinned string) DoctorResultV1 {
	findings := make([]DriverFinding, 0)
	for _, rel := range []string{".specd", ".specd/roles", ".specd/steering", ".specd/specs", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			findings = append(findings, DriverFinding{Code: "LAYOUT_MISSING", Severity: "error", Ref: rel, Message: "required agent-driving layout is missing", RecoveryAction: "run `specd init --repair`"})
		}
	}
	if changes, err := planManagedRepair(root); err == nil {
		for _, change := range changes {
			findings = append(findings, DriverFinding{Code: "MANAGED_GUIDANCE_DRIFT", Severity: "error", Ref: change.RelPath, Message: "managed guidance differs from this binary", RecoveryAction: "run `specd init --refresh` and re-bootstrap"})
		}
	} else {
		findings = append(findings, DriverFinding{Code: "MANAGED_REPAIR_UNAVAILABLE", Severity: "error", Ref: ".specd", Message: err.Error(), RecoveryAction: "repair managed-layout permissions or filesystem errors, then run `specd agents doctor --json` again"})
	}
	if _, err := ResolveSpec(root, "", pinned); err != nil {
		code := FindingCode(err)
		if code != "SPEC_REQUIRED" || pinned != "" {
			findings = append(findings, DriverFinding{Code: code, Severity: "error", Ref: pinned, Message: err.Error(), RecoveryAction: "choose one valid spec explicitly"})
		}
	}
	resolution, resolveErr := ResolveConfigSource(root)
	if resolveErr != nil {
		findings = append(findings, DriverFinding{Code: "CONFIG_INVALID", Severity: "error", Ref: root, Message: resolveErr.Error(), RecoveryAction: "repair configuration sources, then run `specd agents doctor --json` again"})
	}
	config, diagnostics := LoadConfig(ConfigPaths{Project: resolution.SelectedPath}, nil)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			findings = append(findings, DriverFinding{Code: "CONFIG_INVALID", Severity: "error", Ref: diagnostic.Path, Message: diagnostic.Message, RecoveryAction: "repair configuration, then run `specd agents doctor --json` again"})
		}
	}
	if config.Orchestration.Enabled {
		workers := WorkerDefinitions{Root: root, Harness: config.Agent}
		missing, invalid := workers.Problems()
		for _, ref := range missing {
			findings = append(findings, DriverFinding{Code: "WORKER_DEFINITION_MISSING", Severity: "error", Ref: ref, Message: "orchestration is enabled but the handshake agent " + config.Agent + " has no aligned worker definition", RecoveryAction: "run `specd init --repair`"})
		}
		for _, ref := range invalid {
			findings = append(findings, DriverFinding{Code: "WORKER_HARNESS_MISMATCH", Severity: "error", Ref: ref, Message: "orchestration worker definitions are inconsistent with handshake agent " + config.Agent, RecoveryAction: "run `specd init --repair`"})
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Ref < findings[j].Ref
	})
	nextAction := "repair findings, then run `specd agents doctor --json` again"
	if len(findings) == 0 {
		nextAction = "run `specd agents guide <slug> --json`"
	}
	return DoctorResultV1{
		ProtocolVersion: DriverProtocolVersion,
		Healthy:         len(findings) == 0,
		Findings:        findings,
		NextAction:      nextAction,
	}
}
