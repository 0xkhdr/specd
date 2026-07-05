package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	speccontext "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	"github.com/0xkhdr/specd/internal/core/gates/security"
	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
	"github.com/0xkhdr/specd/internal/mcp"
)

type Handler func(root string, args []string, flags map[string]string) error

var Registry = buildRegistry()

// ErrUnknownCommand is returned by Run for a verb that is not registered or
// carries no handler. The dispatcher must fail closed on it (exit 2), never 0.
var ErrUnknownCommand = errors.New("unknown command")

var executable = map[string]Handler{
	"approve":   runApprove,
	"brain":     runBrain,
	"check":     runCheck,
	"context":   runContext,
	"decision":  runDecision,
	"handshake": runHandshake,
	"help":      runHelp,
	"init":      runInit,
	"mcp":       runMCP,
	"version":   runVersion,
	"memory":    runMemory,
	"midreq":    runMidreq,
	"new":       runNew,
	"next":      runNext,
	"report":    runReport,
	"status":    runStatus,
	"task":      runTask,
	"verify":    runVerify,
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

func Run(root, name string, args []string, flags map[string]string) error {
	handler, ok := Registry[name]
	if !ok || handler == nil {
		return fmt.Errorf("%w: %q", ErrUnknownCommand, name)
	}
	return handler(root, args, flags)
}

func runCheck(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: specd check <slug> [--json] [--security] [--schema] [--schema-only]")
	}
	slug := args[0]
	findings := []gates.Finding{}
	if !flagEnabled(flags, "schema-only") {
		spec, err := loadSpec(root, slug)
		if err != nil {
			return err
		}
		registry := gates.CoreRegistry()
		if flagEnabled(flags, "security") {
			registry.Register(security.New())
		}
		findings = append(findings, registry.Run(buildCheckCtx(root, slug, spec, ""))...)
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

func runInit(root string, args []string, flags map[string]string) error {
	if len(args) != 0 {
		return errors.New("usage: specd init [--agent=<name>]")
	}
	return core.WriteScaffold(root)
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
		return writeJSON(manifest)
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
		if flagEnabled(flags, "json") || flagEnabled(flags, "context") {
			return writeJSON(map[string]any{"items": []any{}, "reason": err.Error()})
		}
		return err
	}
	frontier, err := core.Frontier(spec.Tasks, taskStatus(spec.Tasks))
	if err != nil {
		return err
	}
	if flagEnabled(flags, "dispatch") {
		if len(frontier) == 0 {
			return writeJSON(map[string]any{"items": nil})
		}
		manifest, err := speccontext.BuildManifest(root, args[0], spec.Tasks, frontier[0].ID, contextBudget(root))
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
	if len(args) != 0 {
		return errors.New("usage: mcp")
	}
	return mcp.Serve(os.Stdin, os.Stdout, mcp.CoreTools())
}

func runHandshake(root string, args []string, flags map[string]string) error {
	if len(args) != 1 || args[0] != "bootstrap" {
		return errors.New("usage: handshake bootstrap [--json]")
	}
	config, _ := core.LoadConfig(core.ConfigPaths{Project: filepath.Join(root, "project.yml")}, getenv())
	handshake := core.BootstrapHandshake(config)
	if flagEnabled(flags, "json") {
		return writeJSON(handshake)
	}
	fmt.Fprintf(os.Stdout, "version: %s\n", handshake.Version)
	for _, tool := range handshake.Tools {
		fmt.Fprintf(os.Stdout, "tool: %s\n", tool)
	}
	return nil
}

func runStatus(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: status slug [--json]")
	}
	model, err := reportModel(root, args[0])
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
			Records map[string]json.RawMessage `json:"records,omitempty"`
		}{model, state.Records})
	}
	fmt.Fprint(os.Stdout, core.RenderStatus(model))
	return nil
}

func runReport(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: report slug [--pr|--metrics|--json]")
	}
	model, err := reportModel(root, args[0])
	if err != nil {
		return err
	}
	switch {
	case flagEnabled(flags, "json"):
		return writeJSON(model)
	case flagEnabled(flags, "metrics"):
		fmt.Fprint(os.Stdout, core.RenderMetrics(model))
	case flagEnabled(flags, "pr"):
		fmt.Fprint(os.Stdout, core.PRSummary(model))
	default:
		fmt.Fprint(os.Stdout, core.RenderStatus(model))
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

func contextBudget(root string) int {
	paths := core.ConfigPaths{Project: filepath.Join(root, "project.yml")}
	config, _ := core.LoadConfig(paths, getenv())
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
	if len(args) != 2 {
		return errors.New("usage: specd verify <slug> <task>")
	}
	slug, taskID := args[0], args[1]
	if err := requireTaskGate(root, slug); err != nil {
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
	run := func() (verifyexec.Result, error) {
		return verifyexec.Run(context.Background(), verifyexec.Options{
			Command:       task.Verify,
			Dir:           root,
			Sandbox:       flagEnabled(flags, "sandbox"),
			SandboxBinary: flags["sandbox-binary"],
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
	record := core.EvidenceRecord{TaskID: taskID, Command: task.Verify, ExitCode: result.ExitCode, GitHead: head}
	if appendErr := core.AppendEvidence(core.EvidencePath(root, slug), record); appendErr != nil && err == nil {
		err = appendErr
	}
	if result.Stdout != "" {
		fmt.Fprint(os.Stdout, result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
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
	ctx := gates.CheckCtx{
		Root:             root,
		Slug:             slug,
		Tasks:            spec.Tasks,
		Status:           taskStatus(spec.Tasks),
		Evidence:         spec.Evidence,
		MaxContextTokens: contextBudget(root),
		ApproveTarget:    approveTarget,
		RequirementsStub: requirementsStub(slug),
	}
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	if b, err := os.ReadFile(filepath.Join(dir, "requirements.md")); err == nil {
		ctx.RequirementsDoc = string(b)
	}
	if approveTarget == "design" {
		if b, err := os.ReadFile(filepath.Join(dir, "design.md")); err == nil {
			ctx.DesignDoc = string(b)
		}
		ctx.DesignStub = designStub(slug)
	}
	if state, err := core.LoadState(core.StatePath(root, slug)); err == nil {
		ctx.StateLoaded = true
		_, ctx.ApprovedRequirements = state.Records["approval:requirements"]
		_, ctx.ApprovedDesign = state.Records["approval:design"]
		ctx.StateTaskStatus = state.TaskStatus
	}
	return ctx
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
