package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestInitSkillTemplatesExist asserts every skill named in init's skillFiles list
// ships an embedded SKILL.md template with valid frontmatter. It fails if a skill
// is added to the list without a template (skill drift), mirroring the spirit of
// TestRegistryMatchesHelp.
func TestInitSkillTemplatesExist(t *testing.T) {
	for _, s := range skillFiles {
		rel := "skills/" + s + "/SKILL.md"
		content, err := core.ReadTemplate(rel)
		if err != nil {
			t.Errorf("skill %q listed in skillFiles but template %q is missing: %v", s, rel, err)
			continue
		}
		if !strings.Contains(content, "name:") {
			t.Errorf("skill template %q missing frontmatter name: key", rel)
		}
		if !strings.Contains(content, "name: "+s) {
			t.Errorf("skill template %q frontmatter name does not match dir %q", rel, s)
		}
	}
}
