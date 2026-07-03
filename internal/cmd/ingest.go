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

const ingestUsage = "usage: specd ingest new <slug> --path <dir> [--include-ignored] [--json]"

// ingestDefaultExcludes are directories skipped by the non-git walk fallback.
var ingestDefaultExcludes = map[string]bool{
	".git": true, ".specd": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, "target": true, ".venv": true, "__pycache__": true,
}

// RunIngest implements `specd ingest` (V10/P5.3): `ingest new <slug> --path
// <dir>` validates the path, writes a deterministic inventory.json (countable
// facts only — the binary never reads legacy semantics), and scaffolds an
// ingestion-flavored spec. The `specd-ingest` skill teaches the agent to
// reverse-engineer requirements/design/tasks; the `ingest` gate then enforces
// that every inventoried file is mapped or waived.
func RunIngest(args cli.Args) int {
	if len(args.Pos) < 1 || args.Pos[0] != "new" {
		return usageExit(ingestUsage)
	}
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	if len(args.Pos) < 2 {
		return usageExit(ingestUsage)
	}
	slug := args.Pos[1]
	if err := core.ValidateSlug(slug); err != nil {
		return specdExit(err)
	}
	if core.SpecExists(root, slug) {
		return specdExit(core.GateError(fmt.Sprintf("spec '%s' already exists", slug)))
	}
	rawPath := strings.TrimSpace(args.Str("path"))
	if rawPath == "" {
		return usageExit(ingestUsage)
	}
	absDir, base, err := resolveIngestPath(root, rawPath)
	if err != nil {
		return specdExit(err)
	}

	relFiles, err := resolveIngestFiles(root, absDir, base, args.Bool("include-ignored"))
	if err != nil {
		return specdExit(err)
	}
	inv, err := core.BuildInventory(root, base, relFiles)
	if err != nil {
		return specdExit(err)
	}

	title := args.Str("title")
	if title == "" {
		title = titleCase(slug)
	}
	if err := scaffoldIngestSpec(root, slug, title, base, inv); err != nil {
		return specdExit(err)
	}

	invBytes, err := core.MarshalInventory(inv)
	if err != nil {
		return specdExit(err)
	}
	if err := core.AtomicWrite(core.InventoryPath(root, slug), string(invBytes)); err != nil {
		return specdExit(err)
	}

	state := core.InitialState(slug, title)
	state.Ingest = &core.IngestRecord{Files: len(inv.Files), Time: core.NowISO()}
	if err := core.SaveState(root, slug, &state); err != nil {
		return specdExit(err)
	}

	if args.Bool("json") {
		return printJSONExit(map[string]interface{}{"ok": true, "spec": slug, "files": len(inv.Files), "modules": inv.Modules})
	}
	fmt.Printf("specd ingest: created ingestion spec '%s' — %d file(s) inventoried\n", slug, len(inv.Files))
	if len(inv.Modules) > 0 {
		fmt.Printf("  modules: %s\n", strings.Join(inv.Modules, ", "))
	}
	fmt.Printf("Next: use the specd-ingest skill to reverse-engineer requirements.md, then `specd check %s` (enable the ingest gate).\n", slug)
	return core.ExitOK
}

// resolveIngestPath validates that rawPath is inside the repo and is a
// directory, returning the absolute directory and its repo-relative base
// (traversal outside the repo is rejected — V10 §5).
func resolveIngestPath(root, rawPath string) (absDir, base string, err error) {
	abs := rawPath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, rawPath)
	}
	abs = filepath.Clean(abs)
	rootClean := filepath.Clean(root)
	rel, rerr := filepath.Rel(rootClean, abs)
	if rerr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", core.GateError(fmt.Sprintf("--path %q escapes the repository", rawPath))
	}
	info, serr := os.Stat(abs)
	if serr != nil || !info.IsDir() {
		return "", "", core.NotFoundError(fmt.Sprintf("--path %q is not a directory", rawPath))
	}
	if rel == "." {
		rel = ""
	}
	return abs, filepath.ToSlash(rel), nil
}

// resolveIngestFiles returns the repo-relative file list under absDir: from
// `git ls-files` when the repo is git-tracked (respecting .gitignore), otherwise
// a bounded walk applying default excludes. --include-ignored forces the walk.
func resolveIngestFiles(root, absDir, base string, includeIgnored bool) ([]string, error) {
	if !includeIgnored {
		if files, ok := gitLsFiles(absDir, base); ok {
			return files, nil
		}
	}
	return walkIngestFiles(root, absDir, base)
}

// gitLsFiles runs `git ls-files` in absDir. Returns (files, true) on success,
// or (nil, false) when the directory is not under git control.
func gitLsFiles(absDir, base string) ([]string, bool) {
	cmd := exec.Command("git", "-C", absDir, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var files []string
	for _, p := range strings.Split(string(out), "\x00") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		files = append(files, joinBase(base, p))
	}
	return files, true
}

// walkIngestFiles walks absDir applying default directory excludes, returning
// repo-relative paths.
func walkIngestFiles(root, absDir, base string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // tolerate unreadable entries
		}
		if d.IsDir() {
			if ingestDefaultExcludes[d.Name()] && path != absDir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

// joinBase prepends the repo-relative base directory to a dir-relative path.
func joinBase(base, rel string) string {
	if base == "" {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(base + "/" + rel)
}

// scaffoldIngestSpec materializes the standard spec artifacts, seeding
// requirements.md with an ingestion preamble that points at the inventory and
// the reverse-engineering workflow.
func scaffoldIngestSpec(root, slug, title, base string, inv core.Inventory) error {
	dir := core.SpecDir(root, slug)
	vars := map[string]string{"TITLE": title, "SLUG": slug, "DATE": core.Clock().UTC().Format("2006-01-02")}
	for _, name := range core.Artifacts {
		tmpl, err := core.ReadTemplate("specStubs/" + name)
		if err != nil {
			return core.GateError(fmt.Sprintf("missing template specStubs/%s: %v", name, err))
		}
		content := core.ApplyVars(tmpl, vars)
		if name == "requirements.md" {
			content = core.InjectPrompt(content, ingestPrompt(base, inv))
		}
		if err := core.AtomicWrite(filepath.Join(dir, name), content); err != nil {
			return err
		}
	}
	return nil
}

// ingestPrompt is the seeded requirements preamble for an ingestion spec.
func ingestPrompt(base string, inv core.Inventory) string {
	dir := base
	if dir == "" {
		dir = "(repo root)"
	}
	modules := "none detected"
	if len(inv.Modules) > 0 {
		modules = strings.Join(inv.Modules, ", ")
	}
	return fmt.Sprintf("Ingest legacy code under %s (%d files inventoried; modules: %s). "+
		"Reverse-engineer requirements from the inventory in inventory.json — every listed file must be "+
		"referenced by ≥1 requirement or waived with a reason (the ingest gate enforces this).", dir, len(inv.Files), modules)
}
