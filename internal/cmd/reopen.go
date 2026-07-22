package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	corescope "github.com/0xkhdr/specd/internal/core/scope"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// runReopen opens the next attempt of a terminal task. Only `task` is wired:
// artifact and spec reopen resolve to no operation and fail closed before
// dispatch, so this handler never sees them.
func runReopen(root string, args []string, flags map[string]string) error {
	if len(args) != 3 || args[1] != "task" {
		return usageError("reopen")
	}
	slug, taskID := args[0], args[2]
	if err := core.ValidateSlug(slug); err != nil {
		return core.Refusef("SPEC_INVALID", "%v", err)
	}
	reason := strings.TrimSpace(flags["reason"])
	if reason == "" {
		return fmt.Errorf("%w: reopen requires --reason <text>", ErrUsage)
	}
	raw := strings.TrimSpace(flags["expect-revision"])
	if raw == "" {
		return fmt.Errorf("%w: reopen requires --expect-revision <n>", ErrUsage)
	}
	expected, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || expected < 0 {
		return fmt.Errorf("%w: --expect-revision must be a non-negative integer, got %q", ErrUsage, raw)
	}

	plan, err := core.WithSpecLock(root, func() (core.ReopenPlan, error) {
		spec, err := loadSpec(root, slug)
		if err != nil {
			return core.ReopenPlan{}, err
		}
		statePath, eventPath := core.StatePath(root, slug), core.WorkflowEventPath(root, slug)
		state, err := core.RecoverWorkflowState(statePath, eventPath)
		if err != nil {
			return core.ReopenPlan{}, err
		}
		events, err := core.ReadWorkflowEvents(eventPath)
		if err != nil {
			return core.ReopenPlan{}, err
		}
		baseline := gitHead(root)
		req := core.ReopenRequest{
			TaskID:           taskID,
			ExpectedRevision: expected,
			Reason:           reason,
			ActorID:          core.ReopenActor(),
			Baseline:         baseline,
			ScopeAmendment:   splitList(flags["scope"]),
			RepairPaths:      repairPaths(root, baseline, spec.Tasks),
			Leases:           liveTaskLeases(root, slug),
			RevokeLease:      strings.TrimSpace(flags["revoke-lease"]),
		}
		status := taskStatus(spec.Tasks)
		preview := core.PlanTaskReopen(slug, req, spec.Tasks, status, events, state.Revision)
		if !preview.Eligible {
			return preview, preview.Refusal()
		}
		// The lease is surrendered before the attempt event lands: a released
		// lease with no new attempt is recoverable, a live lease over a fresh
		// attempt is two authorities writing the same files (R3.4).
		if err := surrenderLeases(root, slug, preview); err != nil {
			return preview, err
		}
		return core.CommitTaskReopen(statePath, eventPath, slug, req, spec.Tasks, status, preview)
	})
	if err != nil {
		return err
	}
	return writeJSON(plan)
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// repairPaths is the part of the worktree diff this spec governs: changed paths
// that some task declares. A repair already touching another task's files is
// then caught at reopen time and answered with the exact --scope amendment it
// needs (R3.3).
//
// Undeclared paths are deliberately excluded rather than refused here. They are
// the diff-scope gate's business at completion, and treating unrelated worktree
// dirt as this task's repair would refuse reopens that have nothing to do with
// scope. A worktree that cannot be diffed contributes no paths.
func repairPaths(root, baseline string, tasks []core.TaskRow) []string {
	diff, err := corescope.Derive(root, baseline)
	if err != nil {
		return nil
	}
	var governed []string
	for _, task := range tasks {
		governed = append(governed, task.DeclaredFiles...)
	}
	var paths []string
	for _, path := range diff.Paths {
		if strings.HasPrefix(path, ".specd/") || len(corescope.Outside([]string{path}, governed)) > 0 {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

func sessionPathFor(root, slug string) string {
	return filepath.Join(core.SpecdDir(root), "specs", slug, "session.json")
}

// liveTaskLeases resolves the mission/lease claims still owning this spec's
// tasks. Revoked and expired leases are not live and never block a reopen.
func liveTaskLeases(root, slug string) []core.TaskLease {
	session, err := orchestration.LoadSession(sessionPathFor(root, slug))
	if err != nil {
		return nil
	}
	var leases []core.TaskLease
	for _, lease := range session.Leases {
		if lease.State != "" && lease.State != orchestration.LeaseActive {
			continue
		}
		leases = append(leases, core.TaskLease{LeaseID: lease.LeaseID, TaskID: lease.TaskID, Holder: lease.WorkerID})
	}
	return leases
}

// surrenderLeases releases or revokes every lease the eligible plan authorized,
// atomically under the same spec lock the attempt commits in.
func surrenderLeases(root, slug string, plan core.ReopenPlan) error {
	if len(plan.LeaseActions) == 0 {
		return nil
	}
	surrender := map[string]bool{}
	for _, action := range plan.LeaseActions {
		surrender[action.EntityID] = true
	}
	path := sessionPathFor(root, slug)
	session, err := orchestration.LoadSession(path)
	if err != nil {
		return err
	}
	kept := session.Leases[:0:0]
	for _, lease := range session.Leases {
		if surrender[lease.TaskID] {
			continue
		}
		kept = append(kept, lease)
	}
	if len(kept) == len(session.Leases) {
		return nil
	}
	session.Leases = kept
	return orchestration.SaveSessionCAS(root, path, session.Revision, session)
}
