package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunFusion(args cli.Args) int {
	if len(args.Pos) == 0 {
		return usageExit("usage: specd fusion <bootstrap|policy> [flags]")
	}
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	sub := args.Pos[0]
	switch sub {
	case "bootstrap":
		out, err := core.BuildFusionBootstrap(root, args.Bool("include-schema"))
		if err != nil {
			return specdExit(err)
		}
		if args.Bool("json") || core.IsJSONMode() {
			if err := core.PrintJSON(out); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		fmt.Println("fusion bootstrap: run with --json for machine-readable startup oracle")
		return core.ExitOK
	case "policy":
		slug := ""
		if len(args.Pos) > 1 {
			slug = args.Pos[1]
		}
		out, err := core.BuildFusionPolicy(root, slug, args.Str("expect-config-digest"))
		if err != nil {
			return specdExit(err)
		}
		if args.Bool("json") || core.IsJSONMode() {
			if err := core.PrintJSON(out); err != nil {
				return specdExit(err)
			}
		} else {
			fmt.Printf("fusion policy: subagents=%s orchestration=%v verifySandbox=%s digest=%s\n", out.SubagentMode, out.OrchestrationEnabled, out.VerifySandbox, out.ConfigDigest)
			if out.Spec != nil {
				fmt.Printf("spec %s: mode=%s origin=%s recommended=%s\n", out.Spec.Slug, out.Spec.SpecMode, out.Spec.ModeOrigin, out.Spec.Recommended)
			}
			for _, v := range out.Violations {
				fmt.Println("violation: " + v)
			}
		}
		if len(out.Violations) > 0 {
			return core.ExitGate
		}
		return core.ExitOK
	default:
		return usageExit("usage: specd fusion <bootstrap|policy> [flags]")
	}
}
