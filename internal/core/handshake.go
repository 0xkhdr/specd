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

// HandshakeBootstrapVersion is the schema version reported in the Version field
// of HandshakeBootstrap and HandshakePolicy payloads.
const HandshakeBootstrapVersion = 1

// HandshakeBootstrap is the JSON payload returned by `specd handshake bootstrap`,
// bundling the project root, the files an agent should load, command/config
// summaries, scaffold health, active spec modes, and recommended next actions.
type HandshakeBootstrap struct {
	Version     int                       `json:"version"`
	Root        string                    `json:"root"`
	Load        []HandshakeLoadItem       `json:"load"`
	Commands    HandshakeCommandSummary   `json:"commands"`
	Config      HandshakeConfigSummary    `json:"config"`
	Health      HandshakeHealthSummary    `json:"health"`
	Modes       []HandshakeActiveSpecMode `json:"modes"`
	NextActions []string                  `json:"nextActions"`
}

// HandshakeLoadItem describes a single file a handshake-aware agent should load,
// along with how to load it and why.
type HandshakeLoadItem struct {
	Path      string `json:"path"`
	Mode      string `json:"mode"`
	Rationale string `json:"rationale"`
}

// HandshakeCommandSummary summarizes the specd command schema (a digest and
// count, with the full schema optionally attached) so an agent can detect
// drift without always loading the entire schema.
type HandshakeCommandSummary struct {
	SchemaCommand string        `json:"schemaCommand"`
	Digest        string        `json:"digest"`
	Count         int           `json:"count"`
	Schema        []CommandMeta `json:"schema,omitempty"`
}

// HandshakeConfigSummary reports the effective specd config: its path and
// digest, role/orchestration settings, gate severities, and where the
// effective values were sourced from.
type HandshakeConfigSummary struct {
	Path                  string               `json:"path"`
	Digest                string               `json:"digest"`
	Roles                 HandshakeRolesConfig `json:"roles"`
	Orchestration         HandshakeOrchConfig  `json:"orchestration"`
	VerifySandbox         string               `json:"verifySandbox"`
	GateSeverities        map[string]string    `json:"gateSeverities"`
	ConfigFilePresent     bool                 `json:"configFilePresent"`
	EffectiveConfigSource string               `json:"effectiveConfigSource"`
}

// HandshakeRolesConfig holds the subagent role configuration exposed in a
// handshake summary.
type HandshakeRolesConfig struct {
	SubagentMode string `json:"subagentMode"`
}

// HandshakeOrchConfig holds the orchestration settings (enablement, approval
// policy, worker mode) exposed in a handshake summary.
type HandshakeOrchConfig struct {
	Enabled        bool   `json:"enabled"`
	ApprovalPolicy string `json:"approvalPolicy"`
	WorkerMode     string `json:"workerMode"`
}

// HandshakeHealthSummary reports overall scaffold health plus the individual
// checks that were run.
type HandshakeHealthSummary struct {
	OK     bool                   `json:"ok"`
	Checks []HandshakeHealthCheck `json:"checks"`
}

// HandshakeHealthCheck reports the result of one scaffold health check,
// including any required files found missing.
type HandshakeHealthCheck struct {
	Name    string   `json:"name"`
	OK      bool     `json:"ok"`
	Missing []string `json:"missing"`
	Message string   `json:"message,omitempty"`
}

// HandshakeActiveSpecMode summarizes one spec's status, phase, mode, mode
// origin, and gate state for the active-modes listing in a handshake bootstrap.
type HandshakeActiveSpecMode struct {
	Slug   string `json:"slug"`
	Status string `json:"status"`
	Phase  string `json:"phase"`
	Mode   string `json:"mode"`
	Origin string `json:"origin"`
	Gate   string `json:"gate"`
}

// HandshakePolicy is the JSON payload returned by `specd handshake policy`,
// describing the effective orchestration/config policy plus any violations
// and recommendations, optionally narrowed to a single spec.
type HandshakePolicy struct {
	Version              int                  `json:"version"`
	Root                 string               `json:"root"`
	SubagentMode         string               `json:"subagentMode"`
	OrchestrationEnabled bool                 `json:"orchestrationEnabled"`
	ApprovalPolicy       string               `json:"approvalPolicy"`
	WorkerMode           string               `json:"workerMode"`
	MaxWorkers           int                  `json:"maxWorkers"`
	MaxRetries           int                  `json:"maxRetries"`
	TimeoutSeconds       int                  `json:"timeoutSeconds"`
	VerifySandbox        string               `json:"verifySandbox"`
	GateSeverities       map[string]string    `json:"gateSeverities"`
	MCPExposure          HandshakeMCPExposure `json:"mcpExposure"`
	ConfigDigest         string               `json:"configDigest"`
	ConfigFilePresent    bool                 `json:"configFilePresent"`
	DigestMatch          *bool                `json:"digestMatch,omitempty"`
	ExpectedConfigDigest string               `json:"expectedConfigDigest,omitempty"`
	Spec                 *HandshakeSpecPolicy `json:"spec,omitempty"`
	Diagnostics          []ConfigDiagnostic   `json:"diagnostics"`
	Violations           []string             `json:"violations"`
	Recommendations      []string             `json:"recommendations"`
}

// HandshakeMCPExposure describes which MCP tools are exposed and how,
// mirroring the project's MCP exposure configuration.
type HandshakeMCPExposure struct {
	Expose               string   `json:"expose"`
	EssentialTools       []string `json:"essentialTools,omitempty"`
	IncludeMeta          bool     `json:"includeMeta"`
	IncludeOrchestration *bool    `json:"includeOrchestration,omitempty"`
}

// HandshakeSpecPolicy captures the resolved mode and allowed workflows (brain
// orchestration vs. base loop) for one spec within a HandshakePolicy.
type HandshakeSpecPolicy struct {
	Slug            string `json:"slug"`
	SpecMode        string `json:"specMode"`
	ModeOrigin      string `json:"modeOrigin"`
	BrainAllowed    bool   `json:"brainAllowed"`
	BaseLoopAllowed bool   `json:"baseLoopAllowed"`
	Recommended     string `json:"recommendedCommandFamily"`
	NextCommand     string `json:"nextCommand"`
	PolicyViolation string `json:"policyViolation,omitempty"`
}

// BuildHandshakePolicy resolves the effective orchestration policy for root
// (auto-discovering it when root is empty) and, when slug is non-empty,
// layers in spec-specific mode/workflow guidance and violation checks. When
// expectDigest is non-empty it also records whether the loaded config
// digest matches it.
func BuildHandshakePolicy(root, slug, expectDigest string) (HandshakePolicy, error) {
	if root == "" {
		var ok bool
		root, ok = FindSpecdRoot("")
		if !ok {
			return HandshakePolicy{}, NotFoundError("no .specd/ found in this directory or any parent. Run `specd init` first.")
		}
	}
	cfg, diags := LoadConfigStrict(root)
	_, loadResult := LoadConfigWithDiagnostics(root)
	digest, present := loadResult.Digest, loadResult.Present
	policy := HandshakePolicy{
		Version: HandshakeBootstrapVersion, Root: root,
		SubagentMode: cfg.Roles.SubagentMode, OrchestrationEnabled: cfg.Orchestration.Enabled,
		ApprovalPolicy: cfg.Orchestration.ApprovalPolicy, WorkerMode: cfg.Orchestration.WorkerMode,
		MaxWorkers: cfg.Orchestration.MaxWorkers, MaxRetries: cfg.Orchestration.MaxRetries,
		TimeoutSeconds: cfg.Orchestration.SessionTimeoutMinutes * 60,
		VerifySandbox:  cfg.Verify.Sandbox, GateSeverities: handshakeGateSeverities(cfg),
		MCPExposure:  HandshakeMCPExposure{Expose: cfg.MCP.Expose, EssentialTools: cfg.MCP.EssentialTools, IncludeMeta: cfg.MCP.IncludeMeta, IncludeOrchestration: cfg.MCP.IncludeOrchestration},
		ConfigDigest: digest, ConfigFilePresent: present, Diagnostics: diags,
		Violations: []string{}, Recommendations: []string{},
	}
	if expectDigest != "" {
		match := digest == expectDigest
		policy.DigestMatch = &match
		policy.ExpectedConfigDigest = expectDigest
		if !match {
			policy.Violations = append(policy.Violations, "config digest mismatch")
			policy.Recommendations = append(policy.Recommendations, "rerun `specd handshake bootstrap --json`")
		}
	}
	if HasErrorDiagnostics(diags) {
		policy.Violations = append(policy.Violations, "invalid config")
	}
	if slug != "" {
		state, err := LoadState(root, slug)
		if err != nil {
			return HandshakePolicy{}, err
		}
		mode, origin := ResolveMode("", state)
		specPolicy := HandshakeSpecPolicy{Slug: slug, SpecMode: mode, ModeOrigin: origin}
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
				specPolicy.Recommended = "specd status " + slug + " --set-mode simple or enable orchestration"
				specPolicy.NextCommand = "specd status " + slug + " --set-mode simple"
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

// BuildHandshakeBootstrap assembles the full handshake bootstrap payload for root
// (auto-discovering it when root is empty), including the command schema
// digest, config summary, scaffold health checks, active spec modes, and
// recommended next actions. includeSchema controls whether the full command
// schema is embedded in the result.
func BuildHandshakeBootstrap(root string, includeSchema bool) (HandshakeBootstrap, error) {
	if root == "" {
		var ok bool
		root, ok = FindSpecdRoot("")
		if !ok {
			return HandshakeBootstrap{}, NotFoundError("no .specd/ found in this directory or any parent. Run `specd init` first.")
		}
	}
	cfg, loadResult := LoadConfigWithDiagnostics(root)
	commandsDigest, err := commandSchemaDigest()
	if err != nil {
		return HandshakeBootstrap{}, err
	}
	configDigest, configPresent := loadResult.Digest, loadResult.Present
	checks := handshakeHealthChecks(root)
	healthOK := true
	for _, check := range checks {
		if !check.OK {
			healthOK = false
			break
		}
	}
	commands := HandshakeCommandSummary{SchemaCommand: "specd help --json", Digest: commandsDigest, Count: len(Commands)}
	if includeSchema {
		commands.Schema = Commands
	}
	nextActions := []string{"Read all items in load before acting", "Run `specd help --json` to cache command schema", "Run `specd status` to orient active work"}
	if !healthOK {
		nextActions = append(nextActions, "Run `specd doctor --fix` or `specd init --repair` before mutating specs")
	}
	return HandshakeBootstrap{
		Version:  HandshakeBootstrapVersion,
		Root:     root,
		Load:     handshakeLoadItems(),
		Commands: commands,
		Config: HandshakeConfigSummary{
			Path: func() string {
				if loadResult.ProjectPath != "" {
					if rel, err := filepath.Rel(root, loadResult.ProjectPath); err == nil {
						return filepath.ToSlash(rel)
					}
					return loadResult.ProjectPath
				}
				return ".specd/config.yml"
			}(),
			Digest:            configDigest,
			Roles:             HandshakeRolesConfig{SubagentMode: cfg.Roles.SubagentMode},
			Orchestration:     HandshakeOrchConfig{Enabled: cfg.Orchestration.Enabled, ApprovalPolicy: cfg.Orchestration.ApprovalPolicy, WorkerMode: cfg.Orchestration.WorkerMode},
			VerifySandbox:     cfg.Verify.Sandbox,
			GateSeverities:    handshakeGateSeverities(cfg),
			ConfigFilePresent: configPresent,
			EffectiveConfigSource: func() string {
				if loadResult.GlobalPath != "" && loadResult.ProjectPath != "" {
					return "global+project+defaults"
				}
				if loadResult.ProjectPath != "" {
					return "project+defaults"
				}
				if loadResult.GlobalPath != "" {
					return "global+defaults"
				}
				return "defaults"
			}(),
		},
		Health:      HandshakeHealthSummary{OK: healthOK, Checks: checks},
		Modes:       handshakeActiveSpecModes(root),
		NextActions: nextActions,
	}, nil
}

func handshakeLoadItems() []HandshakeLoadItem {
	items := []struct{ path, why string }{
		{".specd/steering/reasoning.md", "reasoning constitution and response discipline"},
		{".specd/steering/workflow.md", "required lifecycle gates"},
		{".specd/steering/product.md", "product intent and constraints"},
		{".specd/steering/tech.md", "technology stack and verification policy"},
		{".specd/steering/structure.md", "repository layout and edit boundaries"},
		{"AGENTS.md", "agent operating contract and host-specific rules"},
	}
	out := make([]HandshakeLoadItem, 0, len(items))
	for _, item := range items {
		out = append(out, HandshakeLoadItem{Path: item.path, Mode: "read-full", Rationale: item.why})
	}
	return out
}

func handshakeGateSeverities(cfg Config) map[string]string {
	return map[string]string{
		"acceptance":     cfg.Gates.Acceptance,
		"contextBudget":  cfg.Gates.ContextBudget,
		"modeCapability": cfg.Gates.ModeCapability,
		"scope":          cfg.Gates.Scope,
		"traceability":   cfg.Gates.Traceability,
	}
}

func handshakeHealthChecks(root string) []HandshakeHealthCheck {
	checks := []HandshakeHealthCheck{
		pathHealth("steering", root, []string{".specd/steering/reasoning.md", ".specd/steering/workflow.md", ".specd/steering/product.md", ".specd/steering/tech.md", ".specd/steering/structure.md"}),
		pathHealth("roles", root, []string{".specd/roles/scout.md", ".specd/roles/craftsman.md", ".specd/roles/auditor.md", ".specd/roles/validator.md", ".specd/roles/brain.md", ".specd/roles/pinky.md"}),
		pathHealth("skills", root, []string{".specd/skills/specd-foundations/SKILL.md", ".specd/skills/specd-steering/SKILL.md", ".specd/skills/specd-requirements/SKILL.md", ".specd/skills/specd-design/SKILL.md", ".specd/skills/specd-tasks/SKILL.md", ".specd/skills/specd-execute/SKILL.md", ".specd/skills/specd-eval-author/SKILL.md"}),
		pathHealth("config", root, []string{".specd/config.yml"}),
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

func pathHealth(name, root string, rels []string) HandshakeHealthCheck {
	missing := []string{}
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			missing = append(missing, rel)
		}
	}
	check := HandshakeHealthCheck{Name: name, OK: len(missing) == 0, Missing: missing}
	if !check.OK {
		check.Message = "missing required scaffold files"
	}
	return check
}

func handshakeActiveSpecModes(root string) []HandshakeActiveSpecMode {
	slugs := ListSpecs(root)
	out := make([]HandshakeActiveSpecMode, 0, len(slugs))
	for _, slug := range slugs {
		state, err := LoadState(root, slug)
		if err != nil || state == nil {
			continue
		}
		origin := state.ModeOrigin
		if origin == "" {
			origin = OriginDefault
		}
		out = append(out, HandshakeActiveSpecMode{Slug: slug, Status: string(state.Status), Phase: string(state.Phase), Mode: state.EffectiveMode(), Origin: origin, Gate: string(state.Gate)})
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
