package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunCheck(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}

	// Repo-global boot-freshness gate: not spec-scoped, so it runs before the
	// slug requirement.
	if args.Bool("boot") {
		return runBootCheck(root, args.Bool("json"))
	}
	if args.Bool("enrich") {
		return runEnrichCheck(root, args.Bool("json"))
	}

	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd check <slug> [--json]  |  specd check --boot  |  specd check --enrich")
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	jsonOut := args.Bool("json")

	ctx, pre, err := buildCheckCtx(root, slug)
	if err != nil {
		return specdExit(err)
	}

	violations := pre
	var warnings []core.Violation
	for _, gate := range core.CheckGates {
		v, w := gate(ctx)
		violations = append(violations, v...)
		warnings = append(warnings, w...)
	}

	if jsonOut {
		out := map[string]interface{}{"ok": len(violations) == 0, "violations": violations, "warnings": warnings}
		if violations == nil {
			out["violations"] = []core.Violation{}
		}
		if warnings == nil {
			out["warnings"] = []core.Violation{}
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		if len(violations) == 0 {
			return 0
		}
		return 1
	}

	for _, w := range warnings {
		fmt.Printf("warn  %s: %s (%s)\n", w.Location, w.Message, w.Gate)
	}
	if len(violations) == 0 {
		warnNote := ""
		if len(warnings) > 0 {
			warnNote = fmt.Sprintf(" (%d warning(s))", len(warnings))
		}
		fmt.Printf("✓ check passed — all gates green for '%s'%s\n", slug, warnNote)
		return 0
	}
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "fail  %s: %s (%s)\n", v.Location, v.Message, v.Gate)
	}
	fmt.Fprintf(os.Stderr, "\n✗ %d violation(s) across gates.\n", len(violations))
	return 1
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

// runBootCheck implements `specd check --boot`: the boot-freshness gate. It
// verifies that .specd/boot.json still matches the repository.
func runBootCheck(root string, jsonOut bool) int {
	res, err := core.CheckBootFreshness(root)
	if err != nil {
		return specdExit(err)
	}
	if jsonOut {
		b, _ := json.MarshalIndent(map[string]interface{}{
			"gate": "boot-freshness", "ok": !res.Stale, "issues": res.Issues,
		}, "", "  ")
		fmt.Println(string(b))
		if res.Stale {
			return core.ExitGate
		}
		return core.ExitOK
	}
	if !res.Stale {
		fmt.Println("✓ boot-freshness: .specd/boot.json matches the repository.")
		return core.ExitOK
	}
	for _, iss := range res.Issues {
		fmt.Fprintf(os.Stderr, "fail  boot.json: %s (boot-freshness)\n", iss)
	}
	fmt.Fprintf(os.Stderr, "\n✗ boot.json is stale — re-run `specd boot --force`.\n")
	return core.ExitGate
}

// runEnrichCheck implements `specd check --enrich`: the enrich-freshness gate.
// It verifies that agent-authored steering enrichment still matches the repo.
func runEnrichCheck(root string, jsonOut bool) int {
	return runEnrichStatus(root, jsonOut)
}
