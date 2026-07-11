package core

import "testing"

func TestRolePromptDedup(t *testing.T) {
	prompt := RolePrompt("validator")
	got := DedupRolePrompts([]string{prompt, prompt})
	if len(got) != 1 || got[0] != prompt {
		t.Fatalf("DedupRolePrompts = %#v", got)
	}
}

func TestRolePromptUnknownNeverCraftsman(t *testing.T) {
	if got := RolePrompt("unknown"); got != "role:invalid\n" {
		t.Fatalf("unknown role prompt=%q", got)
	}
	if got := KnownRoles(); len(got) != 4 {
		t.Fatalf("known roles=%v", got)
	}
}
