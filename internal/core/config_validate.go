package core

import (
	"sort"
	"strconv"
	"strings"
)

func applyConfigMap(cfg *Config, values map[string]string, path string, diagnostics *[]Diagnostic) {
	for key, value := range values {
		if isSecretKey(key) {
			*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "secret value not allowed: " + key})
			continue
		}
		switch key {
		case "version":
			cfg.Version = value
		case "agent":
			cfg.Agent = value
		case "gates.verify":
			cfg.Gates.Verify = value
		case "verify.trivial":
			var trivial []string
			for _, part := range strings.Split(value, ",") {
				if p := strings.TrimSpace(part); p != "" {
					trivial = append(trivial, p)
				}
			}
			if len(trivial) == 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "verify.trivial must list at least one command"})
				continue
			}
			cfg.Verify.Trivial = trivial
		case "verify.timeout_seconds":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "verify.timeout_seconds must be integer >= 0"})
				continue
			}
			cfg.Verify.TimeoutSecs = parsed
		case "context.max_tokens":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "context.max_tokens must be positive integer"})
				continue
			}
			cfg.Context.MaxTokens = parsed
		case "orchestration.enabled":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "orchestration.enabled must be boolean"})
				continue
			}
			cfg.Orchestration.Enabled = parsed
		case "orchestration.model":
			cfg.Orchestration.Model = value
		case "routing.version":
			if value != "1" {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.version must be 1"})
				continue
			}
			cfg.Routing.Version = value
		case "routing.classes":
			items, ok := configList(value)
			if !ok {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.classes must be a unique comma-separated list"})
				continue
			}
			cfg.Routing.Classes = items
			if !contains(items, cfg.Routing.DefaultClass) {
				cfg.Routing.DefaultClass = items[0]
			}
		case "routing.default_class":
			cfg.Routing.DefaultClass = value
		case "routing.fallback":
			items, ok := configList(value)
			if !ok {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.fallback must be a unique comma-separated list"})
				continue
			}
			cfg.Routing.Fallback = items
		case "routing.class_capabilities":
			parsed, ok := parseClassCapabilities(value)
			if !ok {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.class_capabilities must use class=capability+capability entries"})
				continue
			}
			cfg.Routing.ClassCapabilities = parsed
		case "routing.max_tokens":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed < 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.max_tokens must be integer >= 0"})
				continue
			}
			cfg.Routing.MaxTokens = parsed
		case "routing.max_cost_micros":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed < 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.max_cost_micros must be integer >= 0"})
				continue
			}
			cfg.Routing.MaxCostMicros = parsed
		case "routing.deadline_seconds":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.deadline_seconds must be integer >= 0"})
				continue
			}
			cfg.Routing.DeadlineSeconds = parsed
		case "routing.max_retries":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.max_retries must be integer >= 0"})
				continue
			}
			cfg.Routing.MaxRetries = parsed
		case "routing.allow_unknown_telemetry":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "routing.allow_unknown_telemetry must be boolean"})
				continue
			}
			cfg.Routing.AllowUnknownTelemetry = parsed
		case "criteria.required":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "criteria.required must be boolean"})
				continue
			}
			cfg.Criteria.Required = parsed
		case "review.required":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "review.required must be boolean"})
				continue
			}
			cfg.Review.Required = parsed
		case "submit.command":
			cfg.Submit.Command = value
		case "submit.timeout_seconds":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "submit.timeout_seconds must be positive integer"})
				continue
			}
			cfg.Submit.TimeoutSecs = parsed
		case "security.secrets":
			if !validSeverity(value) {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "security.secrets must be off|warn|error"})
				continue
			}
			cfg.Security.Secrets = value
		case "security.profile":
			if value != "prototype" && value != "production" {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "security.profile must be prototype|production"})
				continue
			}
			cfg.Security.Profile = value
		case "security.injection":
			if !validSeverity(value) {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "security.injection must be off|warn|error"})
				continue
			}
			cfg.Security.Injection = value
		case "security.slopsquat":
			if !validSeverity(value) {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "security.slopsquat must be off|warn|error"})
				continue
			}
			cfg.Security.Slopsquat = value
		case "security.clean_worktree":
			if !validSeverity(value) {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "security.clean_worktree must be off|warn|error"})
				continue
			}
			cfg.Security.CleanWorktree = value
		case "security.sandbox":
			if !validSeverity(value) {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "security.sandbox must be off|warn|error"})
				continue
			}
			cfg.Security.Sandbox = value
		case "escalation.max_verify_fails":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "escalation.max_verify_fails must be integer >= 0"})
				continue
			}
			cfg.Escalation.MaxVerifyFails = parsed
		case "promotion_threshold":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "promotion_threshold must be integer >= 1"})
				continue
			}
			cfg.PromotionThreshold = parsed
		default:
			*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: "unknown config key: " + key})
		}
	}
	validateRouting(cfg, path, diagnostics)
}

func configList(value string) ([]string, bool) {
	var out []string
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			return nil, false
		}
		seen[item] = true
		out = append(out, item)
	}
	return out, len(out) > 0
}

func parseClassCapabilities(value string) (map[string][]string, bool) {
	out := map[string][]string{}
	for _, entry := range strings.Split(value, ";") {
		parts := strings.SplitN(strings.TrimSpace(entry), "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, false
		}
		caps, ok := configList(strings.ReplaceAll(parts[1], "+", ","))
		if !ok || out[parts[0]] != nil {
			return nil, false
		}
		sort.Strings(caps)
		out[parts[0]] = caps
	}
	return out, len(out) > 0
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validateRouting(cfg *Config, path string, diagnostics *[]Diagnostic) {
	bad := func(message string) {
		*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Path: path, Message: message})
	}
	if !contains(cfg.Routing.Classes, cfg.Routing.DefaultClass) {
		bad("routing.default_class must name a routing class")
	}
	for _, class := range cfg.Routing.Fallback {
		if !contains(cfg.Routing.Classes, class) {
			bad("routing.fallback contains unknown class: " + class)
		}
	}
	for class := range cfg.Routing.ClassCapabilities {
		if !contains(cfg.Routing.Classes, class) {
			bad("routing.class_capabilities contains unknown class: " + class)
		}
	}
}

func applyEnv(cfg *Config, env map[string]string, diagnostics *[]Diagnostic) {
	for key, value := range env {
		switch key {
		case "SPECD_AGENT":
			cfg.Agent = value
		case "SPECD_GATES_VERIFY":
			cfg.Gates.Verify = value
		case "SPECD_VERIFY_TIMEOUT_SECONDS":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_VERIFY_TIMEOUT_SECONDS must be integer >= 0"})
				continue
			}
			cfg.Verify.TimeoutSecs = parsed
		case "SPECD_CONTEXT_MAX_TOKENS":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_CONTEXT_MAX_TOKENS must be positive integer"})
				continue
			}
			cfg.Context.MaxTokens = parsed
		case "SPECD_ORCHESTRATION_ENABLED":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_ORCHESTRATION_ENABLED must be boolean"})
				continue
			}
			cfg.Orchestration.Enabled = parsed
		case "SPECD_ORCHESTRATION_MODEL":
			cfg.Orchestration.Model = value
		case "SPECD_REVIEW_REQUIRED":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_REVIEW_REQUIRED must be boolean"})
				continue
			}
			cfg.Review.Required = parsed
		case "SPECD_SUBMIT_COMMAND":
			cfg.Submit.Command = value
		case "SPECD_SUBMIT_TIMEOUT_SECONDS":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_SUBMIT_TIMEOUT_SECONDS must be positive integer"})
				continue
			}
			cfg.Submit.TimeoutSecs = parsed
		case "SPECD_CRITERIA_REQUIRED":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_CRITERIA_REQUIRED must be boolean"})
				continue
			}
			cfg.Criteria.Required = parsed
		case "SPECD_PROMOTION_THRESHOLD":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_PROMOTION_THRESHOLD must be integer >= 1"})
				continue
			}
			cfg.PromotionThreshold = parsed
		case "SPECD_SECURITY_CLEAN_WORKTREE":
			if !validSeverity(value) {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_SECURITY_CLEAN_WORKTREE must be off|warn|error"})
				continue
			}
			cfg.Security.CleanWorktree = value
		case "SPECD_SECURITY_PROFILE":
			if value != "prototype" && value != "production" {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_SECURITY_PROFILE must be prototype|production"})
				continue
			}
			cfg.Security.Profile = value
		case "SPECD_SECURITY_SANDBOX":
			if !validSeverity(value) {
				*diagnostics = append(*diagnostics, Diagnostic{Severity: "error", Message: "SPECD_SECURITY_SANDBOX must be off|warn|error"})
				continue
			}
			cfg.Security.Sandbox = value
		}
	}
}

func validSeverity(value string) bool {
	for _, s := range SecuritySeverities {
		if value == s {
			return true
		}
	}
	return false
}

func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	for _, part := range strings.FieldsFunc(key, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	}) {
		if part == "secret" || part == "token" || part == "apikey" {
			return true
		}
	}
	return strings.Contains(key, "api_key")
}
