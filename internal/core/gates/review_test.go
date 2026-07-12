package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestReviewGate(t *testing.T) {
	complete := string(core.StatusComplete)

	base := func() CheckCtx {
		return CheckCtx{
			ApproveTarget:      complete,
			ReviewRequired:     true,
			ReviewVerdict:      core.ReviewApprove,
			ReviewHead:         "abc123",
			ReviewExpectedHead: "abc123",
		}
	}

	cases := []struct {
		name    string
		mutate  func(*CheckCtx)
		wantErr bool
		wantMsg string
	}{
		{"approve_fresh_passes", func(c *CheckCtx) {}, false, ""},
		{"disabled_passes", func(c *CheckCtx) { c.ReviewRequired = false }, false, ""},
		// R7.2: the production profile arms the review ratchet on its own — the
		// explicit switch is off, yet a missing report fails closed.
		{"production_profile_arms_without_switch", func(c *CheckCtx) {
			c.ReviewRequired = false
			c.ProductionProfile = true
			c.ReviewVerdict = ""
			c.ReviewParseErr = "no review report"
		}, true, "no review report"},
		{"non_completion_target_passes", func(c *CheckCtx) { c.ApproveTarget = "design" }, false, ""},
		{"parse_error_fails_closed", func(c *CheckCtx) { c.ReviewParseErr = "no review report" }, true, "no review report"},
		{"reject_surfaces_findings", func(c *CheckCtx) {
			c.ReviewVerdict = core.ReviewReject
			c.ReviewFindings = "Missing tests for T2"
		}, true, "Missing tests for T2"},
		{"needs_changes_refuses", func(c *CheckCtx) { c.ReviewVerdict = core.ReviewNeedsChanges }, true, "needs-changes"},
		{"stale_head_refuses", func(c *CheckCtx) { c.ReviewHead = "old999" }, true, "stale"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := base()
			tc.mutate(&ctx)
			findings := reviewGate(ctx)
			if tc.wantErr && !HasErrors(findings) {
				t.Fatalf("want error finding, got none")
			}
			if !tc.wantErr && HasErrors(findings) {
				t.Fatalf("want no error, got %+v", findings)
			}
			if tc.wantMsg != "" {
				joined := ""
				for _, f := range findings {
					joined += f.Message
				}
				if !strings.Contains(joined, tc.wantMsg) {
					t.Fatalf("finding missing %q: %q", tc.wantMsg, joined)
				}
			}
		})
	}
}
