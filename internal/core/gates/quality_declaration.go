package gates

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// qualityDeclaration validates every task's declared `evidence` cell at
// planning time (spec R1). It parses each non-empty cell with
// core.ParseQualityContract and reports each malformed declaration as a
// blocker, so `specd check` (R1.1) and tasks-phase approval (R1.2) refuse
// before a broken contract can silently weaken execution-time enforcement.
// The parse error itself carries the class enum and format example (R1.3),
// so this gate, complete-task, and approval all emit the same message. Pure
// function of the parsed tasks.md rows; an empty CheckCtx yields no findings.
func qualityDeclaration(ctx CheckCtx) []Finding {
	if !qualityDeclarationArmed(ctx.ApproveTarget) {
		return nil
	}
	slug := ctx.Slug
	if slug == "" {
		slug = "<slug>"
	}
	var findings []Finding
	for _, task := range ctx.Tasks {
		if strings.TrimSpace(task.Evidence) == "" {
			continue
		}
		contract, err := core.ParseQualityContract(task)
		if err != nil {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s quality declaration invalid: %v", task.ID, err)})
			continue
		}
		// R3.1/R3.2: a non-test evidence class needs an external producer that a
		// plain `specd verify` cannot run, so name the producer and print the exact
		// `specd eval import` command at approval — not only when complete-task
		// later refuses. Warning severity: the declaration is legal, it just needs
		// evidence imported before the task can complete. Only at the tasks-phase
		// approval, where these producers are planned.
		if ctx.ApproveTarget != string(core.StatusTasks) {
			continue
		}
		for _, req := range contract.Required {
			if req.EvidenceClass == core.EvidenceTest {
				continue
			}
			findings = append(findings, Finding{Severity: Warn, Message: fmt.Sprintf("%s evidence %s/%s cannot be satisfied by a plain `specd verify`; import external evidence with `specd eval import %s <file> --task %s --check %s`", task.ID, req.EvidenceClass, req.CheckID, slug, task.ID, req.CheckID)})
		}
	}
	return findings
}

// qualityDeclarationArmed arms the structural check for every readiness phase,
// so check and immediate approval cannot disagree about a malformed tasks.md.
func qualityDeclarationArmed(target string) bool {
	switch target {
	case "", string(core.StatusRequirements), string(core.StatusDesign), string(core.StatusTasks), string(core.StatusExecuting), string(core.StatusVerifying), string(core.StatusComplete):
		return true
	}
	return false
}
