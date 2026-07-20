package core

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

// MaintenanceTemplate describes one inspectable, embedded maintenance intake
// template. Templates contain guidance only; gates remain deterministic.
type MaintenanceTemplate struct {
	Name    string
	Schema  string
	Version int
	Body    string
}

// MaintenanceTemplates returns embedded templates in stable name order.
func MaintenanceTemplates() ([]MaintenanceTemplate, error) {
	entries, err := embedtemplates.FS.ReadDir("maintenance")
	if err != nil {
		return nil, err
	}
	var out []MaintenanceTemplate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := embedtemplates.FS.ReadFile("maintenance/" + entry.Name())
		if err != nil {
			return nil, err
		}
		body := string(raw)
		if !strings.Contains(body, "schema: specd-maintenance") || !strings.Contains(body, "version: 1") {
			return nil, fmt.Errorf("maintenance template %s lacks schema/version", entry.Name())
		}
		out = append(out, MaintenanceTemplate{Name: strings.TrimSuffix(entry.Name(), ".md"), Schema: "specd-maintenance", Version: 1, Body: body})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// PolicyTemplates returns optional organization policy starters. Policy text is
// inspectable and operator-owned outside managed markers; it never changes gates.
func PolicyTemplates() ([]MaintenanceTemplate, error) {
	entries, err := embedtemplates.FS.ReadDir("policy")
	if err != nil {
		return nil, err
	}
	out := make([]MaintenanceTemplate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := embedtemplates.FS.ReadFile("policy/" + entry.Name())
		if err != nil {
			return nil, err
		}
		body := string(raw)
		if !strings.Contains(body, "schema: specd-policy") || !strings.Contains(body, "version: 1") {
			return nil, fmt.Errorf("policy template %s lacks schema/version", entry.Name())
		}
		out = append(out, MaintenanceTemplate{Name: strings.TrimSuffix(entry.Name(), ".md"), Schema: "specd-policy", Version: 1, Body: body})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// KnownRoles returns the canonical role names, derived from the embedded role
// files (the single source of truth also written to .specd/roles/). Sorted for
// deterministic gate findings.
func KnownRoles() []string {
	entries, err := embedtemplates.FS.ReadDir("roles")
	if err != nil {
		return nil
	}
	var roles []string
	for _, entry := range entries {
		if name := strings.TrimSuffix(entry.Name(), ".md"); name != entry.Name() {
			roles = append(roles, name)
		}
	}
	sort.Strings(roles)
	return roles
}

// Effect names the one kind of change an operation can make. Roles declare an
// explicit set: "read-only" is not an effect, it is the absence of every write
// effect, and saying so out loud is the point (spec agent-protocol-clarity R2.1).
type RoleEffect string

const (
	RoleEffectWorkspaceRead        RoleEffect = "workspace-read"
	RoleEffectWorkspaceWrite       RoleEffect = "workspace-write"
	RoleEffectHarnessEvidenceWrite RoleEffect = "harness-evidence-write"
	RoleEffectHarnessStateWrite    RoleEffect = "harness-state-write"
	RoleEffectExternalWrite        RoleEffect = "external-write"
)

// RoleCapability is the machine-readable authority contract for one role. Role
// Markdown explains behavior to a reader; this contract alone defines what the
// role may do. Prose that disagrees is a conformance test failure, never a
// runtime resolution (R6.1, R6.2).
type RoleCapability struct {
	Role                string       `json:"role"`
	Effects             []RoleEffect `json:"effects"`
	AllowedOperations   []string     `json:"allowed_operations"`
	CompletionAuthority bool         `json:"completion_authority"`
	HumanAuthority      bool         `json:"human_authority"`
	PathScope           string       `json:"path_scope"`
	NetworkPolicy       string       `json:"network_policy"`
	SandboxRequired     bool         `json:"sandbox_required"`
}

// roleCapabilities is the single source of role authority. No role carries
// HumanAuthority: approval is human-only and no contract may grant it.
var roleCapabilities = map[string]RoleCapability{
	"craftsman": {
		Effects:             []RoleEffect{RoleEffectWorkspaceRead, RoleEffectWorkspaceWrite, RoleEffectHarnessEvidenceWrite, RoleEffectHarnessStateWrite},
		AllowedOperations:   []string{"check", "complete-task", "context", "status", "verify"},
		CompletionAuthority: true,
		PathScope:           "declared-files",
		NetworkPolicy:       "deny",
		SandboxRequired:     true,
	},
	"scout": {
		Effects:           []RoleEffect{RoleEffectWorkspaceRead},
		AllowedOperations: []string{"check", "context", "status"},
		PathScope:         "workspace-read",
		NetworkPolicy:     "deny",
	},
	// The validator writes an evidence record, so it is not read-only (R2.2).
	"validator": {
		Effects:           []RoleEffect{RoleEffectWorkspaceRead, RoleEffectHarnessEvidenceWrite},
		AllowedOperations: []string{"check", "context", "status", "verify"},
		PathScope:         "workspace-read",
		NetworkPolicy:     "deny",
	},
	"auditor": {
		Effects:           []RoleEffect{RoleEffectWorkspaceRead},
		AllowedOperations: []string{"check", "context", "report", "status"},
		PathScope:         "workspace-read",
		NetworkPolicy:     "deny",
	},
}

// RoleCapabilityFor returns the contract for role. An unknown or undeclared
// role fails closed: the empty effect set, no operations, no authority. It
// never defaults to workspace-write.
func RoleCapabilityFor(role string) (RoleCapability, bool) {
	role = strings.TrimSpace(role)
	capability, ok := roleCapabilities[role]
	if !ok {
		return RoleCapability{Role: role, Effects: []RoleEffect{}, AllowedOperations: []string{}, NetworkPolicy: "deny"}, false
	}
	capability.Role = role
	capability.Effects = append([]RoleEffect(nil), capability.Effects...)
	capability.AllowedOperations = append([]string(nil), capability.AllowedOperations...)
	return capability, true
}

// RoleHasEffect reports whether role's contract declares effect.
func RoleHasEffect(role string, effect RoleEffect) bool {
	capability, _ := RoleCapabilityFor(role)
	return slices.Contains(capability.Effects, effect)
}

// RoleAllowsOperation reports whether role's contract permits operation. Default
// deny: an operation absent from the contract is denied, not inherited.
func RoleAllowsOperation(role, operation string) bool {
	capability, _ := RoleCapabilityFor(role)
	return slices.Contains(capability.AllowedOperations, strings.TrimSpace(operation))
}

// IsWriteRole reports whether role is permitted to write product code. Only the
// craftsman writes; scout, validator, and auditor are read-only. Used by the
// verify gate to reject a write task hiding behind a trivial verify (spec 01 R4.2).
func IsWriteRole(role string) bool {
	return RoleHasEffect(role, RoleEffectWorkspaceWrite)
}

// IsKnownRole reports whether role is one of the canonical roles (spec 01 R4.1).
func IsKnownRole(role string) bool {
	role = strings.TrimSpace(role)
	for _, known := range KnownRoles() {
		if known == role {
			return true
		}
	}
	return false
}

// RolePrompt returns the role constitution for role, read from the embedded
// role files — the single source of truth also written to .specd/roles/ by
// WriteScaffold. Unknown roles fail closed and never inherit craftsman prose.
func RolePrompt(role string) string {
	role = strings.TrimSpace(role)
	if !IsKnownRole(role) {
		return "role:invalid\n"
	}
	raw, err := embedtemplates.FS.ReadFile("roles/" + role + ".md")
	if err != nil {
		return "role:invalid\n"
	}
	return string(raw)
}

func DedupRolePrompts(prompts []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, prompt := range prompts {
		if seen[prompt] {
			continue
		}
		seen[prompt] = true
		out = append(out, prompt)
	}
	return out
}
