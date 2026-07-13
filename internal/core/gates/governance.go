package gates

import (
	"fmt"
	"sort"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func governanceGate(ctx CheckCtx) []Finding {
	if !ctx.GovernanceRequired {
		return nil
	}
	if ctx.GovernanceError != "" {
		return []Finding{{Severity: Error, Message: "governance records invalid; review and repair them: " + ctx.GovernanceError}}
	}
	now := ctx.GovernanceNow
	if now.IsZero() {
		return []Finding{{Severity: Error, Message: "governance time is missing; review governance configuration"}}
	}
	decisions := make(map[string]core.DecisionV1, len(ctx.Decisions))
	for _, d := range ctx.Decisions {
		decisions[d.ID] = d
	}
	ids := append([]string(nil), ctx.RequiredDecisionIDs...)
	sort.Strings(ids)
	var out []Finding
	for _, id := range ids {
		d, ok := decisions[id]
		if !ok {
			out = append(out, Finding{Severity: Error, Message: fmt.Sprintf("required decision %s is missing; owner must record and review it", id)})
			continue
		}
		if d.Status != core.GovernanceAccepted || !d.ActiveAt(now) {
			out = append(out, Finding{Severity: Error, Message: fmt.Sprintf("required decision %s is %s; owner %s must review and accept an active decision", id, d.Status, d.Owner)})
		}
	}
	exceptions := append([]core.ExceptionV1(nil), ctx.Exceptions...)
	sort.Slice(exceptions, func(i, j int) bool { return exceptions[i].ID < exceptions[j].ID })
	for _, e := range exceptions {
		if !e.Blocking || e.Status != core.GovernanceAccepted {
			continue
		}
		expires, err := time.Parse(time.RFC3339, e.ExpiresAt)
		if err != nil || !now.Before(expires) {
			out = append(out, Finding{Severity: Error, Message: fmt.Sprintf("blocking exception %s expired; owner %s must review, revoke, or supersede it", e.ID, e.Owner)})
		}
	}
	return out
}
