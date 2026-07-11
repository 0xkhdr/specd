package security

import (
	"encoding/json"
	"fmt"
	"github.com/0xkhdr/specd/internal/core"
)

const PolicyVersion = "1"

type PolicyV1 struct {
	PolicyVersion     string            `json:"policy_version"`
	Profile           string            `json:"profile"`
	ScannerSeverities map[string]string `json:"scanner_severities"`
	RequiredGates     []string          `json:"required_gates"`
	SandboxRequired   bool              `json:"sandbox_required"`
	NetworkDefault    string            `json:"network_default"`
	PolicyDigest      string            `json:"policy_digest"`
}

func ResolvePolicy(cfg core.SecurityConfig) (PolicyV1, error) {
	if cfg.Profile != "prototype" && cfg.Profile != "production" {
		return PolicyV1{}, fmt.Errorf("SECURITY_PROFILE_INVALID: %q", cfg.Profile)
	}
	p := PolicyV1{PolicyVersion: PolicyVersion, Profile: cfg.Profile, ScannerSeverities: map[string]string{"secrets": cfg.Secrets, "injection": cfg.Injection, "slopsquat": cfg.Slopsquat, "clean_worktree": cfg.CleanWorktree, "sandbox": cfg.Sandbox}, NetworkDefault: "allow"}
	if cfg.Profile == "production" {
		p.RequiredGates = []string{"security", "scope", "sandbox", "evidence", "review"}
		p.SandboxRequired = true
		p.NetworkDefault = "deny"
		for k := range p.ScannerSeverities {
			p.ScannerSeverities[k] = "error"
		}
	}
	raw, _ := json.Marshal(p)
	p.PolicyDigest = core.Digest(raw)
	return p, nil
}

func ConfigForPolicy(cfg core.SecurityConfig, p PolicyV1) core.SecurityConfig {
	cfg.Profile = p.Profile
	cfg.Secrets = p.ScannerSeverities["secrets"]
	cfg.Injection = p.ScannerSeverities["injection"]
	cfg.Slopsquat = p.ScannerSeverities["slopsquat"]
	cfg.CleanWorktree = p.ScannerSeverities["clean_worktree"]
	cfg.Sandbox = p.ScannerSeverities["sandbox"]
	return cfg
}
