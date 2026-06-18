package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPinkyMissionDeterministic(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("4", 32)
	cfg := DefaultConfig.Orchestration
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	first, err := BuildPinkyMission(root, "demo", sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPinkyMission(root, "demo", sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.DispatchDigest != second.DispatchDigest || first.Deadline != second.Deadline {
		t.Fatalf("mission not deterministic:\n%#v\n%#v", first, second)
	}
	if first.Contract != "Change one file." || len(first.Files) != 1 || first.Files[0] != "internal/core/demo.go" {
		t.Fatalf("mission missing task contract: %#v", first)
	}
	if first.Authority.ReadOnly || strings.Join(first.Authority.AllowedActions, ",") != "read,edit,verify,report" {
		t.Fatalf("authority = %#v, want builder edit authority", first.Authority)
	}
}

func TestPinkyClaimHeartbeatRelease(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("5", 32)
	cfg := DefaultConfig.Orchestration
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	mission, err := BuildPinkyMission(root, "demo", sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimPinkyMission(root, mission, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimPinkyMission(root, mission, cfg); err == nil || !strings.Contains(err.Error(), "already owns active work") {
		t.Fatalf("duplicate claim error = %v, want active work", err)
	}
	now = now.Add(time.Second)
	if _, err := HeartbeatPinkyClaim(root, sessionID, "pinky-a", 1, cfg); err != nil {
		t.Fatal(err)
	}
	if err := ReleasePinkyClaim(root, sessionID, "pinky-a", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := HeartbeatPinkyClaim(root, sessionID, "pinky-a", 1, cfg); err == nil {
		t.Fatal("heartbeat after release succeeded")
	}
}

func writePinkySpec(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := InitialState("demo", "Demo")
	state.Status = StatusExecuting
	state.Phase = PhaseExecute
	state.Tasks["T1"] = TaskState{
		ID:           "T1",
		Title:        "Demo task",
		Role:         "builder",
		Wave:         1,
		Depends:      []string{},
		Requirements: []int{1},
		Status:       TaskPending,
	}
	if err := SaveState(root, "demo", &state); err != nil {
		t.Fatal(err)
	}
	tasks := `# Tasks — Demo

## Wave 1

- [ ] T1 — Demo task
  - why: Needed.
  - role: builder
  - files: internal/core/demo.go
  - contract: Change one file.
  - acceptance: Works.
  - verify: go test ./internal/core
  - depends: —
  - requirements: 1
`
	if err := AtomicWrite(filepath.Join(specDir, "tasks.md"), tasks); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"requirements.md", "design.md", "decisions.md", "memory.md", "mid-requirements.md"} {
		if err := AtomicWrite(filepath.Join(specDir, name), "\n"); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
