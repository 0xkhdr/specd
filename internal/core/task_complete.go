package core

import "fmt"

// CompleteTaskRequest carries the inputs for the single task-completion integrity
// path. Evidence/Unverified mirror the `specd task --status complete` flags;
// Force bypasses the awaiting-approval gate; Idempotent makes a re-completion of
// an already-complete task a no-op (used by the Pinky evidence reconciler so a
// duplicate worker report can never repeat a transition). Tokens/Cost are
// operator/host annotations recorded verbatim — specd never computes them.
type CompleteTaskRequest struct {
	Evidence   string
	Unverified bool
	Force      bool
	Idempotent bool
	Tokens     *int
	Cost       *string
}

// CompleteTaskResult reports the outcome of CompleteTask. AlreadyComplete is true
// when an idempotent request found the task already complete and made no change.
type CompleteTaskResult struct {
	Status          TaskStatus
	Evidence        string
	AlreadyComplete bool
}

// CompleteTask is the one authoritative path that transitions a task to complete.
// Both `specd task --status complete` and the Pinky evidence reconciler call it,
// so there is exactly one place that enforces the dependency, verification, and
// approval-gate rules and one place that mutates state.json and tasks.md. It
// takes the spec lock itself; callers must not already hold it.
func CompleteTask(root, slug, id string, req CompleteTaskRequest) (CompleteTaskResult, error) {
	return WithSpecLock[CompleteTaskResult](root, slug, func() (CompleteTaskResult, error) {
		loaded, err := LoadSpec(root, slug)
		if err != nil {
			return CompleteTaskResult{}, err
		}
		state := loaded.State
		doc := loaded.Doc

		if state.Gate == GateAwaitingApproval && !req.Force {
			return CompleteTaskResult{}, GateError(fmt.Sprintf("spec '%s' is gated (awaiting-approval) — run `specd approve %s` after the revised plan is approved, or pass --force", slug, slug))
		}

		ts, hasTsState := state.Tasks[id]
		docTask := FindTask(doc, id)
		if !hasTsState || docTask == nil {
			return CompleteTaskResult{}, NotFoundError(fmt.Sprintf("task '%s' not found in spec '%s'", id, slug))
		}

		if req.Idempotent && ts.Status == TaskComplete {
			evidence := ""
			if ts.Evidence != nil {
				evidence = *ts.Evidence
			}
			return CompleteTaskResult{Status: TaskComplete, Evidence: evidence, AlreadyComplete: true}, nil
		}

		evidence, serr := ValidateTaskCompletion(state, ts, docTask, slug, id, req.Evidence, req.Unverified)
		if serr != nil {
			return CompleteTaskResult{}, serr
		}

		stamp := NowISO()
		ts.Status = TaskComplete
		ts.Evidence = &evidence
		ts.FinishedAt = &stamp
		if ts.StartedAt == nil {
			ts.StartedAt = &stamp
		}
		ts.Blocker = nil
		RemoveBlocker(state, id)
		docTask.Checked = true
		docTask.Annotation = &Annotation{Kind: AnnotComplete, Evidence: evidence, Ts: stamp}
		if ts.StartedAt != nil {
			completeTelemetry(&ts).DurationMs = DurationMsBetween(*ts.StartedAt, stamp)
		}
		if req.Tokens != nil {
			completeTelemetry(&ts).Tokens = *req.Tokens
		}
		if req.Cost != nil {
			completeTelemetry(&ts).Cost = *req.Cost
		}
		state.Tasks[id] = ts

		tasksPath := ArtifactPath(root, slug, "tasks.md")
		raw := ReadOrDefault(tasksPath, "")
		updated, err := ApplyTaskAnnotation(raw, id, docTask.Checked, docTask.Annotation)
		if err != nil {
			return CompleteTaskResult{}, err
		}
		if err := AtomicWrite(tasksPath, updated); err != nil {
			return CompleteTaskResult{}, err
		}
		DeriveSpecStatus(state)
		if err := SaveState(root, slug, state); err != nil {
			return CompleteTaskResult{}, err
		}
		return CompleteTaskResult{Status: TaskComplete, Evidence: evidence}, nil
	})
}

// ValidateTaskCompletion enforces the complete-status gate and returns the
// evidence string to record. It mutates nothing; on any gate failure it returns
// a *SpecdError (GateError). When unverified is false it requires a passing
// verification record whose command matches the current `verify:` line, so a
// stale or forged record fails closed.
func ValidateTaskCompletion(state *State, ts TaskState, docTask *ParsedTask, slug, id, evidence string, unverified bool) (string, *SpecdError) {
	for _, d := range ts.Depends {
		if dep, ok := state.Tasks[d]; !ok || dep.Status != TaskComplete {
			return "", GateError(fmt.Sprintf("task %s: cannot complete — dependencies not complete: %s", id, d))
		}
	}
	if unverified {
		if evidence == "" {
			return "", GateError(fmt.Sprintf("task %s: --status complete --unverified requires non-empty --evidence", id))
		}
	} else {
		verifyLine := ""
		if v, ok := docTask.Meta["verify"]; ok {
			verifyLine = v
		}
		rec := ts.Verification
		if rec == nil || !rec.Verified {
			return "", GateError(fmt.Sprintf("task %s: --status complete requires a passing `specd verify %s %s` (exit 0) first — or pass --unverified with --evidence for a manual proof", id, slug, id))
		}
		if rec.Command != verifyLine {
			return "", GateError(fmt.Sprintf("task %s: verification is stale — recorded command (%s) ≠ current verify line (%s); re-run `specd verify %s %s`", id, rec.Command, verifyLine, slug, id))
		}
	}
	derived := ""
	if ts.Verification != nil && ts.Verification.Verified {
		gitHead := "no-git"
		if ts.Verification.GitHead != nil {
			gitHead = *ts.Verification.GitHead
		}
		derived = fmt.Sprintf("verified: `%s` → exit 0 @ %s (%s)", ts.Verification.Command, gitHead, ts.Verification.RanAt)
	}
	if evidence == "" {
		evidence = derived
	}
	return evidence, nil
}

// DeriveSpecStatus recomputes the spec lifecycle status from task states once any
// work has started: all complete → verifying (unless already complete), an
// all-blocked frontier → blocked, otherwise executing. Phase follows status.
func DeriveSpecStatus(state *State) {
	vals := make([]TaskState, 0, len(state.Tasks))
	for _, t := range state.Tasks {
		vals = append(vals, t)
	}
	if len(vals) == 0 {
		return
	}
	started := false
	for _, t := range vals {
		if t.Status != TaskPending {
			started = true
			break
		}
	}
	if !started {
		return
	}
	allComplete := true
	for _, t := range vals {
		if t.Status != TaskComplete {
			allComplete = false
			break
		}
	}
	if allComplete {
		if state.Status != StatusComplete {
			state.Status = StatusVerifying
		}
	} else {
		next := NextRunnable(DagTasksFromState(state))
		if next.Kind == NextAllBlocked {
			state.Status = StatusBlocked
		} else {
			state.Status = StatusExecuting
		}
	}
	state.Phase = PhaseForStatus(state.Status)
}

// completeTelemetry lazily allocates a task's Telemetry record so callers can set
// a field without a nil check, keeping it omitted for tasks that never touch it.
func completeTelemetry(ts *TaskState) *Telemetry {
	if ts.Telemetry == nil {
		ts.Telemetry = &Telemetry{}
	}
	return ts.Telemetry
}
