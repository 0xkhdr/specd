package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// diffScopeRepo builds a real git repository with a spec in execution and one
// declared file. The diff-scope check shells out to git, so this fixture cannot
// be faked the way the pure gate table can.
func diffScopeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustGit(t, root, "init")
	if err := core.WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "requirements.md"), "# Requirements — demo\n\n- **R1** When scope is checked, the system shall refuse undeclared files.\n")
	write(filepath.Join(dir, "design.md"), "# Design — demo\n\n## Modules\nScope.\n\n## On-disk contracts\nState.\n\n## Invariants\nEvidence.\n")
	write(filepath.Join(dir, "tasks.md"), "# Tasks — demo\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | declared.txt | - | test -f declared.txt | R1 |\n")
	write(filepath.Join(root, "declared.txt"), "start\n")

	mustGit(t, root, "add", ".")
	mustGit(t, root, "commit", "-m", "baseline", "--no-gpg-sign")
	return root
}

// R4.5: the check is a core invariant. This fixture never enables the
// production profile, which is the condition that previously gated it.
func TestDiffScopeRunsOnDefaultProfile(t *testing.T) {
	root := diffScopeRepo(t)

	config := loadSpecConfig(root)
	if config.ProductionTaskAuthorityRequired() {
		t.Fatal("fixture enabled the production profile; this test must prove the default path")
	}

	// A session pins the baseline. Without one, nothing bounded the work and
	// the check has no reference point (the graduated case).
	if _, err := core.OpenDriverSession(root, "demo", "test-host", "handshake-digest", gitHead(root), 0, time.Now()); err != nil {
		t.Fatal(err)
	}

	spec, err := loadSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	task, ok := findTaskRow(spec.Tasks, "T1")
	if !ok {
		t.Fatal("fixture task missing")
	}

	// A declared change passes.
	if err := os.WriteFile(filepath.Join(root, "declared.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceDiffScope(root, "demo", "T1", task); err != nil {
		t.Fatalf("declared change refused on default profile: %v", err)
	}

	// An undeclared sibling refuses — on the default profile.
	if err := os.WriteFile(filepath.Join(root, "undeclared.txt"), []byte("sneaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = enforceDiffScope(root, "demo", "T1", task)
	if err == nil {
		t.Fatal("undeclared file accepted on the default profile; R4.5 requires the check on every profile")
	}
	refusal, ok := core.AsRefusal(err)
	if !ok {
		t.Fatalf("got bare error %v, want a typed refusal", err)
	}
	if refusal.Code != "OUTSIDE_SCOPE" {
		t.Errorf("code = %q, want OUTSIDE_SCOPE", refusal.Code)
	}
	if !strings.Contains(err.Error(), "undeclared.txt") {
		t.Errorf("refusal does not name the offending file: %v", err)
	}
	if refusal.RecoveryCommand == "" {
		t.Errorf("refusal names no recovery: %+v", refusal)
	}
}

// R4.3 end to end: editing harness state is refused through the real git path,
// not just in the pure gate. This is the case core.DeriveDiff cannot see.
func TestDiffScopeRejectsDirectSpecdEditOnDefaultProfile(t *testing.T) {
	root := diffScopeRepo(t)
	if _, err := core.OpenDriverSession(root, "demo", "test-host", "handshake-digest", gitHead(root), 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	spec, err := loadSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	task, _ := findTaskRow(spec.Tasks, "T1")

	// Hand-edit the task markers, the exact move R4.3 exists to catch.
	tasksPath := filepath.Join(root, ".specd", "specs", "demo", "tasks.md")
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tasksPath, append(raw, []byte("\nhand-edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	err = enforceDiffScope(root, "demo", "T1", task)
	if err == nil {
		t.Fatal("direct .specd edit accepted; core.DeriveDiff strips these paths, so the check must not use it")
	}
	if !strings.Contains(err.Error(), "harness-owned") {
		t.Errorf("refusal does not name the rule: %v", err)
	}
}

// The graduated case: with nothing pinning a baseline, the work was never
// bounded and completion proceeds. This is deliberate, so it is pinned.
func TestDiffScopeWithoutBaselineProceeds(t *testing.T) {
	root := diffScopeRepo(t)
	spec, err := loadSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	task, _ := findTaskRow(spec.Tasks, "T1")

	if err := os.WriteFile(filepath.Join(root, "undeclared.txt"), []byte("sneaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No session, no mission: nothing pinned a baseline to measure against.
	if err := enforceDiffScope(root, "demo", "T1", task); err != nil {
		t.Fatalf("unbounded work refused with no baseline to measure against: %v", err)
	}
}

// No flag, config value, or role can switch the check off once a baseline
// exists. This pins the absence of a bypass.
func TestDiffScopeHasNoBypass(t *testing.T) {
	root := diffScopeRepo(t)
	if _, err := core.OpenDriverSession(root, "demo", "test-host", "handshake-digest", gitHead(root), 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	spec, err := loadSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	task, _ := findTaskRow(spec.Tasks, "T1")
	if err := os.WriteFile(filepath.Join(root, "undeclared.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Every profile the repo can carry still refuses. The check reads none of
	// them, which is the property under test.
	//
	// The config file is project.yml (internal/cmd/registry.go configPaths) and
	// the key is top-level `profile:`. An earlier version of this test wrote
	// .specd/config.toml with a `[lifecycle]` section, which specd never reads —
	// so it exercised the default profile twice and would have passed with the
	// production branch broken.
	for _, profile := range []string{"default", "production"} {
		body := "version: 1\nagent: claude\nprofile: " + profile + "\n"
		if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if loadSpecConfig(root).ProductionTaskAuthorityRequired() != (profile == "production") {
			t.Fatalf("profile %q did not arm; the config path or key is wrong and this test proves nothing", profile)
		}
		if err := enforceDiffScope(root, "demo", "T1", task); err == nil {
			t.Fatalf("profile %q disabled the diff-scope check", profile)
		}
	}
}

// F1 regression: the production profile refused an unpinned task before this
// spec, and R4.5 does not ask it to stop. The graduated "proceed with no
// baseline" behaviour is for the default profile only.
func TestDiffScopeProductionRefusesUnpinnedTask(t *testing.T) {
	root := diffScopeRepo(t)
	if err := os.WriteFile(filepath.Join(root, "project.yml"),
		[]byte("version: 1\nagent: claude\nprofile: production\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !loadSpecConfig(root).ProductionTaskAuthorityRequired() {
		t.Fatal("production profile did not arm; this test would prove nothing")
	}

	spec, err := loadSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	task, _ := findTaskRow(spec.Tasks, "T1")

	// No session and no mission: nothing pinned a baseline.
	err = enforceDiffScope(root, "demo", "T1", task)
	if err == nil {
		t.Fatal("production accepted a task with no pinned baseline")
	}
	refusal, ok := core.AsRefusal(err)
	if !ok || refusal.Code != "BASELINE_UNPINNED" {
		t.Fatalf("got %v, want BASELINE_UNPINNED", err)
	}
	if refusal.RecoveryCommand == "" {
		t.Fatalf("refusal names no recovery: %+v", refusal)
	}

	// The default profile still proceeds, which is the graduated behaviour.
	if err := os.WriteFile(filepath.Join(root, "project.yml"),
		[]byte("version: 1\nagent: claude\nprofile: default\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceDiffScope(root, "demo", "T1", task); err != nil {
		t.Fatalf("default profile refused an unpinned task: %v", err)
	}
}
