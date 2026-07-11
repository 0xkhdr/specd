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
	findings := Doctor(root, "demo")
	if len(findings) != 0 {
		t.Fatalf("clean doctor findings = %+v", findings)
	}
	after, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(before) != string(after) {
		t.Fatal("doctor mutated project")
	}
}

func TestDoctorFindsMissingLayoutAndInvalidPin(t *testing.T) {
	root := t.TempDir()
	findings := Doctor(root, "missing")
	seen := map[string]bool{}
	for _, f := range findings {
		seen[f.Code] = f.RecoveryAction != ""
	}
	for _, code := range []string{"LAYOUT_MISSING", "SPEC_PIN_INVALID"} {
		if !seen[code] {
			t.Fatalf("missing %s: %+v", code, findings)
		}
	}
}
