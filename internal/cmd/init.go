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
var skillFiles = []string{
	"specd-foundations", "specd-steering", "specd-requirements",
	"specd-design", "specd-tasks", "specd-execute",
}

// listPacks renders the embedded built-in packs as text, or JSON under
// SPECD_JSON. It performs no filesystem writes.
func listPacks() int {
	packs, err := core.BuiltinPacks()
	if err != nil {
		return specdExit(err)
	}
	if core.IsJSONMode() {
		type packView struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Files       int    `json:"files"`
		}
		views := make([]packView, 0, len(packs))
		for _, p := range packs {
			views = append(views, packView{p.Name, p.Version, p.Description, len(p.Files)})
		}
		if err := core.PrintJSON(views); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	fmt.Printf("specd built-in packs (%d):\n", len(packs))
	for _, p := range packs {
		fmt.Printf("  %-12s v%-7s %s (%d file%s)\n", p.Name, p.Version, p.Description, len(p.Files), plural(len(p.Files)))
	}
	fmt.Println("\nApply with: specd init --pack <name>")
	return core.ExitOK
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// applyPack resolves and transactionally applies a pack into root. A bare name
// resolves to a built-in; an http(s) URL requires --sha256 (fail-closed). It
// writes nothing on any resolve/apply error.
func applyPack(root, ref string, args cli.Args) int {
	pack, err := core.ResolvePack(ref, args.Str("sha256"))
	if err != nil {
		return specdExit(err)
	}
	res, err := core.ApplyPack(root, pack, args.Bool("force"))
	if err != nil {
		return specdExit(err)
	}
	if core.IsJSONMode() {
		if err := core.PrintJSON(struct {
			Pack    string   `json:"pack"`
			Version string   `json:"version"`
			Written []string `json:"written"`
		}{pack.Name, pack.Version, res.Written}); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	core.Info(fmt.Sprintf("specd init --pack %s (v%s): wrote %d file(s):", pack.Name, pack.Version, len(res.Written)))
	for _, w := range res.Written {
		core.Info("  + " + w)
	}
	return core.ExitOK
}

func RunInit(args cli.Args) int {
	if args.Bool("list-packs") {
		return listPacks()
	}
	root, err := os.Getwd()
	if err != nil {
		core.Error(err.Error())
		return core.ExitGate
	}
	if ref := args.Str("pack"); ref != "" {
		return applyPack(root, ref, args)
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
	for _, s := range skillFiles {
		place(core.SkillsDir(root)+"/"+s+"/SKILL.md", "skills/"+s+"/SKILL.md")
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
