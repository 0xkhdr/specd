package core

import (
	"runtime"
	"testing"
)

// minimalCtx returns a CheckCtx whose core pipeline produces a stable, non-empty
// set of findings (missing requirements.md → 1 ears violation) so we can assert
// custom gates add to, but never alter, the core gate output.
func minimalCtx(custom []CustomGateCfg) CheckCtx {
	return CheckCtx{
		Root:  ".",
		Slug:  "demo",
		ReqMd: nil, // → GateEars: "requirements.md missing"
		State: &State{Spec: "demo", Status: "executing", Tasks: map[string]TaskState{}},
		Cfg:   Config{Gates: GatesCfg{Custom: custom}},
	}
}

func TestCustomGatesDoNotAlterCoreGates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("custom gate uses POSIX sh")
	}
	baseV, baseW := RunGates(minimalCtx(nil))

	// A passing custom gate (no findings) must leave the core result identical.
	passing := []CustomGateCfg{{Name: "noop", Command: `echo '{"violations":[],"warnings":[]}'`}}
	v, w := RunGates(minimalCtx(passing))
	if len(v) != len(baseV) || len(w) != len(baseW) {
		t.Fatalf("passing custom gate changed core findings: core v=%d w=%d, with-gate v=%d w=%d", len(baseV), len(baseW), len(v), len(w))
	}

	// A failing custom gate adds exactly its finding on top of the core ones.
	failing := []CustomGateCfg{{Name: "lint", Command: `echo '{"violations":[{"location":"x","message":"boom"}]}'`}}
	v, _ = RunGates(minimalCtx(failing))
	if len(v) != len(baseV)+1 {
		t.Fatalf("failing custom gate: want %d violations, got %d (%v)", len(baseV)+1, len(v), v)
	}
	if v[len(v)-1].Gate != "custom:lint" {
		t.Fatalf("custom finding not tagged: %v", v[len(v)-1])
	}
}

func TestCustomGateErrorIsViolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("custom gate uses POSIX sh")
	}
	baseV, _ := RunGates(minimalCtx(nil))
	// Non-zero exit → the gate itself is reported as a violation (fail loud).
	broken := []CustomGateCfg{{Name: "broken", Command: `exit 3`}}
	v, _ := RunGates(minimalCtx(broken))
	if len(v) != len(baseV)+1 {
		t.Fatalf("broken gate: want %d violations, got %d", len(baseV)+1, len(v))
	}
}
