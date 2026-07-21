package cmd

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/core"
)

func runConfig(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("config")
	}
	switch args[0] {
	case "show":
		resolution, err := core.ResolveConfigSource(root)
		if err != nil {
			return err
		}
		return writeJSON(resolution)
	case "validate":
		resolution, err := core.ResolveConfigSource(root)
		if err != nil {
			return err
		}
		if resolution.SelectedPath == "" {
			return fmt.Errorf("no project configuration found")
		}
		for _, warning := range resolution.Deprecations {
			fmt.Fprintln(os.Stderr, "deprecated:", warning)
		}
		return writeJSON(resolution)
	case "migrate":
		var (
			migration core.ConfigMigration
			err       error
		)
		if flagEnabled(flags, "dry-run") {
			migration, err = core.PlanConfigMigration(root, flags["source"])
		} else {
			migration, err = core.MigrateConfig(root, flags["source"])
		}
		if err != nil {
			return err
		}
		return writeJSON(migration)
	default:
		return usageError("config")
	}
}
