package core

import "testing"

// gates_mode_cov_test.go covers GateModeCapability across its severity and
// capability branches.

func TestGateModeCapabilityBranches(t *testing.T) {
	orchestrated := &State{ExecutionMode: ModeOrchestrated}
	base := &State{ExecutionMode: ModeBase}

	mkCfg := func(severity string, enabled bool) Config {
		cfg := DefaultConfig
		cfg.Gates.ModeCapability = severity
		cfg.Orchestration.Enabled = enabled
		return cfg
	}

	// Disabled severities → no-op regardless of state.
	for _, sev := range []string{"", "off", "*"} {
		v, w := GateModeCapability(CheckCtx{Cfg: mkCfg(sev, false), State: orchestrated})
		if v != nil || w != nil {
			t.Errorf("severity %q should be a no-op", sev)
		}
	}

	// Enabled severity but non-orchestrated spec → no-op.
	if v, w := GateModeCapability(CheckCtx{Cfg: mkCfg("error", false), State: base}); v != nil || w != nil {
		t.Error("base spec should not flag")
	}
	// Nil state → no-op.
	if v, w := GateModeCapability(CheckCtx{Cfg: mkCfg("error", false), State: nil}); v != nil || w != nil {
		t.Error("nil state should not flag")
	}
	// Orchestrated spec in a capable project → no-op.
	if v, w := GateModeCapability(CheckCtx{Cfg: mkCfg("error", true), State: orchestrated}); v != nil || w != nil {
		t.Error("capable project should not flag")
	}

	// Orchestrated spec, incapable project, severity error → violation.
	if v, w := GateModeCapability(CheckCtx{Cfg: mkCfg("error", false), State: orchestrated}); len(v) != 1 || w != nil {
		t.Errorf("error severity → v=%v w=%v, want one violation", v, w)
	}
	// Severity warn → warning.
	if v, w := GateModeCapability(CheckCtx{Cfg: mkCfg("warn", false), State: orchestrated}); v != nil || len(w) != 1 {
		t.Errorf("warn severity → v=%v w=%v, want one warning", v, w)
	}
}
