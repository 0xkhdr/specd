package core

import "testing"

// pinky_context_validate_cov_test.go drives every rejection branch of
// validateMissionContextManifest. It is a pure validator, so a single
// table of single-field mutations off a known-good baseline reaches each
// `return error` without any fixture IO.

func validMissionManifest() MissionContextManifest {
	return MissionContextManifest{
		Version:          missionContextManifestVersion,
		SoftTokenCeiling: 8000,
		Strategy:         "balanced",
		Items: []MissionContextItem{{
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
	if err := validateMissionContextManifest(MissionContextManifest{}, true); err == nil {
		t.Error("empty required manifest should error")
	}
	if err := validateMissionContextManifest(MissionContextManifest{}, false); err != nil {
		t.Errorf("empty optional manifest should pass: %v", err)
	}

	// Baseline is valid.
	if err := validateMissionContextManifest(validMissionManifest(), true); err != nil {
		t.Fatalf("baseline manifest should be valid: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*MissionContextManifest)
	}{
		{"bad version", func(m *MissionContextManifest) { m.Version = 99 }},
		{"ceiling too low", func(m *MissionContextManifest) { m.SoftTokenCeiling = 1 }},
		{"ceiling too high", func(m *MissionContextManifest) { m.SoftTokenCeiling = maxMissionContextSoftCeiling + 1 }},
		{"empty strategy", func(m *MissionContextManifest) { m.Strategy = "" }},
		{"no items", func(m *MissionContextManifest) { m.Items = nil }},
		{"order not contiguous", func(m *MissionContextManifest) { m.Items[0].Order = 7 }},
		{"bad kind", func(m *MissionContextManifest) { m.Items[0].Kind = "bogus" }},
		{"bad mode", func(m *MissionContextManifest) { m.Items[0].Mode = "bogus" }},
		{"no path or command", func(m *MissionContextManifest) { m.Items[0].Path = "" }},
		{"bad path", func(m *MissionContextManifest) { m.Items[0].Path = "doc\x00.md" }},
		{"bad command", func(m *MissionContextManifest) { m.Items[0].Path = ""; m.Items[0].Command = "cmd\x00nul" }},
		{"tokenHint negative", func(m *MissionContextManifest) { m.Items[0].TokenHint = -1 }},
		{"tokenHint too high", func(m *MissionContextManifest) { m.Items[0].TokenHint = maxMissionContextSoftCeiling + 1 }},
		{"empty rationale", func(m *MissionContextManifest) { m.Items[0].Rationale = "" }},
		{"estimatedTokens negative", func(m *MissionContextManifest) { m.EstimatedTokens = -1 }},
		{"estimatedTokens too high", func(m *MissionContextManifest) { m.EstimatedTokens = maxMissionContextSoftCeiling + 1 }},
		{"budget negative", func(m *MissionContextManifest) { m.Budget = -1 }},
		{"budget too high", func(m *MissionContextManifest) { m.Budget = maxMissionContextSoftCeiling + 1 }},
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
