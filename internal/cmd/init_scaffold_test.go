package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestCanonicalConfigScaffold(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	path := filepath.Join(root, ".specd", "config.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("canonical config not written: %v", err)
	}
	if !strings.Contains(string(body), "timeout_seconds: 600") {
		t.Fatalf("canonical config missing active bound:\n%s", body)
	}

	// The scaffolded template must parse and yield the bound — guards the template
	// against drifting out of parseSimpleYAML's supported subset.
	cfg, diags := core.LoadConfig(core.ConfigPaths{Project: path}, nil)
	if len(diags) != 0 {
		t.Fatalf("scaffolded canonical config did not parse cleanly: %#v", diags)
	}
	if cfg.Verify.TimeoutSecs != 600 {
		t.Fatalf("verify.timeout_seconds = %d, want 600", cfg.Verify.TimeoutSecs)
	}

	// Idempotent: a second init preserves an operator-edited file byte-for-byte.
	edited := "verify:\n  timeout_seconds: 42\n"
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("second init: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != edited {
		t.Fatalf("second init clobbered operator config:\n%s", got)
	}

	for _, legacy := range []string{"project.yml", "project.yaml"} {
		legacyRoot := t.TempDir()
		legacyPath := filepath.Join(legacyRoot, legacy)
		if err := os.WriteFile(legacyPath, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runInit(legacyRoot, nil, map[string]string{}); err != nil {
			t.Fatalf("init with %s: %v", legacy, err)
		}
		if _, err := os.Stat(filepath.Join(legacyRoot, ".specd", "config.yaml")); !os.IsNotExist(err) {
			t.Fatalf("init silently created canonical config alongside %s", legacy)
		}
		if got, _ := os.ReadFile(legacyPath); string(got) != edited {
			t.Fatalf("init changed %s:\n%s", legacy, got)
		}
	}

	agentsPath := filepath.Join(root, "AGENTS.md")
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentsPath, append([]byte("operator guidance\n"), agents...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("refresh scaffold: %v", err)
	}
	if got, _ := os.ReadFile(agentsPath); !strings.HasPrefix(string(got), "operator guidance\n") {
		t.Fatalf("init clobbered human-owned guidance:\n%s", got)
	}
}

// TestInitScaffoldGuidanceParity pins spec 01 R6.2: the scaffolded AGENTS.md
// points agents at the machine guidance surface and keeps approval human-only —
// it never tells an agent to self-approve.
func TestInitScaffoldGuidanceParity(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	agents := string(body)
	if !strings.Contains(agents, "status <slug> --guide") {
		t.Fatalf("scaffold does not point agents at machine guidance:\n%s", agents)
	}
	if !strings.Contains(agents, "human-only") || !strings.Contains(agents, "self-approve") {
		t.Fatalf("scaffold must mark approval human-only and forbid self-approval:\n%s", agents)
	}
}

func TestInitScaffoldCompactProgressiveGuide(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	guide := string(body)
	if len(body) > 2200 {
		t.Fatalf("always-on guide too large: %d bytes", len(body))
	}
	for _, want := range []string{"specd handshake bootstrap <slug> --json", "specd status <slug> --guide", "specd context <slug> <task> --json", "specd verify <slug> <task>", "specd complete-task <slug> <task>", "specd check <slug>", "human-only", "state.json", ".specd/skills/<id>/SKILL.md", "foundation", "maintenance"} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide missing %q", want)
		}
	}
	if strings.Contains(guide, "verify record is the evidence. This, not your say-so, is\n   what marks a task complete") {
		t.Fatal("guide still claims verify alone completes task")
	}
}

func TestFreshScaffoldConsumerConformance(t *testing.T) {
	t.Run("role_commands_are_agent_permitted", func(t *testing.T) {
		root := t.TempDir()
		if err := runInit(root, nil, map[string]string{}); err != nil {
			t.Fatal(err)
		}
		for role, routes := range map[string][]string{
			"craftsman": {"specd verify <slug> <task>", "specd complete-task <slug> <task>", "specd request-decision <slug> --text <deviation>"},
			"validator": {"specd verify <slug> <task>"},
		} {
			body, err := os.ReadFile(filepath.Join(root, ".specd", "roles", role+".md"))
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range routes {
				if !strings.Contains(string(body), route) {
					t.Errorf("%s role missing %q", role, route)
					continue
				}
				fields := strings.Fields(route)
				operation, ok := core.ResolveOperation(fields[1], fields[2:], nil)
				if !ok || operation.Actor != core.ActorAgent {
					t.Errorf("%s role route %q is not agent-permitted: %+v", role, route, operation)
				}
			}
			if role == "validator" && !strings.Contains(string(body), "Never call `specd complete-task`") {
				t.Error("validator role must explicitly refuse state-writing completion route")
			}
		}
	})

	t.Run("empty_requirement_ids_name_requirements", func(t *testing.T) {
		root := t.TempDir()
		if err := runInit(root, nil, map[string]string{}); err != nil {
			t.Fatal(err)
		}
		if _, err := captureStdout(t, func() error { return runNew(root, []string{"demo"}, nil) }); err != nil {
			t.Fatal(err)
		}
		specDir := filepath.Join(root, ".specd", "specs", "demo")
		if err := os.WriteFile(filepath.Join(specDir, "requirements.md"), []byte("## Requirement R1 — x\n\n- **R1.1** When a, the system shall b.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		tasks := "# Tasks\n\n| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity | capabilities | context | evidence | checks |\n" +
			"|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n" +
			"| T1 | craftsman | x.go | - | true | R1.1 | R1.1 | fix | low | standard | context,sandbox | - | test/x | invalid-input |\n"
		if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
			t.Fatal(err)
		}
		output, err := captureStdout(t, func() error { return runCheck(root, []string{"demo"}, nil) })
		if err == nil {
			t.Fatal("check accepted requirements.md with no parseable IDs")
		}
		if !strings.Contains(output, "task-trace: requirements.md declares no parseable requirement IDs") {
			t.Fatalf("check did not attribute empty IDs to requirements.md:\n%s", output)
		}
		if strings.Contains(output, "task-trace: T1 references unknown requirement") {
			t.Fatalf("check blamed the task for the malformed requirements scaffold:\n%s", output)
		}
	})

	t.Run("repair_creates_required_layout_idempotently", func(t *testing.T) {
		root := t.TempDir()
		if err := runInit(root, nil, map[string]string{}); err != nil {
			t.Fatal(err)
		}
		specsRoot := filepath.Join(root, ".specd", "specs")
		if err := os.RemoveAll(specsRoot); err != nil {
			t.Fatal(err)
		}
		result := core.Doctor(root, "")
		if !hasDoctorFinding(result, "LAYOUT_MISSING", ".specd/specs", "run `specd init --repair`") {
			t.Fatalf("doctor did not name the repair route: %+v", result.Findings)
		}
		output, err := captureStdout(t, func() error { return runInit(root, nil, map[string]string{"repair": ""}) })
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output, "repaired .specd/specs/.gitkeep") {
			t.Fatalf("repair did not report required layout creation: %s", output)
		}
		if _, err := os.Stat(filepath.Join(specsRoot, ".gitkeep")); err != nil {
			t.Fatalf("repair did not create tracked specs root: %v", err)
		}
		if _, err := captureStdout(t, func() error { return runInit(root, nil, map[string]string{"repair": ""}) }); err != nil {
			t.Fatalf("idempotent repair: %v", err)
		}
		if result := core.Doctor(root, ""); !result.Healthy {
			t.Fatalf("doctor unhealthy after named repair: %+v", result.Findings)
		}
	})
}

func hasDoctorFinding(result core.DoctorResultV1, code, ref, recovery string) bool {
	for _, finding := range result.Findings {
		if finding.Code == code && finding.Ref == ref && finding.RecoveryAction == recovery {
			return true
		}
	}
	return false
}

func TestWorkflowCoherenceScaffoldContracts(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	skills, err := filepath.Glob(filepath.Join(root, ".specd", "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 11 {
		t.Fatalf("shipped skills = %d, want 11: %v", len(skills), skills)
	}

	if _, err := captureStdout(t, func() error { return runNew(root, []string{"demo"}, nil) }); err != nil {
		t.Fatalf("new: %v", err)
	}
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	requirements, err := os.ReadFile(filepath.Join(specDir, "requirements.md"))
	if err != nil {
		t.Fatal(err)
	}
	design, err := os.ReadFile(filepath.Join(specDir, "design.md"))
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := os.ReadFile(filepath.Join(specDir, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(requirements), "owner:") || !strings.Contains(string(design), "## Failure") {
		t.Fatal("production-shaped authoring guidance missing")
	}
	parsed, err := core.ParseTasksMd(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tasks), "scaffolded read-only placeholder") || !strings.Contains(string(tasks), "capabilities") || len(parsed.Tasks) != 0 {
		t.Fatalf("task scaffold not production-shaped and empty:\n%s", tasks)
	}
}
