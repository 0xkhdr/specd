package core

import (
	"fmt"
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
	// EstimatedTokens and Budget are additive accounting fields (omitempty for
	// wire back-compat at version 1). EstimatedTokens is the sum of required-item
	// hints; Budget is the effective ceiling derived from phase, role, file count
	// and any host-negotiated cap. Omitting them reproduces the pre-feature bytes.
	EstimatedTokens int `json:"estimatedTokens,omitempty"`
	Budget          int `json:"budget,omitempty"`
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

// BuildMissionContextManifest is the mission-mode adapter over the shared
// context engine (see BuildContextManifest). It gives every host the same
// minimal sufficient context for a mission: role contract, Pinky operating
// skill, one phase-scoped skill, the specd context briefing, scoped files, then
// the source-of-truth artifacts (measured, and sliced to the task's row/covered
// requirements where a selector matches). The injected read closure is the only
// IO; pass nil to fall back to default hints and whole-file modes.
func BuildMissionContextManifest(mission PinkyMission, read func(name string) (string, bool)) MissionContextManifest {
	return BuildContextManifest(ContextRequest{
		Slug:           mission.Spec,
		TaskID:         mission.TaskID,
		Role:           mission.Role,
		Files:          mission.Files,
		Mode:           ContextModeMission,
		ContextCommand: mission.ContextCommand,
		Requirements:   mission.Requirements,
		ReadArtifact:   read,
	})
}

// specArtifactReader returns a reader closure that feeds the context engine raw
// artifact markdown for a spec, performing the IO outside the pure engine.
// SpecArtifactReader returns the injected artifact reader the context engine
// uses for measurement and slicing: it yields the raw markdown for a spec
// artifact and ok=false when absent. Exported so command surfaces feed the same
// reader the mission adapter and gates use.
func SpecArtifactReader(root, slug string) func(name string) (string, bool) {
	return specArtifactReader(root, slug)
}

func specArtifactReader(root, slug string) func(name string) (string, bool) {
	return func(name string) (string, bool) {
		if raw := ReadArtifact(root, slug, name); raw != nil {
			return *raw, true
		}
		return "", false
	}
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
		// Measured hints may legitimately exceed the soft ceiling (that is what
		// the budget gate warns on); bound to the hard manifest maximum.
		if item.TokenHint < 0 || item.TokenHint > maxMissionContextSoftCeiling {
			return fmt.Errorf("pinky: contextManifest tokenHint outside bounds")
		}
		if err := validateACPText("contextManifest.rationale", item.Rationale, true); err != nil {
			return err
		}
	}
	// Additive accounting fields are accepted but never required; absent (zero)
	// reproduces the pre-feature wire bytes at version 1.
	if manifest.EstimatedTokens < 0 || manifest.EstimatedTokens > maxMissionContextSoftCeiling {
		return fmt.Errorf("pinky: contextManifest estimatedTokens outside bounds")
	}
	if manifest.Budget < 0 || manifest.Budget > maxMissionContextSoftCeiling {
		return fmt.Errorf("pinky: contextManifest budget outside bounds")
	}
	return nil
}

var missionContextKindSet = sliceToSet([]string{"role", "skill", "phase-skill", "spec-context", "scope-file", "source-artifact"})
var missionContextModeSet = sliceToSet([]string{"read-full", "run-command", "read-targeted", "reference-if-needed"})
