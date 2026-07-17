package gates

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// earsGate guards requirements.md (P8/F8). It errors when the file is still the
// unedited scaffold stub — byte-compared against the embedded template after
// trimming, so the EARS-shaped placeholder text cannot pass (ADR-10 single
// source). It warns on requirement bullets that lack the "shall" of the
// EARS `When …, the system shall …` shape. The gate is pure: the caller passes
// the file bytes and the stub through CheckCtx; an unset RequirementsDoc
// disables it (parity).
func earsGate(ctx CheckCtx) []Finding {
	if ctx.RequirementsDoc == "" {
		return nil
	}
	if ctx.RequirementsStub != "" &&
		strings.TrimSpace(ctx.RequirementsDoc) == strings.TrimSpace(ctx.RequirementsStub) {
		return []Finding{{Severity: Error, Message: "requirements.md is the unedited scaffold stub"}}
	}
	// Structured `### R<n>` / `- R<n>.<m>:` docs get exact addressable findings
	// (spec 01 R1.2). Plain bullet docs (no structured requirements parsed) fall
	// back to the "shall" shape heuristic below so older projects keep passing.
	if doc, err := core.ParseRequirements([]byte(ctx.RequirementsDoc)); err == nil && len(doc.Requirements) > 0 {
		var findings []Finding
		for _, f := range core.ValidateRequirements(doc) {
			msg := f.Message
			if f.ID != "" {
				msg = f.ID + ": " + f.Message
			}
			findings = append(findings, Finding{Severity: Error, Message: msg})
		}
		return findings
	}
	var findings []Finding
	for i, line := range strings.Split(ctx.RequirementsDoc, "\n") {
		trimmed := strings.TrimSpace(line)
		// ponytail: requirement lines are Markdown bullets; "shall" presence is
		// the shape heuristic, not a full EARS parser. Upgrade path: a real EARS
		// tokenizer if false positives bite.
		if !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "*") {
			continue
		}
		if !strings.Contains(trimmed, "shall") {
			findings = append(findings, Finding{
				Severity: Warn,
				Message:  fmt.Sprintf("requirements.md line %d lacks EARS shape (When …, the system shall …)", i+1),
			})
		}
	}
	return findings
}
