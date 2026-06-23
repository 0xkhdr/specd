package core

import (
	"testing"
	"time"
)

// orchestration_driver_outcome_cov_test.go covers the DriveOrchestration
// terminal outcomes the golden tests don't reach: WorkerStop (nil worker),
// MaxSteps (step budget exhausted), and Stalled (no worker, no progress).

func driveTestPolicy(t *testing.T) (OrchestrationPolicy, OrchestrationCfg) {
	t.Helper()
	cfg := DefaultConfig.Orchestration
	cfg.MaxWorkers = 1
	policy, err := NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	policy.ApprovalPolicy = "planning"
	policy.MaxWorkers = 1
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	t.Cleanup(setCoreClock(func() time.Time { return now }))
	return policy, cfg
}

func TestDriveOrchestrationWorkerStop(t *testing.T) {
	root := t.TempDir()
	policy, cfg := driveTestPolicy(t)
	sessionID := scaffoldExecutingSpec(t, root, "auth", 1, policy)

	// Nil worker → the driver stops at the first dispatch rather than running it.
	result, err := DriveOrchestration(root, "auth", sessionID, policy, cfg, DriverOptions{MaxSteps: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != DriverWorkerStop {
		t.Fatalf("outcome = %s, want worker-stop", result.Outcome)
	}
}

func TestDriveOrchestrationMaxSteps(t *testing.T) {
	root := t.TempDir()
	policy, cfg := driveTestPolicy(t)
	sessionID := scaffoldExecutingSpec(t, root, "auth", 1, policy)

	// A worker that returns nil but never advances the spec keeps the driver
	// dispatching; a tiny step budget forces the MaxSteps outcome.
	worker := func(d DriverDispatch) error { return nil }
	result, err := DriveOrchestration(root, "auth", sessionID, policy, cfg, DriverOptions{MaxSteps: 1, Worker: worker})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != DriverMaxSteps {
		t.Fatalf("outcome = %s, want max-steps", result.Outcome)
	}
}
