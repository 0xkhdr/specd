package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/schema"
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
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd check <slug> [--schema-only] [--security] [--all] [--json]")
	if !ok {
		return code
	}
	if args.Bool("security") {
		return runSecurityCheck(root, slug, args)
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	jsonOut := args.Bool("json")

	ctx, pre, err := buildCheckCtx(root, slug, args.Bool("all"))
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
func buildCheckCtx(root, slug string, guardrailsAll bool) (core.CheckCtx, []core.Violation, error) {
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
		Root:          root,
		Slug:          slug,
		ReqMd:         reqMd,
		Doc:           doc,
		State:         state,
		Cfg:           core.LoadConfig(root),
		GuardrailsAll: guardrailsAll,
	}, pre, nil
}

// runValidate checks a spec's on-disk state.json against the embedded JSON
// Schema. It is a *format* conformance mode — structural shape, required keys,
// closed property sets, enums — explicitly independent of the seven semantic
// gates (`specd check`). The `--schema` flag is required to select this mode so
// the command can grow other validation modes later without changing defaults.
// Read-only: it never writes state. `--version` selects a schema version.
func runValidate(args cli.Args) int {
	if !args.Bool("schema") {
		return usageExit("usage: specd validate <slug> --schema [--version <v>]")
	}
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd validate <slug> --schema [--version <v>]")
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}

	raw := core.ReadOrNull(filepath.Join(core.SpecDir(root, slug), "state.json"))
	if raw == nil {
		return specdExit(core.NotFoundError(fmt.Sprintf("no state.json for spec '%s'", slug)))
	}

	viols, err := schema.ValidateState([]byte(*raw), args.Str("version"))
	if err != nil {
		return specdExit(err)
	}

	if core.IsJSONMode() {
		if viols == nil {
			viols = []string{}
		}
		if err := core.PrintJSON(struct {
			Spec       string   `json:"spec"`
			Schema     string   `json:"schema"`
			Conformant bool     `json:"conformant"`
			Violations []string `json:"violations"`
		}{slug, schema.SchemaVersionID, len(viols) == 0, viols}); err != nil {
			return specdExit(err)
		}
		if len(viols) > 0 {
			return core.ExitGate
		}
		return core.ExitOK
	}

	if len(viols) == 0 {
		core.Info(fmt.Sprintf("schema: %s conforms to open spec format v%s", slug, schema.SchemaVersionID))
		return core.ExitOK
	}
	core.Error(fmt.Sprintf("schema: %s has %d conformance violation(s):", slug, len(viols)))
	for _, v := range viols {
		core.Error("  • " + v)
	}
	return core.ExitGate
}

// runSchema writes the embedded JSON Schema for the open spec format to stdout.
// `--version` selects a schema version (default: the current one); an unknown
// version fails closed. It is pure output — no spec, no .specd/ root required —
// so it works anywhere as the published, machine-readable format contract.
func runSchema(args cli.Args) int {
	doc, err := schema.Schema(args.Str("version"))
	if err != nil {
		return specdExit(err)
	}
	if _, err := os.Stdout.Write(doc); err != nil {
		return specdExit(err)
	}
	// Embedded schema files do not carry a trailing newline guarantee; add one
	// so piping to a terminal or file is clean.
	if len(doc) > 0 && doc[len(doc)-1] != '\n' {
		fmt.Println()
	}
	return core.ExitOK
}
