package gates

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// approvalGate enforces P6: no task may show progress before requirements and
// design are approved. It fires only when the caller loaded state.json
// (StateLoaded) — unit/parity contexts that omit approval state disable it.
// Severity is pinned error: it guards the approval gate.
func approvalGate(ctx CheckCtx) []Finding {
	if !ctx.StateLoaded || (ctx.ApprovedRequirements && ctx.ApprovedDesign) {
		return nil
	}
	for _, task := range ctx.Tasks {
		if status := ctx.Status[task.ID]; status != "" && status != core.TaskPending {
			return []Finding{{
				Severity: Error,
				Message:  "tasks show progress before requirements and design are approved",
			}}
		}
	}
	return nil
}

// designGate refuses to approve a design that is still the unedited scaffold
// stub or has empty sections. It arms only when the gate under approval is
// "design" (ADR-4: pure over CheckCtx, disabled for plain check). Severity
// pinned error.
func designGate(ctx CheckCtx) []Finding {
	if ctx.ApproveTarget != "design" {
		return nil
	}
	doc := strings.TrimSpace(ctx.DesignDoc)
	if doc == "" || (ctx.DesignStub != "" && doc == strings.TrimSpace(ctx.DesignStub)) {
		return []Finding{{Severity: Error, Message: "design.md is the unedited scaffold stub"}}
	}
	if section, empty := firstEmptySection(ctx.DesignDoc); empty {
		return []Finding{{Severity: Error, Message: fmt.Sprintf("design.md section %q is empty", section)}}
	}
	return nil
}

// firstEmptySection reports the first "## " heading with no non-blank content
// before the next heading or EOF.
func firstEmptySection(doc string) (string, bool) {
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		empty := true
		for _, next := range lines[i+1:] {
			if strings.HasPrefix(next, "#") {
				break
			}
			if strings.TrimSpace(next) != "" {
				empty = false
				break
			}
		}
		if empty {
			return strings.TrimSpace(strings.TrimPrefix(line, "##")), true
		}
	}
	return "", false
}
