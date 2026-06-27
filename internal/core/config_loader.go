package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ConfigLoadResult struct {
	Diagnostics []ConfigDiagnostic `json:"diagnostics"`
	ProjectPath string             `json:"projectPath,omitempty"`
	GlobalPath  string             `json:"globalPath,omitempty"`
	Digest      string             `json:"digest"`
	Present     bool               `json:"present"`
}

type loadedConfigFile struct {
	Path string
	Doc  map[string]any
}

func LoadConfig(root string) Config {
	cfg, _ := LoadConfigWithDiagnostics(root)
	return cfg
}

func LoadConfigWithDiagnostics(root string) (Config, ConfigLoadResult) {
	cfg := DefaultConfig
	res := ConfigLoadResult{}
	global := selectConfigCandidate("global", GlobalConfigPaths(), &res.Diagnostics)
	project := selectConfigCandidate("project", ConfigPaths(root), &res.Diagnostics)
	for _, item := range []struct{ layer, path string }{{"global", global}, {"project", project}} {
		if item.path == "" {
			res.Diagnostics = append(res.Diagnostics, ConfigDiagnostic{Path: item.layer, Layer: item.layer, Severity: "info", Message: "missing; defaults in effect"})
			continue
		}
		loaded, diags := loadConfigFromPathLayer(item.path, item.layer)
		res.Diagnostics = append(res.Diagnostics, diags...)
		if hasDiagError(diags) {
			continue
		}
		beforeOrchestration := cfg.Orchestration
		applyConfigDoc(&cfg, loaded.Doc)
		if _, hasOrchestration := loaded.Doc["orchestration"]; hasOrchestration {
			raw, _ := json.Marshal(loaded.Doc)
			if err := rejectSecretBearingOrchestration(raw); err != nil {
				Warn("orchestration config rejected: " + err.Error() + " — using previous/default orchestration")
				cfg.Orchestration = beforeOrchestration
			} else if err := ValidateOrchestrationConfig(&cfg.Orchestration); err != nil {
				Warn("orchestration config rejected: " + err.Error() + " — using previous/default orchestration")
				cfg.Orchestration = beforeOrchestration
			}
		}
		if item.layer == "global" {
			res.GlobalPath = item.path
		} else {
			res.ProjectPath = item.path
		}
	}
	if res.ProjectPath != "" || res.GlobalPath != "" {
		res.Present = true
		res.Digest = effectiveConfigDigest(res.GlobalPath, res.ProjectPath)
	}
	return cfg, res
}

func LoadConfigFromPath(path string) (loadedConfigFile, []ConfigDiagnostic) {
	return loadConfigFromPathLayer(path, "project")
}

func loadConfigFromPathLayer(path, layer string) (loadedConfigFile, []ConfigDiagnostic) {
	ext := strings.ToLower(filepath.Ext(path))
	raw, err := os.ReadFile(path)
	if err != nil {
		sev := "error"
		if os.IsNotExist(err) {
			sev = "info"
		}
		return loadedConfigFile{Path: path}, []ConfigDiagnostic{{Path: path, Source: path, Layer: layer, Severity: sev, Message: err.Error()}}
	}
	var doc map[string]any
	switch ext {
	case ".json":
		if err := json.Unmarshal(raw, &doc); err != nil {
			return loadedConfigFile{Path: path}, []ConfigDiagnostic{{Path: path, Source: path, Layer: layer, Severity: "error", Message: "invalid JSON: " + err.Error()}}
		}
	case ".yml", ".yaml":
		parsed, err := parseSimpleYAML(string(raw))
		if err != nil {
			return loadedConfigFile{Path: path}, []ConfigDiagnostic{{Path: path, Source: path, Layer: layer, Severity: "error", Message: "invalid YAML: " + err.Error()}}
		}
		doc = parsed
	default:
		return loadedConfigFile{Path: path}, []ConfigDiagnostic{{Path: path, Source: path, Layer: layer, Severity: "error", Message: "unsupported config extension " + ext}}
	}
	diags := []ConfigDiagnostic{}
	if ext == ".json" {
		diags = append(diags, ConfigDiagnostic{Path: path, Source: path, Layer: layer, Severity: "warning", Message: "legacy JSON config is deprecated; prefer config.yml"})
	}
	return loadedConfigFile{Path: path, Doc: doc}, diags
}

func selectConfigCandidate(layer string, paths []string, diags *[]ConfigDiagnostic) string {
	selected := ""
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			if selected == "" {
				selected = p
				continue
			}
			*diags = append(*diags, ConfigDiagnostic{Path: p, Source: p, Layer: layer, Severity: "warning", Message: "ignored lower-priority config; using " + selected})
		}
	}
	return selected
}

func effectiveConfigDigest(paths ...string) string {
	h := sha256.New()
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hasDiagError(diags []ConfigDiagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func applyConfigDoc(cfg *Config, doc map[string]any) {
	if v, ok := intAt(doc, "version"); ok {
		cfg.Version = v
	}
	if defs, ok := mapAt(doc, "defaults"); ok {
		if v, ok := stringAt(defs, "verify_command"); ok {
			cfg.DefaultVerify = v
		}
		if v, ok := stringAt(defs, "report_format"); ok {
			cfg.Report.Format = v
		}
		if v, ok := stringAt(defs, "subagent_mode"); ok {
			cfg.Roles.SubagentMode = v
		}
		if v, ok := intAt(defs, "promotion_threshold"); ok {
			cfg.PromotionThreshold = v
		}
	}
	if v, ok := stringAt(doc, "defaultVerify", "default_verify"); ok {
		cfg.DefaultVerify = v
	}
	if v, ok := intAt(doc, "promotionThreshold", "promotion_threshold"); ok {
		cfg.PromotionThreshold = v
	}
	if m, ok := mapAt(doc, "report"); ok {
		if v, ok := stringAt(m, "format"); ok {
			cfg.Report.Format = v
		}
		if v, ok := intAt(m, "autoRefreshSeconds", "auto_refresh_seconds"); ok {
			cfg.Report.AutoRefreshSeconds = v
		}
	}
	if m, ok := mapAt(doc, "roles"); ok {
		if v, ok := stringAt(m, "subagentMode", "subagent_mode"); ok {
			cfg.Roles.SubagentMode = v
		}
	}
	if m, ok := mapAt(doc, "gates"); ok {
		applyGates(&cfg.Gates, m)
	}
	if m, ok := mapAt(doc, "verify"); ok {
		if v, ok := stringAt(m, "sandbox"); ok {
			cfg.Verify.Sandbox = v
		}
	}
	if m, ok := mapAt(doc, "orchestration"); ok {
		applyOrchestration(&cfg.Orchestration, m)
	}
	if m, ok := mapAt(doc, "mcp"); ok {
		applyMCP(&cfg.MCP, m)
	}
}

func applyGates(g *GatesCfg, m map[string]any) {
	if v, ok := stringAt(m, "traceability"); ok {
		g.Traceability = v
	}
	if v, ok := stringAt(m, "acceptance"); ok {
		g.Acceptance = v
	}
	if v, ok := stringAt(m, "scope"); ok {
		g.Scope = v
	}
	if v, ok := stringAt(m, "contextBudget", "context_budget"); ok {
		g.ContextBudget = v
	}
	if v, ok := intAt(m, "maxContextTokens", "max_context_tokens"); ok {
		g.MaxContextTokens = v
	}
	if v, ok := stringAt(m, "modeCapability", "mode_capability"); ok {
		g.ModeCapability = v
	}
	if custom, ok := customGatesAt(m, "custom"); ok {
		g.Custom = custom
	}
}
func applyOrchestration(o *OrchestrationCfg, m map[string]any) {
	if v, ok := boolAt(m, "enabled"); ok {
		o.Enabled = v
	}
	if v, ok := stringAt(m, "approvalPolicy", "approval_policy"); ok {
		o.ApprovalPolicy = v
	}
	if v, ok := stringAt(m, "workerMode", "worker_mode"); ok {
		o.WorkerMode = v
	}
	if v, ok := intAt(m, "maxWorkers", "max_workers"); ok {
		o.MaxWorkers = v
	}
	if v, ok := intAt(m, "maxRetries", "max_retries"); ok {
		o.MaxRetries = v
	}
	if v, ok := intAt(m, "sessionTimeoutMinutes", "session_timeout_minutes"); ok {
		o.SessionTimeoutMinutes = v
	}
	if v, ok := floatAt(m, "hostReportedCostLimitUSD", "host_reported_cost_limit_usd"); ok {
		o.HostReportedCostLimitUSD = v
	}
	if v, ok := stringAt(m, "compactionPolicy", "compaction_policy"); ok {
		o.CompactionPolicy = v
	}
	if v, ok := floatAt(m, "compactionBudgetThreshold", "compaction_budget_threshold"); ok {
		o.CompactionBudgetThreshold = v
	}
	if t, ok := mapAt(m, "transport"); ok {
		applyTransport(&o.Transport, t)
	}
	if p, ok := mapAt(m, "program"); ok {
		if v, ok := intAt(p, "maxConcurrentSpecs", "max_concurrent_specs"); ok {
			o.Program.MaxConcurrentSpecs = v
		}
	}
	if r, ok := mapAt(m, "resilience"); ok {
		applyResilience(o, r)
	}
}
func applyResilience(o *OrchestrationCfg, m map[string]any) {
	if o.Resilience == nil {
		o.Resilience = &ResilienceCfg{}
	}
	if v, ok := boolAt(m, "checkpointEnabled", "checkpoint_enabled"); ok {
		o.Resilience.CheckpointEnabled = v
	}
	if v, ok := intAt(m, "maxSuspendSeconds", "max_suspend_seconds"); ok {
		o.Resilience.MaxSuspendSeconds = v
	}
	if v, ok := boolAt(m, "contextSnapshotEnabled", "context_snapshot_enabled"); ok {
		o.Resilience.ContextSnapshotEnabled = v
	}
	if v, ok := intAt(m, "progressTimeoutSeconds", "progress_timeout_seconds"); ok {
		o.Resilience.ProgressTimeoutSeconds = v
	}
	if a, ok := mapAt(m, "autoResume", "auto_resume"); ok {
		if v, ok := boolAt(a, "enabled"); ok {
			o.Resilience.AutoResume.Enabled = v
		}
		if v, ok := boolAt(a, "onHostStart", "on_host_start"); ok {
			o.Resilience.AutoResume.OnHostStart = v
		}
		if v, ok := intAt(a, "maxAgeMinutes", "max_age_minutes"); ok {
			o.Resilience.AutoResume.MaxAgeMinutes = v
		}
	}
}

func applyTransport(t *TransportCfg, m map[string]any) {
	if v, ok := stringAt(m, "kind"); ok {
		t.Kind = v
	}
	if v, ok := intAt(m, "pollIntervalMillis", "poll_interval_millis"); ok {
		t.PollIntervalMillis = v
	}
	if v, ok := intAt(m, "messageTTLSeconds", "message_ttl_seconds"); ok {
		t.MessageTTLSeconds = v
	}
	if v, ok := intAt(m, "leaseSeconds", "lease_seconds"); ok {
		t.LeaseSeconds = v
	}
	if v, ok := intAt(m, "heartbeatSeconds", "heartbeat_seconds"); ok {
		t.HeartbeatSeconds = v
	}
}
func applyMCP(mcp *MCPConfig, m map[string]any) {
	if v, ok := stringAt(m, "expose"); ok {
		mcp.Expose = v
	}
	if v, ok := boolAt(m, "includeMeta", "include_meta"); ok {
		mcp.IncludeMeta = v
	}
	if v, ok := boolAt(m, "includeOrchestration", "include_orchestration"); ok {
		mcp.IncludeOrchestration = &v
	}
	if a, ok := stringSliceAt(m, "essentialTools", "essential_tools"); ok {
		mcp.EssentialTools = a
	}
}

func mapAt(m map[string]any, keys ...string) (map[string]any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if mm, ok := v.(map[string]any); ok {
				return mm, true
			}
		}
	}
	return nil, false
}
func stringAt(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s, true
			}
		}
	}
	return "", false
}
func boolAt(m map[string]any, keys ...string) (bool, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if b, ok := v.(bool); ok {
				return b, true
			}
		}
	}
	return false, false
}
func intAt(m map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case int:
				return n, true
			case float64:
				return int(n), true
			case json.Number:
				i, _ := n.Int64()
				return int(i), true
			}
		}
	}
	return 0, false
}
func floatAt(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case int:
				return float64(n), true
			case float64:
				return n, true
			}
		}
	}
	return 0, false
}
func customGatesAt(m map[string]any, keys ...string) ([]CustomGateCfg, bool) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		arr, ok := v.([]any)
		if !ok {
			return nil, false
		}
		out := make([]CustomGateCfg, 0, len(arr))
		for _, item := range arr {
			mm, ok := item.(map[string]any)
			if !ok {
				continue
			}
			gate := CustomGateCfg{}
			if v, ok := stringAt(mm, "name"); ok {
				gate.Name = v
			}
			if v, ok := stringAt(mm, "command"); ok {
				gate.Command = v
			}
			if v, ok := stringAt(mm, "severity"); ok {
				gate.Severity = v
			}
			out = append(out, gate)
		}
		return out, true
	}
	return nil, false
}

func stringSliceAt(m map[string]any, keys ...string) ([]string, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			arr, ok := v.([]any)
			if !ok {
				return nil, false
			}
			out := []string{}
			for _, it := range arr {
				if s, ok := it.(string); ok {
					out = append(out, s)
				}
			}
			return out, true
		}
	}
	return nil, false
}

func parseSimpleYAML(raw string) (map[string]any, error) {
	root := map[string]any{}
	stack := []map[string]any{root}
	indents := []int{0}
	for lineNo, line := range strings.Split(raw, "\n") {
		line = stripYAMLComment(line)
		if strings.TrimSpace(line) == "" || strings.TrimSpace(line) == "---" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent%2 != 0 {
			return nil, fmt.Errorf("line %d: indentation must use two-space levels", lineNo+1)
		}
		for len(indents) > 1 && indent < indents[len(indents)-1] {
			indents = indents[:len(indents)-1]
			stack = stack[:len(stack)-1]
		}
		if indent > indents[len(indents)-1] {
			return nil, fmt.Errorf("line %d: unexpected indentation", lineNo+1)
		}
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("line %d: expected key: value", lineNo+1)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		cur := stack[len(stack)-1]
		if val == "" {
			child := map[string]any{}
			cur[key] = child
			stack = append(stack, child)
			indents = append(indents, indent+2)
			continue
		}
		cur[key] = parseYAMLScalar(val)
	}
	return root, nil
}
func stripYAMLComment(s string) string {
	inQ := byte(0)
	for i := 0; i < len(s); i++ {
		if (s[i] == '"' || s[i] == '\'') && (i == 0 || s[i-1] != '\\') {
			if inQ == 0 {
				inQ = s[i]
			} else if inQ == s[i] {
				inQ = 0
			}
		}
		if s[i] == '#' && inQ == 0 {
			return s[:i]
		}
	}
	return s
}
func parseYAMLScalar(s string) any {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	case "null", "~":
		return nil
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		body := strings.TrimSpace(s[1 : len(s)-1])
		if body == "" {
			return []any{}
		}
		parts := strings.Split(body, ",")
		out := make([]any, 0, len(parts))
		for _, p := range parts {
			out = append(out, parseYAMLScalar(strings.TrimSpace(p)))
		}
		return out
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}
