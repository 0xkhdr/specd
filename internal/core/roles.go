package core

import (
	"fmt"
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

// IsWriteRole reports whether role is permitted to write product code. Only the
// craftsman writes; scout, validator, and auditor are read-only. Used by the
// verify gate to reject a write task hiding behind a trivial verify (spec 01 R4.2).
func IsWriteRole(role string) bool {
	return strings.TrimSpace(role) == "craftsman"
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
