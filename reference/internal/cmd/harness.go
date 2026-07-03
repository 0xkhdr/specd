package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

const harnessUsage = "usage: specd harness push <git-url> [--name <n>] [--json]  |  specd harness pull <git-url> [--force] [--json]  |  specd harness list [--json]  |  specd harness enable <path> [--force] [--json]"

// RunHarness implements `specd harness` (V11/P6.1): sharing the configured
// policy (guardrails, deploy templates, roles, routing) as a versioned team asset
// over stdlib-exec git. `push` bundles the current policy and pushes it; `pull`
// imports a bundle with SHA256 pinning, refuses to clobber local modifications
// without --force, and quarantines every imported executable `command` artifact
// until it is explicitly enabled. `list` shows the bundle and quarantine; `enable`
// installs one quarantined artifact and records the decision.
func RunHarness(args cli.Args) int {
	if len(args.Pos) == 0 {
		return usageExit(harnessUsage)
	}
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	jsonOut := args.Bool("json")
	switch args.Pos[0] {
	case "push":
		return runHarnessPush(root, args, jsonOut)
	case "pull":
		return runHarnessPull(root, args, jsonOut)
	case "list":
		return runHarnessList(root, jsonOut)
	case "enable":
		return runHarnessEnable(root, args, jsonOut)
	default:
		return usageExit(harnessUsage)
	}
}

func runHarnessPush(root string, args cli.Args, jsonOut bool) int {
	if len(args.Pos) < 2 {
		return usageExit(harnessUsage)
	}
	m, err := core.PushHarness(root, args.Pos[1], args.Str("name"))
	if err != nil {
		return specdExit(err)
	}
	if jsonOut {
		return printJSONExit(map[string]interface{}{"ok": true, "action": "push", "name": m.Name, "version": m.Version, "files": len(m.Files)})
	}
	fmt.Printf("✓ harness push: %q v%d — %d artifact(s) shared\n", m.Name, m.Version, len(m.Files))
	return core.ExitOK
}

func runHarnessPull(root string, args cli.Args, jsonOut bool) int {
	if len(args.Pos) < 2 {
		return usageExit(harnessUsage)
	}
	res, err := core.PullHarness(root, args.Pos[1], args.Bool("force"))
	if err != nil {
		return specdExit(err)
	}
	if jsonOut {
		return printJSONExit(map[string]interface{}{
			"ok":          len(res.Refused) == 0,
			"action":      "pull",
			"name":        res.Manifest.Name,
			"version":     res.Manifest.Version,
			"installed":   res.Installed,
			"quarantined": res.Quarantined,
			"refused":     res.Refused,
		})
	}
	fmt.Printf("harness pull: %q v%d\n", res.Manifest.Name, res.Manifest.Version)
	fmt.Printf("  installed:   %d\n", len(res.Installed))
	if len(res.Quarantined) > 0 {
		fmt.Printf("  quarantined: %d (executable — enable explicitly):\n", len(res.Quarantined))
		for _, q := range res.Quarantined {
			fmt.Printf("    ⚠ %s → `specd harness enable %s`\n", q, q)
		}
	}
	if len(res.Refused) > 0 {
		errLine("  refused (locally modified, use --force to overwrite):")
		for _, r := range res.Refused {
			errLine("    ✗ %s", r)
		}
		return core.ExitGate
	}
	return core.ExitOK
}

func runHarnessList(root string, jsonOut bool) int {
	m, err := core.LoadHarnessManifest(root)
	if err != nil {
		return specdExit(err)
	}
	quarantined := core.HarnessQuarantined(root)
	if jsonOut {
		return printJSONExit(map[string]interface{}{
			"ok": true, "name": m.Name, "version": m.Version, "provenance": m.Provenance,
			"files": m.Files, "quarantined": quarantined,
		})
	}
	fmt.Printf("harness %q v%d (from %s)\n", m.Name, m.Version, m.Provenance)
	for _, f := range m.Files {
		mark := " "
		if f.Executable {
			mark = "⚠"
		}
		fmt.Printf("  %s %s\n", mark, f.Path)
	}
	if len(quarantined) > 0 {
		fmt.Printf("quarantined (awaiting enable):\n")
		for _, q := range quarantined {
			fmt.Printf("  ⚠ %s\n", q)
		}
	}
	return core.ExitOK
}

func runHarnessEnable(root string, args cli.Args, jsonOut bool) int {
	if len(args.Pos) < 2 {
		return usageExit(harnessUsage)
	}
	path := args.Pos[1]
	if err := core.EnableHarnessItem(root, path, args.Bool("force")); err != nil {
		return specdExit(err)
	}
	if jsonOut {
		return printJSONExit(map[string]interface{}{"ok": true, "action": "enable", "path": path})
	}
	fmt.Printf("✓ harness enable: installed %s (recorded in harness decision log)\n", path)
	return core.ExitOK
}
