package cmd

import (
	"os"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
)

// RunMCP starts the native MCP stdio server. It is a thin JSON-RPC 2.0 transport
// over the existing command handlers — no new business logic, no network, no LLM
// calls. Tool calls re-dispatch through the same cmd.Dispatch the CLI uses, so
// every read-safe and state-mutating command is reachable by an MCP host.
//
// It is handled like help/version (in main.run, not via the dispatch Registry):
// the server is not itself a spec-scoped command and must not be exposed as a
// tool.
func RunMCP(args cli.Args) int {
	// R7: --root scopes every tool call to that project. Handlers resolve their
	// root from the working directory, so honour --root by chdir-ing once before
	// the loop; without it, root is located as the CLI does today.
	if root := args.Str("root"); root != "" && root != "true" {
		if err := os.Chdir(root); err != nil {
			core.Error("mcp: cannot use --root " + root + ": " + err.Error())
			return core.ExitUsage
		}
	}
	if err := mcp.Serve(os.Stdin, os.Stdout, Dispatch); err != nil {
		core.Error("mcp: " + err.Error())
		return core.ExitGate
	}
	return core.ExitOK
}
