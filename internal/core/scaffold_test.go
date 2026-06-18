package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestScaffoldManifest(t *testing.T) {
	t.Run("all templates exist and targets are unique", func(t *testing.T) {
		assets := DefaultScaffoldManifest()
		if err := ValidateScaffoldManifest(assets, ReadTemplate); err != nil {
			t.Fatal(err)
		}
		targets := SortedScaffoldTargets(assets)
		if len(targets) != 18 {
			t.Fatalf("target count = %d, want 18", len(targets))
		}
		for i := 1; i < len(targets); i++ {
			if targets[i] == targets[i-1] {
				t.Fatalf("duplicate target %q", targets[i])
			}
		}
	})

	t.Run("missing template fails preflight", func(t *testing.T) {
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

	t.Run("duplicate target fails preflight", func(t *testing.T) {
		assets := []ScaffoldAsset{
			{Template: "a", Target: ".specd/same", Policy: ScaffoldCreate, Required: true},
			{Template: "b", Target: ".specd/same", Policy: ScaffoldCreate, Required: true},
		}
		err := ValidateScaffoldManifest(assets, func(path string) (string, error) { return path, nil })
		if err == nil || !strings.Contains(err.Error(), "duplicate scaffold target") {
			t.Fatalf("error = %v, want duplicate target", err)
		}
	})

	t.Run("default target set remains compatible", func(t *testing.T) {
		want := []string{
			".specd/config.json",
			".specd/roles/builder.md",
			".specd/roles/investigator.md",
			".specd/roles/reviewer.md",
			".specd/roles/verifier.md",
			".specd/skills/specd-design/SKILL.md",
			".specd/skills/specd-execute/SKILL.md",
			".specd/skills/specd-foundations/SKILL.md",
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
