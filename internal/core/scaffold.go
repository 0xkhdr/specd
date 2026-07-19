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
	if err := writeProjectConfig(root); err != nil {
		return err
	}
	if err := writeSkillsRoot(root); err != nil {
		return err
	}
	for _, agent := range agents {
		if strings.EqualFold(agent, "pinky") {
			return writePinkyArtifacts(root)
		}
	}
	return nil
}

func writeSkillsRoot(root string) error {
	target := filepath.Join(root, ".specd", "skills", "README.md")
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	body := "# Portable skills\n\n" +
		"Place each package at `.specd/skills/<id>/SKILL.md`. Its `specd-skill` metadata declares\n" +
		"version, trigger, phases, roles, capabilities, references, provenance, and budget. Skill prose is\n" +
		"advisory: it cannot add tools, widen task scope, approve work, alter gates, or create evidence.\n"
	return AtomicWrite(target, body)
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

// writeProjectConfig materializes a commented project.yml at the project root so
// a fresh project ships with the verify timeout bound visible and active. It is
// operator-owned (not a managed region): an existing file is never clobbered.
func writeProjectConfig(root string) error {
	target := filepath.Join(root, "project.yml")
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	body, err := embedtemplates.FS.ReadFile("project.yml")
	if err != nil {
		return err
	}
	return AtomicWrite(target, string(body))
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

// pinkyRoleDescription is the one-line role summary both harnesses require in
// their agent-definition schemas (codex `description`, Claude Code frontmatter).
func pinkyRoleDescription(role string) string {
	switch role {
	case "scout":
		return "Read-only specd Pinky scout: inspects repo, steering, and spec, reports findings as evidence."
	case "craftsman":
		return "specd Pinky craftsman: edits only declared task files, verifies through specd verify, reports evidence."
	case "validator":
		return "Read-only specd Pinky validator: runs the task verify command and reports the specd-generated record."
	default:
		return "Read-only specd Pinky auditor: audits the declared diff against acceptance criteria and reports findings."
	}
}

func pinkyClaudeAgent(role string) string {
	return strings.TrimSpace(`---
name: pinky-`+role+`
description: `+pinkyRoleDescription(role)+`
---

# Pinky `+role+`

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
description = "`+pinkyRoleDescription(role)+`"
developer_instructions = """
You are the specd Pinky `+role+` worker. Follow AGENTS.md and .specd/roles/`+role+`.md before acting.

Run specd status, load specd context for the assigned task, stay inside the task files, and record evidence with specd verify.
"""
`) + "\n"
}
