package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/version"
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

func TestMCPCommandResolutionPrefersLocalThenInstalled(t *testing.T) {
	root := t.TempDir()
	installedDir := t.TempDir()
	local := filepath.Join(root, "specd")
	installed := filepath.Join(installedDir, "specd")
	writeMCPVersionBinary(t, local, version.Info{Version: "local"})
	writeMCPVersionBinary(t, installed, version.Info{Version: "installed"})
	t.Setenv("PATH", installedDir)

	got, err := ResolveMCPCommand(root)
	if err != nil || got != local {
		t.Fatalf("local resolution = %q, %v; want %q", got, err, local)
	}
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveMCPCommand(root)
	if err != nil || got != installed {
		t.Fatalf("installed fallback = %q, %v; want %q", got, err, installed)
	}
	snippet, err := MCPConfigSnippet("claude-code", root, "")
	if err != nil || !strings.Contains(snippet, installed) {
		t.Fatalf("generated config did not use installed fallback: %v\n%s", err, snippet)
	}
	if config := MergePinkyCodexConfig("", root); !strings.Contains(config, strconv.Quote(installed)) {
		t.Fatalf("generated Pinky config did not use installed fallback: %s", config)
	}
}

func TestMCPDoctorReportsVersionAndCommitMismatchReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(root, "specd")
	writeMCPVersionBinary(t, command, version.Info{Version: "wrong-version", Commit: "wrong-commit"})
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := MergePinkyCodexConfig("", root)
	configPath := filepath.Join(root, ".codex", "config.toml")
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Doctor(root, "demo")
	seen := map[string]bool{}
	for _, finding := range result.Findings {
		if strings.HasPrefix(finding.Code, "MCP_BINARY_") {
			seen[finding.Code] = strings.Contains(finding.Message, "active handshake pins") &&
				strings.Contains(finding.RecoveryAction, "init --agent=pinky") &&
				strings.Contains(finding.RecoveryAction, "re-bootstrap")
		}
	}
	for _, code := range []string{"MCP_BINARY_VERSION_MISMATCH", "MCP_BINARY_COMMIT_MISMATCH"} {
		if !seen[code] {
			t.Fatalf("missing actionable %s: %+v", code, result.Findings)
		}
	}
	after, err := os.ReadFile(configPath)
	if err != nil || string(after) != config {
		t.Fatalf("doctor mutated MCP config: %v", err)
	}
}

func TestMCPDoctorAcceptsHandshakeIdentity(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMCPVersionBinary(t, filepath.Join(root, "specd"), version.Get())
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex", "config.toml"), []byte(MergePinkyCodexConfig("", root)), 0o644); err != nil {
		t.Fatal(err)
	}
	if result := Doctor(root, "demo"); !result.Healthy {
		t.Fatalf("matching MCP binary reported unhealthy: %+v", result.Findings)
	}
}

func writeMCPVersionBinary(t *testing.T, path string, info version.Info) {
	t.Helper()
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' '" + string(raw) + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
