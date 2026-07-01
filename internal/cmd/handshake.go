package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunHandshake implements `specd handshake`, dispatching to the "bootstrap" subcommand
// (a machine-readable startup oracle for host agents) and the "policy"
// subcommand (effective subagent/orchestration/sandbox policy plus any
// violations, optionally scoped to a spec).
func RunHandshake(args cli.Args) int {
	if len(args.Pos) == 0 {
		return usageExit("usage: specd handshake <bootstrap|policy> [flags]")
	}
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	sub := args.Pos[0]
	switch sub {
	case "bootstrap":
		out, err := core.BuildHandshakeBootstrap(root, args.Bool("include-schema"))
		if err != nil {
			return specdExit(err)
		}
		if args.Bool("json") || core.IsJSONMode() {
			if err := core.PrintJSON(out); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		fmt.Println("handshake bootstrap: run with --json for machine-readable startup oracle")
		return core.ExitOK
	case "policy":
		slug := ""
		if len(args.Pos) > 1 {
			slug = args.Pos[1]
		}
		out, err := core.BuildHandshakePolicy(root, slug, args.Str("expect-config-digest"))
		if err != nil {
			return specdExit(err)
		}
		if args.Bool("json") || core.IsJSONMode() {
			if err := core.PrintJSON(out); err != nil {
				return specdExit(err)
			}
		} else {
			fmt.Printf("handshake policy: subagents=%s orchestration=%v verifySandbox=%s digest=%s\n", out.SubagentMode, out.OrchestrationEnabled, out.VerifySandbox, out.ConfigDigest)
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
		return usageExit("usage: specd handshake <bootstrap|policy> [flags]")
	}
}
