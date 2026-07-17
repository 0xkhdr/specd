package cmd

import (
	"github.com/0xkhdr/specd/internal/core"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrainFakeHostLifecycleE2E(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	tasks := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"
	if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInitRepo(t, root)
	execGit(t, root, "add", ".")
	execGit(t, root, "commit", "-m", "fixture")
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"claim", "demo", "demo.s1.T1", "worker-1", "craftsman"}, nil); err != nil {
		t.Fatal(err)
	}
	s := loadBrainSession(t, root)
	leaseID := s.Leases[0].LeaseID
	if err := runBrain(root, []string{"heartbeat", "demo", leaseID, "worker-1"}, nil); err != nil {
		t.Fatal(err)
	}
	head := gitHead(root)
	if err := core.AppendEvidence(core.EvidencePath(root, "demo"), core.EvidenceRecord{TaskID: "T1", Command: "printf ok", ExitCode: 0, GitHead: head}); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"report", "demo", leaseID, "worker-1"}, nil); err != nil {
		t.Fatal(err)
	}
	// The Brain report shares the manual verify path's run allocator, so the
	// completed attempt lands on the task's run chain (spec 07 R2.2).
	runs, err := core.ReadRuns(core.RunLedgerPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].TaskID != "T1" || runs[0].Attempt != 1 || runs[0].WorkerID != "worker-1" {
		t.Fatalf("brain report did not allocate a run: %+v", runs)
	}
	if err := runBrain(root, []string{"report", "demo", leaseID, "worker-1"}, nil); err == nil {
		t.Fatal("duplicate completion report accepted")
	}
}

func TestBrainReportProductionScopeRejectsUndeclared(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", "orchestration:\n  enabled: true\nprofile: production\n")
	tasks := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"
	os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644)
	os.WriteFile(filepath.Join(root, "a.go"), []byte("a"), 0o644)
	gitInitRepo(t, root)
	execGit(t, root, "add", ".")
	execGit(t, root, "commit", "-m", "tracked")
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "outside.go"), []byte("x"), 0o644)
	if err := core.AppendEvidence(core.EvidencePath(root, "demo"), core.EvidenceRecord{TaskID: "T1", Command: "printf ok", ExitCode: 0, GitHead: gitHead(root)}); err != nil {
		t.Fatal(err)
	}
	err := runTaskComplete(root, []string{"demo", "T1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "outside_scope") {
		t.Fatalf("err=%v", err)
	}
}

func execGit(t *testing.T, root string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}
