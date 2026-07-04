package core

import "strings"

func RolePrompt(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		role = "craftsman"
	}
	return "role:" + role + "\nfollow task files and verify command\n"
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
