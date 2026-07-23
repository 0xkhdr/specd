package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestVerifyTimeout pins R6: the config→exec→evidence loop records a timed-out
// verify as a FAILING evidence record (exit 124), and the task never completes.
// Deterministic — the 1s bound fires well before the 5s command would finish.
func TestVerifyTimeout(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "seed.txt")
	runGit(t, root, "commit", "-m", "seed")

	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte("verify:\n  timeout_seconds: 1\n"), 0o644); err != nil {
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
	tasks := "| id | role | files | depends-on | verify | acceptance |\n" +
		"|---|---|---|---|---|---|\n" +
		"| ⬜ T1 | craftsman | seed.txt | - | exec sleep 5 | never completes |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runVerify(root, []string{"demo", "T1"}, map[string]string{}); err == nil {
		t.Fatal("runVerify succeeded, want timeout failure")
	}

	records, err := core.LoadEvidence(core.EvidencePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := records["T1"]
	if !ok {
		t.Fatal("no evidence recorded for T1 — config→exec→evidence loop broke")
	}
	if rec.ExitCode != 124 {
		t.Fatalf("recorded exit code = %d, want 124 (timeout as failing evidence)", rec.ExitCode)
	}

	// A non-zero record must not satisfy completion.
	if _, err := core.CompleteTask([]byte(tasks), "T1", records); err == nil {
		t.Fatal("timed-out task completed — evidence integrity violated")
	}
}
