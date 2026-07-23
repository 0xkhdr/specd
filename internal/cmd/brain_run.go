package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	"github.com/0xkhdr/specd/internal/orchestration"
)

const brainLeaseTTL = 15 * time.Minute

// runBrain drives the deterministic orchestration controller (R13.7). Dispatch
// authority is fail-closed: without --authority the controller waits and writes
// nothing. No LLM sits in this path — Decide/Sense are pure functions of state.
func runBrain(root string, args []string, flags map[string]string) error {
	if len(args) < 2 {
		return usageError("brain")
	}
	sub, slug := args[0], args[1]
	sessionPath := filepath.Join(core.SpecdDir(root), "specs", slug, "session.json")
	acpPath := filepath.Join(core.SpecdDir(root), "specs", slug, "acp.jsonl")
	checkpointPath := orchestration.CheckpointPath(root, slug)

	switch sub {
	case "claim":
		return brainClaim(root, sessionPath, acpPath, slug, args[2:])
	case "heartbeat":
		return brainHeartbeat(root, sessionPath, acpPath, slug, args[2:])
	case "release":
		return brainRelease(root, sessionPath, acpPath, slug, args[2:])
	case "report":
		return brainWorkerReport(root, sessionPath, acpPath, slug, args[2:])
	case "status":
		if len(args) != 2 {
			return errors.New("usage: specd brain status <spec>")
		}
		return brainStatus(sessionPath, checkpointPath, acpPath, slug)
	case "start":
		if err := requireBrainStartPreconditions(root, slug); err != nil {
			return err
		}
		// SaveSessionCAS with expected revision 0 refuses when a session already
		// exists (revision != 0), so a second start on the same spec fails closed —
		// at most one controller is ever initialized (spec 07 R5). The session id is
		// the slug: stable and persisted, so deterministic mission ids survive a
		// resume.
		if err := orchestration.SaveSessionCAS(root, sessionPath, 0, orchestration.Session{ID: slug}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "brain start: session initialized for %s\n", slug)
		return nil
	case "cancel":
		return brainCancel(root, sessionPath, acpPath, slug)
	case "resume":
		return brainResume(root, sessionPath, checkpointPath, acpPath, slug)
	case "step":
		_, err := runBrainStep(root, sessionPath, acpPath, checkpointPath, slug, flags, "step")
		return err
	case "run":
		return runBrainRun(root, sessionPath, acpPath, checkpointPath, slug, flags)
	default:
		return fmt.Errorf("unsupported brain subcommand %q", sub)
	}
}

// runBrainStep performs exactly one controller step: release leases for finished
// tasks, sense the frontier, dispatch at most one ready task, and persist the
// session. Tasks that already hold a live lease are withheld from the frontier so
// a repeated step never double-dispatches the same task. Returns the decision
// action so `run` can loop until the controller brakes.
func runBrainStep(root, sessionPath, acpPath, checkpointPath, slug string, flags map[string]string, sub string) (orchestration.Decision, error) {
	session, err := orchestration.LoadSession(sessionPath)
	if err != nil {
		return orchestration.Decision{}, err
	}
	if session.ID == "" {
		session.ID = slug
	}
	// A cancelled or complete session refuses further stepping (spec 07 R2). Fail
	// closed (exit 1) rather than silently no-op.
	if session.IsTerminal() {
		return orchestration.Decision{}, fmt.Errorf("brain %s refused: session is %s", sub, session.Status())
	}

	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return orchestration.Decision{}, err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return orchestration.Decision{}, err
	}
	escalated, err := escalatedCounts(root, slug, spec.Tasks)
	if err != nil {
		return orchestration.Decision{}, err
	}
	status := taskStatus(spec.Tasks)

	// Release leases whose task has reached a terminal status: the mission is done,
	// so the lease must not linger as a phantom live worker in status/stress output
	// (gap 5.4). Persisted below alongside any new dispatch.
	kept := session.Leases[:0:0]
	released := false
	for _, lease := range session.Leases {
		if status[lease.TaskID] == core.TaskComplete {
			released = true
			continue
		}
		kept = append(kept, lease)
	}
	session.Leases = kept

	now := time.Now()
	// A dispatched mission reserves its task before any worker claims it, so
	// reservations — leases plus still-pending missions — are what the frontier
	// must be filtered against. Filtering on leases alone re-issued a mission that
	// was dispatched but not yet claimed, because an unclaimed mission has no lease.
	reservations := append([]orchestration.Lease(nil), session.Leases...)
	for _, mission := range session.PendingMissions {
		reservations = append(reservations, orchestration.Lease{TaskID: mission.TaskID, ExpiresAt: mission.ExpiresAt})
	}

	// Escalated tasks are withheld so the Brain never spins on a task a human must
	// clear first (spec 06 R2); reserved tasks are withheld so a repeated step
	// (and each `run` iteration) advances to the next task instead of re-issuing an
	// in-flight one. Reservations expire, so a dispatch nobody claims frees its task
	// again rather than wedging the frontier.
	withheld := escalatedBoolSet(escalated)
	for _, reservation := range reservations {
		if orchestration.LeaseWorkerState(reservation, now) == orchestration.WorkerStateActive {
			if withheld == nil {
				withheld = map[string]bool{}
			}
			withheld[reservation.TaskID] = true
		}
	}
	frontier, err := core.FrontierExcluding(spec.Tasks, status, withheld)
	if err != nil {
		return orchestration.Decision{}, err
	}
	// Fold accepted worker/host/adapter reports off the mission ledger into the
	// honest cost brake input. Absent reports stay unknown (never zero-filled),
	// so an unconfigured limit keeps today's behavior (spec 07 R4.1, R4.2).
	acpEvents, err := orchestration.ReadACP(acpPath)
	if err != nil {
		return orchestration.Decision{}, err
	}
	telemetry := orchestration.AccrueTelemetry(acpEvents)
	snapshot := orchestration.Sense(state, frontier, reservations, telemetry, now)
	authority := orchestration.Authority{Enabled: flagEnabled(flags, "authority")}
	config, diagnostics := core.LoadConfig(configPaths(root), getenv())
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return orchestration.Decision{}, fmt.Errorf("load config: %s", diagnostic.Message)
		}
	}
	limits := orchestration.DecisionLimitsForAuthority(authority, orchestration.DecisionLimits{
		MaxRetries: config.Routing.MaxRetries, MaxCostMicros: config.Routing.MaxCostMicros,
		MaxTokens: config.Routing.MaxTokens, RequireTelemetry: !config.Routing.AllowUnknownTelemetry,
		Workers: core.WorkerDefinitions{Root: root, Harness: config.Agent},
	})
	if config.Routing.DeadlineSeconds > 0 {
		limits.Deadline = now.Add(time.Duration(config.Routing.DeadlineSeconds) * time.Second)
	}
	if len(frontier) == 0 {
		limits.Approval = approvalHandoff(root, slug, state, spec.Tasks, status, flags["grant"], now)
	}
	dispatcher := &sessionDispatcher{root: root, slug: slug, tasks: spec.Tasks, config: config, acpPath: acpPath, checkpointPath: checkpointPath, now: now, session: &session}
	decision, err := orchestration.DispatchFrontier(snapshot, limits, dispatcher)
	if err != nil {
		return orchestration.Decision{}, err
	}
	waiting := ""
	if decision.Action == orchestration.ActionWaitApproval && decision.Handoff != nil {
		waiting = decision.Handoff.Gate
	}
	waitingChanged := session.WaitingApproval != waiting
	session.WaitingApproval = waiting
	if decision.Action == orchestration.ActionDispatch || released || waitingChanged {
		if err := orchestration.SaveSessionCAS(root, sessionPath, session.Revision, session); err != nil {
			if decision.Action == orchestration.ActionDispatch {
				checkpoint, exists, _ := orchestration.LoadCheckpoint(checkpointPath)
				checkpointID := ""
				if exists {
					checkpointID = checkpoint.MissionID
				}
				return orchestration.Decision{}, core.Refusef("SESSION_WRITE_FAILED", "dispatch %s is durable but session update failed: %v", checkpointID, err).
					WithContext(slug, "checkpoint and dispatch ledger persisted; session CAS failed", "session reconciled with durable dispatch").
					WithMutation(true, checkpointID).
					WithSuccessor(core.RefusalActorOperator, "brain.resume", "specd brain resume "+slug).
					Wrapping(err)
			}
			return orchestration.Decision{}, err
		}
	}
	fmt.Fprintf(os.Stdout, "brain %s: %s %s (%s)\n", sub, decision.Action, decision.TaskID, decision.Reason)
	return decision, nil
}

// runBrainRun steps the controller until it brakes: it dispatches every currently
// ready, unleased frontier task, then stops (waiting for workers to report before
// more tasks become ready). Each step persists its own session CAS and checkpoint,
// so a crash mid-run recovers exactly as a sequence of `brain step` calls would
// (spec 07 write-ahead recovery). Each dispatch withholds one more task from the
// frontier, so the loop shrinks monotonically and the task count is a hard ceiling
// on iterations.
func runBrainRun(root, sessionPath, acpPath, checkpointPath, slug string, flags map[string]string) error {
	spec, err := loadSpec(root, slug)
	if err != nil {
		return err
	}
	dispatched := 0
	for range spec.Tasks {
		decision, err := runBrainStep(root, sessionPath, acpPath, checkpointPath, slug, flags, "run")
		if err != nil {
			return err
		}
		action := decision.Action
		if action == orchestration.ActionDispatch {
			dispatched++
			continue
		}
		// R4.1: reaching an approval gate is neither an error nor a finished
		// run. It is a distinct outcome that names the gate and who must act,
		// reported as non-success for the same reason a brake is (R6.3) — a
		// pipeline that reads exit 0 here would call an unapproved spec done.
		if action == orchestration.ActionWaitApproval {
			return approvalHandoffRefusal(slug, decision, dispatched)
		}
		// R6.3: a brake is permanent — no amount of waiting clears a cost/token
		// limit, a deadline, or an escalated task. Returning success here would
		// report a blocked run as a finished one. A wait is different: workers
		// are still running, so the run legitimately ends with nothing to do.
		switch action {
		case orchestration.ActionHalt, orchestration.ActionTimeout, orchestration.ActionEscalate:
			return zeroProgressRefusal(checkpointPath, slug, action, dispatched)
		}
		return nil
	}
	return nil
}

// zeroProgressRefusal reports a permanently braked run as a distinct non-success
// outcome, naming the durable checkpoint effects so the operator knows whether
// any mission became visible before the brake (R6.3).
func zeroProgressRefusal(checkpointPath, slug string, action orchestration.Action, dispatched int) error {
	checkpoint, exists, _ := orchestration.LoadCheckpoint(checkpointPath)
	checkpointID := ""
	if exists {
		checkpointID = checkpoint.MissionID
	}
	refusal := core.Refusef("BRAIN_ZERO_PROGRESS", "brain run braked on %s after %d dispatch(es); no further task can proceed", action, dispatched).
		WithContext(slug, fmt.Sprintf("durable checkpoint mission %q; %d dispatch(es) recorded in the ledger", checkpointID, dispatched), "resolve the brake, then re-run").
		WithMutation(dispatched > 0, checkpointID).
		WithSuccessor(core.RefusalActorOperator, "brain.status", "specd brain status "+slug)
	return classifyControllerHalt(refusal, dispatched)
}

func classifyControllerHalt(refusal core.Refusal, dispatched int) error {
	if dispatched == 0 {
		return refusal.Wrapping(ErrControllerHalt)
	}
	return refusal
}

// sessionDispatcher records a dispatch as ACP evidence and a session lease. It is
// the only mutation surface for a controller step.
func requireBrainStartPreconditions(root, slug string) error {
	config, diagnostics := core.LoadConfig(configPaths(root), getenv())
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return fmt.Errorf("load config: %s", diagnostic.Message)
		}
	}
	if !config.Orchestration.Enabled {
		return errors.New("missing precondition: orchestration.enabled must be true")
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return err
	}
	if state.Mode != "orchestrated" {
		return fmt.Errorf("missing precondition: spec mode must be orchestrated (got %q)", state.Mode)
	}
	return nil
}

type sessionDispatcher struct {
	root           string
	slug           string
	tasks          []core.TaskRow
	config         core.Config
	acpPath        string
	checkpointPath string
	now            time.Time
	session        *orchestration.Session
}

// Dispatch makes a dispatch durable with write-ahead ordering (spec 07 R1): the
// checkpoint is fsynced BEFORE the dispatch becomes visible in the ledger. If the
// process dies between the two, resume finds the checkpoint's mission id absent
// from the ledger and re-issues exactly that dispatch; if it dies after the
// ledger append, resume finds it present and does not re-issue. Either way
// recovery converges with zero double-dispatch. The lease is written into the
// session only after both writes succeed, so a lease in session.json always
// implies a fully-recorded dispatch.
func (d *sessionDispatcher) Dispatch(task core.FrontierTask) error {
	step := d.session.Step + 1
	missionID := orchestration.MissionID(d.session.ID, step, task.ID)
	var selected core.TaskRow
	for _, row := range d.tasks {
		if row.ID == task.ID {
			selected = row
			break
		}
	}
	// R6.2: detect the overlap before the mission is minted, not at claim. The
	// caller holds the spec lock, so the lease set cannot move between here and
	// the append below.
	if err := orchestration.PreDispatchConflict(task.ID, selected.DeclaredFiles,
		append(append([]orchestration.MissionV1(nil), d.session.Missions...), d.session.PendingMissions...),
		d.session.Leases, d.now); err != nil {
		return err
	}
	route, err := orchestration.RouteTask(selected, d.config.Routing)
	if err != nil {
		return err
	}
	timeout := int(brainLeaseTTL.Seconds())
	if d.config.Routing.DeadlineSeconds > 0 && d.config.Routing.DeadlineSeconds < timeout {
		timeout = d.config.Routing.DeadlineSeconds
	}
	maxAttempts := d.config.Routing.MaxRetries + 1
	mission := orchestration.MissionV1{ProtocolVersion: orchestration.MissionProtocolVersion, SessionID: d.session.ID, MissionID: missionID, SpecSlug: d.slug, TaskID: task.ID, Attempt: 1, Role: selected.Role, AuthorityRef: "approval:tasks", DeclaredFiles: append([]string(nil), selected.DeclaredFiles...), Acceptance: []string{selected.Acceptance}, Verify: selected.Verify, ContextRef: "context:" + d.slug + ":" + task.ID, ContextDigest: core.Digest([]byte(selected.ID + selected.Role + selected.Files + selected.Verify + selected.Acceptance)), ConfigDigest: core.ConfigDigest(d.config), PaletteDigest: core.PaletteDigest(), PolicyDigest: core.ConfigDigest(d.config), SubjectHead: gitHead(d.root), RouteClass: route.Class, RouteReason: route.Reason, Worker: strings.TrimSpace(selected.Worker), Limits: orchestration.MissionLimits{MaxAttempts: maxAttempts, TimeoutSeconds: timeout, MaxTokens: d.config.Routing.MaxTokens, MaxCostMicros: d.config.Routing.MaxCostMicros}, IssuedAt: d.now, ExpiresAt: d.now.Add(time.Duration(timeout) * time.Second), Status: orchestration.MissionPending}
	payload, err := orchestration.MissionPayload(mission)
	if err != nil {
		return err
	}
	if err := orchestration.SaveCheckpoint(d.root, d.checkpointPath, orchestration.Checkpoint{
		SessionID: d.session.ID,
		Step:      step,
		Decision:  orchestration.ACPKindDispatch,
		MissionID: missionID,
		TaskID:    task.ID,
		Mission:   &mission,
		Time:      d.now,
	}); err != nil {
		return err
	}
	if err := orchestration.AppendDispatch(d.acpPath, orchestration.ACPEvent{
		Time:      d.now,
		Kind:      orchestration.ACPKindDispatch,
		TaskID:    task.ID,
		MissionID: missionID,
		Payload:   payload,
	}); err != nil {
		return core.Refusef("DISPATCH_LEDGER_FAILED", "dispatch checkpoint %s persisted but ledger append failed: %v", missionID, err).
			WithContext(missionID, "checkpoint persisted; dispatch ledger append failed", "checkpoint reconciled with the dispatch ledger").
			WithInput("mission", []byte(payload)).
			WithMutation(true, missionID).
			WithSuccessor(core.RefusalActorOperator, "brain.resume", "specd brain resume "+d.slug).
			Wrapping(err)
	}
	d.session.Step = step
	d.session.PendingMissions = append(d.session.PendingMissions, mission)
	return nil
}

// brainCancel drives the session to the terminal cancelled state (spec 07 R2).
// Only session.json is touched — task and evidence state are untouched, so a
// cancel never rewrites history. Cancelling a complete session is refused; a
// second cancel is idempotent.
func brainCancel(root, sessionPath, acpPath, slug string) error {
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		session, err := orchestration.LoadSession(sessionPath)
		if err != nil {
			return struct{}{}, err
		}
		switch session.Status() {
		case orchestration.SessionComplete:
			return struct{}{}, errors.New("brain cancel refused: session already complete")
		case orchestration.SessionCancelled:
			fmt.Fprintf(os.Stdout, "brain cancel: session already cancelled for %s\n", slug)
			return struct{}{}, nil
		}
		now := time.Now()
		next := session
		next.State = orchestration.SessionCancelled
		for i := range next.Leases {
			lease, changed, err := orchestration.CancelLease(next.Leases[i], "operator", now)
			if err != nil {
				return struct{}{}, err
			}
			next.Leases[i] = lease
			if changed {
				payload, _ := json.Marshal(lease)
				if err := orchestration.AppendACP(acpPath, orchestration.ACPEvent{Time: now, Kind: orchestration.ACPKindCancel, MissionID: lease.MissionID, TaskID: lease.TaskID, Attempt: lease.Attempt, Payload: string(payload)}); err != nil {
					return struct{}{}, err
				}
			}
		}
		if err := orchestration.SaveSessionCAS(root, sessionPath, session.Revision, next); err != nil {
			return struct{}{}, err
		}
		fmt.Fprintf(os.Stdout, "brain cancel: session cancelled for %s\n", slug)
		return struct{}{}, nil
	})
	return err
}

// brainResume reconstructs the controller from the last checkpoint reconciled
// against the ledger (spec 07 R3/R4). A live lease on a still-running session
// means another controller holds work — resume refuses and only expired or
// crash-orphaned leases are recoverable (R5). An irreconcilable checkpoint/ledger
// conflict refuses (exit 1) rather than guessing. The resume is claimed by a
// session-revision CAS, so two racing resumes conflict and exactly one proceeds.
//
// The entire critical section runs under one core.WithSpecLock: loading the
// session, reading the ledger, planning the reissue, the session CAS, and the
// ledger append are one atomic transaction w.r.t. other resumes. Without it,
// two resumes could interleave their (separately locked) CAS and (unlocked)
// ledger read/append — one winning its CAS while reading a stale-empty ledger
// mid-window of the other's not-yet-appended dispatch, double-dispatching the
// same mission. The lock is reentrant per goroutine, so the nested WithSpecLock
// inside SaveSessionCAS does not deadlock; across processes the file lock
// serializes, so the losing resume sees the winner's dispatch already in the
// ledger and PlanResume declines to re-issue.
func brainResume(root, sessionPath, checkpointPath, acpPath, slug string) error {
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		return struct{}{}, brainResumeLocked(root, sessionPath, checkpointPath, acpPath, slug)
	})
	return err
}

func brainResumeLocked(root, sessionPath, checkpointPath, acpPath, slug string) error {
	session, err := orchestration.LoadSession(sessionPath)
	if err != nil {
		return err
	}
	if session.IsTerminal() {
		return fmt.Errorf("brain resume refused: session is %s", session.Status())
	}
	currentHead := gitHead(root)
	for li := range session.Leases {
		lease := session.Leases[li]
		if lease.State != orchestration.LeaseActive {
			continue
		}
		for _, mission := range session.Missions {
			if mission.MissionID != lease.MissionID || mission.SubjectHead == currentHead {
				continue
			}
			now := time.Now()
			step := session.Step + 1
			reissued, err := orchestration.ReissueMission(mission, step, currentHead, now)
			if err != nil {
				return fmt.Errorf("brain resume refused: %w", err)
			}
			payload, err := orchestration.MissionPayload(reissued)
			if err != nil {
				return err
			}
			if err := orchestration.SaveCheckpoint(root, checkpointPath, orchestration.Checkpoint{
				SessionID: session.ID, Step: step, Decision: orchestration.ACPKindDispatch,
				MissionID: reissued.MissionID, TaskID: reissued.TaskID, Mission: &reissued, Time: now,
			}); err != nil {
				return err
			}
			if err := orchestration.AppendDispatch(acpPath, orchestration.ACPEvent{
				Time: now, Kind: orchestration.ACPKindDispatch, TaskID: reissued.TaskID,
				MissionID: reissued.MissionID, Payload: payload,
			}); err != nil {
				return err
			}
			session.Leases[li] = orchestration.ReleaseMission(lease, "stale baseline")
			session.PendingMissions = append(session.PendingMissions, reissued)
			session.Step = step
			if err := orchestration.SaveSessionCAS(root, sessionPath, session.Revision, session); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "brain resume: revoked stale mission %s at %s and re-issued %s for task %s at current HEAD %s\n",
				mission.MissionID, mission.SubjectHead, reissued.MissionID, mission.TaskID, currentHead)
			return nil
		}
	}
	cp, cpExists, err := orchestration.LoadCheckpoint(checkpointPath)
	if err != nil {
		return err
	}
	events, err := orchestration.ReadACP(acpPath)
	if err != nil {
		return err
	}
	now := time.Now()
	next, _, err := orchestration.ReconcileSession(session, cp, cpExists, events)
	if err != nil {
		return fmt.Errorf("brain resume refused: %w", err)
	}
	// A live lease is recoverable only when the session actually crashed (the
	// checkpoint outran the ledger). A running session with a live lease is a
	// controller mid-flight; refuse rather than clobber it.
	if orchestration.DeriveStatus(next, cp, cpExists, events) == orchestration.SessionRunning &&
		orchestration.HasLiveLease(next.Leases, now) {
		return errors.New("brain resume refused: session is running with a live lease")
	}
	plan := orchestration.PlanResume(cp, cpExists, events)
	if plan.Conflict != "" {
		return fmt.Errorf("brain resume refused: %s", plan.Conflict)
	}
	// Claim the resume: bump the revision and reclaim orphaned leases. Racing
	// resumes both load the same revision and this CAS lets exactly one win.
	kept := next.Leases[:0:0]
	for _, lease := range next.Leases {
		if lease.State == orchestration.LeaseRevoked {
			kept = append(kept, lease)
		}
	}
	next.Leases = kept
	if err := orchestration.SaveSessionCAS(root, sessionPath, session.Revision, next); err != nil {
		return err
	}
	if plan.Reissue {
		if err := orchestration.AppendDispatch(acpPath, orchestration.ACPEvent{
			Time:      cp.Time,
			Kind:      orchestration.ACPKindDispatch,
			TaskID:    cp.TaskID,
			MissionID: cp.MissionID,
			Payload: func() string {
				if cp.Mission == nil {
					return ""
				}
				payload, _ := orchestration.MissionPayload(*cp.Mission)
				return payload
			}(),
		}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "brain resume: re-issued mission %s (task %s) for %s\n", cp.MissionID, cp.TaskID, slug)
		return nil
	}
	fmt.Fprintf(os.Stdout, "brain resume: reconciled, no dispatch to re-issue for %s\n", slug)
	return nil
}

// brainRelease immediately releases a mission without TTL wait (R4.3). The lease
// is marked revoked and removed from the session.
func brainRelease(root, sessionPath, acpPath, slug string, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: specd brain release <mission-id>")
	}
	missionID := args[0]
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		s, err := orchestration.LoadSession(sessionPath)
		if err != nil {
			return struct{}{}, err
		}
		leaseIdx := -1
		var l orchestration.Lease
		for i, candidate := range s.Leases {
			if candidate.MissionID == missionID {
				leaseIdx = i
				l = candidate
				break
			}
		}
		if leaseIdx < 0 {
			return struct{}{}, fmt.Errorf("MISSION_NOT_FOUND: %s", missionID)
		}
		now := time.Now()
		released := orchestration.ReleaseMission(l, "released by controller")
		s.Leases = append(s.Leases[:leaseIdx], s.Leases[leaseIdx+1:]...)
		if err := orchestration.SaveSessionCAS(root, sessionPath, s.Revision, s); err != nil {
			return struct{}{}, err
		}
		payload, _ := json.Marshal(released)
		if err := orchestration.AppendACP(acpPath, orchestration.ACPEvent{
			Time:      now,
			Kind:      orchestration.ACPKindCancel,
			MissionID: missionID,
			TaskID:    l.TaskID,
			Attempt:   l.Attempt,
			Payload:   string(payload),
		}); err != nil {
			return struct{}{}, err
		}
		fmt.Fprintf(os.Stdout, "brain release: released mission %s (task %s) for %s\n", missionID, l.TaskID, slug)
		return struct{}{}, nil
	})
	return err
}

// brainStatusView is the operator-facing status: the derived lifecycle state
// (running|cancelled|complete|crashed), the last checkpoint's step/time, and the
// live lease holders with their expiry (spec 07 R6).
type brainStatusView struct {
	Slug           string                 `json:"slug"`
	Status         string                 `json:"status"`
	SessionID      string                 `json:"session_id,omitempty"`
	Step           int                    `json:"step"`
	CheckpointStep int                    `json:"checkpoint_step,omitempty"`
	CheckpointTime string                 `json:"checkpoint_time,omitempty"`
	Leases         []brainStatusLeaseView `json:"leases,omitempty"`
	WorkerStates   map[string]int         `json:"worker_states,omitempty"`
}

type brainStatusLeaseView struct {
	Holder    string `json:"holder"`
	TaskID    string `json:"task_id"`
	ExpiresAt string `json:"expires_at"`
	State     string `json:"state"`
	Live      bool   `json:"live"`
}

func brainStatus(sessionPath, checkpointPath, acpPath, slug string) error {
	session, err := orchestration.LoadSession(sessionPath)
	if err != nil {
		return err
	}
	cp, cpExists, err := orchestration.LoadCheckpoint(checkpointPath)
	if err != nil {
		return err
	}
	events, err := orchestration.ReadACP(acpPath)
	if err != nil {
		return err
	}
	now := time.Now()
	view := brainStatusView{
		Slug:      slug,
		Status:    string(orchestration.DeriveStatus(session, cp, cpExists, events)),
		SessionID: session.ID,
		Step:      session.Step,
	}
	if cpExists {
		view.CheckpointStep = cp.Step
		view.CheckpointTime = cp.Time.UTC().Format(time.RFC3339)
	}
	for _, lease := range session.Leases {
		state := orchestration.LeaseWorkerState(lease, now)
		if view.WorkerStates == nil {
			view.WorkerStates = map[string]int{}
		}
		view.WorkerStates[string(state)]++
		view.Leases = append(view.Leases, brainStatusLeaseView{
			Holder:    lease.WorkerID,
			TaskID:    lease.TaskID,
			ExpiresAt: lease.ExpiresAt.UTC().Format(time.RFC3339),
			State:     string(state),
			Live:      state == orchestration.WorkerStateActive,
		})
	}
	return writeJSON(view)
}

// approvalHandoff resolves what the controller is actually blocked on when the
// frontier empties (R4.1). It returns nil unless execution is genuinely done —
// at least one task, all of them complete — so a spec with no tasks, or one
// whose frontier is empty because work is escalated or withheld, keeps the
// ordinary frontier wait it has always reported.
//
// The controller never approves and never spends a grant. When an operator
// names one with --grant, this reads it — identity and scope only, no bearer
// token ever enters the controller — and reports the delegated route if that
// grant already covers this transition. A grant that is expired, revoked,
// exhausted, or out of scope simply is not named, and the human route stands
// (R4.3). Naming a route is not authorizing one (R4.4).
func approvalHandoff(root, slug string, state core.State, tasks []core.TaskRow, status map[string]core.TaskRunStatus, grantID string, now time.Time) *orchestration.ApprovalHandoff {
	if len(tasks) == 0 {
		return nil
	}
	for _, task := range tasks {
		if status[task.ID] != core.TaskComplete {
			return nil
		}
	}
	if core.NextStatus(state.Status) == "" {
		return nil // terminal: there is no approval left to wait for
	}
	handoff := &orchestration.ApprovalHandoff{
		Gate:  approvalGateFor(state.Status),
		Route: "specd approve " + slug,
		Actor: core.RefusalActorHuman,
	}
	// Gate drift outranks every authority: if the readiness gates refuse now,
	// no approval route works, and pointing at one would be a lie the operator
	// only discovers by running it.
	readiness, err := buildReadiness(root, slug, state)
	if err != nil || gates.HasErrors(readiness.Findings) {
		handoff.Blocked = true
		handoff.Route = "specd check " + slug
		handoff.Actor = core.RefusalActorAgent
		return handoff
	}
	if grantID == "" {
		return handoff
	}
	projection, err := core.LoadGrant(root, grantID)
	if err != nil || projection.Status(now) != "active" {
		return handoff
	}
	if !slices.Contains(projection.Grant.SpecIDs, slug) ||
		!slices.Contains(projection.Grant.Transitions, "approve."+handoff.Gate) ||
		slices.Contains(projection.Grant.Prohibitions, handoff.Gate) {
		return handoff
	}
	handoff.Route = "specd delegate approve " + slug + " --grant " + grantID + " --token <bearer> --reason <why>"
	handoff.Actor = core.RefusalActorOperator
	return handoff
}

// approvalHandoffRefusal reports a run that reached a lifecycle gate: distinct
// from a brake, distinct from success, and carrying the exact route and actor
// the halt needs. Progress already made is untouched and stays in the session.
func approvalHandoffRefusal(slug string, decision orchestration.Decision, dispatched int) error {
	handoff := decision.Handoff
	if handoff == nil {
		handoff = &orchestration.ApprovalHandoff{Gate: string(orchestration.ActionWaitApproval), Route: "specd approve " + slug, Actor: core.RefusalActorHuman}
	}
	refusal := core.Refusef("APPROVAL_REQUIRED", "brain run reached the %s approval gate after %d dispatch(es); %s",
		handoff.Gate, dispatched, decision.Reason).
		WithContext(slug, "lifecycle gate "+handoff.Gate+" is unapproved", "an approved lifecycle transition").
		WithMutation(dispatched > 0, "").
		WithRecovery(handoff.Actor, handoff.Route)
	return classifyControllerHalt(refusal, dispatched)
}
