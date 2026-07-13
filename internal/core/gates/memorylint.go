package gates

import "github.com/0xkhdr/specd/internal/core"

func memoryConflictLint(ctx CheckCtx) []Finding {
	if !ctx.MemoryLintRequired {
		return nil
	}
	if ctx.MemoryLintError != "" {
		return []Finding{{Severity: Error, Message: "load memory for lint: " + ctx.MemoryLintError}}
	}
	conflicts := core.AnalyzeMemoryConflicts(ctx.MemoryBlocks, ctx.MemoryAsOf)
	findings := make([]Finding, 0, len(conflicts))
	for _, conflict := range conflicts {
		findings = append(findings, Finding{Severity: Error, Message: conflict.Message})
	}
	return findings
}
