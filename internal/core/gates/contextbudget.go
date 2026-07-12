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
		// R3.2: BuildManifest fails closed when the required set exceeds budget,
		// carrying the concise remediation (decompose / narrow declared files). A
		// successful build already fits budget — optional items shed, required
		// items never truncated — so the error is the only over-budget signal.
		if _, err := speccontext.BuildManifest(ctx.Root, ctx.Slug, ctx.Tasks, task.ID, ctx.MaxContextTokens); err != nil {
			findings = append(findings, Finding{
				Severity: Error,
				Message:  fmt.Sprintf("%s: %s", task.ID, err.Error()),
			})
		}
	}
	return findings
}
