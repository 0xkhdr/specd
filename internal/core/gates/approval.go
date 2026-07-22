package gates

import (
	"fmt"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// ApprovalRequestState is the current identity of one immutable approval
// request (spec 03 R5.4): the id it was opened under, the transition it is in
// now, the entity and the governing inputs it pinned, plus the drift and
// expiry the gate refuses on. It is a projection — nothing here is ever
// written back onto the request records.
type ApprovalRequestState struct {
	ID        string                  `json:"id"`
	State     core.ApprovalTransition `json:"state"`
	Entity    string                  `json:"entity"`
	Pins      core.ApprovalPins       `json:"pins"`
	ExpiresAt string                  `json:"expires_at,omitempty"`
	Drift     []string                `json:"drift,omitempty"`
	Expired   bool                    `json:"expired,omitempty"`
}

// ApprovalRequestStates projects the newest transition of every approval
// request in requests, in request-id order. current supplies the governing
// identities as they are now, keyed by request id; a request absent from it is
// projected without drift (the caller could not read its current inputs).
// Pure: the caller owns the disk reads.
func ApprovalRequestStates(requests []core.ApprovalRequestRecord, current map[string]core.ApprovalPins, now time.Time) []ApprovalRequestState {
	states := make([]ApprovalRequestState, 0, len(requests))
	for _, id := range approvalRequestIDs(requests) {
		latest, _ := core.LatestApprovalRequest(requests, id)
		state := ApprovalRequestState{
			ID:        id,
			State:     latest.Transition,
			Entity:    latest.EntityKind + ":" + latest.EntityID,
			Pins:      latest.Pins,
			ExpiresAt: latest.ExpiresAt,
		}
		if pins, ok := current[id]; ok {
			state.Drift = core.ApprovalDrift(latest.Pins, pins)
		}
		if expires, err := time.Parse(time.RFC3339, latest.ExpiresAt); err == nil && now.After(expires) {
			state.Expired = true
		}
		states = append(states, state)
	}
	return states
}

// ApprovalRequestFindings is the staleness half of the approval gate (R5.3): a
// request that can still be answered (draft, requested) but whose pinned inputs
// drifted, or whose expiry passed, is refused — the only legal continuation is
// a new or superseding request, the same refusal core.PlanApprovalRequest
// enforces on write. An already-approved request whose inputs have since
// drifted is a warning: it stands for what it pinned, but no longer covers
// current inputs. Terminal requests are silent.
func ApprovalRequestFindings(states []ApprovalRequestState) []Finding {
	var findings []Finding
	for _, state := range states {
		switch state.State {
		case core.ApprovalDraft, core.ApprovalRequested:
			if state.Expired {
				findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf(
					"approval request %s (%s) expired at %s: open a new or superseding request", state.ID, state.Entity, state.ExpiresAt)})
				continue
			}
			if len(state.Drift) > 0 {
				findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf(
					"approval request %s (%s) is stale (%s changed since it was pinned): open a new or superseding request", state.ID, state.Entity, strings.Join(state.Drift, ", "))})
			}
		case core.ApprovalApproved:
			if len(state.Drift) > 0 {
				findings = append(findings, Finding{Severity: Warn, Message: fmt.Sprintf(
					"approval request %s (%s) approved inputs drifted (%s): a further approval needs a new or superseding request", state.ID, state.Entity, strings.Join(state.Drift, ", "))})
			}
		}
	}
	return findings
}

// StaleDescendantFindings is the parent-readiness half of the approval gate
// (spec 04 R5.4): while a reopen has left a completed descendant stale, the
// parent is not ready, and no digest comparison or elapsed revision changes
// that. Readiness is proved only from the current revisions and attempts the
// caller projected — this gate never re-derives them.
func StaleDescendantFindings(stale []core.StaleDescendant) []Finding {
	var findings []Finding
	for _, blocker := range core.StaleDescendantBlockers(stale) {
		findings = append(findings, Finding{Severity: Error, Message: blocker.Message})
	}
	return findings
}

// approvalRequestIDs lists each request id once, in the append order
// core.ReadApprovalRequests guarantees (record key order, so id order).
func approvalRequestIDs(requests []core.ApprovalRequestRecord) []string {
	seen := make(map[string]bool, len(requests))
	ids := make([]string, 0, len(requests))
	for _, rec := range requests {
		if seen[rec.ID] {
			continue
		}
		seen[rec.ID] = true
		ids = append(ids, rec.ID)
	}
	return ids
}

// approvalGate enforces P6: no task may show progress before requirements and
// design are approved. It fires only when the caller loaded state.json
// (StateLoaded) — unit/parity contexts that omit approval state disable it.
// Severity is pinned error: it guards the approval gate.
func approvalGate(ctx CheckCtx) []Finding {
	if !ctx.StateLoaded {
		return nil
	}
	if len(ctx.StaleRecords) > 0 && (ctx.ApproveTarget == string(core.StatusTasks) || ctx.ApproveTarget == string(core.StatusExecuting)) {
		return []Finding{{Severity: Error, Message: "freshness: approval blocked by stale records: " + strings.Join(ctx.StaleRecords, ", ")}}
	}
	if ctx.ApprovedRequirements && ctx.ApprovedDesign {
		return nil
	}
	if len(ctx.StaleRecords) > 0 && (ctx.ApproveTarget == string(core.StatusTasks) || ctx.ApproveTarget == string(core.StatusExecuting)) {
		return []Finding{{Severity: Error, Message: "freshness: approval blocked by stale records: " + strings.Join(ctx.StaleRecords, ", ")}}
	}
	for _, task := range ctx.Tasks {
		if status := ctx.Status[task.ID]; status != "" && status != core.TaskPending {
			return []Finding{{
				Severity: Error,
				Message:  "tasks show progress before requirements and design are approved",
			}}
		}
	}
	return nil
}

// designGate refuses to approve a design that is still the unedited scaffold
// stub or has empty sections. It arms only when the gate under approval is
// "design" (ADR-4: pure over CheckCtx, disabled for plain check). Severity
// pinned error.
func designGate(ctx CheckCtx) []Finding {
	if ctx.ApproveTarget != "design" {
		return nil
	}
	doc := strings.TrimSpace(ctx.DesignDoc)
	if doc == "" || (ctx.DesignStub != "" && doc == strings.TrimSpace(ctx.DesignStub)) {
		return []Finding{{Severity: Error, Message: "design.md is the unedited scaffold stub"}}
	}
	if section, empty := firstEmptySection(ctx.DesignDoc); empty {
		return []Finding{{Severity: Error, Message: fmt.Sprintf("design.md section %q is empty", section)}}
	}
	// Decision-contract trace (spec 01 R2): a design tracing to a requirement
	// that does not exist is always refused (R2.2); the full decision-metadata
	// contract is required only under the production design profile
	// (DesignContractRequired), keeping default-profile design.md files
	// opt-in (R7.1).
	design := core.ParseDesign([]byte(ctx.DesignDoc))
	known := core.RequirementIDSet(ctx.RequirementsDoc)
	var findings []Finding
	for _, f := range core.ValidateDesign(design, known, ctx.DesignContractRequired) {
		findings = append(findings, Finding{Severity: Error, Message: f.Message})
	}
	return findings
}

// criteriaGate is the opt-in per-acceptance-criterion ratchet (spec 04 R6). It
// arms when config enabled it (CriteriaRequired) or the production lifecycle
// profile requires it (spec 01 R7.2), and the gate under approval is the
// completion transition. It refuses while any acceptance
// criterion lacks a current passing record — one recorded after the last
// requirements approval. A criterion record never substitutes for a task verify
// record, so this gate strengthens, never bypasses, the evidence story (R7).
// Pure over CheckCtx; the caller derives CriteriaUnmet from disk.
func criteriaGate(ctx CheckCtx) []Finding {
	if !criteriaArmed(ctx) || ctx.ApproveTarget != string(core.StatusComplete) {
		return nil
	}
	if len(ctx.CriteriaUnmet) == 0 {
		return nil
	}
	return []Finding{{
		Severity: Error,
		Message:  fmt.Sprintf("criteria.required: %d acceptance criterion/criteria lack a current passing record: %s", len(ctx.CriteriaUnmet), strings.Join(ctx.CriteriaUnmet, ", ")),
	}}
}

func coverageGate(ctx CheckCtx) []Finding {
	severity, armed := coverageSeverity(ctx.ApproveTarget)
	if !armed || !core.HasTaskTrace(ctx.Tasks) {
		return nil
	}
	gaps := ctx.CoverageGaps
	if len(gaps) == 0 {
		requirements, err := core.ParseRequirements([]byte(ctx.RequirementsDoc))
		if err != nil {
			return nil
		}
		for _, finding := range core.AnalyzeCoverage(requirements, core.ParseDesign([]byte(ctx.DesignDoc)), ctx.Tasks) {
			if finding.Requirement != "" {
				gaps = append(gaps, finding.Requirement)
			}
		}
	}
	if len(gaps) == 0 {
		return nil
	}
	// Spec R5.1: the refusal states where matching happens (the tasks.md `refs`
	// column), lists every uncovered id, and names both remedies — so the fix
	// needs no further lookup. Matching semantics are unchanged.
	return []Finding{{Severity: severity, Message: fmt.Sprintf(
		"coverage: requirement/criterion id(s) matched against the tasks.md `refs` column have no implementing task: %s; fix: add each id to an implementing task's `refs` column, or mark its task `kind: deferred`",
		strings.Join(gaps, ", "))}}
}

// coverageSeverity arms the coverage analysis per approval target (spec R5.2):
// the tasks-phase approval runs the same analysis as a non-blocking advisory
// (warning — approval proceeds, the gap is reported early), and the executing
// transition keeps its blocking error severity unchanged. Any other target
// leaves the gate disarmed.
func coverageSeverity(target string) (Severity, bool) {
	switch target {
	case string(core.StatusTasks):
		return Warn, true
	case string(core.StatusExecuting):
		return Error, true
	}
	return "", false
}

func evidencePolicyGate(ctx CheckCtx) []Finding {
	if ctx.ApproveTarget != string(core.StatusExecuting) {
		return nil
	}
	gaps := ctx.IntegrationEvidenceGaps
	if len(gaps) == 0 {
		for _, finding := range core.BoundaryEvidenceFindings(core.ParseDesign([]byte(ctx.DesignDoc)), ctx.Tasks, ctx.ProductionPolicy) {
			gaps = append(gaps, finding.Message)
		}
	}
	findings := make([]Finding, 0, len(gaps))
	for _, gap := range gaps {
		findings = append(findings, Finding{Severity: Error, Message: "evidence-policy: " + gap})
	}
	return findings
}

// firstEmptySection reports the first "## " heading with no non-blank content
// before the next heading or EOF.
func firstEmptySection(doc string) (string, bool) {
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		empty := true
		for _, next := range lines[i+1:] {
			if strings.HasPrefix(next, "#") {
				break
			}
			if strings.TrimSpace(next) != "" {
				empty = false
				break
			}
		}
		if empty {
			return strings.TrimSpace(strings.TrimPrefix(line, "##")), true
		}
	}
	return "", false
}
