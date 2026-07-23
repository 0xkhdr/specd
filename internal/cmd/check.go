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
		return usageError("exception")
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
		return usageError("check")
	}
	if securityOnly || flagEnabled(flags, "security") || flagEnabled(flags, "schema") || flagEnabled(flags, "schema-only") {
		return runDiagnosticCheck(root, args, flags, securityOnly)
	}

	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return core.Refusef("SPEC_INVALID", "%v", err)
	}
	var result readinessResult
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		_, cfg, err := requiredRegistry(root)
		if err != nil {
			return struct{}{}, err
		}
		policy, err := security.ResolvePolicy(cfg.Security)
		if err != nil {
			return struct{}{}, err
		}
		if policy.Profile == "production" {
			if err := recordSecurity(root, slug, policy, security.Analyze(root, security.ConfigForPolicy(cfg.Security, policy))); err != nil {
				return struct{}{}, err
			}
		}
		state, err := core.LoadState(core.StatePath(root, slug))
		if err != nil {
			return struct{}{}, err
		}
		result, err = buildReadiness(root, slug, state)
		return struct{}{}, err
	})
	if err != nil {
		return err
	}
	stateChanged := false
	if policy, policyErr := security.ResolvePolicy(loadSpecConfig(root).Security); policyErr == nil {
		stateChanged = policy.Profile == "production"
	}
	if flags["json"] == "legacy" {
		return json.NewEncoder(os.Stdout).Encode(result.Findings)
	}
	if flagEnabled(flags, "json") {
		if err := json.NewEncoder(os.Stdout).Encode(result.Envelope); err != nil {
			return err
		}
		if gates.HasErrors(result.Findings) {
			return readinessRefusal(slug, result, stateChanged)
		}
		return nil
	}
	for _, finding := range result.Findings {
		fmt.Fprintf(os.Stdout, "%s %s: %s\n", finding.Severity, finding.Gate, finding.Message)
	}
	if gates.HasErrors(result.Findings) {
		return readinessRefusal(slug, result, stateChanged)
	}
	plan := result.Envelope.Plan
	if plan.Terminal {
		fmt.Fprintf(os.Stdout, "checked %s: terminal at %s revision %d plan %s config %s gates %d readiness_checked=true\n", slug, plan.Current, plan.StateRevision, plan.PlanDigest, plan.ConfigDigest, len(plan.ArmedGates))
	} else {
		fmt.Fprintf(os.Stdout, "ready %s: %s → %s revision %d plan %s config %s gates %d readiness_checked=true\n", slug, plan.Current, plan.Target, plan.StateRevision, plan.PlanDigest, plan.ConfigDigest, len(plan.ArmedGates))
	}
	for _, artifact := range plan.ArtifactDigests {
		fmt.Fprintf(os.Stdout, "artifact %s %s\n", artifact.ID, artifact.Digest)
	}
	return nil
}

func readinessRefusal(slug string, result readinessResult, stateChanged bool) error {
	plan := result.Envelope.Plan
	codes := make([]string, 0, len(plan.Blockers))
	for _, blocker := range plan.Blockers {
		codes = append(codes, blocker.Code)
	}
	inputs := map[string]string{"config": plan.ConfigDigest, "policy": plan.PolicyDigest}
	for _, input := range append(append([]core.TransitionDigest{}, plan.Inputs...), plan.ArtifactDigests...) {
		inputs[input.ID] = input.Digest
	}
	return core.Refusef("GATE_FAILED", "readiness plan %s has %d blocker(s)", plan.PlanDigest, len(plan.Blockers)).
		WithContext(slug, strings.Join(codes, ","), "no blocking gate findings").
		WithInputDigests(inputs).
		WithMutation(stateChanged, "").
		WithRecovery(core.RefusalActorAgent, "specd check "+slug)
}

type readinessResult struct {
	Envelope core.ReadinessEnvelope
	Findings []gates.Finding
}

func buildReadiness(root, slug string, state core.State) (readinessResult, error) {
	spec, err := loadSpec(root, slug)
	if err != nil {
		return readinessResult{}, err
	}
	registry, cfg, err := requiredRegistry(root)
	if err != nil {
		return readinessResult{}, err
	}
	target := core.NextStatus(state.Status)
	readinessGate := string(target)
	if state.Status == core.StatusRequirements || state.Status == core.StatusDesign {
		readinessGate = string(state.Status)
	}
	armedGates := registry.Names()
	findings := registry.Run(buildCheckCtx(root, slug, spec, readinessGate))
	if target == core.StatusExecuting {
		armedGates = append(armedGates, "program-dependencies")
		program, err := core.LoadProgram(core.ProgramPath(root))
		if err != nil {
			return readinessResult{}, err
		}
		if blocking := program.IncompleteDeps(slug, func(dep string) bool { return specComplete(root, dep) }); len(blocking) > 0 {
			findings = append(findings, gates.Finding{Gate: "program-dependencies", Severity: gates.Error, Message: fmt.Sprintf("%s blocked by incomplete dependencies: %s", slug, strings.Join(blocking, ", "))})
		}
	}
	artifacts, inputs, err := transitionDigests(root, slug, state)
	if err != nil {
		return readinessResult{}, err
	}
	input := core.TransitionInput{
		Current:               state.Status,
		Target:                target,
		StateRevision:         state.Revision,
		Actor:                 core.ActorHuman,
		ActorAssurance:        "human-only-cli",
		ArmedGates:            armedGates,
		Inputs:                inputs,
		ArtifactDigests:       artifacts,
		ConfigDigest:          core.ConfigDigest(cfg),
		PolicyDigest:          core.PolicyDigest(cfg),
		TransportCapabilities: []string{"cli"},
		RequiredTransport:     []string{"cli"},
		ReadinessChecked:      true,
		MutationIntent:        core.TransitionMutationAdvanceStatus,
	}
	readinessFindings := make([]core.ReadinessFinding, 0, len(findings))
	for i, finding := range findings {
		finding.Message = actionableGateMessage(slug, finding)
		findings[i] = finding
		code := strings.ToUpper(strings.ReplaceAll(finding.Gate, "-", "_")) + "_GATE"
		item := core.TransitionBlocker{Code: code, Gate: finding.Gate, Message: finding.Message}
		readinessFindings = append(readinessFindings, core.ReadinessFinding{Gate: finding.Gate, Severity: string(finding.Severity), Message: finding.Message})
		if finding.Severity == gates.Error {
			input.Blockers = append(input.Blockers, item)
			input.Recoveries = append(input.Recoveries, core.TransitionRecovery{BlockerCode: code, Operation: "check", Actor: core.ActorAgent})
		} else {
			input.Warnings = append(input.Warnings, item)
		}
	}
	plan := core.BuildTransitionPlan(input)
	return readinessResult{
		Envelope: core.ReadinessEnvelope{SchemaVersion: core.ReadinessSchemaVersion, Plan: plan, Findings: readinessFindings},
		Findings: findings,
	}, nil
}

func actionableGateMessage(slug string, finding gates.Finding) string {
	if finding.Gate != "context-budget" || !strings.Contains(finding.Message, "required source contributions:") {
		return finding.Message
	}
	return fmt.Sprintf("%s; recovery: 1. the tasks.md owner edits .specd/specs/%s/tasks.md as directed; 2. an agent runs `specd check %s`; 3. a human runs `specd approve %s`", finding.Message, slug, slug, slug)
}

func transitionDigests(root, slug string, state core.State) (map[string]string, map[string]string, error) {
	artifacts := map[string]string{}
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, err
		}
		artifacts[name] = core.Digest(raw)
	}
	stateRaw, err := json.Marshal(state)
	if err != nil {
		return nil, nil, err
	}
	inputs := map[string]string{"state.json": core.Digest(stateRaw)}
	if raw, err := os.ReadFile(core.EvidencePath(root, slug)); err == nil {
		inputs["evidence.jsonl"] = core.Digest(raw)
	} else if !os.IsNotExist(err) {
		return nil, nil, err
	}
	return artifacts, inputs, nil
}

func runDiagnosticCheck(root string, args []string, flags map[string]string, securityOnly bool) error {
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
		registry := registryForConfig(cfg, policy)
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
	for i, finding := range findings {
		findings[i].Message = actionableGateMessage(slug, finding)
	}
	if flagEnabled(flags, "json") {
		if err := json.NewEncoder(os.Stdout).Encode(findings); err != nil {
			return err
		}
		return diagnosticCheckFailure(findings)
	}
	for _, finding := range findings {
		fmt.Fprintf(os.Stdout, "%s %s: %s\n", finding.Severity, finding.Gate, finding.Message)
	}
	return diagnosticCheckFailure(findings)
}

func diagnosticCheckFailure(findings []gates.Finding) error {
	if gates.HasErrors(findings) {
		return errors.New("check failed")
	}
	return nil
}

func requiredRegistry(root string) (gates.Registry, core.Config, error) {
	cfg, diagnostics := core.LoadConfig(configPaths(root), getenv())
	for _, d := range diagnostics {
		if d.Severity == "error" {
			return gates.Registry{}, core.Config{}, fmt.Errorf("load config: %s", d.Message)
		}
	}
	policy, err := security.ResolvePolicy(cfg.Security)
	if err != nil {
		return gates.Registry{}, core.Config{}, err
	}
	return registryForConfig(cfg, policy), cfg, nil
}

func registryForConfig(cfg core.Config, policy security.PolicyV1) gates.Registry {
	if policy.Profile == "production" {
		return gates.CoreRegistryWith(security.New(security.ConfigForPolicy(cfg.Security, policy)))
	}
	return gates.CoreRegistry()
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
		message = actionableGateMessage(slug, gates.Finding{Gate: "context-budget", Message: message})
		blockers = append(blockers, core.DriverFinding{Code: fmt.Sprintf("GATE_BLOCKER_%03d", i+1), Severity: "error", Ref: slug, Message: message, RecoveryAction: "fix artifact and run `specd check " + slug + "`"})
	}
	return core.ProjectDriverGuide(filepath.Clean(root), slug, state.Status, approvals, frontier, blockers), nil
}
