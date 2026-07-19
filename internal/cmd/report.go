package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	contextpkg "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates/security"
	"github.com/0xkhdr/specd/internal/orchestration"
)

type outcomeInput struct {
	SpecID       string   `json:"spec_id"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	FeedbackRefs []string `json:"feedback_refs,omitempty"`
}

type outcomeRow struct {
	SpecID       string   `json:"spec_id"`
	Outcome      string   `json:"outcome"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	FeedbackRefs []string `json:"feedback_refs,omitempty"`
}

type portfolioExportSpec struct {
	ID           string   `json:"id"`
	Status       string   `json:"status"`
	Risk         string   `json:"risk"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type portfolioExportLink struct {
	From string        `json:"from"`
	To   string        `json:"to"`
	Kind core.LinkKind `json:"kind"`
}

type portfolioExport struct {
	SchemaVersion int                   `json:"schema_version"`
	Specs         []portfolioExportSpec `json:"specs"`
	Links         []portfolioExportLink `json:"links,omitempty"`
	View          core.PortfolioView    `json:"delivery"`
}

func renderOutcomeReview(inputs []outcomeInput) string {
	rows := append([]outcomeInput(nil), inputs...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].SpecID < rows[j].SpecID })
	out := make([]outcomeRow, 0, len(rows))
	for _, row := range rows {
		sort.Strings(row.EvidenceRefs)
		sort.Strings(row.FeedbackRefs)
		outcome := "unknown"
		if len(row.FeedbackRefs) > 0 {
			outcome = "observed"
		}
		out = append(out, outcomeRow{SpecID: row.SpecID, Outcome: outcome, EvidenceRefs: row.EvidenceRefs, FeedbackRefs: row.FeedbackRefs})
	}
	raw, _ := json.Marshal(out)
	var compact bytes.Buffer
	_ = json.Compact(&compact, raw)
	return compact.String() + "\n"
}

func renderDeliveryReport(records []core.DeploymentV1) string {
	rows := append([]core.DeploymentV1(nil), records...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DeploymentID != rows[j].DeploymentID {
			return rows[i].DeploymentID < rows[j].DeploymentID
		}
		return rows[i].Attempt < rows[j].Attempt
	})
	var b strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&b, "deployment=%s attempt=%d release=%s environment=%s status=%s adapter=%s source=%s\n", row.DeploymentID, row.Attempt, row.ReleaseID, row.Environment, row.Status, row.Adapter, row.AdapterTrustSource)
	}
	return b.String()
}

func gatherProgramEconomics(root string) (core.ProgramEconomics, error) {
	dir := filepath.Join(core.SpecdDir(root), "specs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return core.ProgramEconomics{}, err
	}
	inputs := make([]core.SpecEconomics, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		model, err := reportModel(root, slug)
		if err != nil {
			return core.ProgramEconomics{}, err
		}
		report, err := aggregateTelemetry(root, slug, model)
		if err != nil {
			return core.ProgramEconomics{}, err
		}
		records, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
		if err != nil {
			return core.ProgramEconomics{}, err
		}
		input := core.SpecEconomics{SpecID: slug}
		for i, record := range records {
			if record.Telemetry != nil {
				input.SourceRefs = append(input.SourceRefs, fmt.Sprintf("evidence:%s:%d", slug, i+1))
			}
		}
		if len(input.SourceRefs) > 0 {
			input.Telemetry = &report
		}
		inputs = append(inputs, input)
	}
	return core.RollupEconomics(inputs, "")
}

// gatherContextEfficiency joins load-plan estimates with attempt telemetry.
// All inputs are existing local files; absent host/provider measurements stay
// nil and render as "unknown", never as a fabricated zero.
func gatherContextEfficiency(root, slug string, model core.ReportModel) (string, error) {
	spec, err := loadSpec(root, slug)
	if err != nil {
		return "", err
	}
	records, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return "", err
	}
	byTask := make(map[string][]core.EvidenceRecord)
	for _, record := range records {
		byTask[record.TaskID] = append(byTask[record.TaskID], record)
	}
	report := contextpkg.EfficiencyReport{SchemaVersion: contextpkg.EfficiencySchemaV1, SpecID: slug}
	for _, task := range model.Tasks {
		row := contextpkg.TaskEfficiency{TaskID: task.ID, FirstPassResult: "unknown"}
		manifest, buildErr := contextpkg.BuildManifest(root, slug, spec.Tasks, task.ID, contextBudget(root))
		if buildErr != nil {
			return "", fmt.Errorf("build context estimate for %s: %w", task.ID, buildErr)
		}
		estimated := manifest.EstimatedTokens
		row.EstimatedInputTokens = &estimated
		row.OmittedItems = manifest.Omissions
		attempts := byTask[task.ID]
		if len(attempts) > 0 {
			if attempts[0].ExitCode == 0 {
				row.FirstPassResult = "pass"
			} else {
				row.FirstPassResult = "fail"
			}
			row.RetryCount = len(attempts) - 1
			var input, duration int
			hasInput, hasDuration, hasCost := false, false, false
			for _, attempt := range attempts {
				if attempt.Telemetry == nil {
					continue
				}
				if attempt.Telemetry.InputTokens > 0 || attempt.Telemetry.EnvelopeVersion != "" {
					input += attempt.Telemetry.InputTokens
					hasInput = true
				}
				if attempt.Telemetry.DurationMs > 0 || attempt.Telemetry.EnvelopeVersion != "" {
					duration += attempt.Telemetry.DurationMs
					hasDuration = true
				}
				if attempt.Telemetry.Cost != "" {
					hasCost = true
				}
			}
			if hasInput {
				row.ActualInputTokens = &input
			}
			if hasDuration {
				row.DurationMS = &duration
			}
			if hasCost {
				cost := core.AggregateTelemetry(attempts, []string{task.ID}).Cost
				row.Cost = &cost
			}
		}
		report.Tasks = append(report.Tasks, row)
	}
	return contextpkg.RenderEfficiency(report)
}

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
	provenance, err := core.LoadProvenance(core.ProvenancePath(root, slug))
	if err != nil {
		return nil, err
	}
	if provenance != nil {
		events = append(events, core.HistoryEvent{
			Event:      "provenance",
			Reference:  provenanceHistoryReference(*provenance),
			SourceRank: core.HistorySourceProvenance,
		})
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

func provenanceHistoryReference(p core.ProvenanceV1) string {
	parts := []string{fmt.Sprintf("schema_version=%d", p.SchemaVersion), "source_type=" + string(p.SourceType)}
	appendScalar := func(name, value string) {
		if value != "" {
			parts = append(parts, name+"="+value)
		}
	}
	appendList := func(name string, values []string) {
		if len(values) != 0 {
			parts = append(parts, name+"="+strings.Join(values, ","))
		}
	}
	appendScalar("source_ref", p.SourceRef)
	appendList("systems", p.Systems)
	appendList("affected_specs", p.AffectedSpecs)
	appendScalar("severity", p.Severity)
	appendScalar("risk", p.Risk)
	appendScalar("owner", p.Owner)
	links := make([]string, 0, len(p.PriorLinks))
	for _, link := range p.PriorLinks {
		links = append(links, link.To)
	}
	appendList("prior_links", links)
	return strings.Join(parts, " ")
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
	deployments, err := core.ReadDeployments(core.DeploymentLedgerPath(root, slug))
	if err != nil {
		return core.PrometheusMetrics{}, err
	}
	if len(deployments) > 0 {
		metrics.DeliveryBySource = make(map[string]int)
		for _, deployment := range deployments {
			metrics.DeliveryBySource[string(deployment.AdapterTrustSource)]++
		}
	}

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

func runReport(root string, args []string, flags map[string]string) error {
	if len(args) == 0 && flagEnabled(flags, "portfolio") {
		program, err := core.LoadProgram(core.ProgramPath(root))
		if err != nil {
			return err
		}
		var inputs []core.PortfolioSpec
		export := portfolioExport{SchemaVersion: 1}
		for _, slug := range core.ListSpecs(root) {
			state, err := core.LoadState(core.StatePath(root, slug))
			if err != nil {
				return err
			}
			deployments, err := core.ReadDeployments(core.DeploymentLedgerPath(root, slug))
			if err != nil {
				return err
			}
			inputs = append(inputs, core.PortfolioSpec{SpecID: slug, Complete: state.Status == core.StatusComplete, Deployments: deployments})
			risk := "unknown"
			if provenance, err := core.LoadProvenance(core.ProvenancePath(root, slug)); err != nil {
				return err
			} else if provenance != nil && provenance.Risk != "" {
				risk = provenance.Risk
			}
			records, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
			if err != nil {
				return err
			}
			refs := make([]string, 0, len(records))
			for i, record := range records {
				refs = append(refs, fmt.Sprintf("verify:%s:%d:%s", record.TaskID, i+1, record.GitHead))
			}
			export.Specs = append(export.Specs, portfolioExportSpec{ID: slug, Status: string(state.Status), Risk: risk, EvidenceRefs: refs})
		}
		view, err := core.BuildPortfolioView(program, inputs)
		if err != nil {
			return err
		}
		links := append([]core.ProgramLink(nil), program.Links...)
		sort.Slice(links, func(i, j int) bool {
			if links[i].From != links[j].From {
				return links[i].From < links[j].From
			}
			if links[i].To != links[j].To {
				return links[i].To < links[j].To
			}
			return links[i].Kind < links[j].Kind
		})
		for _, link := range links {
			export.Links = append(export.Links, portfolioExportLink{From: link.From, To: link.To, Kind: link.Kind})
		}
		export.View = view
		return writeJSON(export)
	}
	if len(args) != 1 {
		return errors.New("usage: report slug [--pr|--metrics|--efficiency|--rollup|--delivery|--json|--history|--proof|--trace|--format prometheus|event] | report --portfolio")
	}
	model, err := reportModel(root, args[0])
	if err != nil {
		return err
	}
	if flagEnabled(flags, "outcome-review") {
		records, err := core.LoadEvidenceRecords(core.EvidencePath(root, args[0]))
		if err != nil {
			return err
		}
		refs := make([]string, 0, len(records))
		for i, record := range records {
			refs = append(refs, fmt.Sprintf("verify:%s:%d:%s", record.TaskID, i+1, record.GitHead))
		}
		_, err = fmt.Fprint(os.Stdout, renderOutcomeReview([]outcomeInput{{SpecID: args[0], EvidenceRefs: refs}}))
		return err
	}
	if flagEnabled(flags, "delivery") {
		records, err := core.ReadDeployments(core.DeploymentLedgerPath(root, args[0]))
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, renderDeliveryReport(records))
		return nil
	}
	if flagEnabled(flags, "rollup") {
		rollup, err := gatherProgramEconomics(root)
		if err != nil {
			return err
		}
		return writeJSON(rollup)
	}
	// --proof emits the deterministic R8.2 lifecycle proof: requirement-to-evidence
	// coverage, stale records, amendments, and escaped-defect links. Pure projection
	// of on-disk state; honours --json for a machine-readable object.
	if flagEnabled(flags, "proof") {
		proof, err := gatherLifecycleProof(root, args[0])
		if err != nil {
			return err
		}
		if flagEnabled(flags, "json") {
			out, err := core.RenderLifecycleProofJSON(proof)
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, out)
			return nil
		}
		fmt.Fprint(os.Stdout, core.RenderLifecycleProof(proof))
		return nil
	}
	// --history replays the spec's audit trail from existing records (spec 13);
	// it writes nothing and honours --json for machine-readable JSON Lines (R6).
	if flagEnabled(flags, "history") {
		events, err := gatherHistory(root, args[0], model)
		if err != nil {
			return err
		}
		if flagEnabled(flags, "json") {
			out, err := core.RenderHistoryJSON(events)
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, out)
			return nil
		}
		fmt.Fprint(os.Stdout, core.RenderHistory(args[0], events))
		return nil
	}
	// --trace exports the spec's metadata-only run trace as stable JSON Lines
	// (spec 07 R6); a pure projection of on-disk records that writes nothing.
	if flagEnabled(flags, "trace") {
		out, err := runTrace(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	}
	if flagEnabled(flags, "efficiency") {
		out, err := gatherContextEfficiency(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	}
	// --format prometheus emits a textfile-collector exposition (spec 13 R4).
	if flags["format"] == "prometheus" {
		metrics, err := gatherPrometheus(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, core.RenderPrometheus(metrics))
		return nil
	}
	if flags["format"] == "event" {
		out, err := runEvents(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	}
	if format, ok := flags["format"]; ok && format != "" {
		return fmt.Errorf("%w: unsupported --format %q (only prometheus, event)", ErrUsage, format)
	}
	coverage, err := criterionCoverage(root, args[0])
	if err != nil {
		return err
	}
	switch {
	case flagEnabled(flags, "json"):
		return writeJSON(struct {
			core.ReportModel
			Criteria []requirementCoverage `json:"criteria,omitempty"`
		}{model, coverage})
	case flagEnabled(flags, "metrics"):
		fmt.Fprint(os.Stdout, core.RenderMetrics(model))
		telemetry, err := aggregateTelemetry(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, core.RenderTelemetry(args[0], telemetry))
	case flagEnabled(flags, "pr"):
		fmt.Fprint(os.Stdout, core.PRSummary(model))
	default:
		fmt.Fprint(os.Stdout, core.RenderStatus(model))
		fmt.Fprint(os.Stdout, renderCriterionCoverage(coverage))
	}
	return nil
}

func reportModel(root, slug string) (core.ReportModel, error) {
	spec, err := loadSpec(root, slug)
	if err != nil {
		return core.ReportModel{}, err
	}
	evidence, err := core.LoadEvidence(core.EvidencePath(root, slug))
	if err != nil {
		return core.ReportModel{}, err
	}
	return core.BuildReportModel(slug, spec.Tasks, taskStatus(spec.Tasks), evidence), nil
}
