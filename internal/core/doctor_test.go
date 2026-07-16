package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorCleanAndReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, ".specd/specs/demo"), 0o755)
	before, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	result := Doctor(root, "demo")
	if result.ProtocolVersion != DriverProtocolVersion || !result.Healthy {
		t.Fatalf("clean doctor result = %+v", result)
	}
	if result.Findings == nil || len(result.Findings) != 0 {
		t.Fatalf("clean doctor findings = %+v", result.Findings)
	}
	if result.NextAction == "" {
		t.Fatal("clean doctor omitted next action")
	}
	after, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(before) != string(after) {
		t.Fatal("doctor mutated project")
	}
}

func TestDoctorFindsMissingLayoutAndInvalidPin(t *testing.T) {
	root := t.TempDir()
	result := Doctor(root, "missing")
	if result.Healthy {
		t.Fatalf("defective project reported healthy: %+v", result)
	}
	if result.ProtocolVersion != DriverProtocolVersion || result.NextAction == "" {
		t.Fatalf("defective doctor result is not actionable/versioned: %+v", result)
	}
	seen := map[string]bool{}
	for _, f := range result.Findings {
		seen[f.Code] = f.RecoveryAction != ""
	}
	for _, code := range []string{"LAYOUT_MISSING", "SPEC_PIN_INVALID"} {
		if !seen[code] {
			t.Fatalf("missing %s: %+v", code, result.Findings)
		}
	}
}
