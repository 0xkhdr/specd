package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestDispatchPhase pins spec 03 R2: an execution verb invoked against a spec
// still in an early phase fails closed (exit 2 via ErrUsage) naming the verb,
// the current phase, and the allowed phases — before any side effect, so
// state.json is untouched.
func TestDispatchPhase(t *testing.T) {
	root := t.TempDir()
	statePath := core.StatePath(root, "demo")
	if err := os.MkdirAll(strings.TrimSuffix(statePath, "/state.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	// InitialState is status=requirements ⇒ phase=perceive, which verify
	// (allowed: plan/execute/verify) must reject.
	if err := core.SaveState(statePath, core.InitialState("demo")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	err = Run(root, "verify", []string{"demo", "T1"}, nil)
	if err == nil {
		t.Fatal("verify in perceive phase succeeded, want fail-closed rejection")
	}
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("error does not wrap ErrUsage (exit 2): %v", err)
	}
	for _, want := range []string{"verify", "perceive", "plan"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("rejection message %q missing %q", err.Error(), want)
		}
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("state.json mutated on a rejected out-of-phase dispatch")
	}
}

// TestDispatchPhaseAllowed confirms an in-phase spec passes the phase gate; any
// resulting error is a downstream handler error, never the ErrUsage gate.
func TestDispatchPhaseAllowed(t *testing.T) {
	root := t.TempDir()
	statePath := core.StatePath(root, "demo")
	if err := os.MkdirAll(strings.TrimSuffix(statePath, "/state.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := core.InitialState("demo")
	state.Status = core.StatusTasks
	state.Phase = core.PhaseForStatus(core.StatusTasks)
	if err := core.SaveState(statePath, state); err != nil {
		t.Fatal(err)
	}
	err := Run(root, "verify", []string{"demo", "T1"}, nil)
	if errors.Is(err, ErrUsage) {
		t.Fatalf("in-phase verify rejected by phase gate: %v", err)
	}
}

// TestFlagEnum pins spec 03 R3: an enum-declared flag given an out-of-enum
// value fails closed (exit 2) naming the flag and allowed values.
func TestFlagEnum(t *testing.T) {
	root := t.TempDir()
	err := Run(root, "memory", []string{"demo", "add"}, map[string]string{"criticality": "bogus"})
	if err == nil {
		t.Fatal("out-of-enum flag value accepted")
	}
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("enum rejection does not wrap ErrUsage: %v", err)
	}
	if !strings.Contains(err.Error(), "criticality") {
		t.Errorf("message %q does not name the flag", err.Error())
	}

	// A valid enum value passes the enum gate (handler may still fail for
	// unrelated reasons, but not via ErrUsage).
	err = Run(root, "memory", []string{"demo", "add"}, map[string]string{"criticality": "critical"})
	if errors.Is(err, ErrUsage) {
		t.Fatalf("valid enum value rejected by enum gate: %v", err)
	}
}
