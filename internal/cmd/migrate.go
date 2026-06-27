package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunMigrate(args cli.Args) int {
	if len(args.Pos) != 1 || args.Pos[0] != "config" {
		return usageExit("usage: specd migrate config [--dry-run] [--global]")
	}
	for k := range args.Flags {
		if k != "dry-run" && k != "global" && k != "json" {
			return usageExit("unknown migrate flag --" + k)
		}
	}
	var source, target string
	if args.Bool("global") {
		paths := core.GlobalConfigPaths()
		if len(paths) == 0 {
			return specdExit(core.NotFoundError("no global config directory found"))
		}
		target = paths[0]
		for _, p := range paths {
			if filepath.Ext(p) == ".json" {
				if _, err := os.Stat(p); err == nil {
					source = p
					break
				}
			}
		}
		if source == "" {
			return specdExit(core.NotFoundError("no legacy global config.json found"))
		}
	} else {
		root, err := core.RequireSpecdRoot()
		if err != nil {
			return specdExit(err)
		}
		source = core.LegacyConfigPath(root)
		target = filepath.Join(root, ".specd", "config.yml")
	}
	if args.Bool("dry-run") {
		yml, err := core.MigrateConfigPreview(source)
		if err != nil {
			return specdExit(err)
		}
		fmt.Print(yml)
		return core.ExitOK
	}
	if err := core.MigrateConfigFile(source, target); err != nil {
		return specdExit(err)
	}
	if core.IsJSONMode() || args.Bool("json") {
		_ = core.PrintJSON(struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Backup string `json:"backup"`
		}{source, target, source + ".bak"})
	} else {
		core.Info("migrated config: " + source + " -> " + target + " (backup: " + source + ".bak)")
	}
	return core.ExitOK
}
