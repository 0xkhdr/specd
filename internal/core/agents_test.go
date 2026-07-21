package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
