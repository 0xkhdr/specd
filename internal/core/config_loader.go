package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the deterministic runtime configuration used by the harness.
type Config struct {
	Version            string
	Agent              string
	Gates              GatesConfig
	Verify             VerifyConfig
	Context            ContextConfig
	Orchestration      OrchestrationConfig
	Criteria           CriteriaConfig
	Review             ReviewConfig
	Submit             SubmitConfig
	Security           SecurityConfig
	Escalation         EscalationConfig
	PromotionThreshold int
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
