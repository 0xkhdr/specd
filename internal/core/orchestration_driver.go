package core

import (
	"errors"
	"fmt"
	"time"
)

// Reference driver loop (GAP-2, GAP-11).
//
// A single `StepOrchestration` is one bounded decision + at most one action.
// DriveOrchestration is the shipped, testable outer loop that ties steps to
// worker spawns. It realizes the concurrency model: each dispatch's worker is
// spawned in a goroutine, and the loop keeps stepping while fewer than
// `policy.MaxWorkers` leases are held and the runnable frontier still yields
// unleased work. It blocks only when the worker pool is saturated or the
// frontier is empty, reaping on worker-report (or lease-expiry, sensed on the
// next step) before stepping again — until the spec reaches a terminal
// decision.
//
// The lease is the only in-snapshot marker of in-flight work (a claimed task
// keeps Status pending), so the loop avoids double-dispatching a task whose
// worker has not yet claimed its lease by tracking in-flight dispatch keys and
// briefly waiting for the lease to become observable before stepping again.
//
// The loop is orchestration glue, not core authority: it contains zero LLM
// calls and zero creative work. All authoring/execution happens inside the
// host's Worker callback. Concurrency lives here, never in DecideOrchestration,
// which stays one-bounded-decision-per-step and is concurrency-correct via the
// lease snapshot. The boundary keeps `internal/core` deterministic while making
// the harness loop a first-class asset instead of folklore.

// DriverDispatch is the unit handed to a worker: the dispatching decision and
// the fully built, claimable mission.
type DriverDispatch struct {
	Decision OrchestrationDecision
	Mission  PinkyMission
}

// DriverOptions configures a drive. Worker is the host callback that runs one
// mission to a reported terminal state; a nil Worker stops the loop at the first
// dispatch (host-wires-its-own-worker mode). MaxSteps bounds total iterations;
// MaxWaits bounds consecutive non-progress waits before the loop reports a stall
// (fail-closed rather than spin forever).
type DriverOptions struct {
	MaxSteps int
	MaxWaits int
	Worker   func(DriverDispatch) error
}

// DriverOutcome is the terminal reason a drive stopped.
type DriverOutcome string

const (
	DriverComplete   DriverOutcome = "complete"          // spec/session reached a terminal done state
	DriverEscalated  DriverOutcome = "escalated"         // a decision escalated to a human
	DriverAwaiting   DriverOutcome = "awaiting-approval" // a gate needs human approval
	DriverWorkerStop DriverOutcome = "worker-stop"       // dispatch with no Worker callback wired
	DriverMaxSteps   DriverOutcome = "max-steps"         // step budget exhausted
	DriverStalled    DriverOutcome = "stalled"           // too many consecutive waits — no progress
)

// DriverResult reports how a drive ended.
type DriverResult struct {
	Steps   int                   `json:"steps"`
	Outcome DriverOutcome         `json:"outcome"`
	Final   OrchestrationDecision `json:"final"`
}

// workerReport is the outcome of one spawned worker.
type workerReport struct {
	dispatch DriverDispatch
	err      error
}

// DriveOrchestration runs the reference loop against an already-started session.
func DriveOrchestration(root, slug, sessionID string, policy OrchestrationPolicy, cfg OrchestrationCfg, opts DriverOptions) (DriverResult, error) {
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = 100
	}
	if opts.MaxWaits <= 0 {
		opts.MaxWaits = 8
	}
	maxWorkers := policy.MaxWorkers
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	pollInterval := time.Duration(cfg.Transport.PollIntervalMillis) * time.Millisecond
	if pollInterval <= 0 {
		pollInterval = 50 * time.Millisecond
	}

	reports := make(chan workerReport, maxWorkers)
	inflight := 0
	inflightKeys := map[string]bool{}

	spawn := func(d DriverDispatch) {
		inflight++
		inflightKeys[d.Decision.IdempotencyKey] = true
		go func() { reports <- workerReport{dispatch: d, err: opts.Worker(d)} }()
	}
	// reap consumes one report, accounting for the freed slot. A failed worker is
	// turned into a retryable failure (lease released, task marked) so the next
	// step applies the retry/escalate policy already in DecideOrchestration.
	reap := func(r workerReport) error {
		inflight--
		delete(inflightKeys, r.dispatch.Decision.IdempotencyKey)
		if r.err != nil {
			return failWorkerDispatch(root, slug, sessionID, r.dispatch, cfg)
		}
		return nil
	}
	// drainReports collects every still-running worker before a terminal return so
	// no goroutine outlives the drive. It is best-effort: a worker error during
	// drain is folded into the returned error but never blocks the others.
	drainReports := func(firstErr error) error {
		for inflight > 0 {
			if err := reap(<-reports); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	waits := 0
	var last OrchestrationDecision
	for step := 1; step <= opts.MaxSteps; step++ {
		// Reap any finished workers (non-blocking) so the next snapshot reflects
		// their releases/failures/completions before we decide.
		for drained := true; drained; {
			select {
			case r := <-reports:
				if err := reap(r); err != nil {
					return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
				}
			default:
				drained = false
			}
		}

		res, err := StepOrchestration(root, slug, sessionID, policy, cfg)
		if err != nil {
			return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
		}
		last = res.Decision
		switch res.Decision.Action {
		case OrchestrationDispatch, OrchestrationDispatchAuthor:
			if opts.Worker == nil {
				return DriverResult{Steps: step, Outcome: DriverWorkerStop, Final: last}, drainReports(nil)
			}
			key := res.Decision.IdempotencyKey
			if inflightKeys[key] || inflight >= maxWorkers {
				// Either this exact dispatch is already in flight but its lease is
				// not yet observable, or the pool is full though the engine still
				// offered work (an in-flight slot's lease is not yet visible).
				// Spawning would double-dispatch / over-subscribe, so wait for a
				// worker to report or the lease to surface, then re-step. This wait
				// is not productive progress, so it does not consume a step.
				if inflight == 0 {
					return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, nil
				}
				if err := awaitReport(reports, reap, pollInterval); err != nil {
					return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
				}
				step--
				continue
			}
			mission, err := driverMissionFor(root, slug, sessionID, res.Decision, cfg)
			if err != nil {
				return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
			}
			spawn(DriverDispatch{Decision: res.Decision, Mission: mission})
			waits = 0
		case OrchestrationAdvancePhase:
			waits = 0 // real progress: a phase advanced
		case OrchestrationCompleteSession, OrchestrationIdle:
			return DriverResult{Steps: step, Outcome: DriverComplete, Final: last}, drainReports(nil)
		case OrchestrationRequestApproval:
			return DriverResult{Steps: step, Outcome: DriverAwaiting, Final: last}, drainReports(nil)
		case OrchestrationEscalate:
			return DriverResult{Steps: step, Outcome: DriverEscalated, Final: last}, drainReports(nil)
		case OrchestrationWait, OrchestrationCancel, OrchestrationRetry, OrchestrationReplan:
			if inflight > 0 {
				// Workers are running and the engine has no new bounded action: the
				// only thing that can change state is a worker report, so block for
				// one (guaranteed progress, naturally bounded) then re-step.
				if err := reap(<-reports); err != nil {
					return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
				}
				waits = 0
				continue
			}
			waits++
			if waits >= opts.MaxWaits {
				return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, nil
			}
		}
	}
	return DriverResult{Steps: opts.MaxSteps, Outcome: DriverMaxSteps, Final: last}, drainReports(nil)
}

// awaitReport blocks until a worker reports (then reaps it) or the poll interval
// elapses (letting a freshly claimed lease become observable on the next step).
// It is the driver loops' single blocking point, shared by the single-spec and
// program drives.
func awaitReport[T any](reports <-chan T, reap func(T) error, pollInterval time.Duration) error {
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()
	select {
	case r := <-reports:
		return reap(r)
	case <-timer.C:
		return nil
	}
}

// failWorkerDispatch turns a host-side worker failure into a retryable failure
// the deterministic core already knows how to handle. It releases the dispatch's
// lease (a worker self-claims, so on timeout/crash we free it; a missing lease
// is fine — the worker may have died before claiming) and, for an execution
// task, records a non-verified verification record plus a retry bump so the next
// step retries or escalates per policy. An authoring dispatch has no task to
// fail; freeing its lease lets the planning frontier re-dispatch it.
func failWorkerDispatch(root, slug, sessionID string, d DriverDispatch, cfg OrchestrationCfg) error {
	store, err := NewACPStore(root)
	if err != nil {
		return err
	}
	workerID := orchestrationWorkerID(d.Decision)
	if err := store.ReleaseLease(sessionID, workerID, d.Decision.Attempt); err != nil && !errors.Is(err, errACPLeaseNotFound) {
		return err
	}
	if d.Decision.Action == OrchestrationDispatchAuthor {
		return nil
	}
	return recordWorkerFailure(root, slug, d.Decision.TaskID, d.Mission.VerifyCommand)
}

// recordWorkerFailure marks an execution task as having failed verification
// (Verified=false) and increments its retry count, under the spec lock. A
// claimed task keeps Status pending, so this is enough for SenseOrchestration to
// surface it as a retryable failure on the next step; a later successful attempt
// overwrites the record with a passing one through the normal evidence path.
func recordWorkerFailure(root, slug, taskID, verifyCommand string) error {
	_, err := WithSpecLock[struct{}](root, slug, func() (struct{}, error) {
		state, err := LoadState(root, slug)
		if err != nil {
			return struct{}{}, err
		}
		ts, ok := state.Tasks[taskID]
		if !ok {
			return struct{}{}, fmt.Errorf("driver: unknown task %s", taskID)
		}
		if ts.Telemetry == nil {
			ts.Telemetry = &Telemetry{}
		}
		ts.Telemetry.Retries++
		ts.Verification = &VerificationRecord{
			Command:  verifyCommand,
			ExitCode: 124, // conventional timeout/abort exit code
			Verified: false,
			TimedOut: true,
			RanAt:    Clock().UTC().Format(time.RFC3339Nano),
		}
		state.Tasks[taskID] = ts
		return struct{}{}, SaveState(root, slug, state)
	})
	return err
}

// Program-scoped reference driver loop (GAP-7).
//
// DriveOrchestration drives a single spec. DriveProgramOrchestration is its
// program-level peer: it loops StepProgramOrchestration, and whenever a child
// step dispatches a mission it hands that mission to the same kind of host
// worker callback, blocks until the worker returns, then steps again — until the
// program session reaches a terminal decision (complete | escalate).
//
// The per-spec frontier already re-resolves on every program step
// (StepProgramOrchestration releases complete children and re-runs DecideProgram
// before dispatching), so the loop advances to the next spec automatically on
// child completion with no external nudge. This loop proves that cross-spec walk
// end-to-end while keeping the no-LLM-in-core boundary: all creative work stays
// in the host Worker callback.

// ProgramDriverDispatch is the unit handed to a worker during a program drive:
// the child spec/session that produced the dispatch plus the dispatching
// decision and its claimable mission.
type ProgramDriverDispatch struct {
	Slug           string
	ChildSessionID string
	Dispatch       DriverDispatch
}

// ProgramDriverOptions configures a program drive. Worker is the host callback
// that runs one child mission to a reported terminal state; a nil Worker stops
// the loop at the first child dispatch. MaxSteps bounds total program steps;
// MaxWaits bounds consecutive non-progress steps before reporting a stall.
type ProgramDriverOptions struct {
	MaxSteps int
	MaxWaits int
	Worker   func(ProgramDriverDispatch) error
}

// ProgramDriverResult reports how a program drive ended.
type ProgramDriverResult struct {
	Steps   int             `json:"steps"`
	Outcome DriverOutcome   `json:"outcome"`
	Final   ProgramDecision `json:"final"`
}

// programWorkerReport is the outcome of one spawned program-child worker.
type programWorkerReport struct {
	dispatch ProgramDriverDispatch
	err      error
}

// DriveProgramOrchestration runs the reference loop against an already-resolvable
// program (parentSessionID is created on first step if absent). Like the
// single-spec drive it is asynchronous (GAP-11): each child dispatch's worker is
// spawned in a goroutine so multiple specs and multiple workers run at once,
// realizing `max_concurrent_specs`. Per-child worker concurrency is bounded by
// each child's own `MaxWorkers` lease cap, and how many specs step concurrently
// is bounded by `max_concurrent_specs` inside StepProgramOrchestration.
func DriveProgramOrchestration(root, parentSessionID string, policy OrchestrationPolicy, cfg OrchestrationCfg, opts ProgramDriverOptions) (ProgramDriverResult, error) {
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = 200
	}
	if opts.MaxWaits <= 0 {
		opts.MaxWaits = 8
	}

	reports := make(chan programWorkerReport, 64)
	inflight := 0
	inflightKeys := map[string]bool{}

	spawn := func(d ProgramDriverDispatch) {
		inflight++
		inflightKeys[d.Dispatch.Decision.IdempotencyKey] = true
		go func() { reports <- programWorkerReport{dispatch: d, err: opts.Worker(d)} }()
	}
	reap := func(r programWorkerReport) error {
		inflight--
		delete(inflightKeys, r.dispatch.Dispatch.Decision.IdempotencyKey)
		if r.err != nil {
			return failWorkerDispatch(root, r.dispatch.Slug, r.dispatch.ChildSessionID, r.dispatch.Dispatch, cfg)
		}
		return nil
	}
	drainReports := func(firstErr error) error {
		for inflight > 0 {
			if err := reap(<-reports); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	waits := 0
	var last ProgramDecision
	for step := 1; step <= opts.MaxSteps; step++ {
		for drained := true; drained; {
			select {
			case r := <-reports:
				if err := reap(r); err != nil {
					return ProgramDriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
				}
			default:
				drained = false
			}
		}

		res, err := StepProgramOrchestration(root, parentSessionID, policy, cfg)
		if err != nil {
			return ProgramDriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
		}
		last = res.Decision
		switch res.Decision.Action {
		case ProgramDecisionComplete:
			return ProgramDriverResult{Steps: step, Outcome: DriverComplete, Final: last}, drainReports(nil)
		case ProgramDecisionEscalate:
			return ProgramDriverResult{Steps: step, Outcome: DriverEscalated, Final: last}, drainReports(nil)
		}

		// Spawn each child dispatch whose worker is not already in flight; track
		// whether the step made real progress so the loop stall-closes instead of
		// spinning forever.
		progressed := len(res.Started) > 0
		terminal := DriverResult{}
		terminalSet := false
		for _, child := range res.Stepped {
			switch child.Result.Decision.Action {
			case OrchestrationDispatch, OrchestrationDispatchAuthor:
				if opts.Worker == nil {
					return ProgramDriverResult{Steps: step, Outcome: DriverWorkerStop, Final: last}, drainReports(nil)
				}
				key := child.Result.Decision.IdempotencyKey
				if inflightKeys[key] {
					// Already in flight, lease not yet observable — do not re-spawn.
					continue
				}
				mission, err := driverMissionFor(root, child.Slug, child.SessionID, child.Result.Decision, cfg)
				if err != nil {
					return ProgramDriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
				}
				spawn(ProgramDriverDispatch{
					Slug:           child.Slug,
					ChildSessionID: child.SessionID,
					Dispatch:       DriverDispatch{Decision: child.Result.Decision, Mission: mission},
				})
				progressed = true
			case OrchestrationAdvancePhase, OrchestrationCompleteSession, OrchestrationIdle:
				progressed = true
			case OrchestrationEscalate:
				terminal, terminalSet = DriverResult{Outcome: DriverEscalated}, true
			case OrchestrationRequestApproval:
				terminal, terminalSet = DriverResult{Outcome: DriverAwaiting}, true
			}
		}
		if terminalSet {
			return ProgramDriverResult{Steps: step, Outcome: terminal.Outcome, Final: last}, drainReports(nil)
		}
		if progressed {
			waits = 0
			continue
		}
		if inflight > 0 {
			// No child progressed this step but workers are running; their reports
			// are the only state change, so block for one then re-step.
			if err := reap(<-reports); err != nil {
				return ProgramDriverResult{Steps: step, Outcome: DriverStalled, Final: last}, drainReports(err)
			}
			waits = 0
			continue
		}
		waits++
		if waits >= opts.MaxWaits {
			return ProgramDriverResult{Steps: step, Outcome: DriverStalled, Final: last}, nil
		}
	}
	return ProgramDriverResult{Steps: opts.MaxSteps, Outcome: DriverMaxSteps, Final: last}, drainReports(nil)
}

// driverMissionFor rebuilds the claimable mission for a dispatch decision. Build
// is deterministic, so the rebuilt mission matches the one the engine recorded
// (same dispatch digest) — the worker can claim it directly.
func driverMissionFor(root, slug, sessionID string, decision OrchestrationDecision, cfg OrchestrationCfg) (PinkyMission, error) {
	workerID := orchestrationWorkerID(decision)
	if decision.Action == OrchestrationDispatchAuthor {
		return BuildAuthoringMission(root, slug, sessionID, workerID, decision.Artifact, cfg)
	}
	return BuildPinkyMission(root, slug, sessionID, workerID, decision.TaskID, decision.Attempt, cfg)
}
