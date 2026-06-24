package core

import (
	"testing"

	contextpkg "github.com/0xkhdr/specd/internal/context"
)

// pinky_context_validate_cov_test.go drives every rejection branch of
// validateMissionContextManifest. It is a pure validator, so a single
// table of single-field mutations off a known-good baseline reaches each
// `return error` without any fixture IO.

func validMissionManifest() contextpkg.MissionContextManifest {
	return contextpkg.MissionContextManifest{
		Version:          contextpkg.ManifestVersion,
		SoftTokenCeiling: 8000,
		Strategy:         "balanced",
		Items: []contextpkg.MissionContextItem{{
			Order:     1,
			Kind:      "role",
			Mode:      "read-full",
			Path:      "docs/role.md",
			Rationale: "role guidance",
		}},
		EstimatedTokens: 100,
		Budget:          4000,
	}
}

func TestValidateMissionContextManifest(t *testing.T) {
	// Empty manifest: required → error, optional → nil.
	if err := validateMissionContextManifest(contextpkg.MissionContextManifest{}, true); err == nil {
		t.Error("empty required manifest should error")
	}
	if err := validateMissionContextManifest(contextpkg.MissionContextManifest{}, false); err != nil {
		t.Errorf("empty optional manifest should pass: %v", err)
	}

	// Baseline is valid.
	if err := validateMissionContextManifest(validMissionManifest(), true); err != nil {
		t.Fatalf("baseline manifest should be valid: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*contextpkg.MissionContextManifest)
	}{
		{"bad version", func(m *contextpkg.MissionContextManifest) { m.Version = 99 }},
		{"ceiling too low", func(m *contextpkg.MissionContextManifest) { m.SoftTokenCeiling = 1 }},
		{"ceiling too high", func(m *contextpkg.MissionContextManifest) { m.SoftTokenCeiling = contextpkg.MaxSoftCeiling + 1 }},
		{"empty strategy", func(m *contextpkg.MissionContextManifest) { m.Strategy = "" }},
		{"no items", func(m *contextpkg.MissionContextManifest) { m.Items = nil }},
		{"order not contiguous", func(m *contextpkg.MissionContextManifest) { m.Items[0].Order = 7 }},
		{"bad kind", func(m *contextpkg.MissionContextManifest) { m.Items[0].Kind = "bogus" }},
		{"bad mode", func(m *contextpkg.MissionContextManifest) { m.Items[0].Mode = "bogus" }},
		{"no path or command", func(m *contextpkg.MissionContextManifest) { m.Items[0].Path = "" }},
		{"bad path", func(m *contextpkg.MissionContextManifest) { m.Items[0].Path = "doc\x00.md" }},
		{"bad command", func(m *contextpkg.MissionContextManifest) { m.Items[0].Path = ""; m.Items[0].Command = "cmd\x00nul" }},
		{"tokenHint negative", func(m *contextpkg.MissionContextManifest) { m.Items[0].TokenHint = -1 }},
		{"tokenHint too high", func(m *contextpkg.MissionContextManifest) { m.Items[0].TokenHint = contextpkg.MaxSoftCeiling + 1 }},
		{"empty rationale", func(m *contextpkg.MissionContextManifest) { m.Items[0].Rationale = "" }},
		{"estimatedTokens negative", func(m *contextpkg.MissionContextManifest) { m.EstimatedTokens = -1 }},
		{"estimatedTokens too high", func(m *contextpkg.MissionContextManifest) { m.EstimatedTokens = contextpkg.MaxSoftCeiling + 1 }},
		{"budget negative", func(m *contextpkg.MissionContextManifest) { m.Budget = -1 }},
		{"budget too high", func(m *contextpkg.MissionContextManifest) { m.Budget = contextpkg.MaxSoftCeiling + 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validMissionManifest()
			tc.mut(&m)
			if err := validateMissionContextManifest(m, true); err == nil {
				t.Errorf("%s: expected validation error", tc.name)
			}
		})
	}
}
