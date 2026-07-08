package gates

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
)

// syncGate is ADR-1 Gate 6: the tasks.md marker (ctx.Status) must agree with
// the machine truth in state.json (ctx.StateTaskStatus). A marker changed by
// hand — bypassing `task complete` — leaves state behind and is caught here.
// Fires only when the caller loaded state (StateLoaded); a task absent from
// either side reads as pending. Severity pinned error.
func syncGate(ctx CheckCtx) []Finding {
	if !ctx.StateLoaded {
		return nil
	}
	var findings []Finding
	for _, task := range ctx.Tasks {
		marker := statusOr(ctx.Status[task.ID])
		state := statusOr(ctx.StateTaskStatus[task.ID])
		if marker != state {
			findings = append(findings, Finding{
				Severity: Error,
				Message:  fmt.Sprintf("%s: tasks.md marker %q disagrees with state.json %q", task.ID, marker, state),
			})
		}
	}
	return findings
}

func statusOr(s core.TaskRunStatus) core.TaskRunStatus {
	if s == "" {
		return core.TaskPending
	}
	return s
}
