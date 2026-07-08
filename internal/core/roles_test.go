package core

import "testing"

func TestRolePromptDedup(t *testing.T) {
	prompt := RolePrompt("validator")
	got := DedupRolePrompts([]string{prompt, prompt})
	if len(got) != 1 || got[0] != prompt {
		t.Fatalf("DedupRolePrompts = %#v", got)
	}
}
