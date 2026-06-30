package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

// enableOrchestration writes a config.json granting the project orchestration
// capability, so `--set orchestrated` / `new --orchestrated` can succeed.
func enableOrchestration(t *testing.T, h *testharness.Harness) {
	t.Helper()
	if err := os.WriteFile(core.ConfigPath(h.Root), []byte(`{"version":1,"orchestration":{"enabled":true}}`), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}
}

func TestNewDefaultsToBase(t *testing.T) {
	h := testharness.New(t)
	h.RunExpect(core.ExitOK, "new", "auth", "--title", "Auth")

	st := h.State("auth").Raw()
	if st.ExecutionMode != "" {
		t.Errorf("Base spec should leave ExecutionMode empty (byte-stable), got %q", st.ExecutionMode)
	}
	if st.EffectiveMode() != core.ModeBase {
		t.Errorf("EffectiveMode = %q, want base", st.EffectiveMode())
	}

	res := h.RunExpect(core.ExitOK, "status", "auth")
	if !strings.Contains(res.Out(), "mode: base") {
		t.Errorf("status should report mode base, got: %s", res.Out())
	}
}

func TestNewOrchestratedRequiresCapability(t *testing.T) {
	h := testharness.New(t)
	// No orchestration capability → fail closed with remediation.
	res := h.RunExpect(core.ExitGate, "new", "pay", "--orchestrated")
	if !strings.Contains(res.Out(), "orchestration") || !strings.Contains(res.Out(), "specd init --orchestration") {
		t.Errorf("expected remediation pointing at `specd init --orchestration`, got: %s", res.Out())
	}

	// With capability → records orchestrated + user origin.
	enableOrchestration(t, h)
	h.RunExpect(core.ExitOK, "new", "pay", "--orchestrated")
	st := h.State("pay").Raw()
	if st.ExecutionMode != core.ModeOrchestrated || st.ModeOrigin != core.OriginUser {
		t.Errorf("got mode=%q origin=%q, want orchestrated/user", st.ExecutionMode, st.ModeOrigin)
	}
}

// The `mode --set` set/clear and fail-closed behaviors are now exercised through
// the survivor `status --set-mode` entry point by TestStatusSetModeParity and
// TestStatusSetModeFailsClosedWithoutCapability below; the legacy `mode` alias
// itself is guarded by registry_sunset_test.go (warn + functional during grace).

// TestStatusSetModeParity is the Phase 2 survivor-parity guard: the recovered
// `status <slug> --set-mode` / `--recommend` paths must behave identically to
// the merged `mode` command, so retiring the `mode` alias drops no capability
// (optimization-plan GAP-2). It mirrors the mode set/recommend assertions
// through the survivor entry point.
func TestStatusSetModeParity(t *testing.T) {
	h := testharness.New(t)
	enableOrchestration(t, h)
	h.RunExpect(core.ExitOK, "new", "auth")
	before := h.State("auth").Raw().Revision

	// set-mode orchestrated records orchestrated/user and bumps the revision.
	h.RunExpect(core.ExitOK, "status", "auth", "--set-mode", "orchestrated")
	st := h.State("auth").Raw()
	if st.ExecutionMode != core.ModeOrchestrated || st.ModeOrigin != core.OriginUser {
		t.Errorf("got mode=%q origin=%q, want orchestrated/user", st.ExecutionMode, st.ModeOrigin)
	}
	if st.Revision <= before {
		t.Errorf("revision %d did not advance past %d (audit trail)", st.Revision, before)
	}

	// Opting back out clears the fields so Base state stays byte-stable.
	h.RunExpect(core.ExitOK, "status", "auth", "--set-mode", "base")
	back := h.State("auth").Raw()
	if back.ExecutionMode != "" || back.ModeOrigin != "" {
		t.Errorf("switching to base should clear fields, got mode=%q origin=%q", back.ExecutionMode, back.ModeOrigin)
	}

	// --recommend emits the advisory verdict (read-only, never mutates).
	res := h.RunExpect(core.ExitOK, "status", "auth", "--recommend", "--json")
	for _, want := range []string{`"recommended"`, `"userDecides": true`} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("recommend JSON missing %q; got: %s", want, res.Stdout)
		}
	}
}

// TestStatusSetModeFailsClosedWithoutCapability mirrors the mode fail-closed
// path through the survivor flag.
func TestStatusSetModeFailsClosedWithoutCapability(t *testing.T) {
	h := testharness.New(t)
	h.RunExpect(core.ExitOK, "new", "auth")
	res := h.RunExpect(core.ExitGate, "status", "auth", "--set-mode", "orchestrated")
	if !strings.Contains(res.Out(), "specd init --orchestration") {
		t.Errorf("expected enabling-command remediation, got: %s", res.Out())
	}
	if h.State("auth").Raw().EffectiveMode() != core.ModeBase {
		t.Error("spec must stay base after a refused --set-mode")
	}
}

func TestBrainRefusesBaseSpec(t *testing.T) {
	h := testharness.New(t)
	h.Spec("auth").
		Req("auth", "As a user, I want auth.", "THE SYSTEM SHALL authenticate.").
		FullDesign().
		AddTask(testharness.TaskSpec{ID: "T1", Title: "do auth", Files: "internal/core/auth.go", Verify: "true"}).
		Status(core.StatusExecuting).
		Build()

	res := h.RunExpect(core.ExitGate, "brain", "start", "auth", "--session", strings.Repeat("a", 32))
	if !strings.Contains(res.Out(), "base execution mode") || !strings.Contains(res.Out(), "--set orchestrated") {
		t.Errorf("expected base-mode refusal with remediation, got: %s", res.Out())
	}
}

func TestModeRecommendNeutralBeforeTasks(t *testing.T) {
	h := testharness.New(t)
	h.RunExpect(core.ExitOK, "new", "auth")
	res := h.RunExpect(core.ExitOK, "status", "auth", "--recommend", "--json")
	for _, want := range []string{`"recommended": "base"`, `"confidence": "neutral"`, `"userDecides": true`} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("recommend JSON missing %q; got: %s", want, res.Stdout)
		}
	}
}
