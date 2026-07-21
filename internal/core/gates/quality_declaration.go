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
	var findings []Finding
	for _, task := range ctx.Tasks {
		if strings.TrimSpace(task.Evidence) == "" {
			continue
		}
		if _, err := core.ParseQualityContract(task); err != nil {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s quality declaration invalid: %v", task.ID, err)})
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
