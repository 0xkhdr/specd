package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
)

// mcpHostConfig holds the destination path hint and ready-to-paste config for
// a supported MCP host. Keep content in sync with docs/mcp-hosts/.
type mcpHostConfig struct {
	dest    string
	content string
}

var mcpHostConfigs = map[string]mcpHostConfig{
	"claude-desktop": {
		dest: "~/.config/Claude/claude_desktop_config.json  (macOS/Linux)\n# %APPDATA%\\Claude\\claude_desktop_config.json  (Windows)",
		content: `{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"]
    }
  }
}`,
	},
	"cursor": {
		dest: ".cursor/mcp.json  (project)  or  global MCP settings",
		content: `{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "${workspaceFolder}"]
    }
  }
}`,
	},
	"vscode": {
		dest: ".vscode/settings.json  (mcp.servers key)",
		content: `{
  "mcp": {
    "servers": {
      "specd": {
        "type": "stdio",
        "command": "specd",
        "args": ["mcp", "--root", "${workspaceFolder}"]
      }
    }
  }
}`,
	},
	"antigravity": {
		dest: ".agents/mcp_config.json  (workspace-local)\n# ~/.gemini/antigravity-cli/mcp_config.json  (per-CLI global)",
		content: `{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"],
      "env": {
        "MCP_MODE": "stdio",
        "DISABLE_CONSOLE_OUTPUT": "true"
      }
    }
  }
}`,
	},
	"codex": {
		dest: "~/.codex/config.toml  (global)  or  .codex/config.toml  (project)",
		content: `[mcp_servers.specd]
command = "specd"
args = ["mcp", "--root", "/path/to/your/project"]`,
	},
}

// RunMCP starts the native MCP stdio server. It is a thin JSON-RPC 2.0 transport
// over the existing command handlers — no new business logic, no network, no LLM
// calls. Tool calls re-dispatch through the same cmd.Dispatch the CLI uses, so
// every read-safe and state-mutating command is reachable by an MCP host.
//
// It is handled like help/version (in main.run, not via the dispatch Registry):
// the server is not itself a spec-scoped command and must not be exposed as a
// tool.
func RunMCP(args cli.Args) int {
	// --config <host> prints a ready-to-paste config snippet and exits.
	if host := args.Str("config"); host != "" {
		return printMCPHostConfig(host, args.Str("root"))
	}

	// R7: --root scopes every tool call to that project. Handlers resolve their
	// root from the working directory, so honour --root by chdir-ing once before
	// the loop; without it, root is located as the CLI does today.
	if root := args.Str("root"); root != "" && root != "true" {
		if err := os.Chdir(root); err != nil {
			core.Error("mcp: cannot use --root " + root + ": " + err.Error())
			return core.ExitUsage
		}
	}
	// --http <addr> opts into the HTTP/SSE transport adapter for hosts that
	// cannot speak stdio. Absent, the stdio path below is byte-identical to
	// today (R4.3). A bare --http (no value) defaults to loopback:8765.
	if addr := args.Str("http"); addr != "" {
		if addr == "true" {
			addr = ""
		}
		if err := mcp.ServeHTTP(addr, Dispatch); err != nil {
			core.Error("mcp: " + err.Error())
			return core.ExitGate
		}
		return core.ExitOK
	}
	if err := mcp.Serve(os.Stdin, os.Stdout, Dispatch); err != nil {
		core.Error("mcp: " + err.Error())
		return core.ExitGate
	}
	return core.ExitOK
}

func printMCPHostConfig(host, root string) int {
	cfg, ok := mcpHostConfigs[host]
	if !ok {
		names := make([]string, 0, len(mcpHostConfigs))
		for k := range mcpHostConfigs {
			names = append(names, k)
		}
		sort.Strings(names)
		core.Error("mcp: unknown host \"" + host + "\". Available: " + strings.Join(names, ", "))
		return core.ExitUsage
	}
	content := cfg.content
	if root != "" && root != "true" {
		content = strings.ReplaceAll(content, "/path/to/your/project", root)
	}
	fmt.Printf("# Paste into: %s\n\n", cfg.dest)
	fmt.Println(content)
	return core.ExitOK
}
