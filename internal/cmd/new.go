package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func titleCase(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func RunNew(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd new <slug> [--title \"...\"]")
	}
	if !slugRE.MatchString(slug) {
		return usageExit(fmt.Sprintf("invalid slug '%s' (must match ^[a-z0-9][a-z0-9-]*$)", slug))
	}
	if core.SpecExists(root, slug) {
		core.Error(fmt.Sprintf("spec '%s' already exists", slug))
		return core.ExitGate
	}
	title := args.Str("title")
	if title == "" {
		title = titleCase(slug)
	}
	date := time.Now().UTC().Format("2006-01-02")
	vars := map[string]string{"TITLE": title, "SLUG": slug, "DATE": date}

	dir := core.SpecDir(root, slug)
	for _, name := range core.Artifacts {
		tmplPath := "specStubs/" + name
		tmpl, err := core.ReadTemplate(tmplPath)
		if err != nil {
			core.Error(fmt.Sprintf("missing template %s: %v", tmplPath, err))
			return core.ExitGate
		}
		if err := core.AtomicWrite(filepath.Join(dir, name), core.ApplyVars(tmpl, vars)); err != nil {
			core.Error(fmt.Sprintf("write %s: %v", name, err))
			return core.ExitGate
		}
	}
	state := core.InitialState(slug, title)
	if err := core.SaveState(root, slug, &state); err != nil {
		return specdExit(err)
	}
	fmt.Printf("specd new: created spec '%s' (%s)\n", slug, title)
	fmt.Printf("  .specd/specs/%s/ — six artifacts + state.json (status: requirements)\n", slug)
	fmt.Printf("Next: write requirements.md (EARS), then `specd check %s`.\n", slug)
	return core.ExitOK
}
