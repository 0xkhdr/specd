package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestDispatchPausesOnAmendmentWithoutRewind(t *testing.T) {
	root := newDemoSpec(t)
	for range 2 {
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("approve next: %v", err)
		}
	}
	before, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "midreq", []string{"demo"}, map[string]string{"text": "change R1", "scope": "R1"}); err != nil {
		t.Fatalf("midreq: %v", err)
	}
	if err := Run(root, "next", []string{"demo"}, nil); err == nil || !strings.Contains(err.Error(), "dispatch paused") {
		t.Fatalf("stale dispatch accepted: %v", err)
	}
	after, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != before.Status {
		t.Fatalf("amendment rewound status from %q to %q", before.Status, after.Status)
	}
	if _, ok := after.Records["amendment:0"]; !ok {
		t.Fatal("midreq did not append amendment record")
	}
}

func TestDispatchAuthorityDeniesReadOnlyWrite(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := core.TaskRow{ID: "T1", Role: "validator", DeclaredFiles: []string{"a.go"}}
	a, err := core.BuildAuthority(task, "controller", "w", "demo", "execute", "abc", "policy", "required", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	a.AllowedTools = append(a.AllowedTools, core.ToolAuthority{ID: "complete-task"})
	a.Digest = ""
	core.FinalizeAuthority(&a)
	root := t.TempDir()
	err = RunAuthorized(root, "complete-task", []string{"demo", "T1"}, nil, a, nil, now)
	if err == nil || (!strings.Contains(err.Error(), "ROLE_WRITE_DENIED") && !strings.Contains(err.Error(), "authority denied")) {
		t.Fatalf("err=%v", err)
	}
	raw, readErr := os.ReadFile(filepath.Join(root, ".specd/specs/demo/authority-denials.jsonl"))
	if readErr != nil || !strings.Contains(string(raw), `"tool_id":"complete-task"`) {
		t.Fatalf("denial record=%q err=%v", raw, readErr)
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
