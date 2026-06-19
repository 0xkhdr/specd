package core

import (
	"fmt"
	"path/filepath"
	"sort"
)

type ScaffoldPolicy string

const (
	ScaffoldCreate      ScaffoldPolicy = "create"
	ScaffoldMarkerMerge ScaffoldPolicy = "marker-merge"
)

// ScaffoldAsset is one embedded asset managed by the default init flow.
// Target is project-root relative and always uses slash separators.
type ScaffoldAsset struct {
	Template string         `json:"template"`
	Target   string         `json:"target"`
	Policy   ScaffoldPolicy `json:"policy"`
	Required bool           `json:"required"`
	Refresh  bool           `json:"refresh"`
}

// DefaultScaffoldManifest is the single source of truth for files installed by
// the default init flow. Its order is the deterministic execution order.
func DefaultScaffoldManifest() []ScaffoldAsset {
	assets := make([]ScaffoldAsset, 0, 22)
	for _, name := range []string{
		"reasoning.md", "workflow.md", "product.md",
		"tech.md", "structure.md", "memory.md",
	} {
		assets = append(assets, ScaffoldAsset{
			Template: "steering/" + name,
			Target:   ".specd/steering/" + name,
			Policy:   ScaffoldCreate,
			Required: true,
			Refresh:  name == "reasoning.md" || name == "workflow.md",
		})
	}
	for _, name := range []string{"investigator.md", "builder.md", "reviewer.md", "verifier.md", "brain.md", "pinky.md"} {
		assets = append(assets, ScaffoldAsset{
			Template: "roles/" + name,
			Target:   ".specd/roles/" + name,
			Policy:   ScaffoldCreate,
			Required: true,
			Refresh:  true,
		})
	}
	for _, name := range []string{
		"specd-foundations", "specd-steering", "specd-requirements",
		"specd-design", "specd-tasks", "specd-execute",
		"specd-brain", "specd-pinky",
	} {
		assets = append(assets, ScaffoldAsset{
			Template: "skills/" + name + "/SKILL.md",
			Target:   ".specd/skills/" + name + "/SKILL.md",
			Policy:   ScaffoldCreate,
			Required: true,
			Refresh:  true,
		})
	}
	assets = append(assets,
		ScaffoldAsset{
			Template: "config.json",
			Target:   ".specd/config.json",
			Policy:   ScaffoldCreate,
			Required: true,
		},
		ScaffoldAsset{
			Template: "AGENTS.md",
			Target:   "AGENTS.md",
			Policy:   ScaffoldMarkerMerge,
			Required: true,
		},
	)
	return assets
}

// ValidateScaffoldManifest verifies all templates before init writes anything.
func ValidateScaffoldManifest(assets []ScaffoldAsset, readTemplate func(string) (string, error)) error {
	seen := make(map[string]struct{}, len(assets))
	for i, asset := range assets {
		if asset.Template == "" || asset.Target == "" {
			return fmt.Errorf("scaffold asset %d has empty template or target", i)
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(asset.Target)))
		if clean == "." || clean == ".." || filepath.IsAbs(filepath.FromSlash(asset.Target)) ||
			len(clean) >= 3 && clean[:3] == "../" {
			return fmt.Errorf("scaffold target %q escapes project root", asset.Target)
		}
		if _, ok := seen[clean]; ok {
			return fmt.Errorf("duplicate scaffold target %q", asset.Target)
		}
		seen[clean] = struct{}{}
		switch asset.Policy {
		case ScaffoldCreate, ScaffoldMarkerMerge:
		default:
			return fmt.Errorf("scaffold target %q has unknown policy %q", asset.Target, asset.Policy)
		}
		if _, err := readTemplate(asset.Template); err != nil {
			return fmt.Errorf("missing template %s: %w", asset.Template, err)
		}
	}
	return nil
}

// SortedScaffoldTargets returns a stable target set for parity checks.
func SortedScaffoldTargets(assets []ScaffoldAsset) []string {
	targets := make([]string, 0, len(assets))
	for _, asset := range assets {
		targets = append(targets, asset.Target)
	}
	sort.Strings(targets)
	return targets
}
