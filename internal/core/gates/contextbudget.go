package gates

import (
	"fmt"

	speccontext "github.com/0xkhdr/specd/internal/context"
)

func contextBudget(ctx CheckCtx) []Finding {
	if ctx.MaxContextTokens <= 0 {
		return nil
	}
	var findings []Finding
	for _, task := range ctx.Tasks {
		// R2.5: only active or reopened tasks carry a live context cost. The
		// terminal-task predicate lives with the budget rules in internal/context
		// so the sweep and the manifest builder cannot disagree about which tasks
		// are still selectable.
		if !speccontext.SelectableForContext(ctx.Status[task.ID]) {
			continue
		}
		// BuildManifest fails closed when the required set exceeds budget and
		// carries its stable, required-only source contributions. A successful
		// build already fits budget — optional items shed, required items never
		// truncate — so the error is the only over-budget signal.
		if _, err := speccontext.BuildManifest(ctx.Root, ctx.Slug, ctx.Tasks, task.ID, ctx.MaxContextTokens); err != nil {
			findings = append(findings, Finding{
				Severity: Error,
				Message:  fmt.Sprintf("%s: %s", task.ID, err.Error()),
			})
		}
	}
	return findings
}
