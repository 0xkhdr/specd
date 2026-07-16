package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the deterministic runtime configuration used by the harness.
type Config struct {
	Version string
	Agent   string
	// Profile is the lifecycle strictness profile (spec 01 R7). "default" keeps
	// the backward-compatible policy where new completeness checks are opt-in
	// per-flag (R7.1). "production" raises the whole bar: it arms the criterion,
	// review, and integration/negative-path evidence gates together (R7.2),
	// regardless of the individual criteria.required / review.required switches.
	Profile            string
	Gates              GatesConfig
	Verify             VerifyConfig
	Context            ContextConfig
	Orchestration      OrchestrationConfig
	Routing            RoutingConfig
	Criteria           CriteriaConfig
	Review             ReviewConfig
	Submit             SubmitConfig
	Security           SecurityConfig
	Escalation         EscalationConfig
	PromotionThreshold int
	// Environments is the closed delivery policy per environment name (spec 08
	// R7.1). Empty by default — a project opts in. Keys are validated against the
	// closed EnvironmentName set; an unknown name or missing required field fails
	// closed at load time.
	Environments map[EnvironmentName]EnvironmentV1
}

// SecurityConfig sets per-scanner severity for the opt-in security gate (spec
// 05 R5). Each field is off|warn|error: error findings fail the gate (exit 1),
// warn findings print but pass, off skips the scanner. Defaults tuned so a real
// secret blocks while noisier heuristics only warn.
type SecurityConfig struct {
	Profile       string
	Secrets       string
	Injection     string
	Slopsquat     string
	CleanWorktree string
	Sandbox       string
}

// RequiresVerifySandbox reports whether policy makes isolation mandatory.
// Keeping profile resolution in core prevents CLI flag behavior from becoming
// an independent, potentially weaker policy interpretation.
func (c SecurityConfig) RequiresVerifySandbox() bool { return c.Profile == "production" }

// SecuritySeverities enumerates the valid per-scanner severities.
var SecuritySeverities = []string{"off", "warn", "error"}

// EscalationConfig is the opt-in verify-failure ratchet (spec 06 R5). MaxVerifyFails
// is the count of consecutive failing verify records (since the last pass or
// override) that escalates a task and blocks its completion until a human clears
// it with `task <id> --override --reason`. Default 3; 0 disables the ratchet.
type EscalationConfig struct {
	MaxVerifyFails int
}

// EscalationDefaultMaxVerifyFails is the ratchet threshold when config leaves
// escalation.max_verify_fails unset.
const EscalationDefaultMaxVerifyFails = 3

type GatesConfig struct {
	Verify string
}

// VerifyConfig bounds a single task verify command (gap 4.2 / W6-T4). TimeoutSecs
// caps wall-clock for one verify exec; a timeout is recorded as a FAILING evidence
// record (exit 124), never a crash or a silent hang. Zero means unbounded, which
// preserves prior behavior — operators opt into a bound.
type VerifyConfig struct {
	TimeoutSecs int
	// Trivial lists verify commands that do no real checking (spec 01 R4.2). A
	// write task (role craftsman) using one of these is rejected — it must
	// verify its own change — while a read-only task (scout/validator/auditor)
	// may legitimately retain a trivial verify. Configurable to avoid false
	// positives (design 01: role/profile allowlists, exact finding, no opaque ban).
	Trivial []string
}

// DefaultTrivialVerify is the built-in set of no-op verify commands. A write
// task using any of these is rejected by the verify gate; read-only tasks may
// keep them (their `verify` is meant to be a trivial `printf ok`).
var DefaultTrivialVerify = []string{"printf ok", "true", ":"}

// IsTrivialVerify reports whether cmd (trimmed) matches one of the trivial verify
// commands. Matching is exact on the trimmed command; a genuine verify that
// merely contains "true" as a substring is not trivial.
func IsTrivialVerify(cmd string, trivial []string) bool {
	cmd = strings.TrimSpace(cmd)
	for _, t := range trivial {
		if cmd == strings.TrimSpace(t) {
			return true
		}
	}
	return false
}

// SubmitConfig configures the terminal `submit` verb (spec 08). Command is an
// operator-supplied shell line run through the shared exec path with the PR
// summary streamed on stdin; empty means dry-run (print summary, exit 0). The
// binary embeds no git/GitHub logic — the operator owns transport. TimeoutSecs
// bounds the command; zero applies SubmitDefaultTimeoutSecs.
type SubmitConfig struct {
	Command     string
	TimeoutSecs int
}

// SubmitDefaultTimeoutSecs bounds an operator submit command when the config
// leaves submit.timeout_seconds unset.
const SubmitDefaultTimeoutSecs = 120

// Lifecycle strictness profiles (spec 01 R7). ProfileDefault keeps every new
// completeness check opt-in (backward compatible, R7.1); ProfileProduction
// arms the risk-proportionate criterion/review/integration/negative-path
// evidence gates together (R7.2).
const (
	ProfileDefault    = "default"
	ProfileProduction = "production"
)

// ProductionProfile reports whether the production lifecycle profile is armed.
// An empty profile resolves to the default profile.
func (c Config) ProductionProfile() bool {
	return c.Profile == ProfileProduction
}

// ProductionTaskAuthorityRequired is the canonical compatibility predicate for
// task authority and mission-derived scope. The lifecycle profile is normative;
// security.profile=production remains armed for existing projects.
func (c Config) ProductionTaskAuthorityRequired() bool {
	return c.ProductionProfile() || c.Security.Profile == ProfileProduction
}

// CriteriaGateArmed reports whether the per-criterion evidence ratchet must run:
// either the explicit criteria.required switch, or the production profile
// (which requires current criterion evidence, R7.2).
func (c Config) CriteriaGateArmed() bool {
	return c.Criteria.Required || c.ProductionProfile()
}

// ReviewGateArmed reports whether the review ratchet must run: either the
// explicit review.required switch, or the production profile (which requires a
// current-HEAD review, R7.2).
func (c Config) ReviewGateArmed() bool {
	return c.Review.Required || c.ProductionProfile()
}

// IntegrationPolicyArmed reports whether declared external/integration
// boundaries must carry error-path and integration evidence planning (R3.3).
// The production lifecycle profile arms it; security.profile=production keeps
// arming it for backward compatibility.
func (c Config) IntegrationPolicyArmed() bool {
	return c.ProductionProfile() || c.Security.Profile == ProfileProduction
}

// CriteriaConfig is the opt-in per-acceptance-criterion evidence ratchet. When
// Required is true, the completion approval gate refuses while any acceptance
// criterion lacks a current passing record (spec 04 R6). Default off so existing
// flows are unbroken.
type CriteriaConfig struct {
	Required bool
}

// ReviewConfig is the opt-in review gate (spec 09). When Required is true, the
// completion approval gate refuses unless review_report.md carries an approve
// verdict recorded at the current git HEAD. Default off so existing flows are
// unbroken.
type ReviewConfig struct {
	Required bool
}

type ContextConfig struct {
	MaxTokens int
}

type OrchestrationConfig struct {
	Enabled bool
	Model   string
}

// RoutingConfig is approved, provider-neutral dispatch policy. Core selects a
// capability class only; adapters own any provider/model mapping.
type RoutingConfig struct {
	Version               string
	Classes               []string
	DefaultClass          string
	Fallback              []string
	ClassCapabilities     map[string][]string
	MaxTokens             int64
	MaxCostMicros         int64
	DeadlineSeconds       int
	MaxRetries            int
	AllowUnknownTelemetry bool
	// Recommendations maps task complexity to provider-neutral capability class.
	// Adapters decide whether and how that class maps to an available model.
	Recommendations map[string]string
}

type RoutingRecommendation struct {
	Class      string `json:"class"`
	Complexity string `json:"complexity,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
}

// RecommendRouting returns policy metadata only. It never resolves provider
// availability and cannot affect verify/evidence completion authority.
func RecommendRouting(task TaskRow, cfg RoutingConfig) (RoutingRecommendation, error) {
	class := cfg.DefaultClass
	if recommended := cfg.Recommendations[task.Complexity]; recommended != "" {
		class = recommended
	}
	if !contains(cfg.Classes, class) {
		return RoutingRecommendation{}, fmt.Errorf("routing class %q is not declared", class)
	}
	return RoutingRecommendation{Class: class, Complexity: task.Complexity}, nil
}

type Diagnostic struct {
	Severity string
	Path     string
	Message  string
}

type ConfigPaths struct {
	Project string
}

var DefaultConfig = Config{
	Version: "1",
	Agent:   "codex",
	Profile: ProfileDefault,
	Gates: GatesConfig{
		Verify: "error",
	},
	Verify: VerifyConfig{
		Trivial: DefaultTrivialVerify,
	},
	Context: ContextConfig{
		MaxTokens: 12000,
	},
	Orchestration: OrchestrationConfig{
		Enabled: false,
		Model:   "",
	},
	Routing: RoutingConfig{
		Version:               "1",
		Classes:               []string{"default"},
		DefaultClass:          "default",
		Fallback:              []string{"default"},
		ClassCapabilities:     map[string][]string{"default": {"context", "eval", "review", "sandbox"}},
		MaxRetries:            3,
		AllowUnknownTelemetry: true,
		Recommendations:       map[string]string{},
	},
	Security: SecurityConfig{
		Profile:       "prototype",
		Secrets:       "error",
		Injection:     "warn",
		Slopsquat:     "warn",
		CleanWorktree: "off",
		Sandbox:       "off",
	},
	Escalation: EscalationConfig{
		MaxVerifyFails: EscalationDefaultMaxVerifyFails,
	},
	PromotionThreshold: 3,
}

// LoadConfig applies project YAML, then environment overrides. The function is
// deterministic for the explicit path and env input.
func LoadConfig(paths ConfigPaths, env map[string]string) (Config, []Diagnostic) {
	cfg := DefaultConfig
	var diagnostics []Diagnostic
	if path := paths.Project; path != "" && filepath.Ext(path) == ".yml" {
		if raw, err := os.ReadFile(path); err != nil {
			if !os.IsNotExist(err) {
				diagnostics = append(diagnostics, Diagnostic{Severity: "error", Path: path, Message: err.Error()})
			}
		} else if values, err := parseSimpleYAML(string(raw)); err != nil {
			diagnostics = append(diagnostics, Diagnostic{Severity: "error", Path: path, Message: err.Error()})
		} else {
			applyConfigMap(&cfg, values, path, &diagnostics)
		}
	}
	applyEnv(&cfg, env, &diagnostics)
	return cfg, diagnostics
}

func parseSimpleYAML(raw string) (map[string]string, error) {
	out := make(map[string]string)
	var section string
	for lineNo, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "  ") {
			return nil, configError(lineNo+1, "indent must be two spaces")
		}
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, ":") {
			return nil, configError(lineNo+1, "missing ':'")
		}
		parts := strings.SplitN(trimmed, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, configError(lineNo+1, "empty key")
		}
		if strings.HasPrefix(line, "  ") {
			if section == "" {
				return nil, configError(lineNo+1, "nested key without section")
			}
			if value == "" {
				return nil, configError(lineNo+1, "empty scalar")
			}
			out[section+"."+key] = unquote(value)
			continue
		}
		if value == "" {
			section = key
			continue
		}
		section = ""
		out[key] = unquote(value)
	}
	return out, nil
}

func unquote(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func configError(line int, message string) error {
	return ConfigParseError{Line: line, Message: message}
}

type ConfigParseError struct {
	Line    int
	Message string
}

func (e ConfigParseError) Error() string {
	return "config line " + strconv.Itoa(e.Line) + ": " + e.Message
}
