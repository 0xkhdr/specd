package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Crash-consistency stress for RecordCheckpoint (spec A3).
//
// RecordCheckpoint's safety argument is: writes are atomic renames, so a host
// killed mid-checkpoint can only ever land on a step boundary — never a torn
// file. These tests prove that argument under an emulated SIGKILL by re-execing
// the test binary as a child that claims a lease, then hard-exits (os.Exit, no
// deferred flush) at an injected point inside RecordCheckpoint. The parent then
// inspects the on-disk lease/checkpoint state and asserts the two invariants
// that matter: no double-claim (never two owners of one attempt) and no orphaned
// lease (a survivor lease must still expire, never strand resume forever).

const (
	faultChildEnv   = "SPECD_FAULT_CHILD"
	faultRootEnv    = "SPECD_FAULT_ROOT"
	faultSessionHex = "77777777777777777777777777777777" // 32 × '7'
)

// seedFaultCheckpointSpec writes the minimal executing-phase spec scaffold the
// crash child needs, at an explicit root shared with the parent. It mirrors the
// writePinkySpec test helper but takes a caller-owned root (t.TempDir cannot be
// shared across processes) and reports errors instead of failing a *testing.T,
// so the same code runs in both the parent and the re-exec'd child.
func seedFaultCheckpointSpec(root string) error {
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return err
	}
	state := InitialState("demo", "Demo")
	state.Status = StatusExecuting
	state.Phase = PhaseExecute
	state.Tasks["T1"] = TaskState{
		ID:           "T1",
		Title:        "Demo task",
		Role:         "craftsman",
		Wave:         1,
		Depends:      []string{},
		Requirements: []int{1},
		Status:       TaskPending,
	}
	if err := SaveState(root, "demo", &state); err != nil {
		return err
	}
	tasks := `# Tasks — Demo

## Wave 1

- [ ] T1 — Demo task
  - why: Needed.
  - role: craftsman
  - files: internal/core/demo.go
  - contract: Change one file.
  - acceptance: Works.
  - verify: go test ./internal/core
  - depends: —
  - requirements: 1
`
	if err := AtomicWrite(filepath.Join(specDir, "tasks.md"), tasks); err != nil {
		return err
	}
	for _, name := range []string{"requirements.md", "design.md", "decisions.md", "memory.md", "mid-requirements.md"} {
		if err := AtomicWrite(filepath.Join(specDir, name), "\n"); err != nil {
			return err
		}
	}
	return nil
}

// runCheckpointFaultChild is the body executed in the re-exec'd child process:
// it claims the lease in the shared root, then calls RecordCheckpoint, which
// hard-exits at the SPECD_FAULT_CHECKPOINT point. If RecordCheckpoint returns
// (no fault fired), the child exits 0 — the parent treats that as "no crash
// injected" and skips the killed-state assertions.
func runCheckpointFaultChild() {
	root := os.Getenv(faultRootEnv)
	cfg := DefaultConfig.Orchestration
	mission, err := BuildPinkyMission(root, "demo", faultSessionHex, "pinky-a", "T1", 1, cfg)
	if err != nil {
		os.Stderr.WriteString("build mission: " + err.Error() + "\n")
		os.Exit(2)
	}
	if _, err := ClaimPinkyMission(root, mission, cfg); err != nil {
		os.Stderr.WriteString("claim mission: " + err.Error() + "\n")
		os.Exit(2)
	}
	rec := CheckpointRecord{
		Version:         OrchestrationModelVersion,
		SessionID:       faultSessionHex,
		Spec:            "demo",
		TaskID:          "T1",
		Attempt:         1,
		WorkerID:        "pinky-a",
		ProgressPercent: 70,
		ContextManifest: "manifest",
		WorkingNotes:    "wrote the parser, tests pending",
		ChangedFiles:    []string{"internal/core/demo.go"},
		GitHead:         strings.Repeat("a", 40),
		Reason:          "host /clear",
	}
	if _, err := RecordCheckpoint(root, rec, cfg); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestCheckpointFaultInjection(t *testing.T) {
	if os.Getenv(faultChildEnv) == "1" {
		runCheckpointFaultChild()
		return
	}

	// Each point names where inside RecordCheckpoint the host is killed.
	// leaseSurvives is true while the lease has not yet been cleared on disk.
	cases := []struct {
		point         string
		leaseSurvives bool
	}{
		{"after-write", true},
		{"after-event", true},
		{"after-lease-clear", false},
	}

	for _, tc := range cases {
		t.Run(tc.point, func(t *testing.T) {
			root := t.TempDir()
			if err := seedFaultCheckpointSpec(root); err != nil {
				t.Fatalf("seed spec: %v", err)
			}

			cmd := exec.Command(os.Args[0], "-test.run", "^TestCheckpointFaultInjection$", "-test.v")
			cmd.Env = append(os.Environ(),
				faultChildEnv+"=1",
				faultRootEnv+"="+root,
				"SPECD_FAULT_CHECKPOINT="+tc.point,
			)
			var childErr strings.Builder
			cmd.Stderr = &childErr
			err := cmd.Run()
			ee, ok := err.(*exec.ExitError)
			if !ok || ee.ExitCode() != 137 {
				t.Fatalf("child should have been killed at %q (exit 137), got err=%v; child stderr:\n%s", tc.point, err, childErr.String())
			}

			store, err := NewACPStore(root)
			if err != nil {
				t.Fatal(err)
			}
			leases, err := store.loadSessionLeases(faultSessionHex)
			if err != nil {
				t.Fatalf("loadSessionLeases: %v", err)
			}

			// Invariant 1 — no double-claim: at most one lease may exist for the
			// (spec, task, attempt) tuple regardless of where the crash landed.
			owners := 0
			var survivor ACPLease
			for _, l := range leases {
				if l.Spec == "demo" && l.Task == "T1" && l.Attempt == 1 {
					owners++
					survivor = l
				}
			}
			if owners > 1 {
				t.Fatalf("double-claim after crash at %q: %d owners of demo/T1/1", tc.point, owners)
			}

			if tc.leaseSurvives {
				if owners != 1 {
					t.Fatalf("crash at %q before lease-clear should leave the lease held, got %d owners", tc.point, owners)
				}
				// Invariant 2 — no orphaned lease: a surviving lease must still
				// expire, so resume reclaims it rather than blocking forever.
				expiry, err := time.Parse(time.RFC3339Nano, survivor.LeaseUntil)
				if err != nil {
					t.Fatalf("survivor lease has unparseable LeaseUntil %q: %v", survivor.LeaseUntil, err)
				}
				if leaseIsActive(survivor, expiry.Add(time.Second)) {
					t.Fatalf("orphaned lease at %q: still active past its LeaseUntil %s", tc.point, survivor.LeaseUntil)
				}
			} else if owners != 0 {
				t.Fatalf("crash at %q (after lease-clear) must leave no lease, got %d", tc.point, owners)
			}

			// The checkpoint record is written before the lease is touched, so it
			// must exist on disk at every injected point — work is never lost.
			paths, err := NewACPRuntimePaths(root)
			if err != nil {
				t.Fatal(err)
			}
			cp, err := paths.CheckpointPath(faultSessionHex, "T1", 1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(cp); err != nil {
				t.Fatalf("checkpoint record missing after crash at %q: %v", tc.point, err)
			}
		})
	}
}
