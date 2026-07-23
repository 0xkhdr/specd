package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestBrainStaleBaselineReissue(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	tasks := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"
	if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInitRepo(t, root)
	execGit(t, root, "add", ".")
	execGit(t, root, "commit", "-m", "baseline")
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"claim", "demo", "demo.s1.T1", "worker-1", "craftsman"}, nil); err != nil {
		t.Fatal(err)
	}
	old := loadBrainSession(t, root)
	oldMission, oldLease := old.Missions[0], old.Leases[0]
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	execGit(t, root, "add", "a.go")
	execGit(t, root, "commit", "-m", "head moved")
	currentHead := gitHead(root)

	err := runBrain(root, []string{"report", "demo", oldLease.LeaseID, "worker-1"}, nil)
	refusal, ok := core.AsRefusal(err)
	if !ok || refusal.Code != "BASELINE_DRIFTED" {
		t.Fatalf("stale report = %v, want BASELINE_DRIFTED", err)
	}
	if !strings.Contains(err.Error(), oldMission.SubjectHead) || !strings.Contains(err.Error(), currentHead) ||
		refusal.RecoveryCommand != "specd brain resume demo" {
		t.Fatalf("stale refusal lacks heads or deterministic route: %+v", refusal)
	}
	if err := runBrain(root, []string{"resume", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	got := loadBrainSession(t, root)
	if got.Leases[0].State != orchestration.LeaseRevoked || got.Leases[0].RevocationReason != "stale baseline" {
		t.Fatalf("old lease not revoked: %+v", got.Leases)
	}
	if len(got.PendingMissions) != 1 {
		t.Fatalf("pending missions = %+v", got.PendingMissions)
	}
	reissued := got.PendingMissions[0]
	if reissued.TaskID != oldMission.TaskID || reissued.SubjectHead != currentHead || reissued.Attempt != oldMission.Attempt+1 {
		t.Fatalf("reissued mission = %+v", reissued)
	}

	// Brain-only serial completion has no driver session. A marker that is
	// exactly reproducible from state is controller-owned and must not bleed
	// into the next mission's worker scope.
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	state.TaskStatus = map[string]core.TaskRunStatus{"T1": core.TaskComplete}
	if err := core.SaveStateCAS(core.StatePath(root, "demo"), state.Revision, state); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(root, ".specd/specs/demo/tasks.md")
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	marked, err := core.RewriteTaskStatusLine(raw, "T1", "✅")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tasksPath, marked, 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := loadSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	task, _ := findTaskRow(spec.Tasks, "T1")
	if err := enforceDiffScope(root, "demo", "T1", task); err != nil {
		t.Fatalf("Brain-owned marker bled into worker scope: %v", err)
	}
	if err := os.WriteFile(tasksPath, append(marked, []byte("\ndirect edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceDiffScope(root, "demo", "T1", task); err == nil {
		t.Fatal("direct tasks edit was hidden as a controller marker")
	}
}

// TestBrainResumeRaceDispatchesExactlyOnce scaffolds a crashed controller (a
// checkpoint whose mission never reached the ledger) and races N concurrent
// brainResume calls at it. The invariant (BD-01): the crashed mission is
// re-issued exactly once — the ledger carries exactly one dispatch of it — no
// matter how the resumes interleave. Deterministic under -race/-count.
func TestBrainResumeRaceDispatchesExactlyOnce(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(specDir, "session.json")
	checkpointPath := filepath.Join(specDir, "checkpoint.json")
	acpPath := filepath.Join(specDir, "acp.jsonl")

	// running session at revision 1; checkpoint step 1 whose mission is absent
	// from an empty ledger — the crashed-mid-dispatch state resume must reissue.
	writeFile(t, sessionPath, `{"id":"demo","revision":1,"state":"running","leases":[]}`)
	writeFile(t, checkpointPath, `{"session_id":"demo","step":1,"decision":"dispatch","mission_id":"demo.s1.T1","task_id":"T1","time":"2026-01-01T00:00:00Z"}`)
	writeFile(t, acpPath, "")

	const racers = 8
	var wg sync.WaitGroup
	wg.Add(racers)
	for i := 0; i < racers; i++ {
		go func() {
			defer wg.Done()
			// Losers fail closed (revision conflict / no reissue); only the
			// dispatch count matters, so errors are expected and ignored.
			_ = brainResume(root, sessionPath, checkpointPath, acpPath, "demo")
		}()
	}
	wg.Wait()

	events, err := orchestration.ReadACP(acpPath)
	if err != nil {
		t.Fatalf("read acp: %v", err)
	}
	dispatches := 0
	for _, e := range events {
		if e.Kind == orchestration.ACPKindDispatch && e.MissionID == "demo.s1.T1" {
			dispatches++
		}
	}
	if dispatches != 1 {
		t.Fatalf("expected exactly one dispatch of demo.s1.T1, got %d (events=%d)", dispatches, len(events))
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
