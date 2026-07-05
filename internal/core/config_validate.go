package core

import (
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
}

func applyEnv(cfg *Config, env map[string]string, diagnostics *[]Diagnostic) {
	for key, value := range env {
		switch key {
		case "SPECD_AGENT":
			cfg.Agent = value
		case "SPECD_GATES_VERIFY":
			cfg.Gates.Verify = value
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
