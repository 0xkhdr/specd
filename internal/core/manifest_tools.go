package core

import (
	"encoding/json"
	"os"
)

type ToolPolicy struct {
	Optional map[string]bool
}

type toolPolicyFile struct {
	Optional []string `json:"optional"`
}

func ForbiddenTool(name string) bool {
	switch name {
	case "approve", "brain", "decision", "init", "mcp", "memory", "report", "task":
		return true
	default:
		return false
	}
}

func LoadToolPolicy(path string) ToolPolicy {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ToolPolicy{Optional: map[string]bool{}}
	}
	var file toolPolicyFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return ToolPolicy{Optional: map[string]bool{}}
	}
	policy := ToolPolicy{Optional: map[string]bool{}}
	for _, name := range file.Optional {
		if name == "" || ForbiddenTool(name) {
			continue
		}
		policy.Optional[name] = true
	}
	return policy
}
