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
