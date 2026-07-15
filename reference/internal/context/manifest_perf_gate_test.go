package contextpkg

import (
	"testing"

	"github.com/0xkhdr/specd/internal/spec"
)

// disabledModeRequest models the manifest's "disabled" (zero-overhead) path: a
// request with no artifact reader. Per the engine contract (manifest.go), a nil
// ReadArtifact performs no IO and reproduces the pre-measurement manifest. The
// shape is otherwise representative (an executing task with a scoped file) so the
// guard exercises every code path that could regress into doing work.
func disabledModeRequest() ContextRequest {
	return ContextRequest{
		Slug:         "demo",
		Status:       spec.StatusExecuting,
		TaskID:       "T1",
		Role:         "craftsman",
		Files:        []string{"internal/core/demo.go"},
		Mode:         ContextModeMission,
		Requirements: []int{1, 2},
		// ReadArtifact deliberately nil: measurement disabled.
	}
}

// Req1: disabled-mode performs no manifest file reads. A counting reader proves
// the engine *does* consult a reader when one is supplied (so the guard is wired
// to the real read path); the disabled path then leaves every source artifact
// unmeasured — default hint, whole-file mode — which can only happen if zero
// reads occurred.
func TestManifestDisabledModeDoesNoFileReads(t *testing.T) {
	// Control: an enabled request reads its source artifacts.
	var reads int
	enabled := disabledModeRequest()
	enabled.ReadArtifact = func(string) (string, bool) {
		reads++
		return "", false
	}
	BuildContextManifest(enabled)
	if reads == 0 {
		t.Fatal("enabled path consulted no reader; the read-path spy is mis-wired")
	}

	// Disabled: no reader => no measurement. Every source artifact must carry the
	// default token hint in reference-if-needed mode (the unmeasured shape).
	m := BuildContextManifest(disabledModeRequest())
	var sources int
	for _, it := range m.Items {
		if it.Kind != "source-artifact" {
			continue
		}
		sources++
		if it.TokenHint != ctxHintArtifact || it.Mode != "reference-if-needed" {
			t.Errorf("source artifact %q measured on disabled path: hint=%d mode=%q (want hint %d, reference-if-needed)",
				it.Path, it.TokenHint, it.Mode, ctxHintArtifact)
		}
	}
	if sources == 0 {
		t.Fatal("expected at least one source-artifact item to guard")
	}
}

// disabledModeAllocBudget bounds the per-build allocation count on the disabled
// path. The measured floor is 8 allocs (the manifest struct + item slice, all
// non-measurement); the budget keeps a tiny margin. It is a ratchet: lower it as
// the path gets leaner, never raise it to accommodate new disabled-mode work.
// Any read/measurement regression on the disabled path blows well past this.
const disabledModeAllocBudget = 10

// Req2: a deterministic allocation budget fails CI if the disabled path starts
// allocating. testing.AllocsPerRun is exact and CI-safe (no wall-clock).
func TestManifestDisabledModeAllocBudget(t *testing.T) {
	req := disabledModeRequest()
	avg := testing.AllocsPerRun(200, func() {
		BuildContextManifest(req)
	})
	if avg > disabledModeAllocBudget {
		t.Fatalf("disabled-mode manifest allocs/run = %.0f, budget = %d (ratchet: lower the budget, do not raise it)",
			avg, disabledModeAllocBudget)
	}
}
