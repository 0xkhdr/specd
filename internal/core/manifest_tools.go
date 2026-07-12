package core

import (
	"encoding/json"
	"os"
	"sort"
)

type ToolContract struct {
	Name       string     `json:"name"`
	Route      string     `json:"route"`
	Phases     []Phase    `json:"phases"`
	Capability string     `json:"capability"`
	Mutable    bool       `json:"mutable"`
	HumanOnly  bool       `json:"human_only"`
	ExitCodes  []ExitCode `json:"exit_codes"`
}

// ManifestToolContracts derives driver routes from canonical Commands.
func ManifestToolContracts() []ToolContract {
	out := make([]ToolContract, 0, len(Commands))
	for _, command := range Commands {
		if ForbiddenTool(command.Name) {
			continue
		}
		mutable := command.RequiresTask || command.Name == "verify" || command.Name == "submit" || command.Name == "review"
		capability := "read"
		if mutable {
			capability = "write"
		}
		if command.HumanOnly {
			capability, mutable = "human", true
		}
		out = append(out, ToolContract{Name: command.Name, Route: "cli:" + command.Name, Phases: append([]Phase(nil), command.AllowedPhases...), Capability: capability, Mutable: mutable, HumanOnly: command.HumanOnly, ExitCodes: append([]ExitCode(nil), command.ExitCodes...)})
	}
	return out
}

// SupportedToolCapabilities returns deterministic non-human capabilities
// available from the canonical palette in one phase. Skill prose consumes
// this set but can never enlarge it.
func SupportedToolCapabilities(contracts []ToolContract, phase Phase) []string {
	seen := map[string]bool{}
	for _, contract := range contracts {
		if contract.HumanOnly || contract.Capability == "" || contract.Capability == "human" {
			continue
		}
		for _, allowed := range contract.Phases {
			if allowed == phase {
				seen[contract.Capability] = true
				break
			}
		}
	}
	out := make([]string, 0, len(seen))
	for capability := range seen {
		out = append(out, capability)
	}
	sort.Strings(out)
	return out
}

type ToolPolicy struct {
	Optional map[string]bool
}

type toolPolicyFile struct {
	Optional []string `json:"optional"`
}

func ForbiddenTool(name string) bool {
	switch name {
	case "approve", "brain", "decision", "deploy", "init", "mcp", "memory", "release", "report", "task":
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
