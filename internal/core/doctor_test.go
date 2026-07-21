package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsManagedRepairFailure(t *testing.T) {
	original := planManagedRepair
	planManagedRepair = func(string) ([]AssetChange, error) { return nil, errors.New("injected repair failure") }
	t.Cleanup(func() { planManagedRepair = original })

	result := Doctor(t.TempDir(), "")
	if result.Healthy || result.Findings == nil {
		t.Fatalf("repair failure reported healthy: %+v", result)
	}
	for _, finding := range result.Findings {
		if finding.Code == "MANAGED_REPAIR_UNAVAILABLE" && finding.Ref == ".specd" && finding.RecoveryAction != "" {
			return
		}
	}
	t.Fatalf("typed repair failure missing: %+v", result.Findings)
}

func TestDoctorCleanAndReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".specd/specs/demo"), 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestDoctorReportsConfigSourceConflict(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".specd/specs/demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd/config.yaml"), []byte("agent: codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte("agent: other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Doctor(root, "demo")
	for _, finding := range result.Findings {
		if finding.Code == "CONFIG_INVALID" && strings.Contains(finding.Message, "agent") {
			return
		}
	}
	t.Fatalf("config conflict missing: %+v", result.Findings)
}
