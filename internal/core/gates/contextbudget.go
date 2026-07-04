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
		manifest, err := speccontext.BuildManifest(ctx.Slug, ctx.Tasks, task.ID)
		if err != nil {
			findings = append(findings, Finding{Severity: Error, Message: err.Error()})
			continue
		}
		if manifest.EstimatedTokens > ctx.MaxContextTokens {
			findings = append(findings, Finding{
				Severity: Error,
				Message:  fmt.Sprintf("%s context estimate %d exceeds budget %d", task.ID, manifest.EstimatedTokens, ctx.MaxContextTokens),
			})
		}
	}
	return findings
}
