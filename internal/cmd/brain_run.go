package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

const brainLeaseTTL = 15 * time.Minute

// runBrain drives the deterministic orchestration controller (R13.7). Dispatch
// authority is fail-closed: without --authority the controller waits and writes
// nothing. No LLM sits in this path — Decide/Sense are pure functions of state.
func runBrain(root string, args []string, flags map[string]string) error {
	if len(args) < 2 {
		return errors.New("usage: specd brain <start|step|run|status|cancel|resume|claim|heartbeat|report> <spec> [args]")
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
		return brainCancel(root, sessionPath, slug)
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
func runBrainStep(root, sessionPath, acpPath, checkpointPath, slug string, flags map[string]string, sub string) (orchestration.Action, error) {
	session, err := orchestration.LoadSession(sessionPath)
	if err != nil {
		return "", err
	}
	if session.ID == "" {
		session.ID = slug
	}
	// A cancelled or complete session refuses further stepping (spec 07 R2). Fail
	// closed (exit 1) rather than silently no-op.
	if session.IsTerminal() {
		return "", fmt.Errorf("brain %s refused: session is %s", sub, session.Status())
	}

	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return "", err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return "", err
	}
	escalated, err := escalatedCounts(root, slug, spec.Tasks)
	if err != nil {
		return "", err
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
	// Escalated tasks are withheld so the Brain never spins on a task a human must
	// clear first (spec 06 R2); live-leased tasks are withheld so a repeated step
	// (and each `run` iteration) advances to the next task instead of re-issuing an
	// in-flight one.
	withheld := escalatedBoolSet(escalated)
	for _, lease := range session.Leases {
		if orchestration.LeaseWorkerState(lease, now) == orchestration.WorkerStateActive {
			if withheld == nil {
				withheld = map[string]bool{}
			}
			withheld[lease.TaskID] = true
		}
	}
	frontier, err := core.FrontierExcluding(spec.Tasks, status, withheld)
	if err != nil {
		return "", err
	}

	reservations := append([]orchestration.Lease(nil), session.Leases...)
	for _, mission := range session.PendingMissions {
		reservations = append(reservations, orchestration.Lease{TaskID: mission.TaskID, ExpiresAt: mission.ExpiresAt})
	}
	snapshot := orchestration.Sense(state, frontier, reservations, now)
	authority := orchestration.Authority{Enabled: flagEnabled(flags, "authority")}
	limits := orchestration.DecisionLimitsForAuthority(authority, orchestration.DecisionLimits{MaxRetries: 1})

	config, _ := core.LoadConfig(configPaths(root), getenv())
	dispatcher := &sessionDispatcher{root: root, slug: slug, tasks: spec.Tasks, config: config, acpPath: acpPath, checkpointPath: checkpointPath, now: now, session: &session}
	decision, err := orchestration.DispatchFrontier(snapshot, limits, dispatcher)
	if err != nil {
		return "", err
	}
	if decision.Action == orchestration.ActionDispatch || released {
		if err := orchestration.SaveSessionCAS(root, sessionPath, session.Revision, session); err != nil {
			return "", err
		}
	}
	fmt.Fprintf(os.Stdout, "brain %s: %s %s (%s)\n", sub, decision.Action, decision.TaskID, decision.Reason)
	return decision.Action, nil
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
	for range spec.Tasks {
		action, err := runBrainStep(root, sessionPath, acpPath, checkpointPath, slug, flags, "run")
		if err != nil {
			return err
		}
		if action != orchestration.ActionDispatch {
			return nil
		}
	}
	return nil
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
	mission := orchestration.MissionV1{ProtocolVersion: orchestration.MissionProtocolVersion, SessionID: d.session.ID, MissionID: missionID, SpecSlug: d.slug, TaskID: task.ID, Attempt: 1, Role: selected.Role, AuthorityRef: "approval:tasks", DeclaredFiles: append([]string(nil), selected.DeclaredFiles...), Acceptance: []string{selected.Acceptance}, Verify: selected.Verify, ContextRef: "context:" + d.slug + ":" + task.ID, ContextDigest: core.Digest([]byte(selected.ID + selected.Role + selected.Files + selected.Verify + selected.Acceptance)), ConfigDigest: core.ConfigDigest(d.config), PaletteDigest: core.PaletteDigest(), PolicyDigest: core.ConfigDigest(d.config), SubjectHead: gitHead(d.root), RouteClass: "pending-local", RouteReason: "controller recorded; delivery adapter not configured", Limits: orchestration.MissionLimits{MaxAttempts: 1, TimeoutSeconds: int(brainLeaseTTL.Seconds())}, IssuedAt: d.now, ExpiresAt: d.now.Add(brainLeaseTTL), Status: orchestration.MissionPending}
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
		return err
	}
	d.session.Step = step
	d.session.PendingMissions = append(d.session.PendingMissions, mission)
	return nil
}

// brainCancel drives the session to the terminal cancelled state (spec 07 R2).
// Only session.json is touched — task and evidence state are untouched, so a
// cancel never rewrites history. Cancelling a complete session is refused; a
// second cancel is idempotent.
func brainCancel(root, sessionPath, slug string) error {
	session, err := orchestration.LoadSession(sessionPath)
	if err != nil {
		return err
	}
	switch session.Status() {
	case orchestration.SessionComplete:
		return errors.New("brain cancel refused: session already complete")
	case orchestration.SessionCancelled:
		fmt.Fprintf(os.Stdout, "brain cancel: session already cancelled for %s\n", slug)
		return nil
	}
	next := session
	next.State = orchestration.SessionCancelled
	next.Leases = nil // lease released
	if err := orchestration.SaveSessionCAS(root, sessionPath, session.Revision, next); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "brain cancel: session cancelled for %s\n", slug)
	return nil
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
	cp, cpExists, err := orchestration.LoadCheckpoint(checkpointPath)
	if err != nil {
		return err
	}
	events, err := orchestration.ReadACP(acpPath)
	if err != nil {
		return err
	}
	now := time.Now()
	// A live lease is recoverable only when the session actually crashed (the
	// checkpoint outran the ledger). A running session with a live lease is a
	// controller mid-flight; refuse rather than clobber it.
	if orchestration.DeriveStatus(session, cp, cpExists, events) == orchestration.SessionRunning &&
		orchestration.HasLiveLease(session.Leases, now) {
		return errors.New("brain resume refused: session is running with a live lease")
	}
	plan := orchestration.PlanResume(cp, cpExists, events)
	if plan.Conflict != "" {
		return fmt.Errorf("brain resume refused: %s", plan.Conflict)
	}
	// Claim the resume: bump the revision and reclaim orphaned leases. Racing
	// resumes both load the same revision and this CAS lets exactly one win.
	next := session
	next.Leases = nil
	if err := orchestration.SaveSessionCAS(root, sessionPath, session.Revision, next); err != nil {
		return err
	}
	if plan.Reissue {
		if err := orchestration.AppendDispatch(acpPath, orchestration.ACPEvent{
			Time:      cp.Time,
			Kind:      orchestration.ACPKindDispatch,
			TaskID:    cp.TaskID,
			MissionID: cp.MissionID,
		}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "brain resume: re-issued mission %s (task %s) for %s\n", cp.MissionID, cp.TaskID, slug)
		return nil
	}
	fmt.Fprintf(os.Stdout, "brain resume: reconciled, no dispatch to re-issue for %s\n", slug)
	return nil
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
