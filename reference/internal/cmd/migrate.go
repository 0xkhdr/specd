package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

const migrateUsage = "usage: specd migrate [--json]"

// RunMigrate implements `specd migrate` (V12/P6.4): the documented, idempotent
// one-shot that moves a v0.1.x project onto v0.2.0. It rewrites every spec's
// state at the current schema version (the v5→v6 migration is otherwise silent
// on first load) and reports which additive policy blocks are available to adopt.
// It never writes policy content, so a migrated repo keeps the new gates
// default-off. Running it twice is a no-op.
func RunMigrate(args cli.Args) int {
	if len(args.Pos) > 0 {
		return usageExit(migrateUsage)
	}
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	rep, err := core.MigrateProject(root)
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		return printJSONExit(map[string]interface{}{
			"ok":            true,
			"schemaVersion": rep.SchemaVersion,
			"specs":         rep.Specs,
			"hints":         rep.Hints,
		})
	}
	migrated := 0
	for _, s := range rep.Specs {
		if s.Migrated {
			migrated++
		}
	}
	fmt.Printf("specd migrate: schema v%d — %d spec(s), %d migrated\n", rep.SchemaVersion, len(rep.Specs), migrated)
	for _, s := range rep.Specs {
		if s.Migrated {
			fmt.Printf("  ✓ %s: v%d → v%d\n", s.Slug, s.FromVersion, s.ToVersion)
		} else {
			fmt.Printf("  · %s: already v%d\n", s.Slug, s.ToVersion)
		}
	}
	fmt.Println("available v0.2.0 config blocks (not applied — adopt explicitly):")
	for _, h := range rep.Hints {
		mark := "○"
		if h.Present {
			mark = "●"
		}
		fmt.Printf("  %s %-12s %s\n", mark, h.Name, h.Adopt)
	}
	return core.ExitOK
}
