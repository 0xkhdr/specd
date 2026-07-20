package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

type Handler func(root string, args []string, flags map[string]string) error

// Registry is populated in init() rather than as a var initializer: the runMCP
// handler now reaches Run (via the injected MCP executor), and Run reads
// Registry — a static var-initializer graph would flag that as an initialization
// cycle. init() assignment is exempt from that analysis and still runs before
// any dispatch.
var Registry map[string]Handler

func init() { Registry = buildRegistry() }

// ErrUnknownCommand is returned by Run for a verb that is not registered or
// carries no handler. The dispatcher must fail closed on it (exit 2), never 0.
var ErrUnknownCommand = errors.New("unknown command")

// RefuseUnknownCommand is the typed refusal for an unregistered verb. It still
// satisfies errors.Is(err, ErrUnknownCommand), so the dispatcher keeps failing
// closed on it (exit 2) while an agent reads the recovery off the shape
// instead of guessing (R4.1, R4.2).
func RefuseUnknownCommand(name string) error {
	return core.Refusef("UNKNOWN_COMMAND", "unknown command %q", name).Wrapping(ErrUnknownCommand)
}

var executable = map[string]Handler{
	"approve":          runApprove,
	"archive":          runArchive,
	"adapters":         runAdapters,
	"agents":           runAgents,
	"brain":            runBrain,
	"session":          runSession,
	"drive":            runDrive,
	"check":            runCheck,
	"complete-task":    runTaskComplete,
	"context":          runContext,
	"decision":         runDecision,
	"request-decision": runRequestDecision,
	"drift":            runDrift,
	"handshake":        runHandshake,
	"help":             runHelp,
	"init":             runInit,
	"incident":         runIncident,
	"link":             runLink,
	"mcp":              runMCP,
	"version":          runVersion,
	"memory":           runMemory,
	"midreq":           runMidreq,
	"mode":             runMode,
	"new":              runNew,
	"next":             runNext,
	"release":          runRelease,
	"recurring":        runRecurring,
	"deploy":           runDeploy,
	"eval":             runEval,
	"exception":        runSecurityException,
	"report":           runReport,
	"review":           runReview,
	"spike":            runSpike,
	"status":           runStatus,
	"submit":           runSubmit,
	"task":             runTask,
	"unlink":           runUnlink,
	"verify":           runVerify,
}

func buildRegistry() map[string]Handler {
	registry := make(map[string]Handler, len(core.Commands))
	for _, command := range core.Commands {
		if command.Deferred {
			registry[command.Name] = deferredHandler(command.Name)
			continue
		}
		registry[command.Name] = executable[command.Name]
	}
	return registry
}

// deferredHandler reports an explicit deferral notice and exits 0 (R13.8): a
// deferred verb never silently no-ops.
func deferredHandler(name string) Handler {
	return func(string, []string, map[string]string) error {
		fmt.Fprintf(os.Stdout, "specd %s: deferred — not yet wired\n", name)
		return nil
	}
}

func RegisteredCommandNames() []string {
	names := make([]string, 0, len(core.Commands))
	for _, command := range core.Commands {
		names = append(names, command.Name)
	}
	return names
}

func writeJSON(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, string(raw))
	return err
}

// configPaths is the single source of truth for where the CLI looks for config:
// the project.yml at the spec root. Config layers as project YAML then env.
func configPaths(root string) core.ConfigPaths {
	return core.ConfigPaths{Project: filepath.Join(root, "project.yml")}
}

func contextBudget(root string) int {
	config, _ := core.LoadConfig(configPaths(root), getenv())
	return config.Context.MaxTokens
}

func getenv() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		key, value, ok := strings.Cut(kv, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

type specData struct {
	Tasks    []core.TaskRow
	Evidence map[string]core.EvidenceRecord
}

func loadSpec(root, slug string) (specData, error) {
	// Reject traversal slugs before they build a filesystem path: an unchecked
	// slug like "../../x" escapes .specd/specs/ on both reads and writes. This
	// is the central chokepoint every spec-resolving verb funnels through.
	if err := core.ValidateSlug(slug); err != nil {
		return specData{}, core.Refusef("SPEC_INVALID", "%v", err)
	}
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	raw, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
	if err != nil {
		return specData{}, err
	}
	tasks, err := core.ParseTasksMd(raw)
	if err != nil {
		return specData{}, err
	}
	evidence, err := core.LoadEvidence(core.EvidencePath(root, slug))
	if err != nil {
		return specData{}, err
	}
	return specData{Tasks: tasks.Tasks, Evidence: evidence}, nil
}

// buildCheckCtx assembles the pure inputs the gate registry runs over: the
// tasks and their marker status, the requirements/design bytes plus the stubs
// to compare them against, and — when state.json exists — approval state and
// the machine-truth task status. approveTarget is the gate under approval
// ("design" arms the design-stub gate); "" for a plain check.
func buildCheckCtx(root, slug string, spec specData, approveTarget string) gates.CheckCtx {
	cfg := loadSpecConfig(root)
	ctx := gates.CheckCtx{
		Root:                   root,
		Slug:                   slug,
		Tasks:                  spec.Tasks,
		Status:                 taskStatus(spec.Tasks),
		Evidence:               spec.Evidence,
		MaxContextTokens:       contextBudget(root),
		ApproveTarget:          approveTarget,
		RequirementsStub:       requirementsStub(slug),
		TrivialVerify:          cfg.Verify.Trivial,
		ProductionPolicy:       cfg.IntegrationPolicyArmed(),
		ProductionProfile:      cfg.ProductionProfile(),
		DesignContractRequired: cfg.ProductionProfile(),
		TaskTraceRequired:      cfg.ProductionProfile(),
	}
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	if b, err := os.ReadFile(filepath.Join(dir, "requirements.md")); err == nil {
		ctx.RequirementsDoc = string(b)
	}
	if provenance, err := core.LoadProvenance(core.ProvenancePath(root, slug)); err != nil {
		ctx.ProvenanceError = err.Error()
	} else {
		ctx.Provenance = provenance
	}
	if cfg.ProductionProfile() {
		ctx.MemoryLintRequired = true
		ctx.MemoryAsOf = core.Clock()
		for _, path := range []string{filepath.Join(core.SpecdDir(root), "steering", "memory.md"), core.SpecMemoryPath(root, slug)} {
			raw, readErr := os.ReadFile(path)
			if os.IsNotExist(readErr) {
				continue
			}
			if readErr != nil {
				ctx.MemoryLintError = readErr.Error()
				break
			}
			blocks, parseErr := core.IndexMemBlocks(string(raw))
			if parseErr != nil {
				ctx.MemoryLintError = parseErr.Error()
				break
			}
			ctx.MemoryBlocks = append(ctx.MemoryBlocks, blocks...)
		}
		decisions, decisionErr := core.LoadDecisions(core.DecisionPath(root, slug))
		exceptions, exceptionErr := core.LoadGovernanceExceptions(core.ExceptionPath(root, slug))
		if decisionErr != nil {
			ctx.GovernanceError = decisionErr.Error()
		} else if exceptionErr != nil {
			ctx.GovernanceError = exceptionErr.Error()
		}
		ctx.Decisions, ctx.Exceptions = decisions, exceptions
		ctx.GovernanceRequired = len(decisions) > 0 || len(exceptions) > 0 || ctx.GovernanceError != ""
		for _, decision := range decisions {
			ctx.RequiredDecisionIDs = append(ctx.RequiredDecisionIDs, decision.ID)
		}
		ctx.GovernanceNow = core.Clock()
	}
	if b, err := os.ReadFile(filepath.Join(dir, "design.md")); err == nil {
		ctx.DesignDoc = string(b)
		if approveTarget == string(core.StatusExecuting) && core.HasTaskTrace(ctx.Tasks) {
			if requirements, parseErr := core.ParseRequirements([]byte(ctx.RequirementsDoc)); parseErr == nil {
				ctx.CoverageGaps = coverageGaps(requirements, ctx.DesignDoc, ctx.Tasks)
			}
		}
		if approveTarget == string(core.StatusExecuting) {
			ctx.IntegrationEvidenceGaps = evidencePolicyGaps(ctx.DesignDoc, ctx.Tasks, ctx.ProductionPolicy)
		}
		ctx.DesignStub = designStub(slug)
	}
	if state, err := core.LoadState(core.StatePath(root, slug)); err == nil {
		ctx.StateLoaded = true
		_, ctx.ApprovedRequirements = state.Records["approval:requirements"]
		_, ctx.ApprovedDesign = state.Records["approval:design"]
		ctx.StateTaskStatus = state.TaskStatus
		if freshness, freshnessErr := state.StateFreshness(); freshnessErr == nil {
			ctx.StaleRecords = freshness.Stale
		}
	}
	// Opt-in per-criterion ratchet: only the completion transition consults it,
	// and only when config enabled it (spec 04 R6).
	if approveTarget == string(core.StatusComplete) {
		if cfg.CriteriaGateArmed() {
			ctx.CriteriaRequired = true
			ctx.CriteriaUnmet = unmetCriteria(root, slug, ctx.RequirementsDoc)
		}
		if cfg.ReviewGateArmed() {
			applyReviewInputs(&ctx, root, slug)
		}
	}
	return ctx
}

func coverageGaps(requirements core.RequirementsDoc, designRaw string, tasks []core.TaskRow) []string {
	findings := core.AnalyzeCoverage(requirements, core.ParseDesign([]byte(designRaw)), tasks)
	gaps := make([]string, 0, len(findings))
	for _, finding := range findings {
		if finding.Requirement != "" {
			gaps = append(gaps, finding.Requirement)
		}
	}
	return dedupStrings(gaps)
}

func evidencePolicyGaps(designRaw string, tasks []core.TaskRow, production bool) []string {
	findings := core.BoundaryEvidenceFindings(core.ParseDesign([]byte(designRaw)), tasks, production)
	gaps := make([]string, 0, len(findings))
	for _, finding := range findings {
		gaps = append(gaps, finding.Message)
	}
	return gaps
}

func dedupStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// applyReviewInputs reads and parses review_report.md into the gate context for
// the completion transition (spec 09). The gate stays pure: all disk access and
// parsing happen here, in the caller. A missing or malformed report sets
// ReviewParseErr so the gate fails closed (R5).
func applyReviewInputs(ctx *gates.CheckCtx, root, slug string) {
	ctx.ReviewRequired = true
	ctx.ReviewExpectedHead = gitHead(root)
	raw, err := os.ReadFile(core.ReviewReportPath(root, slug))
	if err != nil {
		ctx.ReviewParseErr = fmt.Sprintf("no review report — run `specd review %s`", slug)
		return
	}
	report, err := core.ParseReviewReport(string(raw))
	if err != nil {
		ctx.ReviewParseErr = err.Error()
		return
	}
	ctx.ReviewVerdict = report.Verdict
	ctx.ReviewHead = report.Head
	ctx.ReviewFindings = report.Findings
}

func taskStatus(tasks []core.TaskRow) map[string]core.TaskRunStatus {
	status := make(map[string]core.TaskRunStatus, len(tasks))
	for _, task := range tasks {
		switch task.Marker {
		case "✅", "done", "complete":
			status[task.ID] = core.TaskComplete
		case "🚧", "running":
			status[task.ID] = core.TaskRunning
		case "⛔", "blocked":
			status[task.ID] = core.TaskBlocked
		default:
			status[task.ID] = core.TaskPending
		}
	}
	return status
}

func flagEnabled(flags map[string]string, name string) bool {
	value, ok := flags[name]
	return ok && (value == "" || value == "true" || value == "1")
}

func gitHead(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func gitDiff(root string) string {
	out, err := exec.Command("git", "-C", root, "diff", "--binary").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func gitApply(root, patch string, reverse bool) error {
	args := []string{"-C", root, "apply"}
	if reverse {
		args = append(args, "-R")
	}
	cmd := exec.Command("git", args...)
	cmd.Stdin = strings.NewReader(patch)
	return cmd.Run()
}
