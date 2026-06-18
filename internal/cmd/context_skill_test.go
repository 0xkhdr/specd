package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestPhaseSkill asserts every spec status maps to a stage skill that init
// actually scaffolds — so `specd context` can never point at a missing skill.
func TestPhaseSkill(t *testing.T) {
	cases := map[core.SpecStatus]string{
		core.StatusRequirements: "specd-requirements",
		core.StatusDesign:       "specd-design",
		core.StatusTasks:        "specd-tasks",
		core.StatusExecuting:    "specd-execute",
		core.StatusBlocked:      "specd-execute",
		core.StatusVerifying:    "specd-execute",
		core.StatusComplete:     "specd-foundations",
	}
	shipped := map[string]bool{}
	for _, asset := range core.DefaultScaffoldManifest() {
		const prefix = ".specd/skills/"
		if !strings.HasPrefix(asset.Target, prefix) {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(asset.Target, prefix), "/SKILL.md")
		shipped[name] = true
	}
	for status, want := range cases {
		got := phaseSkill(status)
		if !strings.Contains(got, want) {
			t.Errorf("phaseSkill(%q) = %q, want it to name %q", status, got, want)
		}
		// the named skill must be one init scaffolds
		name := strings.TrimSuffix(strings.TrimPrefix(got, ".specd/skills/"), "/SKILL.md")
		if !shipped[name] {
			t.Errorf("phaseSkill(%q) points at %q which init does not scaffold", status, name)
		}
	}
}
