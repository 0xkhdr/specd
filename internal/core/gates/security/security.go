package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core/gates"
)

type Gate struct{}

type allowEntry struct {
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

func New() Gate {
	return Gate{}
}

func (Gate) Name() string {
	return "security"
}

func (Gate) Run(ctx gates.CheckCtx) []gates.Finding {
	allowed, findings := loadAllowlist(ctx.Root)
	for _, task := range ctx.Tasks {
		text := task.Files + "\n" + task.Verify
		for _, pattern := range []string{"curl | sh", "rm -rf /", "chmod 777", "eval "} {
			if strings.Contains(text, pattern) && !allowed[pattern] {
				findings = append(findings, gates.Finding{
					Severity: gates.Error,
					Message:  fmt.Sprintf("%s contains blocked security pattern %q", task.ID, pattern),
				})
			}
		}
	}
	return findings
}

func loadAllowlist(root string) (map[string]bool, []gates.Finding) {
	allowed := map[string]bool{}
	if root == "" {
		return allowed, nil
	}
	raw, err := os.ReadFile(filepath.Join(root, ".specd", "security", "allow.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return allowed, nil
		}
		return allowed, []gates.Finding{{Severity: gates.Error, Message: err.Error()}}
	}
	var entries []allowEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return allowed, []gates.Finding{{Severity: gates.Error, Message: err.Error()}}
	}
	var findings []gates.Finding
	for i, entry := range entries {
		if strings.TrimSpace(entry.Reason) == "" {
			findings = append(findings, gates.Finding{Severity: gates.Error, Message: fmt.Sprintf("allowlist entry %d missing reason", i)})
			continue
		}
		allowed[entry.Pattern] = true
	}
	return allowed, findings
}
