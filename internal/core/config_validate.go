package core

import (
	"encoding/json"
	"fmt"
	"os"
)

// ConfigDiagnostic is a single config-loading or config-validation finding:
// where it came from (path/source/layer/field), its severity, and a
// human-readable message.
type ConfigDiagnostic struct {
	Path     string `json:"path"`
	Source   string `json:"source,omitempty"`
	Layer    string `json:"layer,omitempty"`
	Field    string `json:"field,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// LoadConfigStrict loads the effective config the same way LoadConfig does,
// then additionally re-parses the raw project config file to validate every
// enum and integer-range field strictly, returning the full diagnostic list
// alongside the loaded config.
func LoadConfigStrict(root string) (Config, []ConfigDiagnostic) {
	cfg, result := LoadConfigWithDiagnostics(root)
	d := append([]ConfigDiagnostic{}, result.Diagnostics...)
	path := result.ProjectPath
	if path == "" {
		path = ConfigPath(root)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		d = append(d, ValidateEffectiveConfig(cfg)...)
		return cfg, d
	}
	loaded, parseDiags := loadConfigFromPathLayer(path, "project")
	d = append(d, parseDiags...)
	if loaded.Doc != nil {
		if err := ValidateConfigDoc(loaded.Doc); err != nil {
			d = append(d, ConfigDiagnostic{Path: path, Severity: "error", Message: err.Error()})
		}
	}
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	validateStringEnum(doc, "roles.subagentMode", []string{"inline", "delegate"}, &d)
	for _, p := range []string{"gates.traceability", "gates.acceptance", "gates.scope", "gates.contextBudget", "gates.modeCapability"} {
		validateStringEnum(doc, p, []string{"", "off", "warn", "error"}, &d)
	}
	validateStringEnum(doc, "gates.eval", []string{"", "off", "required"}, &d)
	validateStringEnum(doc, "gates.ingest", []string{"", "off", "warn", "error"}, &d)
	validateStringEnum(doc, "verify.sandbox", []string{"none", "bwrap", "container"}, &d)
	validateStringEnum(doc, "deploy.sandbox", []string{"", "none", "bwrap", "container"}, &d)
	validateIntRange(doc, "observe.maxPayloadBytes", 0, 16*1024*1024, &d)
	validateStringEnum(doc, "orchestration.approvalPolicy", []string{"manual", "planning", "session"}, &d)
	validateStringEnum(doc, "orchestration.workerMode", []string{"host"}, &d)
	validateStringEnum(doc, "orchestration.transport.kind", []string{"file"}, &d)
	validateStringEnum(doc, "orchestration.compactionPolicy", []string{"", CompactionNone, CompactionPhase, CompactionBudget, CompactionBoth}, &d)
	validateStringEnum(doc, "mcp.expose", []string{"", "all", "essential", "phase"}, &d)
	validateIntRange(doc, "promotionThreshold", 0, 1000000, &d)
	validateIntRange(doc, "gates.maxContextTokens", 0, MaxSoftContextTokens(), &d)
	validateIntRange(doc, "orchestration.maxWorkers", minMaxWorkers, maxMaxWorkers, &d)
	validateIntRange(doc, "orchestration.maxRetries", minMaxRetries, maxMaxRetries, &d)
	validateIntRange(doc, "orchestration.sessionTimeoutMinutes", minSessionTimeoutMinutes, maxSessionTimeoutMinutes, &d)
	validateIntRange(doc, "orchestration.transport.pollIntervalMillis", minPollIntervalMillis, maxPollIntervalMillis, &d)
	validateIntRange(doc, "orchestration.transport.messageTTLSeconds", minMessageTTLSeconds, maxMessageTTLSeconds, &d)
	validateIntRange(doc, "orchestration.transport.leaseSeconds", minLeaseSeconds, maxLeaseSeconds, &d)
	validateIntRange(doc, "orchestration.transport.heartbeatSeconds", minHeartbeatSeconds, maxHeartbeatSeconds, &d)
	validateIntRange(doc, "orchestration.program.maxConcurrentSpecs", minMaxConcurrentSpecs, maxMaxConcurrentSpecs, &d)
	d = append(d, ValidateEffectiveConfig(cfg)...)
	return cfg, d
}

// ValidateEffectiveConfig checks the fully-merged Config for unsupported enum
// values and out-of-range fields (report format, subagent mode, gate modes,
// verify sandbox, max context tokens, orchestration config) and returns one
// error diagnostic per violation.
func ValidateEffectiveConfig(cfg Config) []ConfigDiagnostic {
	d := []ConfigDiagnostic{}
	if !oneOf(cfg.Report.Format, "", "md", "html") {
		d = append(d, ConfigDiagnostic{Path: "report.format", Field: "report.format", Severity: "error", Message: fmt.Sprintf("unsupported %q; allowed: [md html]", cfg.Report.Format)})
	}
	if !oneOf(cfg.Roles.SubagentMode, "inline", "delegate") {
		d = append(d, ConfigDiagnostic{Path: "roles.subagentMode", Field: "roles.subagentMode", Severity: "error", Message: fmt.Sprintf("unsupported %q; allowed: [inline delegate]", cfg.Roles.SubagentMode)})
	}
	for path, value := range map[string]string{
		"gates.traceability":   cfg.Gates.Traceability,
		"gates.acceptance":     cfg.Gates.Acceptance,
		"gates.scope":          cfg.Gates.Scope,
		"gates.contextBudget":  cfg.Gates.ContextBudget,
		"gates.modeCapability": cfg.Gates.ModeCapability,
	} {
		if !oneOf(value, "", "off", "warn", "error") {
			d = append(d, ConfigDiagnostic{Path: path, Field: path, Severity: "error", Message: fmt.Sprintf("unsupported %q; allowed: [off warn error]", value)})
		}
	}
	if !oneOf(cfg.Verify.Sandbox, "", "none", "bwrap", "container") {
		d = append(d, ConfigDiagnostic{Path: "verify.sandbox", Field: "verify.sandbox", Severity: "error", Message: fmt.Sprintf("unsupported %q; allowed: [none bwrap container]", cfg.Verify.Sandbox)})
	}
	if cfg.Gates.MaxContextTokens < 0 || cfg.Gates.MaxContextTokens > MaxSoftContextTokens() {
		d = append(d, ConfigDiagnostic{Path: "gates.maxContextTokens", Field: "gates.maxContextTokens", Severity: "error", Message: fmt.Sprintf("%d outside [0,%d]", cfg.Gates.MaxContextTokens, MaxSoftContextTokens())})
	}
	if err := ValidateOrchestrationConfig(&cfg.Orchestration); err != nil {
		d = append(d, ConfigDiagnostic{Path: "orchestration", Field: "orchestration", Severity: "error", Message: err.Error()})
	}
	return d
}

// MaxSoftContextTokens returns the upper bound allowed for
// gates.maxContextTokens.
func MaxSoftContextTokens() int { return 200000 }

func validateStringEnum(doc map[string]json.RawMessage, path string, allowed []string, d *[]ConfigDiagnostic) {
	raw, ok := rawAtPath(doc, path)
	if !ok {
		return
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		*d = append(*d, ConfigDiagnostic{Path: path, Severity: "error", Message: "must be string"})
		return
	}
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	*d = append(*d, ConfigDiagnostic{Path: path, Severity: "error", Message: fmt.Sprintf("unsupported %q; allowed: %v", value, allowed)})
}

func validateIntRange(doc map[string]json.RawMessage, path string, min, max int, d *[]ConfigDiagnostic) {
	raw, ok := rawAtPath(doc, path)
	if !ok {
		return
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		*d = append(*d, ConfigDiagnostic{Path: path, Severity: "error", Message: "must be integer"})
		return
	}
	if value < min || value > max {
		*d = append(*d, ConfigDiagnostic{Path: path, Severity: "error", Message: fmt.Sprintf("%d outside [%d,%d]", value, min, max)})
	}
}

func rawAtPath(doc map[string]json.RawMessage, path string) (json.RawMessage, bool) {
	parts := stringsSplit(path, '.')
	cur := doc
	for i, p := range parts {
		raw, ok := cur[p]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return raw, true
		}
		var next map[string]json.RawMessage
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

func stringsSplit(s string, sep byte) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// HasErrorDiagnostics reports whether diags contains at least one diagnostic
// with "error" severity.
func HasErrorDiagnostics(diags []ConfigDiagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}
