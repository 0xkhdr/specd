package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates/security"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// gatherHistory projects a spec's audit trail from every on-disk record source
// into a single ordered slice (spec 13 R1/R2). It opens files read-only and
// writes nothing; the caller sorts and renders. Sources that a spec has not
// exercised (no submissions, no ACP ledger) simply contribute no events —
// graceful degradation, no placeholder lines.
func gatherHistory(root, slug string, model core.ReportModel) ([]core.HistoryEvent, error) {
	var events []core.HistoryEvent

	// 1. Stamped state.json records: approvals, decisions, mid-requirement notes.
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return nil, err
	}
	for seq, key := range sortedRecordKeys(state.Records) {
		var rec core.Record
		if err := json.Unmarshal(state.Records[key], &rec); err != nil {
			return nil, err
		}
		event := core.HistoryEvent{
			Timestamp: rec.Timestamp,
			Actor:     rec.Actor,
			Event:     rec.Kind,
			GitHead:   rec.GitHead,
			Seq:       seq,
		}
		switch rec.Kind {
		case "approval":
			event.SourceRank = core.HistorySourceApproval
			event.Reference = "gate=" + rec.Gate
		case "decision":
			event.SourceRank = core.HistorySourceDecision
			event.Reference = recordSummary(rec)
		case "midreq":
			event.SourceRank = core.HistorySourceMidReq
			event.Reference = recordSummary(rec)
		default:
			event.SourceRank = core.HistorySourceDecision
			event.Reference = recordSummary(rec)
		}
		events = append(events, event)
	}

	// 2. Verify evidence (every attempt, in append order) and completions.
	attempts, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return nil, err
	}
	for seq, rec := range attempts {
		verdict := "pass"
		if rec.ExitCode != 0 {
			verdict = "fail"
		}
		events = append(events, core.HistoryEvent{
			Timestamp:  rec.Timestamp,
			Actor:      rec.Actor,
			Event:      "verify:" + verdict,
			Reference:  "task=" + rec.TaskID,
			GitHead:    rec.GitHead,
			SourceRank: core.HistorySourceVerify,
			Seq:        seq,
			TaskID:     rec.TaskID,
		})
	}
	// A completion is a task now marked complete; its provenance is the passing
	// verify record (last-write-wins per task), so no separate store is needed.
	latest, err := core.LoadEvidence(core.EvidencePath(root, slug))
	if err != nil {
		return nil, err
	}
	for seq, task := range model.Tasks {
		if task.Status != core.TaskComplete {
			continue
		}
		rec := latest[task.ID]
		events = append(events, core.HistoryEvent{
			Timestamp:  rec.Timestamp,
			Actor:      rec.Actor,
			Event:      "completion",
			Reference:  "task=" + task.ID,
			GitHead:    rec.GitHead,
			SourceRank: core.HistorySourceCompletion,
			Seq:        seq,
			TaskID:     task.ID,
		})
	}

	// 3. Acceptance-criterion evidence ledger (spec 04).
	criteria, err := core.LoadCriteria(core.CriteriaPath(root, slug))
	if err != nil {
		return nil, err
	}
	for seq, rec := range criteria {
		events = append(events, core.HistoryEvent{
			Timestamp:  rec.Timestamp,
			Actor:      rec.Actor,
			Event:      "criterion:" + rec.Status,
			Reference:  "criterion=" + rec.Criterion,
			GitHead:    rec.GitHead,
			SourceRank: core.HistorySourceCriterion,
			Seq:        seq,
		})
	}

	// 4. Submission ledger (spec 08).
	submissions, err := core.LoadSubmissions(core.SubmissionsPath(root, slug))
	if err != nil {
		return nil, err
	}
	for seq, rec := range submissions {
		events = append(events, core.HistoryEvent{
			Timestamp:  rec.Timestamp,
			Actor:      rec.Actor,
			Event:      "submission",
			Reference:  fmt.Sprintf("exit=%d command=%q", rec.Exit, rec.Command),
			GitHead:    rec.GitHead,
			SourceRank: core.HistorySourceSubmission,
			Seq:        seq,
		})
	}

	// 5. ACP ledger (opt-in brain): claims, reports, dispatches.
	acp, err := orchestration.ReadACP(filepath.Join(core.SpecdDir(root), "specs", slug, "acp.jsonl"))
	if err != nil {
		return nil, err
	}
	for _, e := range acp {
		events = append(events, core.HistoryEvent{
			Timestamp:  e.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Event:      "acp:" + e.Kind,
			Reference:  acpReference(e),
			SourceRank: core.HistorySourceACP,
			Seq:        e.Seq,
			TaskID:     e.TaskID,
		})
	}

	// 6. Governed exception ledger: retain every lifecycle record while marking
	// current suppressions active. Projection exposes governance metadata only.
	exceptions, exceptionFindings := security.LoadExceptions(root, gitHead(root), "production", time.Now().UTC())
	if len(exceptionFindings) != 0 {
		return nil, fmt.Errorf("load security exceptions: %s", exceptionFindings[0].Excerpt)
	}
	for seq, e := range exceptions.Records {
		status := "historical"
		if e.Action == "suppress" && exceptions.Allows(e.Finding) {
			status = "active"
		}
		events = append(events, core.HistoryEvent{
			Timestamp: e.IssuedAt, Event: "exception:" + status,
			Reference: fmt.Sprintf("finding=%s ticket=%s owner=%s scope=%s policy=%s", e.Finding, e.Ticket, e.Owner, e.Scope, exceptions.Digest),
			GitHead:   e.Revision, SourceRank: core.HistorySourceACP, Seq: seq,
		})
	}

	return events, nil
}

// gatherLifecycleProof assembles the deterministic R8.2 proof: requirement
// coverage, stale approval records, amendments, and escaped-defect links. It
// reads only on-disk state and writes nothing.
func gatherLifecycleProof(root, slug string) (core.LifecycleProof, error) {
	coverage, err := criterionCoverage(root, slug)
	if err != nil {
		return core.LifecycleProof{}, err
	}
	proofCoverage := make([]core.ProofCoverage, len(coverage))
	for i, c := range coverage {
		proofCoverage[i] = core.ProofCoverage{Req: c.Req, Passing: c.Passing, Total: c.Total}
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return core.LifecycleProof{}, err
	}
	freshness, err := state.StateFreshness()
	if err != nil {
		return core.LifecycleProof{}, err
	}
	amendments, err := state.Amendments()
	if err != nil {
		return core.LifecycleProof{}, err
	}
	return core.BuildLifecycleProof(slug, proofCoverage, freshness.Stale, amendments), nil
}

// gatherPrometheus assembles the metric snapshot from the same sources report
// already reads: task statuses, verify evidence, criterion coverage, telemetry.
func gatherPrometheus(root, slug string, model core.ReportModel) (core.PrometheusMetrics, error) {
	metrics := core.PrometheusMetrics{
		Slug:          slug,
		TasksByStatus: map[string]int{},
	}
	for _, task := range model.Tasks {
		metrics.TasksByStatus[string(task.Status)]++
	}

	attempts, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return core.PrometheusMetrics{}, err
	}
	metrics.VerifyAttempts = len(attempts)
	for _, rec := range attempts {
		if rec.ExitCode != 0 {
			metrics.VerifyFailures++
		}
	}

	coverage, err := criterionCoverage(root, slug)
	if err != nil {
		return core.PrometheusMetrics{}, err
	}
	for _, req := range coverage {
		metrics.CriteriaTotal += req.Total
		metrics.CriteriaPassing += req.Passing
	}

	telemetry, err := aggregateTelemetry(root, slug, model)
	if err != nil {
		return core.PrometheusMetrics{}, err
	}
	metrics.Tokens = telemetry.Tokens
	metrics.Cost = telemetry.Cost
	metrics.DurationMs = telemetry.DurationMs

	return metrics, nil
}

// recordSummary renders a decision/midreq reference: its scope (when set) and a
// bounded slice of its text, kept short so the history stays one line per event.
func recordSummary(rec core.Record) string {
	text := rec.Text
	if len(text) > 60 {
		text = text[:57] + "..."
	}
	if rec.Scope != "" {
		return fmt.Sprintf("scope=%s %s", rec.Scope, text)
	}
	return text
}

func acpReference(e orchestration.ACPEvent) string {
	if e.AuditID > 0 {
		return fmt.Sprintf("run=%s mission=%s task=%s policy=%s stage=%s audit_id=%d", e.RunID, e.MissionID, e.TaskID, e.PolicyDigest, e.AuditKind, e.AuditID)
	}
	if e.TaskID == "" {
		return ""
	}
	return "task=" + e.TaskID
}

// sortedRecordKeys returns the state.json record keys in a stable order so the
// per-key Seq assigned to each event is deterministic across runs (spec 13 R3).
func sortedRecordKeys(records map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
