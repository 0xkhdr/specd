package gates

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

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
