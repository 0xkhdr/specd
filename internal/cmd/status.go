package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	"github.com/0xkhdr/specd/internal/orchestration"
)

func runStatus(root string, args []string, flags map[string]string) error {
	if flagEnabled(flags, "program") {
		if len(args) != 0 {
			return errors.New("usage: specd status --program (takes no spec)")
		}
		view, err := renderProgram(root)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, view)
		return nil
	}
	if flagEnabled(flags, "guide") {
		if len(args) != 1 {
			return usageError("status")
		}
		return emitGuidance(root, args[0], flagEnabled(flags, "json"))
	}
	if len(args) != 1 {
		return usageError("status")
	}
	model, err := reportModel(root, args[0])
	if err != nil {
		return err
	}
	coverage, err := criterionCoverage(root, args[0])
	if err != nil {
		return err
	}
	spec, err := loadSpec(root, args[0])
	if err != nil {
		return err
	}
	escalated, ratchetActive, err := escalatedAdvisory(root, args[0], spec.Tasks)
	if err != nil {
		return err
	}
	// Clarifications are persisted readiness facts, so status projects them
	// through the one readiness owner rather than re-deriving waits (R4.1).
	specState, err := core.LoadState(core.StatePath(root, args[0]))
	if err != nil {
		return err
	}
	if err := core.ApplyClarificationReadiness(&model, spec.Tasks, taskStatus(spec.Tasks), specState.Records); err != nil {
		return err
	}
	approvals, err := approvalRequestStates(root, args[0], specState)
	if err != nil {
		return err
	}
	// Reopened artifacts carry a draft version beyond 1; the ledger is the only
	// counter, so status projects it rather than storing a second one (R4.1).
	events, err := core.ReadWorkflowEvents(core.WorkflowEventPath(root, args[0]))
	if err != nil {
		return err
	}
	revisions := core.ArtifactVersions(events)
	stale := core.StaleDescendants(events)
	waitingApproval := waitingApprovalGate(root, args[0])
	if flagEnabled(flags, "json") {
		// Records are projected verbatim (RawMessage), never re-synthesized, so
		// decision/midreq text/scope/actor/timestamp round-trip exactly (R3.4).
		guidance, err := guidanceForSpec(root, args[0])
		if err != nil {
			return err
		}
		review, err := statusReview(root, args[0])
		if err != nil {
			return err
		}
		return writeJSON(struct {
			core.ReportModel
			Mode             core.Mode                    `json:"mode"`
			Records          map[string]json.RawMessage   `json:"records,omitempty"`
			Criteria         []requirementCoverage        `json:"criteria,omitempty"`
			Escalated        map[string]int               `json:"escalated,omitempty"`
			ApprovalRequests []gates.ApprovalRequestState `json:"approval_requests,omitempty"`
			Cycle            int                          `json:"cycle,omitempty"`
			ArtifactVersions map[string]int               `json:"artifact_versions,omitempty"`
			StaleDescendants []core.StaleDescendant       `json:"stale_descendants,omitempty"`
			WaitingApproval  string                       `json:"waiting_approval,omitempty"`
			Review           *core.ReviewReport           `json:"review,omitempty"`
			Locator          core.Locator                 `json:"locator"`
		}{model, specState.Mode, specState.Records, coverage, escalated, approvals, specState.Cycle, revisions, stale, waitingApproval, review,
			core.NewLocator(args[0], specState.Revision, guidance, core.ActorAgent, core.AuthorityNone, core.HostCapabilities{})})
	}
	fmt.Fprintf(os.Stdout, "mode: %s\n", specState.Mode)
	fmt.Fprint(os.Stdout, core.RenderStatus(model))
	fmt.Fprint(os.Stdout, renderArtifactVersions(specState.Cycle, revisions))
	fmt.Fprint(os.Stdout, renderStaleDescendants(stale))
	fmt.Fprint(os.Stdout, renderCriterionCoverage(coverage))
	fmt.Fprint(os.Stdout, renderEscalated(escalated, ratchetActive))
	fmt.Fprint(os.Stdout, renderApprovalRequests(approvals))
	fmt.Fprint(os.Stdout, renderWaitingApproval(waitingApproval))
	return nil
}

func statusReview(root, slug string) (*core.ReviewReport, error) {
	path := core.ReviewReportPath(root, slug)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read review report %s: %w", path, err)
	}
	report, err := core.ParseReviewReport(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse review report %s: %w", path, err)
	}
	return &report, nil
}

// waitingApprovalGate reports the lifecycle gate a controller session halted
// on, or "" when there is no session or it is not waiting (R4.1). A missing or
// unreadable session is simply not a halt: status never invents one.
func waitingApprovalGate(root, slug string) string {
	session, err := orchestration.LoadSession(filepath.Join(core.SpecdDir(root), "specs", slug, "session.json"))
	if err != nil {
		return ""
	}
	return session.WaitingApproval
}

// renderWaitingApproval adds one section, and only when a controller is
// actually halted — every other spec's status output is byte-identical to
// before.
func renderWaitingApproval(gate string) string {
	if gate == "" {
		return ""
	}
	return "\nController: waiting_approval at the " + gate + " gate\n" +
		"  approve with `specd approve <spec>`, or with an operator-issued grant via `specd delegate approve <spec> --grant <id> --token <bearer>`\n"
}

// approvalRequestStates projects the immutable approval requests in state
// against the identities that are current on disk (R5.3/R5.4). Only the
// artifact and config digests are re-read: the pinned state revision is the
// pre-approval one by construction, and the transition-plan digest only exists
// inside an approval attempt, so both are carried through unchanged rather than
// reported as permanent drift. Whatever status cannot recompute is still
// checked by core.PlanApprovalRequest when an approval is actually written.
func approvalRequestStates(root, slug string, state core.State) ([]gates.ApprovalRequestState, error) {
	requests, err := state.ApprovalRequests()
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, nil
	}
	cfg, _ := core.LoadConfig(configPaths(root), getenv())
	configDigest := core.ConfigDigest(cfg)
	current := make(map[string]core.ApprovalPins, len(requests))
	for _, rec := range requests {
		pins := rec.Pins
		pins.ConfigDigest = configDigest
		pins.ArtifactDigest = "none"
		if artifact := approvalArtifact(rec.EntityVersion); artifact != "" {
			if raw, readErr := os.ReadFile(filepath.Join(core.SpecdDir(root), "specs", slug, artifact)); readErr == nil {
				pins.ArtifactDigest = core.Digest(raw)
			}
		}
		current[rec.ID] = pins
	}
	return gates.ApprovalRequestStates(requests, current, core.Clock()), nil
}

// renderApprovalRequests formats the approval-request identity section for
// `status` text output: one deterministic line per request naming the id, the
// current transition, and the entity, plus the gate's refusal text for any
// request whose pinned inputs no longer hold.
func renderApprovalRequests(states []gates.ApprovalRequestState) string {
	if len(states) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nApproval requests:\n")
	for _, state := range states {
		fmt.Fprintf(&b, "  %s — %s (%s) expires %s\n", state.ID, state.State, state.Entity, state.ExpiresAt)
	}
	for _, finding := range gates.ApprovalRequestFindings(states) {
		fmt.Fprintf(&b, "  %s: %s\n", finding.Severity, finding.Message)
	}
	return b.String()
}

// renderArtifactVersions formats the reopened-revision section: the lifecycle
// cycle and every artifact that carries a draft version beyond its first. Both
// are silent on a spec that was never reopened.
func renderArtifactVersions(cycle int, versions map[string]int) string {
	if len(versions) == 0 && cycle < 2 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nReopened:\n")
	if cycle > 1 {
		fmt.Fprintf(&b, "  spec — lifecycle cycle %d\n", cycle)
	}
	for _, artifact := range slices.Sorted(maps.Keys(versions)) {
		fmt.Fprintf(&b, "  %s.md — draft version %d (prior revisions under revisions/%s/)\n", artifact, versions[artifact], artifact)
	}
	return b.String()
}

// renderStaleDescendants formats the stale-descendant section: one line per
// completed descendant a reopen invalidated, naming its parent, the revision it
// went stale at, and the resolutions that are allowed to clear it — followed by
// the gate's refusal for every one still unresolved (R5.1, R5.4). Silent on a
// spec that never reopened a task.
func renderStaleDescendants(stale []core.StaleDescendant) string {
	if len(stale) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nStale descendants (completed; explicit resolution required):\n")
	for _, entry := range stale {
		if entry.Unresolved() {
			fmt.Fprintf(&b, "  %s — stale since revision %d (reopen of %s); resolve with: %s\n",
				entry.TaskID, entry.StaleSinceRevision, entry.Parent, strings.Join(entry.Choices, ", "))
			continue
		}
		fmt.Fprintf(&b, "  %s — resolved as %s at revision %d\n", entry.TaskID, entry.Resolution, entry.ResolvedRevision)
	}
	for _, finding := range gates.StaleDescendantFindings(stale) {
		fmt.Fprintf(&b, "  %s: %s\n", finding.Severity, finding.Message)
	}
	return b.String()
}

// renderEscalated formats the escalated-task section for `status` text output.
// When the ratchet is active the tasks are genuinely blocked; when disabled the
// section is advisory (repeated failures still surfaced, spec 06 R2/R6).
func renderEscalated(escalated map[string]int, ratchetActive bool) string {
	if len(escalated) == 0 {
		return ""
	}
	header := "Escalated (advisory; ratchet disabled):"
	if ratchetActive {
		header = "Escalated (blocked — clear with `specd task <id> --override --reason <text>`):"
	}
	var b strings.Builder
	b.WriteString("\n" + header + "\n")
	for _, id := range slices.Sorted(maps.Keys(escalated)) {
		fmt.Fprintf(&b, "  %s — %d consecutive verify failures\n", id, escalated[id])
	}
	return b.String()
}

// emitGuidance writes a spec's machine driving guidance (spec 01 R6). With
// asJSON the guidance round-trips as the core.Guidance contract; otherwise it is
// a compact human summary. The separation of legal commands from human-only
// actions (R6.1) and the suppression of task verify without an executable task
// (R6.2) are computed by guidanceForSpec — this function only renders.
func emitGuidance(root, slug string, asJSON bool) error {
	g, err := guidanceForSpec(root, slug)
	if err != nil {
		return err
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return err
	}
	g.Mode = state.Mode
	criterionBlockers, err := criterionGuidanceBlockers(root, slug, state)
	if err != nil {
		return err
	}
	g.Blockers = append(g.Blockers, criterionBlockers...)
	for i, blocker := range g.Blockers {
		g.Blockers[i] = actionableGateMessage(slug, gates.Finding{Gate: "context-budget", Message: blocker})
	}
	if asJSON {
		// Additive: the Guidance fields stay at the top level exactly where they
		// were, and `locator` is a new sibling key. A consumer that predates it
		// still parses this response unchanged (R5.1).
		return writeJSON(struct {
			core.Guidance
			Locator core.Locator `json:"locator"`
		}{g, core.NewLocator(slug, state.Revision, g, core.ActorAgent, core.AuthorityNone, core.HostCapabilities{})})
	}
	fmt.Fprintf(os.Stdout, "phase: %s (status %s, mode %s)\n", g.Phase, g.Status, g.Mode)
	if g.RequiredArtifact != "" {
		fmt.Fprintf(os.Stdout, "required artifact: %s\n", g.RequiredArtifact)
	}
	if g.NextGate != "" {
		fmt.Fprintf(os.Stdout, "next gate (human approval): %s\n", g.NextGate)
	}
	fmt.Fprintf(os.Stdout, "legal commands: %s\n", strings.Join(g.LegalCommands, ", "))
	fmt.Fprintf(os.Stdout, "human-only: %s\n", strings.Join(g.HumanOnly, ", "))
	for _, handoff := range g.Handoffs {
		fmt.Fprintf(os.Stdout, "handoff: %s requires %s", handoff.Operation, handoff.Actor)
		if handoff.MissingAuthority != "" {
			fmt.Fprintf(os.Stdout, " authority (%s)", handoff.MissingAuthority)
		}
		if handoff.Command != "" {
			fmt.Fprintf(os.Stdout, "; %s", handoff.Command)
		}
		fmt.Fprintln(os.Stdout)
	}
	for _, blocker := range g.RouteBlockers {
		fmt.Fprintf(os.Stdout, "route blocker: %s %s missing %s\n", blocker.Code, blocker.Operation, blocker.Missing)
	}
	for _, blocker := range g.Blockers {
		fmt.Fprintf(os.Stdout, "blocker: %s\n", blocker)
	}
	return nil
}

func criterionGuidanceBlockers(root, slug string, state core.State) ([]string, error) {
	spec, err := loadSpec(root, slug)
	if err != nil {
		return nil, err
	}
	requirements, err := os.ReadFile(filepath.Join(core.SpecdDir(root), "specs", slug, "requirements.md"))
	if err != nil {
		return nil, err
	}
	records, err := core.LoadCriteria(core.CriteriaPath(root, slug))
	if err != nil {
		return nil, err
	}
	passing := core.CurrentPassing(records, requirementsApprovedAt(root, slug))
	status := taskStatus(spec.Tasks)
	for id, current := range state.TaskStatus {
		status[id] = current
	}
	operation, ok := core.OperationByID("verify.criterion")
	if !ok {
		return nil, errors.New("verify.criterion operation missing from palette")
	}

	seen := map[string]bool{}
	var blockers []string
	for _, task := range spec.Tasks {
		if status[task.ID] != core.TaskComplete {
			continue
		}
		for _, criterion := range gates.CriterionIDs(string(requirements)) {
			id, ref := criterion.String(), "R"+criterion.String()
			if passing[id] || seen[id] || !taskReferencesCriterion(task, ref) {
				continue
			}
			seen[id] = true
			command := strings.NewReplacer("<slug>", slug, "<r>.<n>", id).Replace(operation.Usage)
			blockers = append(blockers, fmt.Sprintf("criterion %s lacks current passing evidence; run `%s`", ref, command))
		}
	}
	return blockers, nil
}

func taskReferencesCriterion(task core.TaskRow, ref string) bool {
	if slices.Contains(task.Refs, ref) {
		return true
	}
	for _, token := range strings.FieldsFunc(task.Acceptance, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.'
	}) {
		if strings.EqualFold(token, ref) {
			return true
		}
	}
	return false
}
