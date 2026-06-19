package core

import "fmt"

// Reference driver loop (GAP-2).
//
// A single `StepOrchestration` is one bounded decision + at most one action.
// DriveOrchestration is the shipped, testable outer loop that ties steps to
// worker spawns: it steps, and when a step dispatches a mission it hands that
// mission to a host-provided worker callback, blocks until the worker returns
// (the dispatch→spawn contract: do not step that slot again until the worker
// reports or the lease expires), then steps again — until the spec reaches a
// terminal decision.
//
// The loop is orchestration glue, not core authority: it contains zero LLM
// calls and zero creative work. All authoring/execution happens inside the
// host's Worker callback. The boundary keeps `internal/core` deterministic
// while making the harness loop a first-class asset instead of folklore.

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

// DriveOrchestration runs the reference loop against an already-started session.
func DriveOrchestration(root, slug, sessionID string, policy OrchestrationPolicy, cfg OrchestrationCfg, opts DriverOptions) (DriverResult, error) {
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = 100
	}
	if opts.MaxWaits <= 0 {
		opts.MaxWaits = 8
	}
	waits := 0
	var last OrchestrationDecision
	for step := 1; step <= opts.MaxSteps; step++ {
		res, err := StepOrchestration(root, slug, sessionID, policy, cfg)
		if err != nil {
			return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, err
		}
		last = res.Decision
		switch res.Decision.Action {
		case OrchestrationDispatch, OrchestrationDispatchAuthor:
			if opts.Worker == nil {
				return DriverResult{Steps: step, Outcome: DriverWorkerStop, Final: last}, nil
			}
			mission, err := driverMissionFor(root, slug, sessionID, res.Decision, cfg)
			if err != nil {
				return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, err
			}
			// Dispatch→spawn contract: block on the worker; do not step this
			// slot again until it returns (reports) or its lease expires.
			if err := opts.Worker(DriverDispatch{Decision: res.Decision, Mission: mission}); err != nil {
				return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, fmt.Errorf("driver: worker failed for %s: %w", res.Decision.TaskID, err)
			}
			waits = 0
		case OrchestrationAdvancePhase:
			waits = 0 // real progress: a phase advanced
		case OrchestrationCompleteSession, OrchestrationIdle:
			return DriverResult{Steps: step, Outcome: DriverComplete, Final: last}, nil
		case OrchestrationRequestApproval:
			return DriverResult{Steps: step, Outcome: DriverAwaiting, Final: last}, nil
		case OrchestrationEscalate:
			return DriverResult{Steps: step, Outcome: DriverEscalated, Final: last}, nil
		case OrchestrationWait, OrchestrationCancel, OrchestrationRetry, OrchestrationReplan:
			waits++
			if waits >= opts.MaxWaits {
				return DriverResult{Steps: step, Outcome: DriverStalled, Final: last}, nil
			}
		}
	}
	return DriverResult{Steps: opts.MaxSteps, Outcome: DriverMaxSteps, Final: last}, nil
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
