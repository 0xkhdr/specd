package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

var steeringFiles = []string{"reasoning.md", "workflow.md", "product.md", "tech.md", "structure.md", "memory.md"}
var roleFiles = []string{"investigator.md", "builder.md", "reviewer.md", "verifier.md"}

func RunInit(args cli.Args) int {
	root, err := os.Getwd()
	if err != nil {
		core.Error(err.Error())
		return core.ExitGate
	}
	force := args.Bool("force")
	var written, skipped, merged []string

	place := func(dest, tmplPath string) {
		if _, err := os.Stat(dest); err == nil && !force {
			skipped = append(skipped, dest)
			return
		}
		content, err := core.ReadTemplate(tmplPath)
		if err != nil {
			core.Error(fmt.Sprintf("missing template %s: %v", tmplPath, err))
			return
		}
		if err := core.AtomicWrite(dest, content); err != nil {
			core.Error(fmt.Sprintf("write %s: %v", dest, err))
			return
		}
		written = append(written, dest)
	}

	for _, f := range steeringFiles {
		place(core.SteeringDir(root)+"/"+f, "steering/"+f)
	}
	for _, f := range roleFiles {
		place(core.RolesDir(root)+"/"+f, "roles/"+f)
	}
	place(core.ConfigPath(root), "config.json")

	// AGENTS.md: merge with markers for idempotent updates
	agentsPath := core.AgentsPath(root)
	content, err := core.ReadTemplate("AGENTS.md")
	if err != nil {
		core.Error(fmt.Sprintf("missing template AGENTS.md: %v", err))
		return core.ExitGate
	}
	if err := core.MergeAgentsMD(agentsPath, content, force); err != nil {
		core.Error(fmt.Sprintf("merge %s: %v", agentsPath, err))
		return core.ExitGate
	}
	merged = append(merged, agentsPath)

	rel := func(p string) string { return strings.TrimPrefix(p, root+"/") }
	if len(written) > 0 {
		core.Info(fmt.Sprintf("specd init: wrote %d file(s):", len(written)))
		for _, w := range written {
			core.Info("  + " + rel(w))
		}
	}
	if len(merged) > 0 {
		core.Info(fmt.Sprintf("merged %d file(s) with markers:", len(merged)))
		for _, m := range merged {
			core.Info("  ↻ " + rel(m))
		}
	}
	if len(skipped) > 0 {
		core.Info(fmt.Sprintf("skipped %d existing file(s) (use --force to overwrite):", len(skipped)))
		for _, s := range skipped {
			core.Info("  · " + rel(s))
		}
	}
	if len(written) == 0 && len(merged) == 0 && len(skipped) == 0 {
		core.Info("specd init: nothing to do")
	}
	return core.ExitOK
}
