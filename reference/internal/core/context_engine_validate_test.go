package core

import (
	"testing"

	contextpkg "github.com/0xkhdr/specd/internal/context"
)

// TestEngineOutputValidates is the cross-boundary guard: the pure context
// engine (contextpkg) emits a manifest that core's boundary-layer validator
// accepts. It lives here, not in contextpkg, because validateMissionContextManifest
// stays on core's boundary (the engine never imports core).
func TestEngineOutputValidates(t *testing.T) {
	read := func(name string) (string, bool) {
		if name == "tasks.md" {
			return "## Wave 1\n\n- [ ] T1 — Demo\n  - role: craftsman\n", true
		}
		return "", false
	}
	m := contextpkg.BuildContextManifest(contextpkg.ContextRequest{
		Slug:           "demo",
		Status:         StatusExecuting,
		TaskID:         "T1",
		Role:           "craftsman",
		Files:          []string{"x.go"},
		Mode:           contextpkg.ContextModeMission,
		ContextCommand: "specd context demo",
		ReadArtifact:   read,
	})
	if err := validateMissionContextManifest(m, true); err != nil {
		t.Fatalf("engine output failed validation: %v", err)
	}
}
