package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core/gates"
)

type allowlistEntry struct {
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

type allowlist struct {
	patterns map[string]struct{}
}

func loadAllowlist(root string) (allowlist, []gates.Finding) {
	path := filepath.Join(root, ".specd", "security", "allow.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return allowlist{}, nil
	}
	if err != nil {
		return allowlist{}, []gates.Finding{{
			Severity: gates.Error,
			Message:  fmt.Sprintf("allowlist read failed: %v", err),
		}}
	}

	var entries []allowlistEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return allowlist{}, []gates.Finding{{
			Severity: gates.Error,
			Message:  fmt.Sprintf("allowlist parse failed: %v", err),
		}}
	}

	allowed := allowlist{patterns: make(map[string]struct{}, len(entries))}
	var findings []gates.Finding
	for i, entry := range entries {
		pattern := strings.TrimSpace(entry.Pattern)
		reason := strings.TrimSpace(entry.Reason)
		if pattern == "" {
			findings = append(findings, gates.Finding{
				Severity: gates.Error,
				Message:  fmt.Sprintf("allowlist entry %d missing pattern", i),
			})
			continue
		}
		if reason == "" {
			findings = append(findings, gates.Finding{
				Severity: gates.Error,
				Message:  fmt.Sprintf("allowlist entry %q missing reason", pattern),
			})
			continue
		}
		allowed.patterns[strings.ToLower(pattern)] = struct{}{}
	}
	return allowed, findings
}

func (a allowlist) allows(pattern string) bool {
	if len(a.patterns) == 0 {
		return false
	}
	_, ok := a.patterns[strings.ToLower(strings.TrimSpace(pattern))]
	return ok
}
