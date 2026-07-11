package gates

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

type CheckCtx struct {
	Root             string
	Slug             string
	Tasks            []core.TaskRow
	Status           map[string]core.TaskRunStatus // marker-derived (tasks.md truth)
	Evidence         map[string]core.EvidenceRecord
	MaxContextTokens int

	// W4 gate inputs. Gate bodies stay pure over CheckCtx: the caller reads
	// files and state, the gates never touch disk. Zero values disable the
	// gate (parity: an empty CheckCtx yields no findings).
	// TrivialVerify is the configured set of no-op verify commands (config
	// verify.trivial, default core.DefaultTrivialVerify). The verify gate rejects
	// a write task using one of these (spec 01 R4.2). Nil disables the check
	// (parity: an empty CheckCtx yields no findings).
	TrivialVerify []string

	StateLoaded          bool                          // caller loaded state.json for this spec
	StateTaskStatus      map[string]core.TaskRunStatus // machine truth from state.json
	ApprovedRequirements bool
	ApprovedDesign       bool
	ApproveTarget        string // the gate being approved ("design" arms the design-stub gate)
	RequirementsDoc      string // requirements.md bytes ("" = not provided)
	RequirementsStub     string // the scaffold stub to compare against (ADR-10 single source)
	DesignDoc            string
	DesignStub           string

	// Criteria gate inputs (spec 04 R6, opt-in). CriteriaRequired mirrors
	// config criteria.required; CriteriaUnmet lists acceptance-criterion ids
	// with no current passing record. Both zero ⇒ gate disabled.
	CriteriaRequired bool
	CriteriaUnmet    []string

	// Review gate inputs (spec 09 R3/R4/R5, opt-in). ReviewRequired mirrors
	// config review.required. The caller reads review_report.md, parses it, and
	// fills these — the gate never touches disk. ReviewParseErr non-empty means
	// the report is missing or malformed (fail closed). ReviewExpectedHead is the
	// current git HEAD the approval must be fresh against. All zero ⇒ disabled.
	ReviewRequired     bool
	ReviewParseErr     string
	ReviewVerdict      string
	ReviewHead         string
	ReviewFindings     string
	ReviewExpectedHead string

	// Program-link gate input (spec 12 R5). When the gate under approval is the
	// execution transition, the caller fills ProgramDepsIncomplete with the
	// cross-spec dependencies that are not yet complete; a non-empty list refuses
	// the approval. Empty ⇒ disabled (planning phases are never program-gated).
	ProgramDepsIncomplete []string
}

func CoreRegistry() Registry {
	registry := NewRegistry()
	registry.Register(gateFunc{name: "task-ids", run: taskIDs})
	registry.Register(gateFunc{name: "dependencies", run: dependencies})
	registry.Register(gateFunc{name: "dag", run: dag})
	registry.Register(gateFunc{name: "roles", run: roles})
	registry.Register(gateFunc{name: "files", run: files})
	registry.Register(gateFunc{name: "verify", run: verifyCommands})
	registry.Register(gateFunc{name: "evidence", run: evidence})
	registry.Register(gateFunc{name: "context-budget", run: contextBudget})
	registry.Register(gateFunc{name: "ears", run: earsGate})
	registry.Register(gateFunc{name: "approval", run: approvalGate})
	registry.Register(gateFunc{name: "sync", run: syncGate})
	registry.Register(gateFunc{name: "design", run: designGate})
	registry.Register(gateFunc{name: "criteria", run: criteriaGate})
	registry.Register(gateFunc{name: "review", run: reviewGate})
	return registry
}

type gateFunc struct {
	name string
	run  func(CheckCtx) []Finding
}

func (g gateFunc) Name() string { return g.name }
func (g gateFunc) Run(ctx CheckCtx) []Finding {
	return g.run(ctx)
}

func taskIDs(ctx CheckCtx) []Finding {
	seen := map[string]bool{}
	var findings []Finding
	for _, task := range ctx.Tasks {
		if task.ID == "" {
			findings = append(findings, Finding{Severity: Error, Message: "task id is required"})
			continue
		}
		if seen[task.ID] {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("duplicate task id %s", task.ID)})
		}
		seen[task.ID] = true
	}
	return findings
}

func dependencies(ctx CheckCtx) []Finding {
	ids := map[string]bool{}
	for _, task := range ctx.Tasks {
		ids[task.ID] = true
	}
	var findings []Finding
	for _, task := range ctx.Tasks {
		for _, dep := range task.DependsOn {
			if dep != "" && !ids[dep] {
				findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s depends on missing task %s", task.ID, dep)})
			}
		}
	}
	return findings
}

func dag(ctx CheckCtx) []Finding {
	if _, err := core.NewTaskDAG(ctx.Tasks); err != nil {
		return []Finding{{Severity: Error, Message: err.Error()}}
	}
	return nil
}

func roles(ctx CheckCtx) []Finding {
	var findings []Finding
	for _, task := range ctx.Tasks {
		role := strings.TrimSpace(task.Role)
		if role == "" {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s role is required", task.ID)})
			continue
		}
		if !core.IsKnownRole(role) {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s has unknown role %q (known: %s)", task.ID, role, strings.Join(core.KnownRoles(), ", "))})
		}
	}
	return findings
}

func files(ctx CheckCtx) []Finding {
	var findings []Finding
	for _, task := range ctx.Tasks {
		if strings.TrimSpace(task.Files) == "" {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s files are required", task.ID)})
		}
	}
	return findings
}

func verifyCommands(ctx CheckCtx) []Finding {
	var findings []Finding
	for _, task := range ctx.Tasks {
		cmd := strings.TrimSpace(task.Verify)
		if cmd == "" {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s verify command is required", task.ID)})
			continue
		}
		if core.IsWriteRole(task.Role) && core.IsTrivialVerify(cmd, ctx.TrivialVerify) {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s is a write task with trivial verify %q; a write task must verify its change", task.ID, cmd)})
		}
	}
	return findings
}

func evidence(ctx CheckCtx) []Finding {
	var findings []Finding
	for _, task := range ctx.Tasks {
		if ctx.Status[task.ID] == core.TaskComplete && !core.HasPassingEvidence(ctx.Evidence, task.ID) {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s is complete without passing evidence", task.ID)})
		}
	}
	return findings
}
