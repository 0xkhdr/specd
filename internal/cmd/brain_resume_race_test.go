package cmd

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/0xkhdr/specd/internal/orchestration"
)

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
