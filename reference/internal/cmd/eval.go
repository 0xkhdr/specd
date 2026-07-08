package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

const evalUsage = "usage: specd eval <slug> [init|trend] [--suite <name>] [--json]"

// RunEval dispatches the eval command. Subcommands `init` and `trend` are
// selected by the second positional; the bare form scores the spec against its
// rubric. Scoring is deterministic (invariant 6/7): the same rubric and
// artifacts always yield the same score; only the recorded timestamp varies.
func RunEval(args cli.Args) int {
	if len(args.Pos) >= 2 {
		switch args.Pos[1] {
		case "init":
			return runEvalInit(args)
		case "trend":
			return runEvalTrend(args)
		}
	}
	root, slug, code, ok := requireRootAndSlug(args, evalUsage)
	if !ok {
		return code
	}
	return runEvalScore(root, slug, args)
}

func runEvalScore(root, slug string, args cli.Args) int {
	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		state, err := core.LoadState(root, slug)
		if err != nil {
			return core.ExitGate, err
		}
		path := core.DefaultEvalRubricPath(root, slug)
		if args.Str("suite") != "" {
			path = core.ArtifactPath(root, slug, "eval-"+args.Str("suite")+".json")
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Distinct "no suite" status so migrated repos without a rubric can
			// tell the difference between a failing eval and an absent one.
			return core.ExitNotFound, core.GateError(fmt.Sprintf("no eval rubric at %s (run `specd eval %s init`)", path, slug))
		}
		rubric, digest, err := core.LoadEvalRubric(path)
		if err != nil {
			return core.ExitGate, err
		}
		report, err := core.RunEval(root, slug, rubric, digest)
		if err != nil {
			return core.ExitGate, err
		}
		saved, err := core.SaveEvalReport(root, slug, report)
		if err != nil {
			return core.ExitGate, err
		}
		seq := seqFromReportPath(saved)
		if state.Evals == nil {
			state.Evals = map[string]core.EvalSummary{}
		}
		state.Evals[report.Suite] = core.EvalSummary{
			Suite:    report.Suite,
			Score:    report.Score,
			MinScore: report.MinScore,
			Pass:     report.Passed,
			Seq:      seq,
			Time:     report.GeneratedAt,
		}
		if err := core.SaveState(root, slug, state); err != nil {
			return core.ExitGate, err
		}
		if args.Bool("json") {
			if err := core.PrintJSON(report); err != nil {
				return core.ExitGate, err
			}
		} else {
			printEvalReport(report)
		}
		if !report.Passed {
			return core.ExitGate, nil
		}
		return core.ExitOK, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}

// runEvalInit compiles the approved requirements into a rubric skeleton — one
// stub check per acceptance criterion ID. The transform is deterministic and
// interpretation-free: the criterion count and IDs fully determine the output.
// The agent refines patterns/kinds afterwards (the `specd-eval-author` skill).
func runEvalInit(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	if len(args.Pos) < 1 {
		return usageExit(evalUsage)
	}
	slug := args.Pos[0]
	reqPath := core.ArtifactPath(root, slug, "requirements.md")
	data, err := os.ReadFile(reqPath)
	if err != nil {
		return specdExit(core.GateError(fmt.Sprintf("cannot read requirements: %v", err)))
	}
	rubric := core.EvalRubricSkeleton(string(data))
	if len(rubric.Checks) == 0 {
		return specdExit(core.GateError("no acceptance criteria found in requirements.md"))
	}
	out := core.DefaultEvalRubricPath(root, slug)
	if _, err := os.Stat(out); err == nil && !args.Bool("force") {
		return specdExit(core.GateError(fmt.Sprintf("%s already exists (pass --force to overwrite)", out)))
	}
	body, err := core.MarshalEvalRubric(rubric)
	if err != nil {
		return specdExit(err)
	}
	if err := core.AtomicWrite(out, body); err != nil {
		return specdExit(err)
	}
	fmt.Printf("wrote %d stub checks to %s\n", len(rubric.Checks), out)
	return core.ExitOK
}

// runEvalTrend reports score deltas and failure clustering over the recorded
// result-file history. Clustering is by exact check ID (a deterministic key) —
// no interpretation of failure prose.
func runEvalTrend(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	if len(args.Pos) < 1 {
		return usageExit(evalUsage)
	}
	slug := args.Pos[0]
	trend, err := core.EvalTrend(root, slug, args.Str("suite"))
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		if err := core.PrintJSON(trend); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	if len(trend.Runs) == 0 {
		fmt.Println("no eval runs recorded")
		return core.ExitOK
	}
	for _, r := range trend.Runs {
		fmt.Printf("%s #%d score %.3f delta %+.3f pass %v\n", r.Suite, r.Seq, r.Score, r.Delta, r.Passed)
	}
	for _, c := range trend.Clusters {
		fmt.Printf("failures %s x%d\n", c.CheckID, c.Count)
	}
	return core.ExitOK
}

func printEvalReport(report *core.EvalReport) {
	fmt.Printf("suite %s score %.3f min %.3f pass %v\n", report.Suite, report.Score, report.MinScore, report.Passed)
	for _, c := range report.Checks {
		status := "pass"
		if !c.Passed {
			status = "FAIL"
		}
		fmt.Printf("  %-4s %s %s\n", status, c.ID, c.Message)
	}
}

func seqFromReportPath(path string) int {
	base := strings.TrimSuffix(filepath.Base(path), ".json")
	if i := strings.LastIndex(base, "-"); i >= 0 {
		n := 0
		for _, r := range base[i+1:] {
			if r < '0' || r > '9' {
				return 0
			}
			n = n*10 + int(r-'0')
		}
		return n
	}
	return 0
}

// RunPromote converts a prototype spec to a full spec after a passing eval. The
// evidence string is mandatory — micro-approval never bypasses the evidence
// discipline (invariant 5).
func RunPromote(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd promote <slug> --evidence <text> [--suite <name>]")
	if !ok {
		return code
	}
	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		state, err := core.LoadState(root, slug)
		if err != nil {
			return core.ExitGate, err
		}
		path := core.DefaultEvalRubricPath(root, slug)
		if args.Str("suite") != "" {
			path = core.ArtifactPath(root, slug, "eval-"+args.Str("suite")+".json")
		}
		rubric, digest, err := core.LoadEvalRubric(path)
		if err != nil {
			return core.ExitGate, err
		}
		report, err := core.RunEval(root, slug, rubric, digest)
		if err != nil {
			return core.ExitGate, err
		}
		if _, err := core.SaveEvalReport(root, slug, report); err != nil {
			return core.ExitGate, err
		}
		if err := core.MarkPrototypePromoted(state, report, args.Str("evidence")); err != nil {
			return core.ExitGate, err
		}
		if err := core.SaveState(root, slug, state); err != nil {
			return core.ExitGate, err
		}
		if args.Bool("json") {
			return core.ExitOK, core.PrintJSON(state.Prototype)
		}
		fmt.Printf("promoted %s (score %.3f)\n", slug, report.Score)
		return core.ExitOK, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}
