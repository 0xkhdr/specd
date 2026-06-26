package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestScaffoldEmbedBrainGuidance(t *testing.T) {
	role, err := ReadTemplate("roles/brain.md")
	if err != nil {
		t.Fatal(err)
	}
	skill, err := ReadTemplate("skills/specd-brain/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"zero model/provider SDK, zero LLM calls",
		"at most one externally visible action per step",
		"Preserve manual approval by default",
		"Dispatch only tasks from runnable frontier",
		"Escalate unknown state",
	} {
		if !strings.Contains(role+skill, want) {
			t.Fatalf("Brain guidance missing %q", want)
		}
	}
}

func TestScaffoldEmbedPinkyGuidance(t *testing.T) {
	role, err := ReadTemplate("roles/pinky.md")
	if err != nil {
		t.Fatal(err)
	}
	skill, err := ReadTemplate("skills/specd-pinky/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Never flip `tasks.md` checkboxes",
		"Never edit `state.json`",
		"Never forge evidence refs",
		"Run proof through `specd verify`",
		"host-reported and untrusted",
	} {
		if !strings.Contains(role+skill, want) {
			t.Fatalf("Pinky guidance missing %q", want)
		}
	}
}

func TestScaffoldManifest(t *testing.T) {
	t.Run("all_templates_exist_and_targets_are_unique", func(t *testing.T) {
		assets := DefaultScaffoldManifest()
		if err := ValidateScaffoldManifest(assets, ReadTemplate); err != nil {
			t.Fatal(err)
		}
		targets := SortedScaffoldTargets(assets)
		if len(targets) != 27 {
			t.Fatalf("target count = %d, want 27", len(targets))
		}
		for i := 1; i < len(targets); i++ {
			if targets[i] == targets[i-1] {
				t.Fatalf("duplicate target %q", targets[i])
			}
		}
	})

	t.Run("missing_template_fails_preflight", func(t *testing.T) {
		assets := []ScaffoldAsset{{
			Template: "missing.txt",
			Target:   ".specd/missing.txt",
			Policy:   ScaffoldCreate,
			Required: true,
		}}
		err := ValidateScaffoldManifest(assets, func(string) (string, error) {
			return "", errors.New("not found")
		})
		if err == nil || !strings.Contains(err.Error(), "missing template") {
			t.Fatalf("error = %v, want missing template", err)
		}
	})

	t.Run("duplicate_target_fails_preflight", func(t *testing.T) {
		assets := []ScaffoldAsset{
			{Template: "a", Target: ".specd/same", Policy: ScaffoldCreate, Required: true},
			{Template: "b", Target: ".specd/same", Policy: ScaffoldCreate, Required: true},
		}
		err := ValidateScaffoldManifest(assets, func(path string) (string, error) { return path, nil })
		if err == nil || !strings.Contains(err.Error(), "duplicate scaffold target") {
			t.Fatalf("error = %v, want duplicate target", err)
		}
	})

	t.Run("default_target_set_remains_compatible", func(t *testing.T) {
		want := []string{
			".claude/agents/pinky-builder.md",
			".claude/agents/pinky-investigator.md",
			".claude/agents/pinky-reviewer.md",
			".claude/agents/pinky-verifier.md",
			".specd/config.json",
			".specd/roles/brain.md",
			".specd/roles/builder.md",
			".specd/roles/investigator.md",
			".specd/roles/pinky.md",
			".specd/roles/reviewer.md",
			".specd/roles/verifier.md",
			".specd/runtime/.gitignore",
			".specd/skills/specd-brain/SKILL.md",
			".specd/skills/specd-design/SKILL.md",
			".specd/skills/specd-execute/SKILL.md",
			".specd/skills/specd-foundations/SKILL.md",
			".specd/skills/specd-pinky/SKILL.md",
			".specd/skills/specd-requirements/SKILL.md",
			".specd/skills/specd-steering/SKILL.md",
			".specd/skills/specd-tasks/SKILL.md",
			".specd/steering/memory.md",
			".specd/steering/product.md",
			".specd/steering/reasoning.md",
			".specd/steering/structure.md",
			".specd/steering/tech.md",
			".specd/steering/workflow.md",
			"AGENTS.md",
		}
		if got := SortedScaffoldTargets(DefaultScaffoldManifest()); !reflect.DeepEqual(got, want) {
			t.Fatalf("targets changed:\ngot  %#v\nwant %#v", got, want)
		}
	})
}
