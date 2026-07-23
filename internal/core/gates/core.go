package gates

import (
	"fmt"
	"strings"
	"time"

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
	// DesignContractRequired arms the full decision-contract check (spec 01
	// R2.1): under the production design profile the design must declare every
	// decision-metadata field and a resolvable requirement reference. Zero ⇒
	// default profile, where the contract fields are optional (R7.1) and only
	// unknown references are refused. Unknown-reference resolution is always on.
	DesignContractRequired bool

	// Criteria gate inputs (spec 04 R6, opt-in). CriteriaRequired mirrors
	// config criteria.required; CriteriaUnmet lists acceptance-criterion ids
	// with no current passing record. Both zero ⇒ gate disabled.
	CriteriaRequired        bool
	CriteriaUnmet           []string
	CoverageGaps            []string
	IntegrationEvidenceGaps []string
	ProductionPolicy        bool

	// ProductionProfile mirrors the production lifecycle profile (spec 01 R7.2).
	// It arms the criterion and review ratchets on its own, so the production
	// profile requires current criterion evidence and a current-HEAD review even
	// when the individual criteria.required / review.required switches are off.
	ProductionProfile bool

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

	// TaskTraceRequired arms the full task trace/risk contract (spec 01 R3.1):
	// under the production planning profile every task must declare its
	// references, work kind, risk tier, required context, evidence classes, and
	// edge checks. Zero ⇒ default profile, where those fields are optional
	// (R7.1); unknown references and unknown risk tiers are always refused.
	TaskTraceRequired bool

	// Quality-freshness gate inputs (spec 04 R3.3, opt-in). QualityContracts maps
	// a task id to its declared required evidence classes/checks; Evals is the
	// imported eval store; QualitySubject is the current subject revision/digests.
	// The evidence gate refuses a completed task whose required evidence is
	// missing or stale for the subject. All zero ⇒ no contracts ⇒ no new findings
	// (parity: an empty CheckCtx yields no quality findings).
	QualityContracts map[string]core.QualityContract
	Evals            []core.EvidenceEnvelopeV1
	QualitySubject   core.FreshnessSubject

	// QualityPolicyRequired arms Domain 04 production coverage enforcement.
	// Callers supply parsed policy/criterion registries; the gate stays pure and
	// offline. Zero values leave default projects unchanged.
	QualityPolicyRequired bool
	QualityPolicies       map[string]core.QualityPolicy
	KnownCriteria         map[string]bool

	// Program-link gate input (spec 12 R5). When the gate under approval is the
	// execution transition, the caller fills ProgramDepsIncomplete with the
	// cross-spec dependencies that are not yet complete; a non-empty list refuses
	// the approval. Empty ⇒ disabled (planning phases are never program-gated).
	ProgramDepsIncomplete []string
	StaleRecords          []string

	// Provenance is operator-recorded typed intake. A nil record or empty
	// RequiredFields leaves default projects unchanged. The gate is pure:
	// callers own disk reads and supply this immutable snapshot.
	Provenance      *core.ProvenanceV1
	ProvenanceError string

	// Governance snapshots are caller-loaded, immutable inputs. Zero value is
	// unconfigured and changes nothing.
	GovernanceRequired  bool
	GovernanceNow       time.Time
	RequiredDecisionIDs []string
	Decisions           []core.DecisionV1
	Exceptions          []core.ExceptionV1
	GovernanceError     string
	MemoryLintRequired  bool
	MemoryBlocks        []core.MemBlock
	MemoryAsOf          time.Time
	MemoryLintError     string
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
	registry.Register(gateFunc{name: "task-trace", run: taskTrace})
	registry.Register(gateFunc{name: "coverage", run: coverageGate})
	registry.Register(gateFunc{name: "evidence-policy", run: evidencePolicyGate})
	registry.Register(gateFunc{name: "intake", run: intakeReadiness})
	registry.Register(gateFunc{name: "governance", run: governanceGate})
	registry.Register(gateFunc{name: "memory-lint", run: memoryConflictLint})
	registry.Register(gateFunc{name: "quality-declaration", run: qualityDeclaration})
	registry.Register(gateFunc{name: "dispatch-parity", run: dispatchParity})
	registry.Register(gateFunc{name: "palette-scope", run: paletteScope})
	registry.Register(gateFunc{name: "verify-lint", run: verifyLint})
	registry.Register(gateFunc{name: "steering-applicability", run: steeringApplicability})
	return registry
}

// CoreRegistryWith appends profile-required gates after stable core order.
func CoreRegistryWith(required ...Gate) Registry {
	r := CoreRegistry()
	for _, gate := range required {
		r.Register(gate)
	}
	return r
}

// taskTrace enforces the task trace/risk contract (spec 01 R3.1). It always
// refuses a task whose declared requirement reference does not resolve or whose
// risk tier is unrecognized; under the production planning profile
// (TaskTraceRequired) it also refuses a task that omits any required trace
// field. Pure over CheckCtx: the caller supplies the requirements bytes, the
// gate never touches disk. Empty CheckCtx ⇒ no tasks ⇒ no findings (parity).
func taskTrace(ctx CheckCtx) []Finding {
	known := core.RequirementIDSet(ctx.RequirementsDoc)
	traceFindings := core.ValidateTaskTrace(ctx.Tasks, known, ctx.TaskTraceRequired)
	// R3.1: when requirements.md has content but no parseable requirement IDs and
	// tasks cite R-ids, the malformed file is requirements.md — not tasks.md.
	// Name it once instead of emitting a misleading unknown-reference finding
	// against each task; keep any other trace findings unchanged.
	if len(known) == 0 && strings.TrimSpace(ctx.RequirementsDoc) != "" && citesUnknownRequirement(traceFindings) {
		findings := []Finding{{Severity: Error, Message: "requirements.md declares no parseable requirement IDs (expected '## R<n>' headings and '- R<n>.<m>:' criteria)"}}
		for _, f := range traceFindings {
			if !strings.Contains(f.Message, "references unknown requirement") {
				findings = append(findings, Finding{Severity: Error, Message: f.Message})
			}
		}
		return findings
	}
	var findings []Finding
	for _, f := range traceFindings {
		findings = append(findings, Finding{Severity: Error, Message: f.Message})
	}
	return findings
}

func citesUnknownRequirement(findings []core.TaskTraceFinding) bool {
	for _, f := range findings {
		if strings.Contains(f.Message, "references unknown requirement") {
			return true
		}
	}
	return false
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
		if ctx.Status[task.ID] != core.TaskComplete {
			continue
		}
		if !core.HasPassingEvidence(ctx.Evidence, task.ID) {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s is complete without passing evidence", task.ID)})
		}
		contract, ok := ctx.QualityContracts[task.ID]
		if !ok {
			continue
		}
		st := core.EvaluateQuality(contract, ctx.Evals, ctx.QualitySubject)
		for _, req := range st.Missing {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s is complete without passing evidence for %s/%s", task.ID, req.EvidenceClass, req.CheckID)})
		}
		for _, req := range st.Stale {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s has EVIDENCE_STALE for %s/%s (not current for subject)", task.ID, req.EvidenceClass, req.CheckID)})
		}
	}
	return findings
}
