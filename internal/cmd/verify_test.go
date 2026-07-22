package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
)

func TestRevertOnFail(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-m", "base")

	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := "| id | role | files | depends-on | verify | acceptance |\n" +
		"|---|---|---|---|---|---|\n" +
		"| ⬜ T1 | craftsman | tracked.txt | - | printf changed > tracked.txt; false | fails after edit |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runVerify(root, []string{"demo", "T1"}, map[string]string{"revert-on-fail": "true"})
	if err == nil {
		t.Fatalf("runVerify succeeded, want failure")
	}
	got, err := os.ReadFile(filepath.Join(root, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "base\n" {
		t.Fatalf("tracked.txt = %q, want base restored", got)
	}
}

// TestVerifyFailureLeavesCleanTree pins deterministic cleanup (SPEC-03 T-03-03):
// a failing verify under --revert-on-fail must restore tracked state, release
// the per-spec lock, and leak no temp artifacts (git-apply .orig/.rej, stray
// tmp files, or a leftover specd.lock).
func TestVerifyFailureLeavesCleanTree(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-m", "base")

	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := "| id | role | files | depends-on | verify | acceptance |\n" +
		"|---|---|---|---|---|---|\n" +
		"| ⬜ T1 | craftsman | tracked.txt | - | printf changed > tracked.txt; false | fails after edit |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runVerify(root, []string{"demo", "T1"}, map[string]string{"revert-on-fail": "true"}); err == nil {
		t.Fatal("runVerify succeeded, want failure")
	}

	// Tracked file restored.
	if got, _ := os.ReadFile(filepath.Join(root, "tracked.txt")); string(got) != "base\n" {
		t.Fatalf("tracked.txt = %q, want base restored", got)
	}
	// No temp/lock artifacts anywhere in the tree.
	if err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if strings.HasSuffix(name, ".orig") || strings.HasSuffix(name, ".rej") ||
			name == "specd.lock" || strings.HasPrefix(name, "tmp") {
			t.Errorf("leaked artifact after verify+revert: %s", p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyProductionRequiresSandbox(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte("security:\n  profile: production\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"schema_version":1,"slug":"demo","mode":"default","status":"tasks","phase":"plan","revision":1,"records":{}}`
	if err := os.WriteFile(filepath.Join(specDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	tasks := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| ⬜ T1 | craftsman | x | - | touch must-not-run | isolated |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runVerify(root, []string{"demo", "T1"}, map[string]string{"sandbox-binary": "definitely-missing-specd-sandbox"})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("production verify error = %v, want missing sandbox refusal", err)
	}
	if _, err := os.Stat(filepath.Join(root, "must-not-run")); !os.IsNotExist(err) {
		t.Fatal("verify shell started before sandbox refusal")
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestVerifyEvidenceSemantics pins strong evidence semantics (spec 05 R3.1-R3.4).
func TestVerifyEvidenceSemantics(t *testing.T) {
	// R3.2: Evidence status distinguishes states
	t.Run("R3.2_EvidenceStatusDistinguishes", func(t *testing.T) {
		head := "abc123"

		tests := []struct {
			name       string
			record     core.EvidenceRecord
			wantStatus core.EvidenceStatusType
		}{
			{
				name:       "passing",
				record:     core.EvidenceRecord{TaskID: "T1", ExitCode: 0, GitHead: head},
				wantStatus: core.EvidencePassing,
			},
			{
				name:       "failing",
				record:     core.EvidenceRecord{TaskID: "T1", ExitCode: 1, GitHead: head},
				wantStatus: core.EvidenceFailing,
			},
			{
				name:       "malformed_no_head",
				record:     core.EvidenceRecord{TaskID: "T1", ExitCode: 0, GitHead: ""},
				wantStatus: core.EvidenceMalformed,
			},
			{
				name:       "malformed_unknown_head",
				record:     core.EvidenceRecord{TaskID: "T1", ExitCode: 0, GitHead: "unknown"},
				wantStatus: core.EvidenceMalformed,
			},
			{
				name:       "invalid_zero_test",
				record:     core.EvidenceRecord{TaskID: "T1", ExitCode: 0, GitHead: head, ZeroTestDetected: true},
				wantStatus: core.EvidenceInvalid,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				status := core.EvidenceStatus(tt.record)
				if status != tt.wantStatus {
					t.Fatalf("EvidenceStatus = %v, want %v", status, tt.wantStatus)
				}
			})
		}
	})

	// R3.3: Multiple attempts bind correctly
	t.Run("R3.3_MultipleAttemptsBindCorrectly", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, ".specd", "specs", "demo", "evidence.jsonl")

		// Initial attempt: create attempt 1 evidence
		rec1 := core.EvidenceRecord{
			TaskID:       "T1",
			ExitCode:     1,
			GitHead:      "abc",
			Attempt:      1,
			PlanRevision: 0,
		}
		if err := core.AppendEvidence(path, rec1); err != nil {
			t.Fatal(err)
		}

		// Load evidence - should have the attempt 1 record
		evidence, err := core.LoadEvidence(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(evidence) != 1 {
			t.Fatalf("expected 1 evidence record for attempt 1, got %d", len(evidence))
		}
		rec, ok := evidence["T1"]
		if !ok {
			t.Fatal("T1 evidence not found")
		}
		if rec.Attempt != 1 {
			t.Fatalf("expected attempt 1, got %d", rec.Attempt)
		}

		// Test that attempt 2 evidence is separate from attempt 1
		rec2 := core.EvidenceRecord{
			TaskID:       "T1",
			ExitCode:     0,
			GitHead:      "abc",
			Attempt:      2,
			PlanRevision: 1,
		}
		if err := core.AppendEvidence(path, rec2); err != nil {
			t.Fatal(err)
		}

		// Evidence records should maintain attempt binding
		allRecords, err := core.LoadEvidenceRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(allRecords) != 2 {
			t.Fatalf("expected 2 evidence records total, got %d", len(allRecords))
		}
	})

	// R3.1: Zero-test evidence blocks completion
	// R3.1: a Go selector that no package executed is invalid, whatever the
	// module host. A multi-package run stays valid when any package executed.
	t.Run("R3.1_ZeroTestSelectorDetection", func(t *testing.T) {
		const cmd = "go test ./a ./b -run TestX"
		for _, tt := range []struct {
			name    string
			command string
			stdout  string
			want    bool
		}{
			{"single_package_no_tests", cmd, "ok  \texample.test/a\t0.001s [no tests to run]\n", true},
			{"non_github_host_no_tests", cmd, "ok  \tgit.example.org/x/a\t0.001s [no tests to run]\n", true},
			{"no_test_files_only", cmd, "?   \texample.test/a\t[no test files]\n", true},
			{"one_package_executed", cmd, "ok  \texample.test/a\t0.010s\nok  \texample.test/b\t0.001s [no tests to run]\n", false},
			{"all_packages_executed", cmd, "ok  \texample.test/a\t0.010s\n", false},
			{"not_a_selector_command", "go test ./...", "ok  \texample.test/a\t0.001s [no tests to run]\n", false},
			{"non_go_command", "printf ok", "ok", false},
		} {
			t.Run(tt.name, func(t *testing.T) {
				if got := isZeroTestGoSelector(tt.command, verifyexec.Result{Stdout: tt.stdout}); got != tt.want {
					t.Fatalf("isZeroTestGoSelector = %v, want %v for %q", got, tt.want, tt.stdout)
				}
			})
		}
	})

	t.Run("R3.1_InvalidEvidenceBlocksCompletion", func(t *testing.T) {
		root := t.TempDir()
		runGit(t, root, "init")
		runGit(t, root, "config", "user.email", "specd@example.test")
		runGit(t, root, "config", "user.name", "specd")
		runGit(t, root, "commit", "--allow-empty", "-m", "base")

		specDir := filepath.Join(root, ".specd", "specs", "demo")
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			t.Fatal(err)
		}

		tasks := "| id | role | files | depends-on | verify | acceptance |\n" +
			"|---|---|---|---|---|---|\n" +
			"| ⬜ T1 | craftsman | x | - | go test ./... | ok |\n"
		if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create invalid zero-test evidence
		head := "abc123"
		evidence := map[string]core.EvidenceRecord{
			"T1": {
				TaskID:           "T1",
				ExitCode:         0,
				GitHead:          head,
				ZeroTestDetected: true,
			},
		}

		// Completion should be blocked on invalid evidence
		_, err := core.CompleteTask([]byte(tasks), "T1", evidence)
		if err == nil {
			t.Fatal("completion should be blocked on invalid zero-test evidence")
		}
		if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "zero test") {
			t.Fatalf("expected 'invalid' or 'zero test' in error, got: %v", err)
		}
	})

	// R3.4: Read-only tasks keep trivial verify without weakening write-task verification
	t.Run("R3.4_ReadOnlyVsWriteTaskVerification", func(t *testing.T) {
		// Both scout (read-only) and craftsman (write) can complete with passing evidence
		head := "abc123"
		evidence := map[string]core.EvidenceRecord{
			"T1": {
				TaskID:   "T1",
				ExitCode: 0,
				GitHead:  head,
			},
		}

		// Scout task completes
		scoutTasks := "| id | role | files | depends-on | verify | acceptance |\n" +
			"|---|---|---|---|---|---|\n" +
			"| ⬜ T1 | scout | x | - | printf ok | ok |\n"
		_, err := core.CompleteTask([]byte(scoutTasks), "T1", evidence)
		if err != nil {
			t.Fatalf("scout completion failed: %v", err)
		}

		// Craftsman task also completes with same evidence
		craftsmanTasks := "| id | role | files | depends-on | verify | acceptance |\n" +
			"|---|---|---|---|---|---|\n" +
			"| ⬜ T1 | craftsman | x | - | printf ok | ok |\n"
		_, err = core.CompleteTask([]byte(craftsmanTasks), "T1", evidence)
		if err != nil {
			t.Fatalf("craftsman completion failed: %v", err)
		}

		// But both fail with invalid evidence (R3.1 - no bypass)
		invalidEvidence := map[string]core.EvidenceRecord{
			"T1": {
				TaskID:           "T1",
				ExitCode:         0,
				GitHead:          head,
				ZeroTestDetected: true,
			},
		}
		_, err = core.CompleteTask([]byte(scoutTasks), "T1", invalidEvidence)
		if err == nil {
			t.Fatal("scout should reject invalid zero-test evidence")
		}
		_, err = core.CompleteTask([]byte(craftsmanTasks), "T1", invalidEvidence)
		if err == nil {
			t.Fatal("craftsman should reject invalid zero-test evidence")
		}
	})
}
