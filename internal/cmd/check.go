package cmd

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunCheck implements `specd check`: it loads the spec's artifacts and state,
// runs the gate pipeline (or, with --schema/--schema-only, just regenerates or
// validates the schema), and renders the resulting violations and warnings as
// JSON or human-readable text.
func RunCheck(args cli.Args) int {
	if args.Bool("schema") {
		return runSchema(args)
	}
	if args.Bool("schema-only") {
		validateArgs := args
		validateArgs.Flags = cloneFlags(args.Flags)
		validateArgs.Flags["schema"] = "true"
		return runValidate(validateArgs)
	}
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd check <slug> [--schema-only] [--json]")
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	jsonOut := args.Bool("json")

	ctx, pre, err := buildCheckCtx(root, slug)
	if err != nil {
		return specdExit(err)
	}

	v, warnings := core.RunGates(ctx)
	violations := append([]core.Violation{}, pre...)
	violations = append(violations, v...)

	if jsonOut {
		return renderCheckJSON(violations, warnings)
	}
	return renderCheckHuman(slug, violations, warnings)
}

// renderCheckJSON emits the machine-readable check result and maps the gate
// outcome to an exit code.
func renderCheckJSON(violations, warnings []core.Violation) int {
	if violations == nil {
		violations = []core.Violation{}
	}
	if warnings == nil {
		warnings = []core.Violation{}
	}
	out := map[string]interface{}{"ok": len(violations) == 0, "violations": violations, "warnings": warnings}
	if err := core.PrintJSON(out); err != nil {
		return specdExit(err)
	}
	if len(violations) == 0 {
		return core.ExitOK
	}
	return core.ExitGate
}

// renderCheckHuman prints warnings then the pass/fail summary for a check run.
func renderCheckHuman(slug string, violations, warnings []core.Violation) int {
	for _, w := range warnings {
		fmt.Printf("warn  %s: %s (%s)\n", w.Location, w.Message, w.Gate)
	}
	if len(violations) == 0 {
		warnNote := ""
		if len(warnings) > 0 {
			warnNote = fmt.Sprintf(" (%d warning(s))", len(warnings))
		}
		fmt.Printf("✓ check passed — all gates green for '%s'%s\n", slug, warnNote)
		return core.ExitOK
	}
	for _, v := range violations {
		errLine("fail  %s: %s (%s)", v.Location, v.Message, v.Gate)
	}
	errLine("\n✗ %d violation(s) across gates.", len(violations))
	return core.ExitGate
}

// buildCheckCtx loads the artifacts and state the gate pipeline reads. It
// returns the context, any pre-gate violations (currently only a tasks.md parse
// error surfaced as a task-schema violation), and a hard error for
// non-recoverable load failures.
func buildCheckCtx(root, slug string) (core.CheckCtx, []core.Violation, error) {
	var pre []core.Violation

	reqMd := core.ReadArtifact(root, slug, "requirements.md")

	tasksMdRaw := core.ReadArtifact(root, slug, "tasks.md")
	tasksMd := ""
	if tasksMdRaw != nil {
		tasksMd = *tasksMdRaw
	}
	var doc *core.ParsedTasks
	if strings.TrimSpace(tasksMd) != "" {
		parsed, parseErr := core.ParseTasks(tasksMd)
		if parseErr != nil {
			if se, ok := core.IsSpecdError(parseErr); ok {
				pre = append(pre, core.Violation{Gate: "task-schema", Location: "tasks.md", Message: se.Message})
			} else {
				return core.CheckCtx{}, nil, parseErr
			}
		} else {
			doc = &parsed
		}
	}

	state, err := core.LoadState(root, slug)
	if err != nil {
		return core.CheckCtx{}, nil, err
	}

	return core.CheckCtx{
		Root:  root,
		Slug:  slug,
		ReqMd: reqMd,
		Doc:   doc,
		State: state,
		Cfg:   core.LoadConfig(root),
	}, pre, nil
}
