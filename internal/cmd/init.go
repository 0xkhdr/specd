package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xkhdr/specd/internal/core"
)

func runInit(root string, args []string, flags map[string]string) error {
	if len(args) != 0 {
		return errors.New("usage: specd init [--agent=<name>] [--repair|--refresh] [--dry-run]")
	}
	repair := flagEnabled(flags, "repair")
	refresh := flagEnabled(flags, "refresh")
	dryRun := flagEnabled(flags, "dry-run")

	// Plain init: scaffold missing assets (idempotent — existing files preserved).
	if !repair && !refresh {
		if dryRun {
			// A dry-run init still reports what a repair would change on top of the
			// scaffold, so a fresh run previews the managed regions it would write.
			return previewManaged(root)
		}
		return core.WriteScaffold(root, flags["agent"])
	}

	// Repair/refresh re-sync every managed region from the current templates,
	// leaving content outside the markers untouched (R3/R4).
	if dryRun {
		return previewManaged(root)
	}
	changes, err := core.ApplyManagedRepair(root)
	if err != nil {
		return err
	}
	specsRootCreated, err := core.EnsureSpecsRoot(root)
	if err != nil {
		return err
	}
	verb := "repaired"
	if refresh {
		verb = "refreshed"
	}
	if len(changes) == 0 && !specsRootCreated {
		fmt.Fprintln(os.Stdout, "all managed regions already in sync")
		return nil
	}
	if specsRootCreated {
		fmt.Fprintf(os.Stdout, "%s .specd/specs/.gitkeep\n", verb)
	}
	for _, change := range changes {
		fmt.Fprintf(os.Stdout, "%s %s\n", verb, change.RelPath)
	}
	return nil
}

// previewManaged prints the unified-diff-style preview of every managed-region
// change and writes nothing (spec 11 R5).
func previewManaged(root string) error {
	changes, err := core.PlanManagedRepair(root)
	if err != nil {
		return err
	}
	configExists := false
	for _, rel := range []string{filepath.Join(".specd", "config.yaml"), "project.yml", "project.yaml"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			configExists = true
			break
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if !configExists {
		fmt.Fprintln(os.Stdout, "+ .specd/config.yaml (new operator config)")
	}
	if _, err := os.Stat(filepath.Join(root, ".specd", "specs", ".gitkeep")); os.IsNotExist(err) {
		fmt.Fprintln(os.Stdout, "+ .specd/specs/.gitkeep (required layout)")
	} else if err != nil {
		return err
	}
	if len(changes) == 0 {
		fmt.Fprintln(os.Stdout, "no managed-region changes")
		return nil
	}
	for _, change := range changes {
		fmt.Fprint(os.Stdout, core.Unifiedish(change))
	}
	return nil
}
