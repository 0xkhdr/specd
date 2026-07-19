package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	"github.com/0xkhdr/specd/internal/core/gates/security"
)

func runSecurityException(root string, args []string, flags map[string]string) error {
	if len(args) != 2 || (args[0] != "approve" && args[0] != "revoke") {
		return errors.New("usage: specd exception <approve|revoke> <finding> [governed exception fields]")
	}
	action := "suppress"
	if args[0] == "revoke" {
		action = "revoke"
	}
	// Evidence integrity and worker authority are constitutional constraints,
	// never policy findings an exception may suppress.
	scope := strings.ToLower(strings.TrimSpace(flags["scope"]))
	finding := strings.ToLower(strings.TrimSpace(args[1]))
	if strings.Contains(scope, "evidence") || strings.Contains(scope, "authority") || strings.Contains(finding, "evidence-integrity") || strings.Contains(finding, "worker-authority") {
		return errors.New("security exception cannot waive evidence integrity or broaden worker authority")
	}
	return security.AppendException(root, security.Exception{
		Finding: args[1], Action: action, Reason: flags["reason"], Ticket: flags["ticket"], Owner: flags["owner"], Scope: flags["scope"], Revision: flags["revision"], Environment: flags["environment"], IssuedAt: flags["issued-at"], ExpiresAt: flags["expires-at"], CompensatingControl: flags["control"], Approver: flags["approver"],
	})
}

func runCheck(root string, args []string, flags map[string]string) error {
	// `check --security` with no slug runs a repo-wide security scan independent
	// of any spec (the scanners read tracked files, not spec state). All other
	// forms require a slug.
	securityOnly := len(args) == 0 && flagEnabled(flags, "security") &&
		!flagEnabled(flags, "schema") && !flagEnabled(flags, "schema-only")
	if !securityOnly && len(args) != 1 {
		return errors.New("usage: specd check <slug> [--json] [--security] [--schema] [--schema-only]")
	}
	slug := ""
	if len(args) == 1 {
		slug = args[0]
	}
	findings := []gates.Finding{}
	if securityOnly {
		cfg, _ := core.LoadConfig(configPaths(root), getenv())
		findings = append(findings, security.GateFindings(security.Analyze(root, cfg.Security))...)
	} else if !flagEnabled(flags, "schema-only") {
		spec, err := loadSpec(root, slug)
		if err != nil {
			return err
		}
		cfg, diagnostics := core.LoadConfig(configPaths(root), getenv())
		for _, d := range diagnostics {
			if d.Severity == "error" {
				return fmt.Errorf("load config: %s", d.Message)
			}
		}
		policy, err := security.ResolvePolicy(cfg.Security)
		if err != nil {
			return err
		}
		registry := gates.CoreRegistry()
		if policy.Profile == "production" {
			registry = gates.CoreRegistryWith(security.New(security.ConfigForPolicy(cfg.Security, policy)))
		}
		findings = append(findings, registry.Run(buildCheckCtx(root, slug, spec, ""))...)
		if flagEnabled(flags, "security") || policy.Profile == "production" {
			result := security.Analyze(root, security.ConfigForPolicy(cfg.Security, policy))
			if policy.Profile != "production" {
				findings = append(findings, security.GateFindings(result)...)
			}
			if err := recordSecurity(root, slug, policy, result); err != nil {
				return err
			}
		}
	}
	if flagEnabled(flags, "schema") || flagEnabled(flags, "schema-only") {
		findings = append(findings, schemaFindings(root, slug)...)
	}
	if flagEnabled(flags, "json") {
		return json.NewEncoder(os.Stdout).Encode(findings)
	}
	for _, finding := range findings {
		fmt.Fprintf(os.Stdout, "%s %s: %s\n", finding.Severity, finding.Gate, finding.Message)
	}
	if gates.HasErrors(findings) {
		return errors.New("check failed")
	}
	return nil
}

func requiredRegistry(root string) (gates.Registry, security.PolicyV1, error) {
	cfg, diagnostics := core.LoadConfig(configPaths(root), getenv())
	for _, d := range diagnostics {
		if d.Severity == "error" {
			return gates.Registry{}, security.PolicyV1{}, fmt.Errorf("load config: %s", d.Message)
		}
	}
	policy, err := security.ResolvePolicy(cfg.Security)
	if err != nil {
		return gates.Registry{}, security.PolicyV1{}, err
	}
	if policy.Profile == "production" {
		return gates.CoreRegistryWith(security.New(security.ConfigForPolicy(cfg.Security, policy))), policy, nil
	}
	return gates.CoreRegistry(), policy, nil
}

// recordSecurity persists the security analysis under state.security so reports
// and history can consume it (spec 05 R6). A missing state.json (unscaffolded or
// non-spec slug) is not fatal — the gate still reports; recording is best-effort
// for consumers. Findings are stored verbatim, including allowlisted ones.
type securityEvidenceRecord struct {
	PolicyVersion   string             `json:"policy_version"`
	PolicyDigest    string             `json:"policy_digest"`
	SubjectHead     string             `json:"subject_head"`
	SubjectRevision int64              `json:"subject_revision"`
	Findings        []security.Finding `json:"findings"`
}

func recordSecurity(root, slug string, policy security.PolicyV1, result security.Result) error {
	statePath := core.StatePath(root, slug)
	if _, err := os.Stat(statePath); err != nil {
		return nil // nothing to record against; gate output already emitted
	}
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		if state.Records == nil {
			state.Records = map[string]json.RawMessage{}
		}
		record := securityEvidenceRecord{PolicyVersion: policy.PolicyVersion, PolicyDigest: policy.PolicyDigest, SubjectHead: gitHead(root), SubjectRevision: state.Revision, Findings: result.Findings}
		raw, err := json.Marshal(record)
		if err != nil {
			return struct{}{}, err
		}
		state.Records["security"] = raw
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	return err
}

func driverGuideForSpec(root, slug string) (core.DriverGuideV1, error) {
	guidance, err := guidanceForSpec(root, slug)
	if err != nil {
		return core.DriverGuideV1{}, err
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return core.DriverGuideV1{}, err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return core.DriverGuideV1{}, err
	}
	var frontier []string
	if requireTaskGate(root, slug) == nil {
		if rows, e := core.FrontierExcluding(spec.Tasks, taskStatus(spec.Tasks), nil); e == nil {
			for _, row := range rows {
				frontier = append(frontier, row.ID)
			}
		}
	}
	var approvals []string
	for _, name := range []string{"requirements", "design", "tasks", "complete"} {
		if _, ok := state.Records["approval:"+name]; ok {
			approvals = append(approvals, name)
		}
	}
	var blockers []core.DriverFinding
	for i, message := range guidance.Blockers {
		blockers = append(blockers, core.DriverFinding{Code: fmt.Sprintf("GATE_BLOCKER_%03d", i+1), Severity: "error", Ref: slug, Message: message, RecoveryAction: "fix artifact and run `specd check " + slug + "`"})
	}
	return core.ProjectDriverGuide(filepath.Clean(root), slug, state.Status, approvals, frontier, blockers), nil
}
