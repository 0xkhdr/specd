package core

import (
	"strings"

	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

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
