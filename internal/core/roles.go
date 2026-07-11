package core

import (
	"sort"
	"strings"

	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

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
// WriteScaffold. An unknown role falls back to craftsman.
func RolePrompt(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		role = "craftsman"
	}
	raw, err := embedtemplates.FS.ReadFile("roles/" + role + ".md")
	if err != nil {
		raw, err = embedtemplates.FS.ReadFile("roles/craftsman.md")
		if err != nil {
			return "role:" + role + "\n"
		}
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
