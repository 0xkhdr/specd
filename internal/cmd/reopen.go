package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	corescope "github.com/0xkhdr/specd/internal/core/scope"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// runReopen opens the next attempt of a terminal task (`task`), the next draft
// version of a spec artifact (`artifact`), or the next lifecycle cycle of the
// whole spec (`spec`).
func runReopen(root string, args []string, flags map[string]string) error {
	if len(args) < 2 {
		return usageError("reopen")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return core.Refusef("SPEC_INVALID", "%v", err)
	}
	reason, expected, err := reopenIntent(flags)
	if err != nil {
		return err
	}
	switch {
	case args[1] == "task" && len(args) == 3:
		return reopenTask(root, slug, args[2], reason, expected, flags)
	case args[1] == "scope" && len(args) == 4:
		return amendTaskScope(root, slug, args[2], args[3], reason, expected, flags)
	case args[1] == "artifact" && len(args) == 3:
		return reopenArtifact(root, slug, args[2], reason, expected)
	case args[1] == "spec" && len(args) == 2:
		return reopenArtifact(root, slug, "", reason, expected)
	case args[1] == "descendant" && len(args) == 4:
		return resolveDescendant(root, slug, args[2], args[3], reason, expected)
	}
	return usageError("reopen")
}

func amendTaskScope(root, slug, taskID, path, reason string, expected int64, flags map[string]string) error {
	plan, err := core.WithSpecLock(root, func() (core.ScopeAmendPlan, error) {
		spec, err := loadSpec(root, slug)
		if err != nil {
			return core.ScopeAmendPlan{}, err
		}
		statePath, eventPath := core.StatePath(root, slug), core.WorkflowEventPath(root, slug)
		state, err := core.RecoverWorkflowState(statePath, eventPath)
		if err != nil {
			return core.ScopeAmendPlan{}, err
		}
		if err := validateSessionBinding(root, slug, taskID, state, flags, time.Now()); err != nil {
			return core.ScopeAmendPlan{}, err
		}
		req := core.ScopeAmendRequest{
			TaskID: taskID, Path: path, Reason: reason, ActorID: core.ReopenActor(),
			GitHead: gitHead(root), ExpectedRevision: expected,
		}
		status := taskStatus(spec.Tasks)
		preview := core.PlanScopeAmend(slug, req, spec.Tasks, status, state.Revision)
		if !preview.Eligible {
			return preview, preview.Refusal()
		}
		if err := enforceSessionBinding(root, slug, taskID, state, flags, time.Now()); err != nil {
			return preview, err
		}
		tasksPath, err := core.SpecArtifactPath(root, slug, "tasks")
		if err != nil {
			return preview, err
		}
		return core.CommitScopeAmend(tasksPath, statePath, eventPath, slug, req, spec.Tasks, status, preview)
	})
	if err != nil {
		return err
	}
	return writeJSON(plan)
}

// reopenIntent parses the two flags every reopen requires.
func reopenIntent(flags map[string]string) (string, int64, error) {
	reason := strings.TrimSpace(flags["reason"])
	if reason == "" {
		return "", 0, fmt.Errorf("%w: reopen requires --reason <text>", ErrUsage)
	}
	raw := strings.TrimSpace(flags["expect-revision"])
	if raw == "" {
		return "", 0, fmt.Errorf("%w: reopen requires --expect-revision <n>", ErrUsage)
	}
	expected, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || expected < 0 {
		return "", 0, fmt.Errorf("%w: --expect-revision must be a non-negative integer, got %q", ErrUsage, raw)
	}
	return reason, expected, nil
}

// reopenArtifact preserves the current artifact bytes as a content-addressed
// revision and opens a new draft version (or, with no artifact, a new lifecycle
// cycle). Released, deployed, and archived work refuses here with the successor
// route (R4.1-R4.3).
func reopenArtifact(root, slug, artifact, reason string, expected int64) error {
	plan, err := core.WithSpecLock(root, func() (core.ArtifactReopenPlan, error) {
		statePath, eventPath := core.StatePath(root, slug), core.WorkflowEventPath(root, slug)
		state, err := core.RecoverWorkflowState(statePath, eventPath)
		if err != nil {
			return core.ArtifactReopenPlan{}, err
		}
		events, err := core.ReadWorkflowEvents(eventPath)
		if err != nil {
			return core.ArtifactReopenPlan{}, err
		}
		req := core.ArtifactReopenRequest{
			Artifact:         artifact,
			ExpectedRevision: expected,
			Reason:           reason,
			ActorID:          core.ReopenActor(),
			GitHead:          gitHead(root),
			Digests:          artifactDigests(root, slug),
			Consumptions:     externalConsumptions(root, slug),
		}
		preview := core.PlanArtifactReopen(slug, req, state, events)
		if !preview.Eligible {
			return preview, preview.Refusal()
		}
		return core.CommitArtifactReopen(root, slug, req, preview)
	})
	if err != nil {
		return err
	}
	return writeJSON(plan)
}

// artifactDigests reads the current bytes digest of every reopenable artifact.
// A missing or unreadable artifact contributes nothing, so planning refuses it
// rather than reopening work whose prior revision cannot be preserved (R4.4).
func artifactDigests(root, slug string) map[string]string {
	digests := map[string]string{}
	for _, artifact := range core.ReopenableArtifacts {
		path, err := core.SpecArtifactPath(root, slug, artifact)
		if err != nil {
			continue
		}
		if raw, readErr := os.ReadFile(path); readErr == nil {
			digests[artifact] = core.Digest(raw)
		}
	}
	return digests
}

// externalConsumptions resolves what has already consumed this spec's work from
// durable delivery records only. Release, deployment, and archive records are
// immutable outside specd, so they make in-place reopen forbidden; a submission
// is withdrawable, so it blocks without being successor-only (R4.3).
func externalConsumptions(root, slug string) []core.ImpactConsumption {
	var consumptions []core.ImpactConsumption
	for _, record := range []struct {
		kind, path string
		external   bool
	}{
		{"release", core.ReleaseLedgerPath(root, slug), true},
		{"deployment", core.DeploymentLedgerPath(root, slug), true},
		{"archive", core.ArchiveRecordPath(root, slug), true},
		{"submission", core.SubmissionsPath(root, slug), false},
	} {
		if info, err := os.Stat(record.path); err == nil && info.Size() > 0 {
			consumptions = append(consumptions, core.ImpactConsumption{Record: record.path, Kind: record.kind, External: record.external})
		}
	}
	return consumptions
}

func reopenTask(root, slug, taskID, reason string, expected int64, flags map[string]string) error {
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
		tasksPath, err := core.SpecArtifactPath(root, slug, "tasks")
		if err != nil {
			return preview, err
		}
		return core.CommitTaskReopen(tasksPath, statePath, eventPath, slug, req, spec.Tasks, status, preview)
	})
	if err != nil {
		return err
	}
	return writeJSON(plan)
}

// resolveDescendant records the explicit resolution of one stale descendant
// (R5.2). Everything the resolution must stand on is resolved from durable
// state here — the descendant's current attempt, its attempt-current evidence
// at the current HEAD, the approval that authorized a retain, and the
// acceptance criteria tasks.md still routes only to it — and judged by the
// planner. `reopen` is not accepted here: it is the reopen route itself.
func resolveDescendant(root, slug, taskID, resolution, reason string, expected int64) error {
	plan, err := core.WithSpecLock(root, func() (core.DescendantResolutionPlan, error) {
		spec, err := loadSpec(root, slug)
		if err != nil {
			return core.DescendantResolutionPlan{}, err
		}
		statePath, eventPath := core.StatePath(root, slug), core.WorkflowEventPath(root, slug)
		state, err := core.RecoverWorkflowState(statePath, eventPath)
		if err != nil {
			return core.DescendantResolutionPlan{}, err
		}
		events, err := core.ReadWorkflowEvents(eventPath)
		if err != nil {
			return core.DescendantResolutionPlan{}, err
		}
		evidence, err := core.LoadEvidence(core.EvidencePath(root, slug))
		if err != nil {
			return core.DescendantResolutionPlan{}, err
		}
		record, hasEvidence := evidence[taskID]
		criteria, reassignments := descendantCoverage(spec.Tasks, taskID)
		req := core.DescendantResolutionRequest{
			TaskID:           taskID,
			Resolution:       resolution,
			Reason:           reason,
			ActorID:          core.ReopenActor(),
			ExpectedRevision: expected,
			CurrentHead:      gitHead(root),
			Attempt:          core.CurrentTaskAttempt(events, taskID),
			Evidence:         record,
			HasEvidence:      hasEvidence,
			ApprovalRef:      approvedImpactRef(state, taskID),
			Successor:        successorOf(reassignments),
			Criteria:         criteria,
			Reassignments:    reassignments,
		}
		preview := core.PlanDescendantResolution(slug, req, core.StaleDescendants(events), state.Revision)
		if !preview.Eligible {
			return preview, preview.Refusal()
		}
		return core.CommitDescendantResolution(statePath, eventPath, slug, req, preview)
	})
	if err != nil {
		return err
	}
	return writeJSON(plan)
}

// descendantCoverage is the acceptance coverage this descendant carries and
// where tasks.md already hands each criterion. A criterion another task also
// declares in its `refs` column is reassigned by that task; one only this
// descendant declares stays uncovered, which is what refuses a supersede or
// cancel until tasks.md is amended.
func descendantCoverage(tasks []core.TaskRow, taskID string) ([]string, []core.CriterionReassignment) {
	var criteria []string
	for _, task := range tasks {
		if task.ID == taskID {
			criteria = append(criteria, task.Refs...)
		}
	}
	var reassignments []core.CriterionReassignment
	for _, ref := range criteria {
		for _, task := range tasks {
			if task.ID == taskID || !slices.Contains(task.Refs, ref) {
				continue
			}
			reassignments = append(reassignments, core.CriterionReassignment{Criterion: ref, From: taskID, To: task.ID})
			break
		}
	}
	return criteria, reassignments
}

// successorOf names the task that took over the descendant's coverage; with no
// reassignment there is no successor and a supersede refuses.
func successorOf(reassignments []core.CriterionReassignment) string {
	if len(reassignments) == 0 {
		return ""
	}
	return reassignments[0].To
}

// approvedImpactRef is the approved approval request covering this task, which
// is the explicit impact approval a retain needs on top of fresh evidence.
func approvedImpactRef(state core.State, taskID string) string {
	requests, err := state.ApprovalRequests()
	if err != nil {
		return ""
	}
	for _, rec := range requests {
		if rec.EntityID != taskID {
			continue
		}
		if latest, count := core.LatestApprovalRequest(requests, rec.ID); count > 0 && latest.Transition == core.ApprovalApproved {
			return rec.ID
		}
	}
	return ""
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
