package core

import (
	"os"
	"path/filepath"
	"strings"

	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

func WriteScaffold(root string, agents ...string) error {
	if err := writeManagedAssets(root); err != nil {
		return err
	}
	if err := writeAgents(root); err != nil {
		return err
	}
	for _, agent := range agents {
		if strings.EqualFold(agent, "pinky") {
			return writePinkyArtifacts(root)
		}
	}
	return nil
}

func writeManagedAssets(root string) error {
	assets, err := ManagedAssets()
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if err := AtomicWrite(filepath.Join(root, asset.RelPath), asset.Block()+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// writeAgents materializes AGENTS.md at the project root, merging into any
// existing file through the managed specd block.
func writeAgents(root string) error {
	generated, err := embedtemplates.FS.ReadFile("AGENTS.md")
	if err != nil {
		return err
	}
	target := filepath.Join(root, "AGENTS.md")
	existing, err := os.ReadFile(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return AtomicWrite(target, MergeAgents(string(existing), string(generated)))
}

func writePinkyArtifacts(root string) error {
	claudeDir := filepath.Join(root, ".claude", "agents")
	codexDir := filepath.Join(root, ".codex", "agents")
	for _, dir := range []string{claudeDir, codexDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	for _, role := range []string{"scout", "craftsman", "validator", "auditor"} {
		if err := AtomicWrite(filepath.Join(claudeDir, "pinky-"+role+".md"), pinkyClaudeAgent(role)); err != nil {
			return err
		}
		if err := AtomicWrite(filepath.Join(codexDir, "pinky-"+role+".toml"), pinkyCodexAgent(role)); err != nil {
			return err
		}
	}
	configPath := filepath.Join(root, ".codex", "config.toml")
	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return AtomicWrite(configPath, MergePinkyCodexConfig(string(existing)))
}

func pinkyClaudeAgent(role string) string {
	return strings.TrimSpace(`# Pinky `+role+`

You are the specd Pinky `+role+` worker. Follow AGENTS.md and .specd/roles/`+role+`.md before acting.

Rules:
- Run specd status before choosing work.
- Run specd context <slug> <task> before task work.
- Stay inside declared files for the task role.
- Record evidence through specd verify; do not mark work complete by prose.
- Stop and report blocked when specd gates or verify fail twice.
`) + "\n"
}

func pinkyCodexAgent(role string) string {
	return strings.TrimSpace(`name = "pinky-`+role+`"
instructions = """
You are the specd Pinky `+role+` worker. Follow AGENTS.md and .specd/roles/`+role+`.md before acting.

Run specd status, load specd context for the assigned task, stay inside the task files, and record evidence with specd verify.
"""
`) + "\n"
}
