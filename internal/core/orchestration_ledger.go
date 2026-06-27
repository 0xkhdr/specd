package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// AppendContextLedger appends an entry to the session's context ledger and
// recomputes the peak-token high-water mark. It is pure (no IO): callers persist
// the mutated session through the normal save path so the write is atomic and
// serialized by the session lock.
func AppendContextLedger(session *OrchestrationSession, entry ContextLedgerEntry) {
	if session == nil {
		return
	}
	session.ContextLedger = append(session.ContextLedger, entry)
	RecomputePeakTokens(session)
}

// RecomputePeakTokens sets session.PeakTokens to the maximum estimated- or
// host-reported token count seen across the whole ledger. It is monotonic by
// construction (it scans the full trail) so recomputing twice is a no-op.
func RecomputePeakTokens(session *OrchestrationSession) {
	if session == nil {
		return
	}
	peak := 0
	for _, entry := range session.ContextLedger {
		if entry.EstimatedTokens > peak {
			peak = entry.EstimatedTokens
		}
		if entry.HostReportedTokens > peak {
			peak = entry.HostReportedTokens
		}
	}
	session.PeakTokens = peak
}

// lastContextLedgerEntry returns the most recent ledger entry and whether one
// exists. It is the tail the decision engine reads for the budget-threshold
// trigger.
func lastContextLedgerEntry(session OrchestrationSession) (ContextLedgerEntry, bool) {
	if len(session.ContextLedger) == 0 {
		return ContextLedgerEntry{}, false
	}
	return session.ContextLedger[len(session.ContextLedger)-1], true
}

// CompactionOutcome reports a performed compaction: the appended ledger entry,
// the workspace-relative summary path the host can reference, and the
// pre-compaction estimate that motivated it (for observability).
type CompactionOutcome struct {
	Entry              ContextLedgerEntry
	SummaryFile        string
	PreEstimatedTokens int
}

// CompactOrchestrationSession performs a manual compaction (R3/R4): it validates
// the session is running, then renders + persists a phase summary and ledger
// checkpoint under the spec lock. It is the locked entrypoint the CLI calls; the
// decision engine calls performCompaction directly under its own lock.
func CompactOrchestrationSession(root, slug, sessionID, reason string) (CompactionOutcome, error) {
	if reason == "" {
		reason = "manual-clear"
	}
	session, err := LoadOrchestrationSession(root, sessionID)
	if err != nil {
		return CompactionOutcome{}, err
	}
	if session.Status != OrchestrationSessionRunning {
		return CompactionOutcome{}, fmt.Errorf("orchestration compact: session %s is %s, not running", sessionID, session.Status)
	}
	preBudget, preSoft, preEst := 0, 0, 0
	if tail, ok := lastContextLedgerEntry(session); ok {
		preBudget, preSoft, preEst = tail.Budget, tail.SoftCeiling, tail.EstimatedTokens
	}
	var outcome CompactionOutcome
	_, err = WithSpecLock[struct{}](root, slug, func() (struct{}, error) {
		state, err := LoadState(root, slug)
		if err != nil {
			return struct{}{}, err
		}
		if state == nil {
			return struct{}{}, NotFoundError(fmt.Sprintf("spec '%s' not found", slug))
		}
		out, err := performCompaction(root, slug, sessionID, reason, uint64(state.Revision), preBudget, preSoft, preEst)
		if err != nil {
			return struct{}{}, err
		}
		outcome = out
		return struct{}{}, nil
	})
	return outcome, err
}

// performCompaction is the single effecting compaction routine shared by the
// decision engine and the manual CLI. It assumes the caller holds the spec lock
// (SaveState asserts it). It renders a phase summary, writes it atomically under
// the session dir, bumps the spec Turn, and appends a compacted ledger entry
// while advancing LastCompactionStep to the post-compaction revision (so the
// phase-boundary trigger fires at most once per boundary).
func performCompaction(root, slug, sessionID, reason string, stepSeq uint64, preBudget, preSoftCeiling, preEstimated int) (CompactionOutcome, error) {
	loaded, err := LoadSpec(root, slug)
	if err != nil {
		return CompactionOutcome{}, err
	}
	state := loaded.State
	phase := state.Phase
	turn := state.Turn

	summary := renderCompactionSummary(loaded, reason)

	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return CompactionOutcome{}, err
	}
	dir, err := paths.SessionDir(sessionID)
	if err != nil {
		return CompactionOutcome{}, err
	}
	fileName := fmt.Sprintf("compact-%s-%d.md", phase, turn)
	fullPath := filepath.Join(dir, fileName)
	if err := atomicWritePrivate(fullPath, []byte(summary)); err != nil {
		return CompactionOutcome{}, fmt.Errorf("orchestration compact: write summary: %w", err)
	}
	relFile := fullPath
	if rel, err := filepath.Rel(root, fullPath); err == nil {
		relFile = filepath.ToSlash(rel)
	}

	// Bumping Turn advances the spec revision (SaveState CAS + Revision++), giving
	// the next phase boundary a strictly higher step than LastCompactionStep.
	state.Turn++
	if err := SaveState(root, slug, state); err != nil {
		return CompactionOutcome{}, err
	}
	newRevision := uint64(state.Revision)

	entry := ContextLedgerEntry{
		StepSequence: stepSeq,
		Phase:        phase,
		Action:       "compact",
		// After a clear the live context is the summary, not the prior working set,
		// so the entry records the (small) summary estimate. This also settles the
		// budget trigger: the new ledger tail no longer exceeds the threshold.
		EstimatedTokens: EstimateTokensString(summary),
		Budget:          preBudget,
		SoftCeiling:     preSoftCeiling,
		Compacted:       true,
		CompactedAt:     NowISO(),
		Reason:          reason,
	}
	if _, err := updateOrchestrationSession(root, sessionID, func(session *OrchestrationSession) error {
		AppendContextLedger(session, entry)
		session.LastCompactionStep = newRevision
		return nil
	}); err != nil {
		return CompactionOutcome{}, err
	}
	return CompactionOutcome{Entry: entry, SummaryFile: relFile, PreEstimatedTokens: preEstimated}, nil
}

// renderCompactionSummary produces the markdown phase summary the host keeps
// across a `/clear`: the current lifecycle, task DAG status, and any blockers.
// It is deterministic (sorted task ids) so golden comparisons are stable.
func renderCompactionSummary(loaded LoadedSpec, reason string) string {
	state := loaded.State
	var b strings.Builder
	fmt.Fprintf(&b, "# Compaction Summary — %s\n\n", state.Spec)
	fmt.Fprintf(&b, "- Status: %s\n", state.Status)
	fmt.Fprintf(&b, "- Phase: %s\n", state.Phase)
	fmt.Fprintf(&b, "- Turn: %d\n", state.Turn)
	fmt.Fprintf(&b, "- Reason: %s\n\n", reason)

	ids := make([]string, 0, len(state.Tasks))
	for id := range state.Tasks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return taskOrdinalLess(ids[i], ids[j]) })

	done := 0
	fmt.Fprintf(&b, "## Task DAG status\n\n")
	if len(ids) == 0 {
		b.WriteString("_No tasks reconciled into state yet._\n\n")
	} else {
		for _, id := range ids {
			task := state.Tasks[id]
			if task.Status == TaskComplete {
				done++
			}
			verified := ""
			if task.Verification != nil && task.Verification.Verified {
				verified = " (verified)"
			}
			fmt.Fprintf(&b, "- %s [%s]%s\n", id, task.Status, verified)
		}
		fmt.Fprintf(&b, "\n%d/%d tasks complete.\n\n", done, len(ids))
	}

	fmt.Fprintf(&b, "## Blockers\n\n")
	if len(state.Blockers) == 0 {
		b.WriteString("_None._\n")
	} else {
		for _, blocker := range state.Blockers {
			fmt.Fprintf(&b, "- %s: %s\n", blocker.Task, blocker.Reason)
		}
	}
	return b.String()
}

// recordSessionLedgerEntry appends a ledger entry to a persisted session under
// the session lock, atomically. It no-ops when no session exists (base-mode or
// plain-controller dispatch has nowhere to record), so callers on shared paths
// need not branch. Persistence reuses updateOrchestrationSession, so the write
// goes through AtomicWrite + the session lock like every other session mutation.
func recordSessionLedgerEntry(root, sessionID string, entry ContextLedgerEntry) error {
	if _, ok, err := loadOrchestrationSessionIfExists(root, sessionID); err != nil || !ok {
		return err
	}
	_, err := updateOrchestrationSession(root, sessionID, func(session *OrchestrationSession) error {
		AppendContextLedger(session, entry)
		return nil
	})
	if errors.Is(err, errOrchestrationSessionNotFound) {
		// Session vanished between the existence check and the locked update — treat
		// as a no-op rather than failing the dispatch/evidence path it rode in on.
		return nil
	}
	return err
}
