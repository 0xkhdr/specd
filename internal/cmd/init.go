package cmd

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

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
	return runInit(args, core.DefaultInitExecutor())
}

func runInit(args cli.Args, executor core.InitExecutor) int {
	if args.Bool("list-packs") {
		return listPacks()
	}
	root, err := os.Getwd()
	if err != nil {
		core.Error(err.Error())
		return core.ExitGate
	}
	if ref := args.Str("pack"); ref != "" {
		if args.Bool("repair") || args.Bool("refresh") || args.Bool("dry-run") {
			return usageExit("--pack cannot be combined with --repair, --refresh, or --dry-run")
		}
		return applyPack(root, ref, args)
	}
	options := core.InitOptions{
		Root:    root,
		Force:   args.Bool("force"),
		Repair:  args.Bool("repair"),
		Refresh: args.Bool("refresh"),
		DryRun:  args.Bool("dry-run"),
		Scope:   "project",
	}
	if err := core.ValidateInitOptions(options); err != nil {
		return usageExit(err.Error())
	}
	plan, err := core.PlanInit(options, core.DefaultScaffoldManifest(), core.ReadTemplate)
	if err != nil {
		result := core.NewInitResult(root)
		result.Status = "failed"
		result.Warnings = append(result.Warnings, core.InitWarning{
			Code:    "preflight-failed",
			Message: err.Error(),
		})
		result.Normalize()
		return emitInitResult(result, args.Bool("json"))
	}
	result := core.ExecuteInitPlan(plan, options.Force, executor)
	return emitInitResult(result, args.Bool("json"))
}

func emitInitResult(result core.InitResult, jsonOut bool) int {
	result.Normalize()
	if jsonOut || core.IsJSONMode() {
		if err := core.PrintJSON(result); err != nil {
			return specdExit(err)
		}
	} else {
		ready := len(result.Files.Written) + len(result.Files.Updated) + len(result.Files.Skipped)
		if result.Status == "planned" {
			core.Info(fmt.Sprintf("specd init %s dry run in %s", result.Mode, result.Root))
			for _, path := range result.Files.Written {
				core.Info("would write: " + path)
			}
			for _, path := range result.Files.Updated {
				core.Info("would update: " + path)
			}
			for _, path := range result.Files.Skipped {
				core.Info("would preserve: " + path)
			}
		} else if result.Status == "ready" {
			core.Info(fmt.Sprintf("Initialized specd in %s", result.Root))
			core.Info(fmt.Sprintf("Project assets: %d ready, 0 failed", ready))
			core.Info("Next: " + result.NextAction.Text)
			if len(result.Files.Written) > 0 {
				core.Info(fmt.Sprintf("wrote %d file(s)", len(result.Files.Written)))
			}
			if len(result.Files.Updated) > 0 {
				core.Info(fmt.Sprintf("updated %d managed file(s)", len(result.Files.Updated)))
			}
			if len(result.Files.Skipped) > 0 {
				core.Info(fmt.Sprintf("skipped %d existing file(s)", len(result.Files.Skipped)))
			}
		} else {
			core.Error(fmt.Sprintf("specd init failed in %s", result.Root))
			core.Error(fmt.Sprintf("Project assets: %d ready, %d failed", ready, len(result.Files.Failed)))
			for _, path := range result.Files.Failed {
				core.Error("failed: " + path)
			}
			for _, warning := range result.Warnings {
				core.Error(warning.Message)
			}
		}
	}
	if result.Status != "ready" && result.Status != "planned" {
		return core.ExitGate
	}
	return core.ExitOK
}
