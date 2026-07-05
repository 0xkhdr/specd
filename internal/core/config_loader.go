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
	Context            ContextConfig
	Orchestration      OrchestrationConfig
	Criteria           CriteriaConfig
	Review             ReviewConfig
	Submit             SubmitConfig
	PromotionThreshold int
}

type GatesConfig struct {
	Verify string
}

// SubmitConfig configures the terminal `submit` verb (spec 08). Command is an
// operator-supplied shell line run through the sandboxed exec path with the PR
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
	Global  string
	Project string
}

var DefaultConfig = Config{
	Version: "1",
	Agent:   "codex",
	Gates: GatesConfig{
		Verify: "error",
	},
	Context: ContextConfig{
		MaxTokens: 12000,
	},
	Orchestration: OrchestrationConfig{
		Enabled: false,
		Model:   "",
	},
	PromotionThreshold: 3,
}

// LoadConfig applies global YAML, project YAML, then environment overrides.
// The function is deterministic for explicit paths and env input.
func LoadConfig(paths ConfigPaths, env map[string]string) (Config, []Diagnostic) {
	cfg := DefaultConfig
	var diagnostics []Diagnostic
	for _, path := range []string{paths.Global, paths.Project} {
		if path == "" || filepath.Ext(path) != ".yml" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			diagnostics = append(diagnostics, Diagnostic{Severity: "error", Path: path, Message: err.Error()})
			continue
		}
		values, err := parseSimpleYAML(string(raw))
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Severity: "error", Path: path, Message: err.Error()})
			continue
		}
		applyConfigMap(&cfg, values, path, &diagnostics)
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
