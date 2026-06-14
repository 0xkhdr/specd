package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunReport(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd report <slug> [--format md|html] [--out <path>]")
	}
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	state := loaded.State
	cfg := core.LoadConfig(root)

	format := args.Str("format")
	if format == "" {
		format = cfg.Report.Format
	}
	if format != "md" && format != "html" {
		return usageExit("--format must be md or html")
	}

	data := core.ReportData{
		State:        state,
		Requirements: core.ReadArtifact(root, slug, "requirements.md"),
		Design:       core.ReadArtifact(root, slug, "design.md"),
		Tasks:        core.ReadArtifact(root, slug, "tasks.md"),
		Decisions:    core.ReadArtifact(root, slug, "decisions.md"),
		Memory:       core.ReadArtifact(root, slug, "memory.md"),
		MidReqs:      core.ReadArtifact(root, slug, "mid-requirements.md"),
	}

	var out string
	if format == "html" {
		out = core.RenderHTML(data, cfg.Report.AutoRefreshSeconds)
	} else {
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
