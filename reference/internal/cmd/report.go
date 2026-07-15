package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// gitRecentCommits returns recent commit subjects from the local repository for
// the commit↔task link map. It is best-effort and read-only: outside a git repo
// (or on any git error) it returns nil so the PR summary degrades gracefully
// without the commit section. It shells out to local git only — no network.
func gitRecentCommits(root string) []core.Commit {
	out, err := exec.Command("git", "-C", root, "log", "--max-count=50", "--no-color", "--pretty=format:%H\x1f%s").Output()
	if err != nil {
		return nil
	}
	var commits []core.Commit
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 2)
		if len(parts) != 2 {
			continue
		}
		commits = append(commits, core.Commit{SHA: parts[0], Subject: parts[1]})
	}
	return commits
}

// runPRSummary renders a deterministic, network-free PR summary (Markdown, or
// JSON under SPECD_JSON): wave/task progress + gate status, plus the commit↔task
// link map when a local git history is available. It runs the same gate pipeline
// as `specd check` and makes no network calls.
func runPRSummary(root, slug string, state *core.State) int {
	ctx, pre, err := buildCheckCtx(root, slug, false)
	if err != nil {
		return specdExit(err)
	}
	v, warnings := core.RunGates(ctx)
	violations := append([]core.Violation{}, pre...)
	violations = append(violations, v...)

	commits := core.LinkCommits(gitRecentCommits(root))
	summary := core.BuildPRSummary(state, violations, warnings, commits)

	if core.IsJSONMode() {
		if err := core.PrintJSON(summary); err != nil {
			return specdExit(err)
		}
	} else {
		fmt.Print(summary.Markdown())
	}
	if !summary.GatesOK {
		return core.ExitGate
	}
	return core.ExitOK
}

// loadReportData assembles the ReportData a report/serve render consumes: the
// already-loaded state plus the six on-disk artifacts. It is the single source
// of report-data construction shared by `specd report` and `specd serve`, so the
// served view stays byte-identical to the static report.
func loadReportData(root, slug string, state *core.State) core.ReportData {
	return core.ReportData{
		State:        state,
		Requirements: core.ReadArtifact(root, slug, "requirements.md"),
		Design:       core.ReadArtifact(root, slug, "design.md"),
		Tasks:        core.ReadArtifact(root, slug, "tasks.md"),
		Decisions:    core.ReadArtifact(root, slug, "decisions.md"),
		Memory:       core.ReadArtifact(root, slug, "memory.md"),
		MidReqs:      core.ReadArtifact(root, slug, "mid-requirements.md"),
	}
}

// RunReport implements `specd report`. It dispatches to the --history, --diff,
// --serve, --watch, and --pr-summary modes; with none of those flags it loads
// the spec and renders the static report (Markdown, HTML, or Prometheus
// metrics) to stdout or, with --out, to a file.
func RunReport(args cli.Args) int {
	if args.Bool("history") {
		return runReplay(args)
	}
	if args.Bool("diff") {
		return runDiff(args)
	}
	if args.Bool("serve") {
		return runServe(args)
	}
	if args.Bool("watch") {
		watchArgs := args
		watchArgs.Pos = nil
		watchArgs.Flags = cloneFlags(args.Flags)
		if len(args.Pos) > 0 && watchArgs.Str("spec") == "" {
			watchArgs.Flags["spec"] = args.Pos[0]
		}
		return runWatch(watchArgs)
	}
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd report <slug> [--format md|html|prometheus] [--out <path>]")
	if !ok {
		return code
	}
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	state := loaded.State
	cfg := core.LoadConfig(root)

	if args.Bool("pr-summary") {
		return runPRSummary(root, slug, state)
	}
	if args.Bool("conductor") {
		return runConductorReport(root, slug, args.Bool("json"))
	}

	format := args.Str("format")
	if format == "" {
		format = cfg.Report.Format
	}
	if format != "md" && format != "html" && format != "prometheus" {
		return usageExit("--format must be md, html, or prometheus")
	}

	data := loadReportData(root, slug, state)

	var out string
	switch format {
	case "html":
		out = core.RenderHTML(data, cfg.Report.AutoRefreshSeconds)
	case "prometheus":
		out = core.RenderPrometheusMetrics(data)
	default:
		out = core.RenderMarkdown(data)
	}

	outPath := args.Str("out")
	if outPath != "" {
		abs := outPath
		if !filepath.IsAbs(outPath) {
			cwd, _ := os.Getwd()
			abs = filepath.Join(cwd, outPath)
		}
		if err := core.AtomicWrite(abs, out); err != nil {
			return specdExit(err)
		}
		fmt.Printf("report: wrote %s → %s\n", format, outPath)
	} else {
		fmt.Print(out)
	}
	return core.ExitOK
}
