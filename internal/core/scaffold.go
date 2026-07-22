package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

// RequirementsScaffold is the single source for the requirements.md starter
// document: it seeds `specd new` and is the stub the design gate compares a
// filled doc against. Its shape must satisfy core.ParseRequirements — a `## R<n>`
// heading and `- R<n>.<m>:` criteria — so the scaffold passes its own gates
// (R2.1). Changing it silently breaks that contract; the template-conformance
// suite pins it.
func RequirementsScaffold(slug string) string {
	return "# Requirements — " + slug + "\n\n" +
		"> Use stable requirement and criterion IDs. Write testable EARS behavior; replace all prompts.\n\n" +
		"## R1 — <name>\n\n" +
		"owner: <human owner>\npriority: <must|should|could>\nrisk: <low|medium|high|critical>\n\n" +
		"- R1.1: When <trigger>, the system shall <observable response>.\n\n" +
		"## Edge and failure behavior\n\n- <invalid input, dependency failure, boundary condition>\n\n" +
		"## Non-goals\n\n- <explicitly excluded outcome>\n"
}

// TasksScaffold is the single source for the tasks.md starter document. Its
// example-values comment must teach values every armed consumer accepts —
// `evidence=` as class/check-id and never a bare class (R2.2), and `kind`,
// `risk`, and `capabilities` drawn from the canonical vocabularies so the
// example also routes (spec 05 R1.2). TestTaskContractConformance pins the
// example against ParseTaskContract and RouteTask.
func TasksScaffold(slug string) string {
	return "# Tasks — " + slug + "\n\n" +
		"> Add only real work. The optional columns beyond the six required ones may be omitted.\n" +
		"> Production rows declare full trace, risk, routing, context, capability, evidence, and edge-check intent.\n\n" +
		"| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity | capabilities | context | evidence | checks |\n" +
		"|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n\n" +
		"<!-- Example field values (not a runnable task): id=T<n>; role=craftsman; files=<paths>; depends-on=-; verify=<command>; acceptance=<criterion IDs>; refs=R1.1; kind=feature; risk=medium; complexity=standard; capabilities=context,sandbox; context=<required sources>; evidence=test/readme-purpose; checks=<negative and edge cases>. -->\n"
}

// TasksScaffoldExampleRow parses the tasks scaffold's example-values comment
// into a TaskRow, so every armed consumer can be run against the shipped bytes
// instead of a hand-copied duplicate that silently drifts (spec 05 R1.2).
// Fields are `key=value` pairs separated by "; " on one comment line.
func TasksScaffoldExampleRow() TaskRow {
	fields := map[string]string{}
	for _, line := range strings.Split(TasksScaffold("demo"), "\n") {
		_, rest, ok := strings.Cut(line, "Example field values (not a runnable task):")
		if !ok {
			continue
		}
		rest = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "-->")), ".")
		for _, pair := range strings.Split(rest, "; ") {
			if key, value, ok := strings.Cut(pair, "="); ok {
				fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		}
	}
	files := fields["files"]
	declared, _ := normalizeDeclaredFiles(files)
	return TaskRow{
		ID: fields["id"], Role: fields["role"], Files: files, DeclaredFiles: declared,
		DependsOn: splitCanonical(fields["depends-on"]), Verify: fields["verify"], Acceptance: fields["acceptance"],
		Refs: splitCanonical(fields["refs"]), Kind: fields["kind"], Risk: fields["risk"],
		Complexity: fields["complexity"], Capabilities: sortedUnique(splitCanonical(fields["capabilities"])),
		Context: fields["context"], Evidence: fields["evidence"], Checks: fields["checks"],
	}
}

func WriteScaffold(root string, agents ...string) error {
	if err := writeManagedAssets(root); err != nil {
		return err
	}
	if err := writeAgents(root); err != nil {
		return err
	}
	if err := writeCanonicalConfig(root); err != nil {
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

// writeCanonicalConfig creates config only when the operator owns no config
// spelling. Config is not a managed region: init never replaces or relocates it.
func writeCanonicalConfig(root string) error {
	for _, rel := range []string{filepath.Join(".specd", "config.yaml"), "project.yml", "project.yaml"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	body, err := embedtemplates.FS.ReadFile("config.yaml")
	if err != nil {
		return err
	}
	target := filepath.Join(root, ".specd", "config.yaml")
	if err := AtomicWrite(target, string(body)); err != nil {
		return err
	}
	_, diagnostics := LoadConfig(ConfigPaths{Project: target}, nil)
	if len(diagnostics) != 0 {
		return fmt.Errorf("validate scaffolded config: %s", diagnostics[0].Message)
	}
	return nil
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
	return AtomicWrite(configPath, MergePinkyCodexConfig(string(existing), root))
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
	sandbox := ""
	if role != "craftsman" { // scout, validator, auditor are read-only
		sandbox = "sandbox_mode = \"read-only\"\n"
	}
	return strings.TrimSpace(`name = "pinky-`+role+`"
description = "`+pinkyRoleDescription(role)+`"
`+sandbox+`developer_instructions = """
You are the specd Pinky `+role+` worker. Follow AGENTS.md and .specd/roles/`+role+`.md before acting.

Run specd status, load specd context for the assigned task, stay inside the task files, and record evidence with specd verify.
"""
`) + "\n"
}
