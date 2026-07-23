package gates

import (
	"github.com/0xkhdr/specd/internal/core"
)

// dispatchParity proves every task row against the same contract the dispatcher
// and session-ack consume, so a closed-vocabulary fact (kind, risk,
// capabilities, files) — and the worker dispatch-policy id (spec R6.2) — is
// rejected at tasks-phase approval instead of at dispatch or session ack (spec
// R1.1). core.ParseTaskContract validates the worker charset, so a malformed
// worker column is refused here like every other pinned task fact. Each row is
// parsed with
// core.ParseTaskContract; a parse error becomes an Error finding carrying the
// parser's own TASK_FIELD_UNKNOWN message verbatim, which already names the
// offending row, the rejected value, and the accepted vocabulary (R1.2). Every
// nonconforming row is reported, not just the first, so a plan mixing good and
// bad rows can be corrected in one pass (R1.3). Pure function of the parsed
// tasks.md rows; an empty CheckCtx yields no findings.
func dispatchParity(ctx CheckCtx) []Finding {
	if !dispatchParityArmed(ctx.ApproveTarget) {
		return nil
	}
	var findings []Finding
	for _, task := range ctx.Tasks {
		if _, err := core.ParseTaskContract(task); err != nil {
			findings = append(findings, Finding{Severity: Error, Message: err.Error()})
		}
	}
	return findings
}

// dispatchParityArmed arms the parity check ONLY at the tasks→executing
// approval, where R1.1 belongs. It deliberately does NOT arm at plain check
// (target ""), so specComplete's full-registry run does not retroactively reject
// a historical spec whose legacy tasks.md used an out-of-vocabulary kind.
func dispatchParityArmed(target string) bool {
	return target == string(core.StatusTasks)
}
