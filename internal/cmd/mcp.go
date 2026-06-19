package cmd

import (
	"fmt"
	"os"
	"strings"

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
	// --config <host> prints a ready-to-paste config snippet and exits without
	// starting the server. Root path substitution is applied when --root is also
	// given, so the snippet is immediately usable without manual editing.
	if host := args.Str("config"); host != "" {
		return printMCPHostConfig(host, args.Str("root"))
	}

	// --root scopes every tool call to that project. Handlers resolve their root
	// from the working directory, so honour --root by chdir-ing once before the
	// loop; without it, root is located as the CLI does today.
	if root := args.Str("root"); root != "" && root != "true" {
		if err := os.Chdir(root); err != nil {
			core.Error("mcp: cannot use --root " + root + ": " + err.Error())
			return core.ExitUsage
		}
	}

	// Load the project config once (cwd is already the project root after any
	// --root chdir above) so tools/list can filter the advertised tool set. A
	// missing config yields DefaultConfig, whose zero-value mcp block means
	// "expose everything" — byte-identical to the pre-config surface.
	cfg := core.LoadConfig(".")

	// --http <addr> opts into the HTTP/SSE transport adapter for hosts that
	// cannot speak stdio. Absent, the stdio path below is used. A bare --http
	// (no value) defaults to loopback:8765.
	if addr := args.Str("http"); addr != "" {
		if addr == "true" {
			addr = ""
		}
		if err := mcp.ServeHTTP(addr, Dispatch, &cfg); err != nil {
			core.Error("mcp: " + err.Error())
			return core.ExitGate
		}
		return core.ExitOK
	}

	if err := mcp.Serve(os.Stdin, os.Stdout, Dispatch, &cfg); err != nil {
		core.Error("mcp: " + err.Error())
		return core.ExitGate
	}
	return core.ExitOK
}

func printMCPHostConfig(host, root string) int {
	if root == "true" {
		root = ""
	}
	dest, content, ok := mcp.HostConfig(host, root)
	if !ok {
		core.Error("mcp: unknown host \"" + host + "\". Available: " + strings.Join(mcp.HostNames(), ", "))
		return core.ExitUsage
	}
	fmt.Printf("# Paste into: %s\n\n", dest)
	fmt.Println(content)
	return core.ExitOK
}
