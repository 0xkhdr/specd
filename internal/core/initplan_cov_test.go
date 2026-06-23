package core

import (
	"os"
	"path/filepath"
	"testing"
)

// initplan_cov_test.go covers PlanInit option/branch handling and the
// ExecuteInitPlan dry-run + non-fresh write/merge paths.

func tmplFn(content string) func(string) (string, error) {
	return func(string) (string, error) { return content, nil }
}

func TestPlanInitOptionBranches(t *testing.T) {
	assets := []ScaffoldAsset{{Template: "a", Target: ".specd/asset", Policy: ScaffoldCreate, Required: true}}

	// Empty root → error.
	if _, err := PlanInit(InitOptions{}, assets, tmplFn("x")); err == nil {
		t.Error("empty root should error")
	}
	// Mutually-exclusive modes → error.
	if _, err := PlanInit(InitOptions{Root: t.TempDir(), Force: true, Repair: true}, assets, tmplFn("x")); err == nil {
		t.Error("force+repair should error")
	}

	// Force adds a destructive warning.
	plan, err := PlanInit(InitOptions{Root: t.TempDir(), Force: true}, assets, tmplFn("x"))
	if err != nil {
		t.Fatal(err)
	}
	sawWarn := false
	for _, w := range plan.Warnings {
		if w.Code == "destructive-force" {
			sawWarn = true
		}
	}
	if !sawWarn {
		t.Error("force should add destructive-force warning")
	}

	// Existing create-target without force → skip.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd", "asset"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = PlanInit(InitOptions{Root: root}, assets, tmplFn("new"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != "skip" {
		t.Fatalf("existing target action = %q, want skip", plan.Actions[0].Kind)
	}
	// Not fresh: .specd already exists.
	if plan.Fresh {
		t.Error("plan should not be fresh when .specd exists")
	}
}

func TestExecuteInitPlanDryRun(t *testing.T) {
	root := t.TempDir()
	plan := InitPlan{
		Root:   root,
		Mode:   "init",
		DryRun: true,
		Actions: []InitAction{
			{Target: filepath.Join(root, ".specd/written"), Kind: "write"},
			{Target: filepath.Join(root, ".specd/skipped"), Kind: "skip"},
			{Target: filepath.Join(root, "AGENTS.md"), Kind: "merge"},
		},
	}
	res := ExecuteInitPlan(plan, false, DefaultInitExecutor())
	if res.Status != "planned" {
		t.Fatalf("dry-run status = %q, want planned", res.Status)
	}
	if len(res.Files.Written) != 1 || len(res.Files.Skipped) != 1 || len(res.Files.Updated) != 1 {
		t.Fatalf("dry-run classification wrong: %+v", res.Files)
	}
	// Dry-run must not touch the filesystem.
	if _, err := os.Stat(filepath.Join(root, ".specd/written")); !os.IsNotExist(err) {
		t.Error("dry-run wrote a file")
	}
}

func TestExecuteInitPlanNonFreshWriteAndMerge(t *testing.T) {
	root := t.TempDir()
	// Pre-create .specd so the plan is not fresh (skips the staged path).
	if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	written := []string{}
	merged := []string{}
	exec := InitExecutor{
		WriteFile:   func(path, content string) error { written = append(written, path); return nil },
		MergeAgents: func(path, content string, force bool) error { merged = append(merged, path); return nil },
	}
	plan := InitPlan{
		Root: root,
		Mode: "init",
		Actions: []InitAction{
			{Target: filepath.Join(root, ".specd/file"), Kind: "write"},
			{Target: filepath.Join(root, "AGENTS.md"), Kind: "merge"},
			{Target: filepath.Join(root, ".specd/skip"), Kind: "skip"},
			{Target: filepath.Join(root, ".specd/bad"), Kind: "bogus"},
		},
	}
	res := ExecuteInitPlan(plan, false, exec)
	if len(written) != 1 || len(merged) != 1 {
		t.Fatalf("write=%v merge=%v", written, merged)
	}
	// The unknown action kind is recorded as a failure.
	if len(res.Files.Failed) != 1 {
		t.Fatalf("failed = %+v, want one (unknown action)", res.Files.Failed)
	}
}
