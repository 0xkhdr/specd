package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	speccontext "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	"github.com/0xkhdr/specd/internal/core/gates/security"
	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
	"github.com/0xkhdr/specd/internal/mcp"
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

var executable = map[string]Handler{
	"approve":   runApproveOrException,
	"adapters":  runAdapters,
	"agents":    runAgents,
	"brain":     runBrain,
	"check":     runCheck,
	"context":   runContext,
	"decision":  runDecision,
	"handshake": runHandshake,
	"help":      runHelp,
	"init":      runInit,
	"incident":  runIncident,
	"link":      runLink,
	"mcp":       runMCP,
	"version":   runVersion,
	"memory":    runMemory,
	"midreq":    runMidreq,
	"new":       runNew,
	"next":      runNext,
	"release":   runRelease,
	"deploy":    runDeploy,
	"eval":      runEval,
	"report":    runReport,
	"review":    runReview,
	"spike":     runSpike,
	"status":    runStatus,
	"submit":    runSubmit,
	"task":      runTask,
	"unlink":    runUnlink,
	"verify":    runVerify,
}

func runApproveOrException(root string, args []string, flags map[string]string) error {
	if len(args) > 0 && args[0] == "exception" {
		return runSecurityException(root, args[1:], flags)
	}
	return runApprove(root, args, flags)
}

func runSecurityException(root string, args []string, flags map[string]string) error {
	if len(args) != 2 || (args[0] != "approve" && args[0] != "revoke") {
		return errors.New("usage: specd exception <approve|revoke> <finding> [governed exception fields]")
	}
	action := "suppress"
	if args[0] == "revoke" {
		action = "revoke"
	}
	// Evidence integrity and worker authority are constitutional constraints,
	// never policy findings an exception may suppress.
	scope := strings.ToLower(strings.TrimSpace(flags["scope"]))
	finding := strings.ToLower(strings.TrimSpace(args[1]))
	if strings.Contains(scope, "evidence") || strings.Contains(scope, "authority") || strings.Contains(finding, "evidence-integrity") || strings.Contains(finding, "worker-authority") {
		return errors.New("security exception cannot waive evidence integrity or broaden worker authority")
	}
	return security.AppendException(root, security.Exception{
		Finding: args[1], Action: action, Reason: flags["reason"], Ticket: flags["ticket"], Owner: flags["owner"], Scope: flags["scope"], Revision: flags["revision"], Environment: flags["environment"], IssuedAt: flags["issued-at"], ExpiresAt: flags["expires-at"], CompensatingControl: flags["control"], Approver: flags["approver"],
	})
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

func runCheck(root string, args []string, flags map[string]string) error {
	// `check --security` with no slug runs a repo-wide security scan independent
	// of any spec (the scanners read tracked files, not spec state). All other
	// forms require a slug.
	securityOnly := len(args) == 0 && flagEnabled(flags, "security") &&
		!flagEnabled(flags, "schema") && !flagEnabled(flags, "schema-only")
	if !securityOnly && len(args) != 1 {
		return errors.New("usage: specd check <slug> [--json] [--security] [--schema] [--schema-only]")
	}
	slug := ""
	if len(args) == 1 {
		slug = args[0]
	}
	findings := []gates.Finding{}
	if securityOnly {
		cfg, _ := core.LoadConfig(configPaths(root), getenv())
		findings = append(findings, security.GateFindings(security.Analyze(root, cfg.Security))...)
	} else if !flagEnabled(flags, "schema-only") {
		spec, err := loadSpec(root, slug)
		if err != nil {
			return err
		}
		cfg, diagnostics := core.LoadConfig(configPaths(root), getenv())
		for _, d := range diagnostics {
			if d.Severity == "error" {
				return fmt.Errorf("load config: %s", d.Message)
			}
		}
		policy, err := security.ResolvePolicy(cfg.Security)
		if err != nil {
			return err
		}
		registry := gates.CoreRegistry()
		if policy.Profile == "production" {
			registry = gates.CoreRegistryWith(security.New(security.ConfigForPolicy(cfg.Security, policy)))
		}
		findings = append(findings, registry.Run(buildCheckCtx(root, slug, spec, ""))...)
		if flagEnabled(flags, "security") || policy.Profile == "production" {
			result := security.Analyze(root, security.ConfigForPolicy(cfg.Security, policy))
			if policy.Profile != "production" {
				findings = append(findings, security.GateFindings(result)...)
			}
			if err := recordSecurity(root, slug, policy, result); err != nil {
				return err
			}
		}
	}
	if flagEnabled(flags, "schema") || flagEnabled(flags, "schema-only") {
		findings = append(findings, schemaFindings(root, slug)...)
	}
	if flagEnabled(flags, "json") {
		return json.NewEncoder(os.Stdout).Encode(findings)
	}
	for _, finding := range findings {
		fmt.Fprintf(os.Stdout, "%s %s: %s\n", finding.Severity, finding.Gate, finding.Message)
	}
	if gates.HasErrors(findings) {
		return errors.New("check failed")
	}
	return nil
}

func requiredRegistry(root string) (gates.Registry, security.PolicyV1, error) {
	cfg, diagnostics := core.LoadConfig(configPaths(root), getenv())
	for _, d := range diagnostics {
		if d.Severity == "error" {
			return gates.Registry{}, security.PolicyV1{}, fmt.Errorf("load config: %s", d.Message)
		}
	}
	policy, err := security.ResolvePolicy(cfg.Security)
	if err != nil {
		return gates.Registry{}, security.PolicyV1{}, err
	}
	if policy.Profile == "production" {
		return gates.CoreRegistryWith(security.New(security.ConfigForPolicy(cfg.Security, policy))), policy, nil
	}
	return gates.CoreRegistry(), policy, nil
}

// recordSecurity persists the security analysis under state.security so reports
// and history can consume it (spec 05 R6). A missing state.json (unscaffolded or
// non-spec slug) is not fatal — the gate still reports; recording is best-effort
// for consumers. Findings are stored verbatim, including allowlisted ones.
type securityEvidenceRecord struct {
	PolicyVersion   string             `json:"policy_version"`
	PolicyDigest    string             `json:"policy_digest"`
	SubjectHead     string             `json:"subject_head"`
	SubjectRevision int64              `json:"subject_revision"`
	Findings        []security.Finding `json:"findings"`
}

func recordSecurity(root, slug string, policy security.PolicyV1, result security.Result) error {
	statePath := core.StatePath(root, slug)
	if _, err := os.Stat(statePath); err != nil {
		return nil // nothing to record against; gate output already emitted
	}
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		if state.Records == nil {
			state.Records = map[string]json.RawMessage{}
		}
		record := securityEvidenceRecord{PolicyVersion: policy.PolicyVersion, PolicyDigest: policy.PolicyDigest, SubjectHead: gitHead(root), SubjectRevision: state.Revision, Findings: result.Findings}
		raw, err := json.Marshal(record)
		if err != nil {
			return struct{}{}, err
		}
		state.Records["security"] = raw
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	return err
}

func runAgents(root string, args []string, flags map[string]string) error {
	if len(args) == 2 && args[0] == "guide" {
		guide, err := driverGuideForSpec(root, args[1])
		if err != nil {
			return err
		}
		if flagEnabled(flags, "json") {
			return writeJSON(guide)
		}
		for _, action := range guide.NextActions {
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s %s\n", action.Actor, action.SideEffect, action.Command, strings.Join(action.Args, " "))
		}
		return nil
	}
	if len(args) == 1 && args[0] == "doctor" {
		findings := core.Doctor(root, getenv()["SPECD_SPEC"])
		if flagEnabled(flags, "json") {
			return writeJSON(findings)
		}
		for _, finding := range findings {
			fmt.Fprintf(os.Stdout, "%s %s %s: %s; fix: %s\n", finding.Severity, finding.Code, finding.Ref, finding.Message, finding.RecoveryAction)
		}
		return nil
	}
	if len(args) != 0 {
		return errors.New("usage: specd agents [doctor | guide <slug>] [--json]")
	}
	discovery := core.DiscoverAgents(root)
	if flags["json"] == "true" {
		return writeJSON(discovery)
	}
	for _, agent := range discovery {
		fmt.Fprintf(os.Stdout, "%s\t%s\n", agent.Name, agent.Status)
		for _, rel := range agent.Missing {
			fmt.Fprintf(os.Stdout, "  missing\t%s\n", rel)
		}
		for _, rel := range agent.Invalid {
			fmt.Fprintf(os.Stdout, "  invalid\t%s\n", rel)
		}
	}
	return nil
}

func driverGuideForSpec(root, slug string) (core.DriverGuideV1, error) {
	legacy, err := guidanceForSpec(root, slug)
	if err != nil {
		return core.DriverGuideV1{}, err
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return core.DriverGuideV1{}, err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return core.DriverGuideV1{}, err
	}
	var frontier []string
	if requireTaskGate(root, slug) == nil {
		if rows, e := core.FrontierExcluding(spec.Tasks, taskStatus(spec.Tasks), nil); e == nil {
			for _, row := range rows {
				frontier = append(frontier, row.ID)
			}
		}
	}
	var approvals []string
	for _, name := range []string{"requirements", "design", "tasks", "complete"} {
		if _, ok := state.Records["approval:"+name]; ok {
			approvals = append(approvals, name)
		}
	}
	var blockers []core.DriverFinding
	for i, message := range legacy.Blockers {
		blockers = append(blockers, core.DriverFinding{Code: fmt.Sprintf("GATE_BLOCKER_%03d", i+1), Severity: "error", Ref: slug, Message: message, RecoveryAction: "fix artifact and run `specd check " + slug + "`"})
	}
	return core.ProjectDriverGuide(filepath.Clean(root), slug, state.Status, approvals, frontier, blockers), nil
}

func runInit(root string, args []string, flags map[string]string) error {
	if len(args) != 0 {
		return errors.New("usage: specd init [--agent=<name>] [--repair|--refresh] [--dry-run]")
	}
	repair := flagEnabled(flags, "repair")
	refresh := flagEnabled(flags, "refresh")
	dryRun := flagEnabled(flags, "dry-run")

	// Plain init: scaffold missing assets (idempotent — existing files preserved).
	if !repair && !refresh {
		if dryRun {
			// A dry-run init still reports what a repair would change on top of the
			// scaffold, so a fresh run previews the managed regions it would write.
			return previewManaged(root)
		}
		return core.WriteScaffold(root, flags["agent"])
	}

	// Repair/refresh re-sync every managed region from the current templates,
	// leaving content outside the markers untouched (R3/R4).
	if dryRun {
		return previewManaged(root)
	}
	changes, err := core.ApplyManagedRepair(root)
	if err != nil {
		return err
	}
	verb := "repaired"
	if refresh {
		verb = "refreshed"
	}
	if len(changes) == 0 {
		fmt.Fprintln(os.Stdout, "all managed regions already in sync")
		return nil
	}
	for _, change := range changes {
		fmt.Fprintf(os.Stdout, "%s %s\n", verb, change.RelPath)
	}
	return nil
}

// previewManaged prints the unified-diff-style preview of every managed-region
// change and writes nothing (spec 11 R5).
func previewManaged(root string) error {
	changes, err := core.PlanManagedRepair(root)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "project.yml")); os.IsNotExist(err) {
		fmt.Fprintln(os.Stdout, "+ project.yml (new operator config)")
	} else if err != nil {
		return err
	}
	if len(changes) == 0 {
		fmt.Fprintln(os.Stdout, "no managed-region changes")
		return nil
	}
	for _, change := range changes {
		fmt.Fprint(os.Stdout, core.Unifiedish(change))
	}
	return nil
}

func runContext(root string, args []string, flags map[string]string) error {
	if len(args) != 2 {
		return errors.New("usage: specd context <slug> <task> [--json]")
	}
	spec, err := loadSpec(root, args[0])
	if err != nil {
		return err
	}
	manifest, err := speccontext.BuildManifest(root, args[0], spec.Tasks, args[1], contextBudget(root))
	if err != nil {
		return err
	}
	hud := flagEnabled(flags, "hud")
	asJSON := flagEnabled(flags, "json")
	if hud && asJSON {
		return errors.New("usage: specd context <slug> <task> [--json|--hud]: choose one render")
	}
	if asJSON {
		config, _ := core.LoadConfig(configPaths(root), getenv())
		machine, err := speccontext.BuildManifestV2(root, args[0], spec.Tasks, args[1], "context", "execute", contextBudget(root), core.BootstrapHandshake(config))
		if err != nil {
			return err
		}
		return writeJSON(machine)
	}
	if hud {
		fmt.Fprint(os.Stdout, speccontext.RenderHUD(manifest))
		return nil
	}
	for _, item := range manifest.Items {
		if item.Path != "" {
			fmt.Fprintln(os.Stdout, item.Path)
		}
	}
	return nil
}

func runNext(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: specd next <slug> [--json|--waves]")
	}
	spec, err := loadSpec(root, args[0])
	if err != nil {
		return err
	}
	if flagEnabled(flags, "waves") {
		waves, err := core.ProjectWaves(spec.Tasks)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(waves)
	}
	if err := requireTaskGate(root, args[0]); err != nil {
		// Machine callers (--json / --dispatch) get an empty frontier plus the
		// gate reason rather than a bare error, so a dispatch loop can read the
		// blocker without parsing stderr.
		if flagEnabled(flags, "json") || flagEnabled(flags, "dispatch") {
			return writeJSON(map[string]any{"items": []any{}, "reason": err.Error()})
		}
		return err
	}
	escalated, err := escalatedCounts(root, args[0], spec.Tasks)
	if err != nil {
		return err
	}
	frontier, err := core.FrontierExcluding(spec.Tasks, taskStatus(spec.Tasks), escalatedBoolSet(escalated))
	if err != nil {
		return err
	}
	if flagEnabled(flags, "dispatch") {
		if len(frontier) == 0 {
			return writeJSON(map[string]any{"items": nil})
		}
		config, _ := core.LoadConfig(configPaths(root), getenv())
		manifest, err := speccontext.BuildManifestV2(root, args[0], spec.Tasks, frontier[0].ID, "dispatch", "execute", contextBudget(root), core.BootstrapHandshake(config))
		if err != nil {
			return err
		}
		return writeJSON(map[string]any{"items": manifest})
	}
	if flagEnabled(flags, "json") {
		return writeJSON(frontier)
	}
	for _, task := range frontier {
		fmt.Fprintln(os.Stdout, task.ID)
	}
	return nil
}

func runMCP(root string, args []string, flags map[string]string) error {
	// `--config <host>` prints a paste-ready MCP config snippet instead of serving
	// (spec 11 R1). --root/--spec pin the server's cwd and active spec.
	if host, ok := flags["config"]; ok {
		if len(args) != 0 {
			return errors.New("usage: specd mcp --config <host> [--root <path>] [--spec <slug>]")
		}
		snippet, err := core.MCPConfigSnippet(host, flags["root"], flags["spec"])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUsage, err)
		}
		fmt.Fprint(os.Stdout, snippet)
		return nil
	}
	if len(args) != 0 {
		return errors.New("usage: mcp")
	}
	return mcp.Serve(os.Stdin, os.Stdout, mcp.CoreTools(), mcpExecutor(root))
}

func runHandshake(root string, args []string, flags map[string]string) error {
	if len(args) < 1 || len(args) > 2 || args[0] != "bootstrap" {
		return errors.New("usage: handshake bootstrap [<spec>] [--json] [--expect-<identity> <value>]")
	}
	config, _ := core.LoadConfig(configPaths(root), getenv())
	explicit := ""
	if len(args) == 2 {
		explicit = args[1]
	}
	var state *core.State
	var nextCommands []string
	resolution, resolveErr := core.ResolveSpec(root, explicit, os.Getenv("SPECD_SPEC"))
	if resolveErr == nil {
		current, err := core.LoadState(core.StatePath(root, resolution.Slug))
		if err != nil {
			return err
		}
		state = &current
		if guide, err := driverGuideForSpec(root, resolution.Slug); err == nil {
			for _, action := range guide.NextActions {
				nextCommands = append(nextCommands, strings.TrimSpace("specd "+action.Command+" "+strings.Join(action.Args, " ")))
			}
		} else {
			nextCommands = []string{"specd status " + resolution.Slug + " --guide --json"}
		}
	} else if core.FindingCode(resolveErr) == "SPEC_REQUIRED" {
		nextCommands = []string{"specd new <slug> --title <title>"}
	} else {
		return resolveErr
	}
	handshake, err := core.BootstrapHandshakeForRoot(root, config, state, nextCommands)
	if err != nil {
		return err
	}
	activeSlug, revision := "<none>", "<none>"
	if handshake.ActiveSpec != nil {
		activeSlug = handshake.ActiveSpec.Slug
		revision = strconv.FormatInt(handshake.ActiveSpec.Revision, 10)
	}
	preconditions := []struct{ flag, current string }{
		{"binary-version", handshake.Binary.Version},
		{"binary-commit", handshake.Binary.Commit},
		{"state-schema", strconv.Itoa(handshake.StateSchemaVersion)},
		{"context-schema", handshake.ContextSchemaVersion},
		{"template-schema", strconv.Itoa(handshake.TemplateSchemaVersion)},
		{"root", handshake.WorkspaceRoot},
		{"spec", activeSlug},
		{"revision", revision},
		{"palette-digest", handshake.PaletteDigest},
		{"config-digest", handshake.ConfigDigest},
		{"managed-digest", handshake.ManagedDigest},
	}
	for _, precondition := range preconditions {
		if expected, ok := flags["expect-"+precondition.flag]; ok && expected != precondition.current {
			legacy := ""
			if precondition.flag == "palette-digest" {
				legacy = " (palette digest drift)"
			} else if precondition.flag == "config-digest" {
				legacy = " (config digest drift)"
			}
			return fmt.Errorf("precondition %s mismatch: expected %s, current %s%s", precondition.flag, expected, precondition.current, legacy)
		}
	}

	if flagEnabled(flags, "json") {
		return writeJSON(handshake)
	}
	fmt.Fprintf(os.Stdout, "version: %s\n", handshake.Version)
	fmt.Fprintf(os.Stdout, "palette_digest: %s\n", handshake.PaletteDigest)
	fmt.Fprintf(os.Stdout, "config_digest: %s\n", handshake.ConfigDigest)
	fmt.Fprintf(os.Stdout, "managed_digest: %s\n", handshake.ManagedDigest)
	for _, tool := range handshake.Tools {
		fmt.Fprintf(os.Stdout, "tool: %s\n", tool)
	}
	return nil
}

// guidanceForSpec builds the machine driving guidance for a spec (spec 01
// R6.1): current phase, the artifact it must produce, the machine-legal
// commands, the human-only actions, and the deterministic blockers that stop the
// next approval. Blockers come from the gate registry run for the next gate; the
// guidance never invents them.
func guidanceForSpec(root, slug string) (core.Guidance, error) {
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return core.Guidance{}, err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return core.Guidance{}, err
	}
	var blockers []string
	if next := core.NextStatus(state.Status); next != "" {
		for _, f := range gates.CoreRegistry().Run(buildCheckCtx(root, slug, spec, string(next))) {
			if f.Severity == gates.Error {
				blockers = append(blockers, f.Message)
			}
		}
	}
	g := core.GuidanceForPhase(state.Status, blockers)
	// R6.2: only suggest task-bearing commands (task verify/context) when the
	// spec actually has an executable task. requireTaskGate fails closed before
	// execution is approved; an empty frontier means nothing to run.
	if !hasExecutableTask(root, slug, spec) {
		kept := g.LegalCommands[:0]
		for _, name := range g.LegalCommands {
			if c, ok := core.CommandByName(name); ok && c.RequiresTask {
				continue
			}
			kept = append(kept, name)
		}
		g.LegalCommands = kept
	}
	return g, nil
}

// hasExecutableTask reports whether the spec has a task ready to run: execution
// must be gate-approved and the frontier non-empty.
func hasExecutableTask(root, slug string, spec specData) bool {
	if requireTaskGate(root, slug) != nil {
		return false
	}
	frontier, err := core.FrontierExcluding(spec.Tasks, taskStatus(spec.Tasks), nil)
	return err == nil && len(frontier) > 0
}

func runStatus(root string, args []string, flags map[string]string) error {
	if flagEnabled(flags, "program") {
		if len(args) != 0 {
			return errors.New("usage: specd status --program (takes no spec)")
		}
		view, err := renderProgram(root)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, view)
		return nil
	}
	if flagEnabled(flags, "guide") {
		if len(args) != 1 {
			return errors.New("usage: specd status <spec> --guide [--json]")
		}
		return emitGuidance(root, args[0], flagEnabled(flags, "json"))
	}
	if len(args) != 1 {
		return errors.New("usage: status slug [--json]")
	}
	model, err := reportModel(root, args[0])
	if err != nil {
		return err
	}
	coverage, err := criterionCoverage(root, args[0])
	if err != nil {
		return err
	}
	spec, err := loadSpec(root, args[0])
	if err != nil {
		return err
	}
	escalated, ratchetActive, err := escalatedAdvisory(root, args[0], spec.Tasks)
	if err != nil {
		return err
	}
	if flagEnabled(flags, "json") {
		// Records are projected verbatim (RawMessage), never re-synthesized, so
		// decision/midreq text/scope/actor/timestamp round-trip exactly (R3.4).
		state, err := core.LoadState(core.StatePath(root, args[0]))
		if err != nil {
			return err
		}
		return writeJSON(struct {
			core.ReportModel
			Records   map[string]json.RawMessage `json:"records,omitempty"`
			Criteria  []requirementCoverage      `json:"criteria,omitempty"`
			Escalated map[string]int             `json:"escalated,omitempty"`
		}{model, state.Records, coverage, escalated})
	}
	fmt.Fprint(os.Stdout, core.RenderStatus(model))
	fmt.Fprint(os.Stdout, renderCriterionCoverage(coverage))
	fmt.Fprint(os.Stdout, renderEscalated(escalated, ratchetActive))
	return nil
}

// renderEscalated formats the escalated-task section for `status` text output.
// When the ratchet is active the tasks are genuinely blocked; when disabled the
// section is advisory (repeated failures still surfaced, spec 06 R2/R6).
func renderEscalated(escalated map[string]int, ratchetActive bool) string {
	if len(escalated) == 0 {
		return ""
	}
	header := "Escalated (advisory; ratchet disabled):"
	if ratchetActive {
		header = "Escalated (blocked — clear with `specd task <id> --override --reason <text>`):"
	}
	var b strings.Builder
	b.WriteString("\n" + header + "\n")
	for _, id := range sortedKeys(escalated) {
		fmt.Fprintf(&b, "  %s — %d consecutive verify failures\n", id, escalated[id])
	}
	return b.String()
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func runReport(root string, args []string, flags map[string]string) error {
	if len(args) == 0 && flagEnabled(flags, "portfolio") {
		program, err := core.LoadProgram(core.ProgramPath(root))
		if err != nil {
			return err
		}
		var inputs []core.PortfolioSpec
		for _, slug := range core.ListSpecs(root) {
			state, err := core.LoadState(core.StatePath(root, slug))
			if err != nil {
				return err
			}
			deployments, err := core.ReadDeployments(core.DeploymentLedgerPath(root, slug))
			if err != nil {
				return err
			}
			inputs = append(inputs, core.PortfolioSpec{SpecID: slug, Complete: state.Status == core.StatusComplete, Deployments: deployments})
		}
		view, err := core.BuildPortfolioView(program, inputs)
		if err != nil {
			return err
		}
		return writeJSON(view)
	}
	if len(args) != 1 {
		return errors.New("usage: report slug [--pr|--metrics|--efficiency|--rollup|--delivery|--json|--history|--proof|--trace|--format prometheus|event|otel] | report --portfolio")
	}
	model, err := reportModel(root, args[0])
	if err != nil {
		return err
	}
	if flagEnabled(flags, "delivery") {
		records, err := core.ReadDeployments(core.DeploymentLedgerPath(root, args[0]))
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, renderDeliveryReport(records))
		return nil
	}
	if flagEnabled(flags, "rollup") {
		rollup, err := gatherProgramEconomics(root)
		if err != nil {
			return err
		}
		return writeJSON(rollup)
	}
	// --proof emits the deterministic R8.2 lifecycle proof: requirement-to-evidence
	// coverage, stale records, amendments, and escaped-defect links. Pure projection
	// of on-disk state; honours --json for a machine-readable object.
	if flagEnabled(flags, "proof") {
		proof, err := gatherLifecycleProof(root, args[0])
		if err != nil {
			return err
		}
		if flagEnabled(flags, "json") {
			out, err := core.RenderLifecycleProofJSON(proof)
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, out)
			return nil
		}
		fmt.Fprint(os.Stdout, core.RenderLifecycleProof(proof))
		return nil
	}
	// --history replays the spec's audit trail from existing records (spec 13);
	// it writes nothing and honours --json for machine-readable JSON Lines (R6).
	if flagEnabled(flags, "history") {
		events, err := gatherHistory(root, args[0], model)
		if err != nil {
			return err
		}
		if flagEnabled(flags, "json") {
			out, err := core.RenderHistoryJSON(events)
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, out)
			return nil
		}
		fmt.Fprint(os.Stdout, core.RenderHistory(args[0], events))
		return nil
	}
	// --trace exports the spec's metadata-only run trace as stable JSON Lines
	// (spec 07 R6); a pure projection of on-disk records that writes nothing.
	if flagEnabled(flags, "trace") {
		out, err := runTrace(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	}
	if flagEnabled(flags, "efficiency") {
		out, err := gatherContextEfficiency(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	}
	// --format prometheus emits a textfile-collector exposition (spec 13 R4).
	if flags["format"] == "prometheus" {
		metrics, err := gatherPrometheus(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, core.RenderPrometheus(metrics))
		return nil
	}
	// --format otel maps the spec's local observable-event traces to
	// OpenTelemetry-compatible spans via the external adapter (spec 10 R10.2).
	if flags["format"] == "otel" {
		out, err := runOTelExport(root, args[0])
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	}
	if flags["format"] == "event" {
		out, err := runEvents(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	}
	if format, ok := flags["format"]; ok && format != "" {
		return fmt.Errorf("%w: unsupported --format %q (only prometheus, event, otel)", ErrUsage, format)
	}
	coverage, err := criterionCoverage(root, args[0])
	if err != nil {
		return err
	}
	switch {
	case flagEnabled(flags, "json"):
		return writeJSON(struct {
			core.ReportModel
			Criteria []requirementCoverage `json:"criteria,omitempty"`
		}{model, coverage})
	case flagEnabled(flags, "metrics"):
		fmt.Fprint(os.Stdout, core.RenderMetrics(model))
		telemetry, err := aggregateTelemetry(root, args[0], model)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, core.RenderTelemetry(args[0], telemetry))
	case flagEnabled(flags, "pr"):
		fmt.Fprint(os.Stdout, core.PRSummary(model))
	default:
		fmt.Fprint(os.Stdout, core.RenderStatus(model))
		fmt.Fprint(os.Stdout, renderCriterionCoverage(coverage))
	}
	return nil
}

func reportModel(root, slug string) (core.ReportModel, error) {
	spec, err := loadSpec(root, slug)
	if err != nil {
		return core.ReportModel{}, err
	}
	evidence, err := core.LoadEvidence(core.EvidencePath(root, slug))
	if err != nil {
		return core.ReportModel{}, err
	}
	return core.BuildReportModel(slug, spec.Tasks, taskStatus(spec.Tasks), evidence), nil
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

func runVerify(root string, args []string, flags map[string]string) error {
	if _, ok := flags["criterion"]; ok {
		return runVerifyCriterion(root, args, flags)
	}
	if len(args) != 2 {
		return errors.New("usage: specd verify <slug> <task>")
	}
	slug, taskID := args[0], args[1]
	if err := requireTaskGate(root, slug); err != nil {
		return err
	}
	annotations, err := parseAnnotations(flags)
	if err != nil {
		return err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return err
	}
	var task core.TaskRow
	for _, candidate := range spec.Tasks {
		if candidate.ID == taskID {
			task = candidate
			break
		}
	}
	if task.ID == "" {
		return fmt.Errorf("task %s not found", taskID)
	}
	// Escalation ratchet (spec 06 R2): once a task has failed verify N times in a
	// row, block further attempts until a human clears it. This is not a bypass —
	// the override only resets the counter; the task still needs a passing verify.
	if count, err := taskFailCount(root, slug, taskID); err != nil {
		return err
	} else if core.IsEscalated(count, escalationMaxFails(root)) {
		return fmt.Errorf("task %s is escalated after %d consecutive verify failures; clear it with `specd task %s --override --reason <text>` before re-attempting", taskID, count, taskID)
	}
	run := func() (verifyexec.Result, error) {
		cfg, diagnostics := core.LoadConfig(configPaths(root), getenv())
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == "error" {
				return verifyexec.Result{ExitCode: 2}, fmt.Errorf("load config: %s", diagnostic.Message)
			}
		}
		return verifyexec.Run(context.Background(), verifyexec.Options{
			Command:        task.Verify,
			Dir:            root,
			Sandbox:        flagEnabled(flags, "sandbox"),
			RequireSandbox: cfg.Security.RequiresVerifySandbox(),
			SandboxBinary:  flags["sandbox-binary"],
			TimeoutSecs:    verifyTimeoutSecs(root),
		})
	}
	var result verifyexec.Result
	if flagEnabled(flags, "revert-on-fail") {
		result, err = withRevertOnFail(root, run)
	} else {
		result, err = run()
	}
	head := gitHead(root)
	if !core.HeadPinned(head) {
		fmt.Fprintf(os.Stderr, "warning: git HEAD unresolved (%q); this evidence cannot pin to a commit and will not count toward `task complete`\n", head)
	}
	record := core.EvidenceRecord{TaskID: taskID, Command: task.Verify, ExitCode: result.ExitCode, GitHead: head, Telemetry: annotations}
	if appendErr := core.AppendEvidence(core.EvidencePath(root, slug), record); appendErr != nil && err == nil {
		err = appendErr
	}
	// Allocate this attempt's run/attempt identity through the shared core
	// allocator (spec 07 R2.1/R2.2): a manual verify accrues an attempt on the
	// task's run chain, monotonic through the fail/fail/pass loop. The ledger is
	// additive — an allocation failure never blocks the verify record above.
	if _, allocErr := core.AllocateRun(root, slug, taskID, head, "", "", core.TelemetrySourceWorker); allocErr != nil && err == nil {
		err = allocErr
	}
	if result.Stdout != "" {
		fmt.Fprint(os.Stdout, core.TruncateEvidenceOutput(result.Stdout))
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, core.TruncateEvidenceOutput(result.Stderr))
	}
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("verify failed with exit code %d", result.ExitCode)
	}
	return nil
}

type specData struct {
	Tasks    []core.TaskRow
	Evidence map[string]core.EvidenceRecord
}

func requireTaskGate(root, slug string) error {
	state, err := core.LoadState(filepath.Join(root, ".specd", "specs", slug, "state.json"))
	if err != nil {
		return err
	}
	switch state.Status {
	case core.StatusTasks, core.StatusComplete:
		return nil
	default:
		if state.Records != nil {
			if _, ok := state.Records["approval:requirements"]; ok {
				if _, ok := state.Records["approval:design"]; ok {
					return nil
				}
			}
		}
		return fmt.Errorf("missing approval: requirements and design gates must be approved before task execution")
	}
}

func loadSpec(root, slug string) (specData, error) {
	// Reject traversal slugs before they build a filesystem path: an unchecked
	// slug like "../../x" escapes .specd/specs/ on both reads and writes. This
	// is the central chokepoint every spec-resolving verb funnels through.
	if err := core.ValidateSlug(slug); err != nil {
		return specData{}, err
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

func withRevertOnFail(root string, run func() (verifyexec.Result, error)) (verifyexec.Result, error) {
	before := gitDiff(root)
	result, err := run()
	if err == nil && result.ExitCode == 0 {
		return result, nil
	}
	after := gitDiff(root)
	if after != "" {
		_ = gitApply(root, after, true)
	}
	if before != "" {
		_ = gitApply(root, before, false)
	}
	return result, err
}

// runVerifyCriterion records a per-acceptance-criterion evidence record. It
// never runs a command and never writes a task verify record — a criterion
// record is operator-supplied and can never substitute for a task's passing
// verify (spec 04 R1, R7). Unknown criterion ids fail closed (exit 2, R2).
func runVerifyCriterion(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return fmt.Errorf("%w: specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text>", ErrUsage)
	}
	slug := args[0]
	if err := requireTaskGate(root, slug); err != nil {
		return err
	}
	id := flags["criterion"]
	status := flags["status"]
	if status != core.CriterionStatusPass && status != core.CriterionStatusFail {
		return fmt.Errorf("%w: --status must be pass or fail", ErrUsage)
	}
	evidence := strings.TrimSpace(flags["evidence"])
	if evidence == "" {
		return fmt.Errorf("%w: --evidence <text-or-path> required", ErrUsage)
	}
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	reqDoc, err := os.ReadFile(filepath.Join(dir, "requirements.md"))
	if err != nil {
		return fmt.Errorf("read requirements.md: %w", err)
	}
	if !gates.HasCriterion(string(reqDoc), id) {
		return fmt.Errorf("%w: unknown criterion %q — not an acceptance criterion in approved requirements.md", ErrUsage, id)
	}
	head := gitHead(root)
	if !core.HeadPinned(head) {
		fmt.Fprintf(os.Stderr, "warning: git HEAD unresolved (%q); this criterion record cannot pin to a commit\n", head)
	}
	rec := core.CriterionRecord{Criterion: id, Status: status, Evidence: evidence, GitHead: head}
	path := core.CriteriaPath(root, slug)
	if _, err := core.WithSpecLock(root, func() (struct{}, error) {
		return struct{}{}, core.AppendCriterion(path, rec)
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "recorded criterion %s = %s for %s\n", id, status, slug)
	return nil
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
