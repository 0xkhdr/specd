package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitResultDeterministicJSON(t *testing.T) {
	result := NewInitResult("/tmp/project")
	result.Files.Written = append(result.Files.Written, "z", "a")
	result.Agents.Detected = append(result.Agents.Detected, "cursor", "codex")
	result.Warnings = append(result.Warnings,
		InitWarning{Code: "z", Message: "last"},
		InitWarning{Code: "a", Message: "first"},
	)
	result.Normalize()

	first, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("JSON is not deterministic:\n%s\n%s", first, second)
	}
	if result.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", result.SchemaVersion)
	}
	for _, field := range []string{
		`"updated":[]`, `"skipped":[]`, `"failed":[]`,
		`"configured":[]`, `"manual":[]`,
	} {
		if !strings.Contains(string(first), field) {
			t.Fatalf("JSON missing non-null array %s: %s", field, first)
		}
	}
}

func TestInitPlanPerformsNoWrites(t *testing.T) {
	root := t.TempDir()
	assets := []ScaffoldAsset{{
		Template: "asset",
		Target:   ".specd/asset",
		Policy:   ScaffoldCreate,
		Required: true,
	}}

	plan, err := PlanInit(InitOptions{Root: root}, assets, func(string) (string, error) {
		return "content", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != "write" {
		t.Fatalf("actions = %#v", plan.Actions)
	}
	if _, err := os.Stat(filepath.Join(root, ".specd")); !os.IsNotExist(err) {
		t.Fatalf("planner wrote project state: %v", err)
	}
}

func TestInitPlanPreflightFailurePerformsNoWrites(t *testing.T) {
	root := t.TempDir()
	assets := []ScaffoldAsset{{
		Template: "missing",
		Target:   ".specd/asset",
		Policy:   ScaffoldCreate,
		Required: true,
	}}

	if _, err := PlanInit(InitOptions{Root: root}, assets, func(string) (string, error) {
		return "", os.ErrNotExist
	}); err == nil {
		t.Fatal("PlanInit succeeded with missing required template")
	}
	if _, err := os.Stat(filepath.Join(root, ".specd")); !os.IsNotExist(err) {
		t.Fatalf("failed preflight wrote project state: %v", err)
	}
}

func TestExecuteInitPlanStopsAfterRequiredFailure(t *testing.T) {
	root := t.TempDir()
	plan := InitPlan{
		Root:     root,
		Warnings: []InitWarning{},
		Actions: []InitAction{
			{Kind: "write", Target: filepath.Join(root, ".specd", "first"), Content: "a", Required: true},
			{Kind: "merge", Target: filepath.Join(root, "AGENTS.md"), Content: "b", Required: true},
		},
	}
	mergeCalled := false
	result := ExecuteInitPlan(plan, false, InitExecutor{
		WriteFile: func(string, string) error { return os.ErrPermission },
		MergeAgents: func(string, string, bool) error {
			mergeCalled = true
			return nil
		},
	})
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if mergeCalled {
		t.Fatal("executor continued after required failure")
	}
	if len(result.Files.Failed) != 1 || result.Files.Failed[0] != ".specd/first" {
		t.Fatalf("failed = %#v", result.Files.Failed)
	}
}

func TestInitPlanRepairRestoresOnlyMissingFiles(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, ".specd", "existing")
	if err := AtomicWrite(existing, "user"); err != nil {
		t.Fatal(err)
	}
	assets := []ScaffoldAsset{
		{Template: "existing", Target: ".specd/existing", Policy: ScaffoldCreate, Required: true, Refresh: true},
		{Template: "missing", Target: ".specd/missing", Policy: ScaffoldCreate, Required: true, Refresh: true},
	}
	plan, err := PlanInit(InitOptions{Root: root, Repair: true}, assets, func(name string) (string, error) {
		return name, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != "skip" || plan.Actions[1].Kind != "write" {
		t.Fatalf("repair actions = %#v", plan.Actions)
	}
}

func TestInitPlanRefreshPreservesAuthoredAssets(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"managed", "authored"} {
		if err := AtomicWrite(filepath.Join(root, ".specd", name), "user"); err != nil {
			t.Fatal(err)
		}
	}
	assets := []ScaffoldAsset{
		{Template: "managed", Target: ".specd/managed", Policy: ScaffoldCreate, Required: true, Refresh: true},
		{Template: "authored", Target: ".specd/authored", Policy: ScaffoldCreate, Required: true},
	}
	plan, err := PlanInit(InitOptions{Root: root, Refresh: true}, assets, func(name string) (string, error) {
		return name + "-template", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != "write" || plan.Actions[1].Kind != "skip" {
		t.Fatalf("refresh actions = %#v", plan.Actions)
	}
}

func TestExecuteFreshInitRollbackBeforeCommit(t *testing.T) {
	root := t.TempDir()
	plan := InitPlan{
		Root:  root,
		Mode:  "init",
		Fresh: true,
		Actions: []InitAction{
			{Kind: "write", Target: filepath.Join(root, ".specd", "first"), Content: "a", Required: true},
			{Kind: "write", Target: filepath.Join(root, ".specd", "second"), Content: "b", Required: true},
		},
	}
	executor := DefaultInitExecutor()
	executor.WriteFile = func(path, content string) error {
		if strings.HasSuffix(filepath.ToSlash(path), "/second") {
			return os.ErrPermission
		}
		return AtomicWrite(path, content)
	}
	result := ExecuteInitPlan(plan, false, executor)
	if result.Status != "failed" {
		t.Fatalf("status = %q", result.Status)
	}
	if _, err := os.Stat(filepath.Join(root, ".specd")); !os.IsNotExist(err) {
		t.Fatalf("rollback left .specd behind: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".specd.init-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("rollback left staging dirs: %v", matches)
	}
}

func TestExecuteFreshInitExternalMergeFailurePreservesOriginal(t *testing.T) {
	root := t.TempDir()
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("user content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := InitPlan{
		Root:  root,
		Mode:  "init",
		Fresh: true,
		Actions: []InitAction{
			{Kind: "write", Target: filepath.Join(root, ".specd", "asset"), Content: "a", Required: true},
			{Kind: "merge", Target: agents, Content: "managed", Required: true},
		},
	}
	executor := DefaultInitExecutor()
	executor.MergeAgents = func(string, string, bool) error { return os.ErrPermission }
	result := ExecuteInitPlan(plan, false, executor)
	if result.Status != "failed" {
		t.Fatalf("status = %q", result.Status)
	}
	got, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "user content\n" {
		t.Fatalf("AGENTS.md changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".specd", "asset")); err != nil {
		t.Fatalf("committed scaffold missing: %v", err)
	}
}

func TestExecuteFreshInitRollbackUserFileSurvival(t *testing.T) {
	root := t.TempDir()
	userFile := filepath.Join(root, "user_file.txt")
	if err := os.WriteFile(userFile, []byte("important user data"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := InitPlan{
		Root:  root,
		Mode:  "init",
		Fresh: true,
		Actions: []InitAction{
			{Kind: "write", Target: filepath.Join(root, ".specd", "first"), Content: "a", Required: true},
			{Kind: "write", Target: filepath.Join(root, ".specd", "second"), Content: "b", Required: true},
		},
	}
	executor := DefaultInitExecutor()
	executor.WriteFile = func(path, content string) error {
		if strings.HasSuffix(filepath.ToSlash(path), "/second") {
			return os.ErrPermission
		}
		return AtomicWrite(path, content)
	}
	result := ExecuteInitPlan(plan, false, executor)
	if result.Status != "failed" {
		t.Fatalf("status = %q", result.Status)
	}
	if _, err := os.Stat(filepath.Join(root, ".specd")); !os.IsNotExist(err) {
		t.Fatalf("rollback left .specd behind: %v", err)
	}

	got, err := os.ReadFile(userFile)
	if err != nil {
		t.Fatalf("user file did not survive rollback: %v", err)
	}
	if string(got) != "important user data" {
		t.Fatalf("user file content changed: %q", got)
	}
}
