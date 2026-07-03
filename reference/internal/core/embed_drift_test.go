package core

import (
	"strings"
	"testing"
)

// TestSkillIndexesAgree asserts every skill directory under
// embed_templates/skills/ is listed in BOTH canonical indexes: the AGENTS.md
// anchor and the specd-foundations SKILL.md. A new skill that is not added to
// both indexes fails CI rather than silently drifting out of discovery.
func TestSkillIndexesAgree(t *testing.T) {
	entries, err := TemplatesFS.ReadDir("embed_templates/skills")
	if err != nil {
		t.Fatalf("ReadDir(skills): %v", err)
	}

	agents, err := ReadTemplate("AGENTS.md")
	if err != nil {
		t.Fatalf("ReadTemplate(AGENTS.md): %v", err)
	}
	foundations, err := ReadTemplate("skills/specd-foundations/SKILL.md")
	if err != nil {
		t.Fatalf("ReadTemplate(foundations): %v", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skill := e.Name()
		// foundations indexes the other skills, not itself in the table.
		if !strings.Contains(agents, skill) {
			t.Errorf("AGENTS.md skill index missing %q", skill)
		}
		if skill == "specd-foundations" {
			continue
		}
		if !strings.Contains(foundations, skill) {
			t.Errorf("specd-foundations SKILL.md index missing %q", skill)
		}
	}
}
