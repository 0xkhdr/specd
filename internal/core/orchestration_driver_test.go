package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const passingRequirements = `# Requirements — Auth

## Requirement 1 — Login
**User story:** As a user, I want to log in, so that I can access my account.

**Acceptance criteria:**
1. WHEN a user submits valid credentials the system SHALL grant access
`

const passingDesign = `# Design — Auth

## Overview
Auth feature overview.

## Architecture
Layered architecture.

## Components and interfaces
Handlers and stores.

## Data models
User and Session.

## Error handling
Errors are returned, not panicked.

## Verification strategy
go test ./...

## Risks and open questions
Token rotation is open.
`

const passingTasks = `# Tasks — Auth

## Wave 1
- [ ] T1 — Build login
  - why: Requirement 1 needs a handler
  - role: builder
  - files: auth.go
  - contract: Implement login
  - acceptance: returns 200
  - verify: go test ./...
  - depends: —
  - requirements: 1
`

// TestDriveOrchestrationBeginningToDelivery is the Milestone A golden test: a
// fresh spec at `requirements` with no planning artifacts is driven by the
// reference loop, under the planning policy, through authoring → advance →
// executing → verifying → complete, with a stub worker that does the creative
// work. It proves the dispatch→spawn handoff with zero model call in core.
func TestDriveOrchestrationBeginningToDelivery(t *testing.T) {
	root := t.TempDir()
	slug := "auth"
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	state := InitialState(slug, "Auth")
	state.Status = StatusRequirements
	state.Phase = PhaseForStatus(StatusRequirements)
	if err := SaveState(root, slug, &state); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig.Orchestration
	policy, err := NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	policy.ApprovalPolicy = "planning"
	sessionID := strings.Repeat("7", 32)
	if _, err := StartOrchestrationSession(root, slug, sessionID, "tester", policy); err != nil {
		t.Fatal(err)
	}

	authored := map[string]bool{}
	dispatched := []string{}
	worker := func(d DriverDispatch) error {
		dispatched = append(dispatched, string(d.Decision.Action)+":"+d.Decision.TaskID)
		switch d.Decision.Action {
		case OrchestrationDispatchAuthor:
			authored[d.Decision.Artifact] = true
			return os.WriteFile(filepath.Join(SpecDir(root, slug), d.Decision.Artifact), []byte(artifactFixture(d.Decision.Artifact)), 0o644)
		case OrchestrationDispatch:
			// Stand in for a real worker: complete the task with evidence and
			// move the spec to verifying once the frontier is empty.
			st, err := LoadState(root, slug)
			if err != nil {
				return err
			}
			ts := st.Tasks[d.Decision.TaskID]
			ts.Status = TaskComplete
			ts.Verification = &VerificationRecord{Verified: true, ExitCode: 0, Command: "go test ./..."}
			st.Tasks[d.Decision.TaskID] = ts
			st.Status = StatusVerifying
			st.Phase = PhaseForStatus(StatusVerifying)
			return SaveState(root, slug, st)
		}
		return nil
	}

	result, err := DriveOrchestration(root, slug, sessionID, policy, cfg, DriverOptions{MaxSteps: 40, Worker: worker})
	if err != nil {
		t.Fatalf("drive failed: %v (dispatched=%v)", err, dispatched)
	}
	if result.Outcome != DriverComplete {
		t.Fatalf("outcome = %s, want complete (dispatched=%v)", result.Outcome, dispatched)
	}
	for _, want := range []string{"requirements.md", "design.md", "tasks.md"} {
		if !authored[want] {
			t.Fatalf("planning artifact %s was never authored (dispatched=%v)", want, dispatched)
		}
	}
	// The execution task must have been handed to the worker too.
	sawExec := false
	for _, d := range dispatched {
		if strings.HasPrefix(d, "dispatch:T1") {
			sawExec = true
		}
	}
	if !sawExec {
		t.Fatalf("execution task never dispatched (dispatched=%v)", dispatched)
	}
}

func artifactFixture(artifact string) string {
	switch artifact {
	case "requirements.md":
		return passingRequirements
	case "design.md":
		return passingDesign
	case "tasks.md":
		return passingTasks
	}
	return ""
}

// waveTasksMD builds a tasks.md with n independent builder tasks in wave 1.
func waveTasksMD(n int) string {
	var b strings.Builder
	b.WriteString("# Tasks — Auth\n\n## Wave 1\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `- [ ] T%d — Build %d
  - why: Requirement 1 needs handler %d
  - role: builder
  - files: f%d.go
  - contract: Implement %d
  - acceptance: returns 200
  - verify: go test ./...
  - depends: —
  - requirements: 1
`, i, i, i, i, i)
	}
	return b.String()
}

// scaffoldExecutingSpec writes a spec already in `executing` with n runnable
// wave-1 tasks and an active session, ready for the driver to dispatch.
func scaffoldExecutingSpec(t *testing.T, root, slug string, n int, policy OrchestrationPolicy) string {
	t.Helper()
	dir := SpecDir(root, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "requirements.md"), passingRequirements)
	mustWrite(t, filepath.Join(dir, "design.md"), passingDesign)
	mustWrite(t, filepath.Join(dir, "tasks.md"), waveTasksMD(n))
	state := InitialState(slug, "Auth")
	state.Status = StatusExecuting
	state.Phase = PhaseForStatus(StatusExecuting)
	if err := SaveState(root, slug, &state); err != nil {
		t.Fatal(err)
	}
	// LoadSpec auto-reconciles tasks.md into state; persist the reconciled tasks.
	loaded, err := LoadSpec(root, slug)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, slug, loaded.State); err != nil {
		t.Fatal(err)
	}
	if len(loaded.State.Tasks) != n {
		t.Fatalf("scaffold: got %d tasks, want %d", len(loaded.State.Tasks), n)
	}
	sessionID := strings.Repeat("5", 32)
	if _, err := StartOrchestrationSession(root, slug, sessionID, "tester", policy); err != nil {
		t.Fatal(err)
	}
	return sessionID
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDriveOrchestrationConcurrentWave is the GAP-11 golden test: with
// MaxWorkers=N and N runnable tasks in a wave, the driver holds N leases
// simultaneously (proving real concurrency and that the
// ActiveLeases>=MaxWorkers→Wait branch is reachable), and the drive completes
// deterministically regardless of worker finish order.
func TestDriveOrchestrationConcurrentWave(t *testing.T) {
	const n = 4
	for iter := 0; iter < 5; iter++ {
		root := t.TempDir()
		slug := "auth"
		cfg := DefaultConfig.Orchestration
		policy, err := NewOrchestrationPolicy(cfg)
		if err != nil {
			t.Fatal(err)
		}
		policy.ApprovalPolicy = "planning"
		policy.MaxWorkers = n
		cfg.MaxWorkers = n
		sessionID := scaffoldExecutingSpec(t, root, slug, n, policy)

		leaseDur := time.Duration(cfg.Transport.LeaseSeconds) * time.Second
		var mu sync.Mutex
		var active, maxActive, arrived int
		proceed := make(chan struct{})
		var once sync.Once

		worker := func(d DriverDispatch) error {
			store, err := NewACPStore(root)
			if err != nil {
				return err
			}
			if _, err := store.ClaimLease(d.Mission.SessionID, d.Mission.WorkerID, d.Mission.Spec, d.Mission.TaskID, d.Decision.Attempt, leaseDur, Clock().UTC().Add(time.Hour)); err != nil {
				return fmt.Errorf("claim %s: %w", d.Decision.TaskID, err)
			}
			mu.Lock()
			active++
			arrived++
			if active > maxActive {
				maxActive = active
			}
			full := arrived == n
			mu.Unlock()
			if full {
				once.Do(func() { close(proceed) })
			}
			<-proceed // every worker blocks here until all n leases are held

			mu.Lock()
			st, err := LoadState(root, slug)
			if err != nil {
				mu.Unlock()
				return err
			}
			ts := st.Tasks[d.Decision.TaskID]
			ts.Status = TaskComplete
			ts.Verification = &VerificationRecord{Verified: true, ExitCode: 0, Command: "go test ./..."}
			st.Tasks[d.Decision.TaskID] = ts
			allDone := true
			for _, tk := range st.Tasks {
				if tk.Status != TaskComplete {
					allDone = false
					break
				}
			}
			if allDone {
				st.Status = StatusVerifying
				st.Phase = PhaseForStatus(StatusVerifying)
			}
			err = SaveState(root, slug, st)
			active--
			mu.Unlock()
			if err != nil {
				return err
			}
			return store.ReleaseLease(d.Mission.SessionID, d.Mission.WorkerID, d.Decision.Attempt)
		}

		result, err := DriveOrchestration(root, slug, sessionID, policy, cfg, DriverOptions{MaxSteps: 200, Worker: worker})
		if err != nil {
			t.Fatalf("iter %d: drive failed: %v", iter, err)
		}
		if result.Outcome != DriverComplete {
			t.Fatalf("iter %d: outcome = %s, want complete", iter, result.Outcome)
		}
		if maxActive != n {
			t.Fatalf("iter %d: max simultaneous leases = %d, want %d (concurrency not realized)", iter, maxActive, n)
		}
	}
}

// TestDriveOrchestrationWorkerFailureEscalates is the GAP-12 fail-closed test:
// a worker that always errors (as a host-side timeout would) is turned into a
// retryable failure each attempt; once retries are exhausted the drive escalates
// rather than hanging, and no lease is left active.
func TestDriveOrchestrationWorkerFailureEscalates(t *testing.T) {
	root := t.TempDir()
	slug := "auth"
	cfg := DefaultConfig.Orchestration
	policy, err := NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	policy.ApprovalPolicy = "planning"
	policy.MaxWorkers = 1
	policy.MaxRetries = 2
	cfg.MaxWorkers = 1
	cfg.MaxRetries = 2
	sessionID := scaffoldExecutingSpec(t, root, slug, 1, policy)

	attempts := 0
	worker := func(d DriverDispatch) error {
		attempts++
		// Claim then "time out": leave the lease for the driver to reclaim.
		store, err := NewACPStore(root)
		if err != nil {
			return err
		}
		leaseDur := time.Duration(cfg.Transport.LeaseSeconds) * time.Second
		if _, err := store.ClaimLease(d.Mission.SessionID, d.Mission.WorkerID, d.Mission.Spec, d.Mission.TaskID, d.Decision.Attempt, leaseDur, Clock().UTC().Add(time.Hour)); err != nil {
			return err
		}
		return fmt.Errorf("simulated worker timeout")
	}

	result, err := DriveOrchestration(root, slug, sessionID, policy, cfg, DriverOptions{MaxSteps: 200, Worker: worker})
	if err != nil {
		t.Fatalf("drive returned error: %v", err)
	}
	if result.Outcome != DriverEscalated {
		t.Fatalf("outcome = %s, want escalated", result.Outcome)
	}
	// DecideOrchestration escalates once the next attempt would exceed MaxRetries
	// (failure.Attempt == 1+Retries is the next attempt), so a task is dispatched
	// exactly MaxRetries times before escalation.
	if attempts != policy.MaxRetries {
		t.Fatalf("attempts = %d, want %d (MaxRetries)", attempts, policy.MaxRetries)
	}
	// No lease must remain active after escalation.
	snap, err := SenseOrchestration(root, slug, sessionID, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.ActiveLeases) != 0 {
		t.Fatalf("active leases after escalation = %d, want 0", len(snap.ActiveLeases))
	}
}

// scaffoldExecutingChildSpec writes a program child spec already in `executing`
// with one runnable wave-1 task, without starting an orchestration session (the
// program driver creates child sessions itself).
func scaffoldExecutingChildSpec(t *testing.T, root, slug string) {
	t.Helper()
	dir := SpecDir(root, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "requirements.md"), passingRequirements)
	mustWrite(t, filepath.Join(dir, "design.md"), passingDesign)
	mustWrite(t, filepath.Join(dir, "tasks.md"), waveTasksMD(1))
	state := InitialState(slug, slug)
	state.Status = StatusExecuting
	state.Phase = PhaseForStatus(StatusExecuting)
	if err := SaveState(root, slug, &state); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSpec(root, slug)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, slug, loaded.State); err != nil {
		t.Fatal(err)
	}
}

// TestDriveProgramOrchestrationConcurrentSpecs is the GAP-11 program-level golden
// test: two independent specs with MaxConcurrentSpecs=2 dispatch workers from
// both specs simultaneously (proving max_concurrent_specs is live), and the
// program drive completes.
func TestDriveProgramOrchestrationConcurrentSpecs(t *testing.T) {
	root := t.TempDir()
	specs := []string{"auth", "api"}
	for _, slug := range specs {
		scaffoldExecutingChildSpec(t, root, slug)
	}
	if err := SaveProgram(root, ProgramManifest{Version: ProgramVersion, DependsOn: map[string][]string{}}); err != nil {
		t.Fatal(err)
	}

	cfg, policy := programTestPolicy(t)
	policy.ApprovalPolicy = "planning"
	cfg.ApprovalPolicy = "planning"
	cfg.Program.MaxConcurrentSpecs = 2
	parentID := strings.Repeat("8", 32)

	leaseDur := time.Duration(cfg.Transport.LeaseSeconds) * time.Second
	var mu sync.Mutex
	var active, maxActive, arrived int
	proceed := make(chan struct{})
	var once sync.Once

	worker := func(d ProgramDriverDispatch) error {
		dec := d.Dispatch.Decision
		mission := d.Dispatch.Mission
		store, err := NewACPStore(root)
		if err != nil {
			return err
		}
		if _, err := store.ClaimLease(mission.SessionID, mission.WorkerID, mission.Spec, mission.TaskID, dec.Attempt, leaseDur, Clock().UTC().Add(time.Hour)); err != nil {
			return fmt.Errorf("claim %s/%s: %w", d.Slug, dec.TaskID, err)
		}
		mu.Lock()
		active++
		arrived++
		if active > maxActive {
			maxActive = active
		}
		full := arrived == len(specs)
		mu.Unlock()
		if full {
			once.Do(func() { close(proceed) })
		}
		<-proceed // both specs' workers are leased simultaneously here

		mu.Lock()
		st, err := LoadState(root, d.Slug)
		if err != nil {
			mu.Unlock()
			return err
		}
		ts := st.Tasks[dec.TaskID]
		ts.Status = TaskComplete
		ts.Verification = &VerificationRecord{Verified: true, ExitCode: 0, Command: "go test ./..."}
		st.Tasks[dec.TaskID] = ts
		st.Status = StatusComplete
		st.Phase = PhaseForStatus(StatusComplete)
		err = SaveState(root, d.Slug, st)
		active--
		mu.Unlock()
		if err != nil {
			return err
		}
		return store.ReleaseLease(mission.SessionID, mission.WorkerID, dec.Attempt)
	}

	result, err := DriveProgramOrchestration(root, parentID, policy, cfg, ProgramDriverOptions{MaxSteps: 400, Worker: worker})
	if err != nil {
		t.Fatalf("program drive failed: %v", err)
	}
	if result.Outcome != DriverComplete {
		t.Fatalf("outcome = %s, want complete", result.Outcome)
	}
	if maxActive != len(specs) {
		t.Fatalf("max simultaneous specs in flight = %d, want %d (program concurrency not realized)", maxActive, len(specs))
	}
}
