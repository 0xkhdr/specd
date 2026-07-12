package gates

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
)

// reviewGate is the opt-in review ratchet (spec 09). It arms only when config
// enabled it (ReviewRequired) and the gate under approval is the completion
// transition. It refuses completion unless review_report.md carries an approve
// verdict recorded at the current git HEAD:
//
//   - a missing or malformed report fails closed and is never read as approve
//     (R5 — the caller sets ReviewParseErr);
//   - a reject / needs-changes verdict refuses and surfaces the findings (R4);
//   - an approve verdict pinned to a stale HEAD refuses (R3 freshness) — a
//     review is a fact about *this* code, mirroring evidence pinning.
//
// Pure over CheckCtx; the LLM writes the report through the host agent, the
// harness only checks it — the enforcement thesis is preserved.
func reviewGate(ctx CheckCtx) []Finding {
	if !reviewArmed(ctx) || ctx.ApproveTarget != string(core.StatusComplete) {
		return nil
	}
	if ctx.ReviewParseErr != "" {
		return []Finding{{Severity: Error, Message: "review.required: " + ctx.ReviewParseErr}}
	}
	if ctx.ReviewVerdict != core.ReviewApprove {
		msg := fmt.Sprintf("review.required: verdict is %q, completion refused", ctx.ReviewVerdict)
		if ctx.ReviewFindings != "" {
			msg += "\nfindings:\n" + ctx.ReviewFindings
		}
		return []Finding{{Severity: Error, Message: msg}}
	}
	if ctx.ReviewHead != ctx.ReviewExpectedHead {
		return []Finding{{
			Severity: Error,
			Message:  fmt.Sprintf("review.required: approve verdict is stale (report HEAD %s, current HEAD %s); re-review at the current commit", shortHead(ctx.ReviewHead), shortHead(ctx.ReviewExpectedHead)),
		}}
	}
	return nil
}

// shortHead trims a git SHA for readable gate output without losing identity.
func shortHead(head string) string {
	if len(head) > 12 {
		return head[:12]
	}
	return head
}

// reviewArmed reports whether the review ratchet must run: either config armed
// it explicitly (review.required) or the production lifecycle profile requires a
// current-HEAD review (spec 01 R7.2). Default profile with the switch off keeps
// the ratchet disabled (R7.1).
func reviewArmed(ctx CheckCtx) bool {
	return ctx.ReviewRequired || ctx.ProductionProfile
}
