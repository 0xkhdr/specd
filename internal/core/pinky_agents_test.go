package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentRegistryIncludesPinky(t *testing.T) {
	found := false
	for _, host := range AgentHosts() {
		if host.Name == "pinky" {
			found = host.Install == "claude+codex"
		}
	}
	if !found {
		t.Fatal("AgentHosts missing pinky host")
	}
}

func TestMergePinkyCodexConfigPreservesUserContent(t *testing.T) {
	existing := "model = \"gpt-5\"\n\n" + pinkyCodexBegin + "\nold\n" + pinkyCodexEnd + "\n\n[profiles.default]\n"
	got := MergePinkyCodexConfig(existing)
	if !strings.Contains(got, "model = \"gpt-5\"") || !strings.Contains(got, "[profiles.default]") {
		t.Fatalf("MergePinkyCodexConfig lost user content: %q", got)
	}
	if strings.Contains(got, "old") {
		t.Fatalf("MergePinkyCodexConfig did not replace managed block: %q", got)
	}
	if !strings.Contains(got, `[agents.pinky-craftsman]`) {
		t.Fatalf("MergePinkyCodexConfig missing pinky agent block: %q", got)
	}
}

func TestWriteScaffoldPinkyArtifacts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex", "config.toml"), []byte("model = \"gpt-5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteScaffold(root, "pinky"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(".claude", "agents", "pinky-scout.md"),
		filepath.Join(".claude", "agents", "pinky-craftsman.md"),
		filepath.Join(".claude", "agents", "pinky-validator.md"),
		filepath.Join(".claude", "agents", "pinky-auditor.md"),
		filepath.Join(".codex", "agents", "pinky-scout.toml"),
		filepath.Join(".codex", "agents", "pinky-craftsman.toml"),
		filepath.Join(".codex", "agents", "pinky-validator.toml"),
		filepath.Join(".codex", "agents", "pinky-auditor.toml"),
	}
	for _, rel := range want {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	config, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "model = \"gpt-5\"") {
		t.Fatalf("config lost user content: %s", config)
	}
	if !strings.Contains(string(config), `[agents.pinky-auditor]`) {
		t.Fatalf("config missing pinky agents: %s", config)
	}
}

func TestDiscoverAgentsPinkyStates(t *testing.T) {
	missing := DiscoverAgents(t.TempDir())
	if missing[0].Status != "missing" {
		t.Fatalf("empty workspace status = %q, want missing", missing[0].Status)
	}

	root := t.TempDir()
	if err := WriteScaffold(root, "pinky"); err != nil {
		t.Fatal(err)
	}
	installed := DiscoverAgents(root)
	if installed[0].Status != "installed" {
		t.Fatalf("pinky workspace status = %q, want installed: %#v", installed[0].Status, installed[0])
	}

	if err := os.WriteFile(filepath.Join(root, ".codex", "config.toml"), []byte("model = \"gpt-5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := DiscoverAgents(root)
	if invalid[0].Status != "invalid" {
		t.Fatalf("broken config status = %q, want invalid: %#v", invalid[0].Status, invalid[0])
	}
}
