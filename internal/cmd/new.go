package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func titleCase(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// RunNew implements `specd new`: it validates the slug, refuses to overwrite
// an existing spec, materializes the six spec artifacts from templates
// (seeding requirements.md with --from, if given), writes the initial
// state.json (optionally starting in orchestrated mode with --orchestrated),
// and prints the next step.
func RunNew(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd new <slug> [--title \"...\"]")
	if !ok {
		return code
	}
	if err := core.ValidateSlug(slug); err != nil {
		return specdExit(err)
	}
	if core.SpecExists(root, slug) {
		core.Error(fmt.Sprintf("spec '%s' already exists", slug))
		return core.ExitGate
	}
	title := args.Str("title")
	if title == "" {
		title = titleCase(slug)
	}
	date := core.Clock().UTC().Format("2006-01-02")
	vars := map[string]string{"TITLE": title, "SLUG": slug, "DATE": date}
	prompt := strings.TrimSpace(args.Str("from"))

	dir := core.SpecDir(root, slug)
	for _, name := range core.Artifacts {
		tmplPath := "specStubs/" + name
		tmpl, err := core.ReadTemplate(tmplPath)
		if err != nil {
			core.Error(fmt.Sprintf("missing template %s: %v", tmplPath, err))
			return core.ExitGate
		}
		content := core.ApplyVars(tmpl, vars)
		if name == "requirements.md" {
			content = core.InjectPrompt(content, prompt)
		}
		if err := core.AtomicWrite(filepath.Join(dir, name), content); err != nil {
			core.Error(fmt.Sprintf("write %s: %v", name, err))
			return core.ExitGate
		}
	}
	state := core.InitialState(slug, title)
	state.Prompt = prompt
	if args.Bool("orchestrated") {
		if !core.ProjectOrchestrationEnabled(root) {
			core.Error("--orchestrated: project has no orchestration capability. Enable it with `specd init --orchestration session` (or manual|planning), then retry.")
			return core.ExitGate
		}
		state.ExecutionMode = core.ModeOrchestrated
		state.ModeOrigin = core.OriginUser
	}
	if err := core.SaveState(root, slug, &state); err != nil {
		return specdExit(err)
	}
	fmt.Printf("specd new: created spec '%s' (%s)\n", slug, title)
	fmt.Printf("  .specd/specs/%s/ — six artifacts + state.json (status: requirements)\n", slug)
	fmt.Printf("Next: write requirements.md (EARS), then `specd check %s`.\n", slug)
	return core.ExitOK
}
