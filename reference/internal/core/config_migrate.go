package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RenderConfigYAML renders Config in the canonical v2 human-authored YAML shape.
// Field order is intentionally stable so migration output is reviewable.
func RenderConfigYAML(cfg Config) string {
	var b strings.Builder
	b.WriteString("# specd configuration (YAML v2)\n")
	b.WriteString("# Machine runtime state remains JSON; this file is for human-authored policy.\n")
	wKV(&b, 0, "version", cfg.Version)
	b.WriteString("defaults:\n")
	wKV(&b, 2, "verify_command", cfg.DefaultVerify)
	wKV(&b, 2, "report_format", cfg.Report.Format)
	wKV(&b, 2, "subagent_mode", cfg.Roles.SubagentMode)
	wKV(&b, 2, "promotion_threshold", cfg.PromotionThreshold)
	b.WriteString("report:\n")
	wKV(&b, 2, "format", cfg.Report.Format)
	wKV(&b, 2, "auto_refresh_seconds", cfg.Report.AutoRefreshSeconds)
	b.WriteString("roles:\n")
	wKV(&b, 2, "subagent_mode", cfg.Roles.SubagentMode)
	b.WriteString("gates:\n")
	wKV(&b, 2, "traceability", cfg.Gates.Traceability)
	wKV(&b, 2, "acceptance", cfg.Gates.Acceptance)
	wKV(&b, 2, "scope", cfg.Gates.Scope)
	wKV(&b, 2, "context_budget", cfg.Gates.ContextBudget)
	wKV(&b, 2, "max_context_tokens", cfg.Gates.MaxContextTokens)
	wKV(&b, 2, "mode_capability", cfg.Gates.ModeCapability)
	b.WriteString("  custom: []\n")
	b.WriteString("verify:\n")
	wKV(&b, 2, "sandbox", cfg.Verify.Sandbox)
	b.WriteString("orchestration:\n")
	wKV(&b, 2, "enabled", cfg.Orchestration.Enabled)
	wKV(&b, 2, "approval_policy", cfg.Orchestration.ApprovalPolicy)
	wKV(&b, 2, "worker_mode", cfg.Orchestration.WorkerMode)
	wKV(&b, 2, "max_workers", cfg.Orchestration.MaxWorkers)
	wKV(&b, 2, "max_retries", cfg.Orchestration.MaxRetries)
	wKV(&b, 2, "session_timeout_minutes", cfg.Orchestration.SessionTimeoutMinutes)
	wKV(&b, 2, "host_reported_cost_limit_usd", cfg.Orchestration.HostReportedCostLimitUSD)
	if cfg.Orchestration.CompactionPolicy != "" {
		wKV(&b, 2, "compaction_policy", cfg.Orchestration.CompactionPolicy)
	}
	if cfg.Orchestration.CompactionBudgetThreshold != 0 {
		wKV(&b, 2, "compaction_budget_threshold", cfg.Orchestration.CompactionBudgetThreshold)
	}
	b.WriteString("  transport:\n")
	wKV(&b, 4, "kind", cfg.Orchestration.Transport.Kind)
	wKV(&b, 4, "poll_interval_millis", cfg.Orchestration.Transport.PollIntervalMillis)
	wKV(&b, 4, "message_ttl_seconds", cfg.Orchestration.Transport.MessageTTLSeconds)
	wKV(&b, 4, "lease_seconds", cfg.Orchestration.Transport.LeaseSeconds)
	wKV(&b, 4, "heartbeat_seconds", cfg.Orchestration.Transport.HeartbeatSeconds)
	b.WriteString("  program:\n")
	wKV(&b, 4, "max_concurrent_specs", cfg.Orchestration.Program.MaxConcurrentSpecs)
	if cfg.Orchestration.Resilience != nil {
		b.WriteString("  resilience:\n")
		wKV(&b, 4, "checkpoint_enabled", cfg.Orchestration.Resilience.CheckpointEnabled)
		wKV(&b, 4, "max_suspend_seconds", cfg.Orchestration.Resilience.MaxSuspendSeconds)
		wKV(&b, 4, "context_snapshot_enabled", cfg.Orchestration.Resilience.ContextSnapshotEnabled)
		wKV(&b, 4, "progress_timeout_seconds", cfg.Orchestration.Resilience.ProgressTimeoutSeconds)
		b.WriteString("    auto_resume:\n")
		wKV(&b, 6, "enabled", cfg.Orchestration.Resilience.AutoResume.Enabled)
		wKV(&b, 6, "on_host_start", cfg.Orchestration.Resilience.AutoResume.OnHostStart)
		wKV(&b, 6, "max_age_minutes", cfg.Orchestration.Resilience.AutoResume.MaxAgeMinutes)
	}
	if cfg.MCP.Configured() {
		b.WriteString("mcp:\n")
		wKV(&b, 2, "expose", cfg.MCP.Expose)
		wKV(&b, 2, "include_meta", cfg.MCP.IncludeMeta)
		if cfg.MCP.IncludeOrchestration != nil {
			wKV(&b, 2, "include_orchestration", *cfg.MCP.IncludeOrchestration)
		}
		if len(cfg.MCP.EssentialTools) > 0 {
			b.WriteString("  essential_tools: [")
			for i, s := range cfg.MCP.EssentialTools {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(quoteYAML(s))
			}
			b.WriteString("]\n")
		}
	}
	return b.String()
}

func wKV(b *strings.Builder, indent int, key string, v any) {
	b.WriteString(strings.Repeat(" ", indent))
	b.WriteString(key)
	b.WriteString(": ")
	switch x := v.(type) {
	case string:
		b.WriteString(quoteYAML(x))
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(x))
	case float64:
		b.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
	default:
		b.WriteString(fmt.Sprint(x))
	}
	b.WriteByte('\n')
}

func quoteYAML(s string) string {
	return strconv.Quote(s)
}

// ValidateConfigDoc checks a parsed config document's enum-valued fields
// (accepting both camelCase and snake_case keys) against their allowed
// values and returns a single combined error listing every violation, or nil
// if the document is valid.
func ValidateConfigDoc(doc map[string]any) error {
	var errs []string
	checkEnumDoc(doc, "roles.subagentMode", []string{"inline", "delegate"}, &errs)
	checkEnumDoc(doc, "roles.subagent_mode", []string{"inline", "delegate"}, &errs)
	for _, p := range []string{"gates.traceability", "gates.acceptance", "gates.scope", "gates.contextBudget", "gates.context_budget", "gates.modeCapability", "gates.mode_capability"} {
		checkEnumDoc(doc, p, []string{"", "off", "warn", "error"}, &errs)
	}
	checkEnumDoc(doc, "verify.sandbox", []string{"none", "bwrap", "container"}, &errs)
	checkEnumDoc(doc, "orchestration.approvalPolicy", []string{"manual", "planning", "session"}, &errs)
	checkEnumDoc(doc, "orchestration.approval_policy", []string{"manual", "planning", "session"}, &errs)
	checkEnumDoc(doc, "orchestration.workerMode", []string{"host"}, &errs)
	checkEnumDoc(doc, "orchestration.worker_mode", []string{"host"}, &errs)
	checkEnumDoc(doc, "orchestration.transport.kind", []string{"file"}, &errs)
	checkEnumDoc(doc, "orchestration.compactionPolicy", []string{"", CompactionNone, CompactionPhase, CompactionBudget, CompactionBoth}, &errs)
	checkEnumDoc(doc, "orchestration.compaction_policy", []string{"", CompactionNone, CompactionPhase, CompactionBudget, CompactionBoth}, &errs)
	checkEnumDoc(doc, "mcp.expose", []string{"", "all", "essential", "phase"}, &errs)
	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func checkEnumDoc(doc map[string]any, path string, allowed []string, errs *[]string) {
	v, ok := valueAtPath(doc, path)
	if !ok || v == nil {
		return
	}
	s, ok := v.(string)
	if !ok {
		*errs = append(*errs, path+" must be string")
		return
	}
	for _, a := range allowed {
		if s == a {
			return
		}
	}
	*errs = append(*errs, fmt.Sprintf("%s unsupported %q", path, s))
}

func valueAtPath(doc map[string]any, path string) (any, bool) {
	cur := any(doc)
	for _, p := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
