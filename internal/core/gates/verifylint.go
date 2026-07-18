package gates

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// verifyLint flags shell patterns in verify cells that behave unreliably under
// the deterministic runner (spec R8.1): job-control `kill %N` (sh -c runs
// without an interactive job table), and a trailing `&` that backgrounds the
// command without capturing `$!` (the verify exit code then never observes the
// backgrounded work). It is a pure, deterministic string lint at warning
// severity — it reports, it never blocks — armed for plain `specd check` and
// the tasks-phase approval. Clean commands pass silently.
func verifyLint(ctx CheckCtx) []Finding {
	if !verifyLintArmed(ctx.ApproveTarget) {
		return nil
	}
	var findings []Finding
	for _, task := range ctx.Tasks {
		cmd := strings.TrimSpace(task.Verify)
		if cmd == "" {
			continue
		}
		if strings.Contains(cmd, "kill %") {
			findings = append(findings, Finding{Severity: Warn, Message: fmt.Sprintf("%s verify uses job-control `kill %%N`; sh -c has no job table — capture the PID (`$!`) and kill that instead", task.ID)})
		}
		if strings.HasSuffix(cmd, "&") && !strings.HasSuffix(cmd, "&&") && !strings.Contains(cmd, "$!") {
			findings = append(findings, Finding{Severity: Warn, Message: fmt.Sprintf("%s verify ends with a backgrounding `&` without capturing `$!`; the verify exit code never observes the backgrounded command", task.ID)})
		}
	}
	return findings
}

// verifyLintArmed arms the lint for plain `specd check` (empty target) and the
// tasks-phase approval, where verify cells are authored. Later transitions
// re-run `specd check` anyway; earlier phases have no tasks table to lint.
func verifyLintArmed(target string) bool {
	switch target {
	case "", string(core.StatusTasks):
		return true
	}
	return false
}
