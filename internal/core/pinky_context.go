package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	missionContextManifestVersion = 1
	missionContextSoftCeiling     = 12000
	minMissionContextSoftCeiling  = 1000
	maxMissionContextSoftCeiling  = 200000
)

// MissionContextManifest is the deterministic context-engineering contract for
// a Pinky mission. It names exactly what a host should load, in order, and how
// aggressively to expand each item under the soft token ceiling. The manifest is
// advisory context, not completion evidence.
type MissionContextManifest struct {
	Version          int                  `json:"version"`
	SoftTokenCeiling int                  `json:"softTokenCeiling"`
	Strategy         string               `json:"strategy"`
	Items            []MissionContextItem `json:"items"`
}

type MissionContextItem struct {
	Order     int    `json:"order"`
	Kind      string `json:"kind"`
	Path      string `json:"path,omitempty"`
	Command   string `json:"command,omitempty"`
	Mode      string `json:"mode"`
	Required  bool   `json:"required"`
	TokenHint int    `json:"tokenHint,omitempty"`
	Rationale string `json:"rationale"`
}

// BuildMissionContextManifest gives every host the same minimal sufficient
// context for the same mission: role contract, Pinky operating skill, one
// phase-scoped skill, the specd context briefing, scoped files, then optional
// source-of-truth artifacts as references. It performs no IO.
func BuildMissionContextManifest(mission PinkyMission) MissionContextManifest {
	items := make([]MissionContextItem, 0, 8+len(mission.Files))
	add := func(kind, path, command, mode string, required bool, hint int, rationale string) {
		items = append(items, MissionContextItem{
			Order:     len(items) + 1,
			Kind:      kind,
			Path:      filepath.ToSlash(path),
			Command:   command,
			Mode:      mode,
			Required:  required,
			TokenHint: hint,
			Rationale: rationale,
		})
	}

	add("role", filepath.Join(".specd", "roles", mission.Role+".md"), "", "read-full", true, 800, "role authority and constraints")
	add("skill", filepath.Join(".specd", "skills", "specd-pinky", "SKILL.md"), "", "read-full", true, 1200, "Pinky lease/report lifecycle")
	if skill := missionPhaseSkillPath(mission); skill != "" {
		add("phase-skill", skill, "", "read-full", true, 1600, "phase-scoped workflow; do not load unrelated stage guidance")
	}
	add("spec-context", "", mission.ContextCommand, "run-command", true, 1800, "phase briefing, load list, blockers, approvals")
	for _, file := range mission.Files {
		add("scope-file", file, "", "read-targeted", true, 1200, "mission-declared file in scope")
	}
	for _, artifact := range []string{"requirements.md", "design.md", "tasks.md"} {
		path := filepath.Join(".specd", "specs", mission.Spec, artifact)
		add("source-artifact", path, "", "reference-if-needed", false, 1200, "source of truth; expand only if required by contract or context command")
	}

	return MissionContextManifest{
		Version:          missionContextManifestVersion,
		SoftTokenCeiling: missionContextSoftCeiling,
		Strategy:         "Load required items in order. Keep optional/reference items collapsed unless needed. Stop expanding optional items before the soft ceiling; never skip required role, skill, context, or scoped files.",
		Items:            items,
	}
}

func missionPhaseSkillPath(mission PinkyMission) string {
	if strings.HasPrefix(mission.TaskID, "A") {
		for _, file := range mission.Files {
			switch filepath.Base(file) {
			case "requirements.md":
				return filepath.ToSlash(filepath.Join(".specd", "skills", "specd-requirements", "SKILL.md"))
			case "design.md":
				return filepath.ToSlash(filepath.Join(".specd", "skills", "specd-design", "SKILL.md"))
			case "tasks.md":
				return filepath.ToSlash(filepath.Join(".specd", "skills", "specd-tasks", "SKILL.md"))
			}
		}
		return ""
	}
	return filepath.ToSlash(filepath.Join(".specd", "skills", "specd-execute", "SKILL.md"))
}

func validateMissionContextManifest(manifest MissionContextManifest, required bool) error {
	if manifest.Version == 0 && len(manifest.Items) == 0 {
		if required {
			return fmt.Errorf("pinky: contextManifest is required")
		}
		return nil
	}
	if manifest.Version != missionContextManifestVersion {
		return fmt.Errorf("pinky: unsupported contextManifest version %d", manifest.Version)
	}
	if manifest.SoftTokenCeiling < minMissionContextSoftCeiling || manifest.SoftTokenCeiling > maxMissionContextSoftCeiling {
		return fmt.Errorf("pinky: contextManifest softTokenCeiling outside bounds")
	}
	if err := validateACPText("contextManifest.strategy", manifest.Strategy, true); err != nil {
		return err
	}
	if len(manifest.Items) == 0 || len(manifest.Items) > ACPMaxListItems {
		return fmt.Errorf("pinky: contextManifest.items must contain 1..%d items", ACPMaxListItems)
	}
	for i, item := range manifest.Items {
		if item.Order != i+1 {
			return fmt.Errorf("pinky: contextManifest item order must be contiguous")
		}
		if !missionContextKindSet[item.Kind] {
			return fmt.Errorf("pinky: invalid contextManifest kind %q", item.Kind)
		}
		if !missionContextModeSet[item.Mode] {
			return fmt.Errorf("pinky: invalid contextManifest mode %q", item.Mode)
		}
		if item.Path == "" && item.Command == "" {
			return fmt.Errorf("pinky: contextManifest item needs path or command")
		}
		if item.Path != "" {
			if err := validateACPPaths("contextManifest.path", []string{item.Path}); err != nil {
				return err
			}
		}
		if err := validateACPText("contextManifest.command", item.Command, false); err != nil {
			return err
		}
		if item.TokenHint < 0 || item.TokenHint > manifest.SoftTokenCeiling {
			return fmt.Errorf("pinky: contextManifest tokenHint outside bounds")
		}
		if err := validateACPText("contextManifest.rationale", item.Rationale, true); err != nil {
			return err
		}
	}
	return nil
}

var missionContextKindSet = sliceToSet([]string{"role", "skill", "phase-skill", "spec-context", "scope-file", "source-artifact"})
var missionContextModeSet = sliceToSet([]string{"read-full", "run-command", "read-targeted", "reference-if-needed"})
