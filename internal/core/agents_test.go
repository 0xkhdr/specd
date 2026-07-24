package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/version"
)

func TestAgentRegistry(t *testing.T) {
	hosts := AgentHosts()
	if len(hosts) < 2 {
		t.Fatalf("AgentHosts returned %d hosts", len(hosts))
	}
	for _, host := range hosts {
		if host.Name == "" || host.Detect == "" || host.Verify == "" {
			t.Fatalf("incomplete host: %+v", host)
		}
	}
}

func TestAgentsWorkerDefinitionsFollowHandshakeHarness(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root, "pinky"); err != nil {
		t.Fatal(err)
	}
	for _, harness := range []string{"codex", "claude"} {
		if !((WorkerDefinitions{Root: root, Harness: harness}).WorkerAvailable()) {
			t.Fatalf("%s workers not detected", harness)
		}
	}
	if err := os.Remove(filepath.Join(root, ".codex", "agents", "pinky-craftsman.toml")); err != nil {
		t.Fatal(err)
	}
	if (WorkerDefinitions{Root: root, Harness: "codex"}).WorkerAvailable() {
		t.Fatal("codex handshake accepted with missing codex worker")
	}
	if !(WorkerDefinitions{Root: root, Harness: "claude"}).WorkerAvailable() {
		t.Fatal("codex damage incorrectly invalidated claude harness")
	}
	if (WorkerDefinitions{Root: root, Harness: "other"}).WorkerAvailable() {
		t.Fatal("unknown handshake harness accepted")
	}
}

func TestDoctorChecksOrchestrationWorkerAlignment(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	project := "version: 1\nagent: codex\norchestration:\n  enabled: true\n"
	if err := os.WriteFile(filepath.Join(root, ".specd", "config.yaml"), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	result := Doctor(root, "demo")
	assertDoctorWorkerRepair(t, result, "WORKER_DEFINITION_MISSING")

	writeMCPVersionBinary(t, filepath.Join(root, "specd"), version.Get())
	if err := WriteScaffold(root, "pinky"); err != nil {
		t.Fatal(err)
	}
	if result = Doctor(root, "demo"); !result.Healthy {
		t.Fatalf("repaired codex workers still unhealthy: %+v", result.Findings)
	}

	project = "version: 1\nagent: claude\norchestration:\n  enabled: true\n"
	if err := os.WriteFile(filepath.Join(root, ".specd", "config.yaml"), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".claude", "agents", "pinky-validator.md")); err != nil {
		t.Fatal(err)
	}
	result = Doctor(root, "demo")
	assertDoctorWorkerRepair(t, result, "WORKER_DEFINITION_MISSING")
}

func assertDoctorWorkerRepair(t *testing.T, result DoctorResultV1, code string) {
	t.Helper()
	for _, finding := range result.Findings {
		if finding.Code == code && strings.Contains(finding.Message, "handshake agent") && finding.RecoveryAction == "run `specd init --repair`" {
			return
		}
	}
	t.Fatalf("doctor missing actionable %s finding: %+v", code, result.Findings)
}

func TestAgentsMergePreservesUser(t *testing.T) {
	existing := "user note\n\n" + agentsBegin + "\nold\n" + agentsEnd + "\n\nkeep me\n"
	got := MergeAgents(existing, "new")
	if !strings.Contains(got, "user note") || !strings.Contains(got, "keep me") {
		t.Fatalf("MergeAgents lost user content: %q", got)
	}
	if strings.Contains(got, "old") || !strings.Contains(got, "new") {
		t.Fatalf("MergeAgents did not replace managed block: %q", got)
	}
}

func TestMCPAgentConfigPinsLocalExecutable(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "specd")
	if err := os.WriteFile(local, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteScaffold(root, "pinky"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "command = "+strconv.Quote(local)) {
		t.Fatalf("generated MCP config does not pin local executable: %s", raw)
	}
}

func TestAgentsMCPConfigRefusesUnresolvedCommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", "")
	existing := "model = \"gpt-5\"\n\n" + pinkyCodexBegin + "\n[mcp_servers.specd]\ncommand = \"specd\"\n" + pinkyCodexEnd + "\n\n[profiles.default]\n"

	got := MergePinkyCodexConfig(existing, root)
	if strings.Contains(got, pinkyCodexBegin) || strings.Contains(got, `command = "specd"`) {
		t.Fatalf("unresolved Pinky hosting did not fail closed: %q", got)
	}
	if !strings.Contains(got, `model = "gpt-5"`) || !strings.Contains(got, "[profiles.default]") {
		t.Fatalf("unresolved Pinky hosting lost user config: %q", got)
	}
	if snippet, err := MCPConfigSnippet("claude-code", root, ""); err == nil || snippet != "" {
		t.Fatalf("unresolved MCP snippet = %q, %v; want empty error result", snippet, err)
	}
}

func TestAgentsMCPConfigPreservesUnmatchedManagedBegin(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", "")
	existing := "model = \"gpt-5\"\n\n" + pinkyCodexBegin + "\n[mcp_servers.specd]\ncommand = \"specd\"\n\n[profiles.user]\nmodel = \"keep-me\"\n"

	if got := MergePinkyCodexConfig(existing, root); got != existing {
		t.Fatalf("unmatched managed begin changed user config:\nwant %q\ngot  %q", existing, got)
	}
}
