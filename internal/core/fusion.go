package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const FusionBootstrapVersion = 1

type FusionBootstrap struct {
	Version     int                    `json:"version"`
	Root        string                 `json:"root"`
	Load        []FusionLoadItem       `json:"load"`
	Commands    FusionCommandSummary   `json:"commands"`
	Config      FusionConfigSummary    `json:"config"`
	Health      FusionHealthSummary    `json:"health"`
	Modes       []FusionActiveSpecMode `json:"modes"`
	NextActions []string               `json:"nextActions"`
}

type FusionLoadItem struct {
	Path      string `json:"path"`
	Mode      string `json:"mode"`
	Rationale string `json:"rationale"`
}

type FusionCommandSummary struct {
	SchemaCommand string        `json:"schemaCommand"`
	Digest        string        `json:"digest"`
	Count         int           `json:"count"`
	Schema        []CommandMeta `json:"schema,omitempty"`
}

type FusionConfigSummary struct {
	Path                  string            `json:"path"`
	Digest                string            `json:"digest"`
	Roles                 FusionRolesConfig `json:"roles"`
	Orchestration         FusionOrchConfig  `json:"orchestration"`
	VerifySandbox         string            `json:"verifySandbox"`
	GateSeverities        map[string]string `json:"gateSeverities"`
	ConfigFilePresent     bool              `json:"configFilePresent"`
	EffectiveConfigSource string            `json:"effectiveConfigSource"`
}

type FusionRolesConfig struct {
	SubagentMode string `json:"subagentMode"`
}

type FusionOrchConfig struct {
	Enabled        bool   `json:"enabled"`
	ApprovalPolicy string `json:"approvalPolicy"`
	WorkerMode     string `json:"workerMode"`
}

type FusionHealthSummary struct {
	OK     bool                `json:"ok"`
	Checks []FusionHealthCheck `json:"checks"`
}

type FusionHealthCheck struct {
	Name    string   `json:"name"`
	OK      bool     `json:"ok"`
	Missing []string `json:"missing"`
	Message string   `json:"message,omitempty"`
}

type FusionActiveSpecMode struct {
	Slug   string `json:"slug"`
	Status string `json:"status"`
	Phase  string `json:"phase"`
	Mode   string `json:"mode"`
	Origin string `json:"origin"`
	Gate   string `json:"gate"`
}

type FusionPolicy struct {
	Version              int                `json:"version"`
	Root                 string             `json:"root"`
	SubagentMode         string             `json:"subagentMode"`
	OrchestrationEnabled bool               `json:"orchestrationEnabled"`
	ApprovalPolicy       string             `json:"approvalPolicy"`
	WorkerMode           string             `json:"workerMode"`
	MaxWorkers           int                `json:"maxWorkers"`
	MaxRetries           int                `json:"maxRetries"`
	TimeoutSeconds       int                `json:"timeoutSeconds"`
	VerifySandbox        string             `json:"verifySandbox"`
	GateSeverities       map[string]string  `json:"gateSeverities"`
	MCPExposure          FusionMCPExposure  `json:"mcpExposure"`
	ConfigDigest         string             `json:"configDigest"`
	ConfigFilePresent    bool               `json:"configFilePresent"`
	DigestMatch          *bool              `json:"digestMatch,omitempty"`
	ExpectedConfigDigest string             `json:"expectedConfigDigest,omitempty"`
	Spec                 *FusionSpecPolicy  `json:"spec,omitempty"`
	Diagnostics          []ConfigDiagnostic `json:"diagnostics"`
	Violations           []string           `json:"violations"`
	Recommendations      []string           `json:"recommendations"`
}

type FusionMCPExposure struct {
	Expose               string   `json:"expose"`
	EssentialTools       []string `json:"essentialTools,omitempty"`
	IncludeMeta          bool     `json:"includeMeta"`
	IncludeOrchestration *bool    `json:"includeOrchestration,omitempty"`
}

type FusionSpecPolicy struct {
	Slug            string `json:"slug"`
	SpecMode        string `json:"specMode"`
	ModeOrigin      string `json:"modeOrigin"`
	BrainAllowed    bool   `json:"brainAllowed"`
	BaseLoopAllowed bool   `json:"baseLoopAllowed"`
	Recommended     string `json:"recommendedCommandFamily"`
	NextCommand     string `json:"nextCommand"`
	PolicyViolation string `json:"policyViolation,omitempty"`
}

func BuildFusionPolicy(root, slug, expectDigest string) (FusionPolicy, error) {
	if root == "" {
		var ok bool
		root, ok = FindSpecdRoot("")
		if !ok {
			return FusionPolicy{}, NotFoundError("no .specd/ found in this directory or any parent. Run `specd init` first.")
		}
	}
	cfg, diags := LoadConfigStrict(root)
	digest, present := fileDigest(ConfigPath(root))
	policy := FusionPolicy{
		Version: FusionBootstrapVersion, Root: root,
		SubagentMode: cfg.Roles.SubagentMode, OrchestrationEnabled: cfg.Orchestration.Enabled,
		ApprovalPolicy: cfg.Orchestration.ApprovalPolicy, WorkerMode: cfg.Orchestration.WorkerMode,
		MaxWorkers: cfg.Orchestration.MaxWorkers, MaxRetries: cfg.Orchestration.MaxRetries,
		TimeoutSeconds: cfg.Orchestration.SessionTimeoutMinutes * 60,
		VerifySandbox:  cfg.Verify.Sandbox, GateSeverities: fusionGateSeverities(cfg),
		MCPExposure:  FusionMCPExposure{Expose: cfg.MCP.Expose, EssentialTools: cfg.MCP.EssentialTools, IncludeMeta: cfg.MCP.IncludeMeta, IncludeOrchestration: cfg.MCP.IncludeOrchestration},
		ConfigDigest: digest, ConfigFilePresent: present, Diagnostics: diags,
		Violations: []string{}, Recommendations: []string{},
	}
	if expectDigest != "" {
		match := digest == expectDigest
		policy.DigestMatch = &match
		policy.ExpectedConfigDigest = expectDigest
		if !match {
			policy.Violations = append(policy.Violations, "config digest mismatch")
			policy.Recommendations = append(policy.Recommendations, "rerun `specd fusion bootstrap --json`")
		}
	}
	if HasErrorDiagnostics(diags) {
		policy.Violations = append(policy.Violations, "invalid config")
	}
	if slug != "" {
		state, err := LoadState(root, slug)
		if err != nil {
			return FusionPolicy{}, err
		}
		mode, origin := ResolveMode("", state)
		specPolicy := FusionSpecPolicy{Slug: slug, SpecMode: mode, ModeOrigin: origin}
		switch mode {
		case ModeOrchestrated:
			if cfg.Orchestration.Enabled {
				specPolicy.BrainAllowed = true
				specPolicy.BaseLoopAllowed = false
				specPolicy.Recommended = "brain run or MCP brain_orchestrate"
				specPolicy.NextCommand = "specd brain run " + slug
			} else {
				specPolicy.BrainAllowed = false
				specPolicy.BaseLoopAllowed = false
				specPolicy.PolicyViolation = "spec is orchestrated but project orchestration is disabled"
				specPolicy.Recommended = "specd mode " + slug + " --set base or enable orchestration"
				specPolicy.NextCommand = "specd mode " + slug + " --set base"
				policy.Violations = append(policy.Violations, specPolicy.PolicyViolation)
			}
		default:
			specPolicy.BrainAllowed = false
			specPolicy.BaseLoopAllowed = true
			specPolicy.Recommended = "context/next/verify/task"
			specPolicy.NextCommand = "specd context " + slug
		}
		policy.Spec = &specPolicy
		policy.Recommendations = append(policy.Recommendations, specPolicy.Recommended)
	}
	return policy, nil
}

func BuildFusionBootstrap(root string, includeSchema bool) (FusionBootstrap, error) {
	if root == "" {
		var ok bool
		root, ok = FindSpecdRoot("")
		if !ok {
			return FusionBootstrap{}, NotFoundError("no .specd/ found in this directory or any parent. Run `specd init` first.")
		}
	}
	cfg := LoadConfig(root)
	commandsDigest, err := commandSchemaDigest()
	if err != nil {
		return FusionBootstrap{}, err
	}
	configDigest, configPresent := fileDigest(ConfigPath(root))
	checks := fusionHealthChecks(root)
	healthOK := true
	for _, check := range checks {
		if !check.OK {
			healthOK = false
			break
		}
	}
	commands := FusionCommandSummary{SchemaCommand: "specd help --json", Digest: commandsDigest, Count: len(Commands)}
	if includeSchema {
		commands.Schema = Commands
	}
	nextActions := []string{"Read all items in load before acting", "Run `specd help --json` to cache command schema", "Run `specd status` to orient active work"}
	if !healthOK {
		nextActions = append(nextActions, "Run `specd doctor --fix` or `specd init --repair` before mutating specs")
	}
	return FusionBootstrap{
		Version:  FusionBootstrapVersion,
		Root:     root,
		Load:     fusionLoadItems(),
		Commands: commands,
		Config: FusionConfigSummary{
			Path:              ".specd/config.json",
			Digest:            configDigest,
			Roles:             FusionRolesConfig{SubagentMode: cfg.Roles.SubagentMode},
			Orchestration:     FusionOrchConfig{Enabled: cfg.Orchestration.Enabled, ApprovalPolicy: cfg.Orchestration.ApprovalPolicy, WorkerMode: cfg.Orchestration.WorkerMode},
			VerifySandbox:     cfg.Verify.Sandbox,
			GateSeverities:    fusionGateSeverities(cfg),
			ConfigFilePresent: configPresent,
			EffectiveConfigSource: func() string {
				if configPresent {
					return "config.json+defaults"
				}
				return "defaults"
			}(),
		},
		Health:      FusionHealthSummary{OK: healthOK, Checks: checks},
		Modes:       fusionActiveSpecModes(root),
		NextActions: nextActions,
	}, nil
}

func fusionLoadItems() []FusionLoadItem {
	items := []struct{ path, why string }{
		{".specd/steering/reasoning.md", "reasoning constitution and response discipline"},
		{".specd/steering/workflow.md", "required lifecycle gates"},
		{".specd/steering/product.md", "product intent and constraints"},
		{".specd/steering/tech.md", "technology stack and verification policy"},
		{".specd/steering/structure.md", "repository layout and edit boundaries"},
		{"AGENTS.md", "agent operating contract and host-specific rules"},
	}
	out := make([]FusionLoadItem, 0, len(items))
	for _, item := range items {
		out = append(out, FusionLoadItem{Path: item.path, Mode: "read-full", Rationale: item.why})
	}
	return out
}

func fusionGateSeverities(cfg Config) map[string]string {
	return map[string]string{
		"acceptance":     cfg.Gates.Acceptance,
		"contextBudget":  cfg.Gates.ContextBudget,
		"modeCapability": cfg.Gates.ModeCapability,
		"scope":          cfg.Gates.Scope,
		"traceability":   cfg.Gates.Traceability,
	}
}

func fusionHealthChecks(root string) []FusionHealthCheck {
	checks := []FusionHealthCheck{
		pathHealth("steering", root, []string{".specd/steering/reasoning.md", ".specd/steering/workflow.md", ".specd/steering/product.md", ".specd/steering/tech.md", ".specd/steering/structure.md"}),
		pathHealth("roles", root, []string{".specd/roles/investigator.md", ".specd/roles/builder.md", ".specd/roles/reviewer.md", ".specd/roles/verifier.md", ".specd/roles/brain.md", ".specd/roles/pinky.md"}),
		pathHealth("skills", root, []string{".specd/skills/specd-foundations/SKILL.md", ".specd/skills/specd-steering/SKILL.md", ".specd/skills/specd-requirements/SKILL.md", ".specd/skills/specd-design/SKILL.md", ".specd/skills/specd-tasks/SKILL.md", ".specd/skills/specd-execute/SKILL.md"}),
		pathHealth("config", root, []string{".specd/config.json"}),
	}
	agents := pathHealth("agents", root, []string{"AGENTS.md"})
	if agents.OK {
		raw := ReadOrNull(AgentsPath(root))
		if raw == nil || !strings.Contains(*raw, markerBegin()) || !strings.Contains(*raw, markerEnd()) {
			agents.OK = false
			agents.Message = "AGENTS.md missing specd managed markers"
		}
	}
	checks = append(checks, agents)
	return checks
}

func pathHealth(name, root string, rels []string) FusionHealthCheck {
	missing := []string{}
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			missing = append(missing, rel)
		}
	}
	check := FusionHealthCheck{Name: name, OK: len(missing) == 0, Missing: missing}
	if !check.OK {
		check.Message = "missing required scaffold files"
	}
	return check
}

func fusionActiveSpecModes(root string) []FusionActiveSpecMode {
	slugs := ListSpecs(root)
	out := make([]FusionActiveSpecMode, 0, len(slugs))
	for _, slug := range slugs {
		state, err := LoadState(root, slug)
		if err != nil || state == nil {
			continue
		}
		origin := state.ModeOrigin
		if origin == "" {
			origin = OriginDefault
		}
		out = append(out, FusionActiveSpecMode{Slug: slug, Status: string(state.Status), Phase: string(state.Phase), Mode: state.EffectiveMode(), Origin: origin, Gate: string(state.Gate)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

func commandSchemaDigest() (string, error) {
	b, err := json.Marshal(Commands)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func fileDigest(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), true
}
