package core

import (
	"encoding/json"
	"os"
	"sort"
)

type ToolContract struct {
	OperationID       string     `json:"operation_id"`
	Name              string     `json:"name"`
	Route             string     `json:"route"`
	Phases            []Phase    `json:"phases"`
	Capability        string     `json:"capability"`
	Mutable           bool       `json:"mutable"`
	HumanOnly         bool       `json:"human_only"`
	AuthorityRequired bool       `json:"authority_required"`
	TaskRequired      bool       `json:"task_required"`
	ScopeSource       string     `json:"scope_source"`
	NetworkClass      string     `json:"network_class"`
	ExitCodes         []ExitCode `json:"exit_codes"`
}

// ManifestToolContracts derives driver routes from canonical Operations.
func ManifestToolContracts() []ToolContract {
	out := make([]ToolContract, 0, len(Operations))
	for _, operation := range Operations {
		if ForbiddenTool(operation.Command) {
			continue
		}
		mutable := operation.Effect != EffectRead
		capability := "read"
		if mutable {
			capability = "write"
		}
		humanOnly := operation.Actor == ActorHuman
		if humanOnly {
			capability, mutable = "human", true
		}
		out = append(out, ToolContract{OperationID: operation.ID, Name: operation.Command, Route: "cli:" + operation.ID, Phases: append([]Phase(nil), operation.AllowedPhases...), Capability: capability, Mutable: mutable, HumanOnly: humanOnly, AuthorityRequired: operation.AuthorityRequired, TaskRequired: operation.TaskRequired, ScopeSource: operation.ScopeSource, NetworkClass: operation.NetworkClass, ExitCodes: append([]ExitCode(nil), operation.ExitCodes...)})
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
	case "approve", "brain", "decision", "deploy", "incident", "init", "mcp", "memory", "release", "report", "task":
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
