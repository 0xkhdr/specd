package mcp

import (
	"context"
	"embed"
	"strings"

	"github.com/0xkhdr/specd/internal/integration"
)

//go:embed embed_hosts
var hostsFS embed.FS

// HostEntry describes a supported MCP host.
type HostEntry struct {
	// Dest is a human-readable hint for where to paste the config snippet.
	Dest string
	file string
}

// hostRegistry maps the CLI host name to its metadata and embedded file path.
var hostRegistry = map[string]HostEntry{
	"claude-desktop": {
		Dest: "~/.config/Claude/claude_desktop_config.json  (macOS/Linux)\n# %APPDATA%\\Claude\\claude_desktop_config.json  (Windows)",
		file: "embed_hosts/claude-desktop.json",
	},
	"cursor": {
		Dest: ".cursor/mcp.json  (project)  or  global MCP settings",
		file: "embed_hosts/cursor.json",
	},
	"vscode": {
		Dest: ".vscode/settings.json  (mcp.servers key)",
		file: "embed_hosts/vscode.json",
	},
	"antigravity": {
		Dest: ".agents/mcp_config.json  (workspace-local)\n# ~/.gemini/antigravity-cli/mcp_config.json  (per-CLI global)",
		file: "embed_hosts/antigravity.json",
	},
	"codex": {
		Dest: "~/.codex/config.toml  (global)  or  .codex/config.toml  (project)",
		file: "embed_hosts/codex.toml",
	},
}

type legacyHostAdapter struct {
	name  string
	entry HostEntry
}

func (a legacyHostAdapter) Name() string { return a.name }

func (a legacyHostAdapter) Scopes() []integration.Scope {
	return []integration.Scope{integration.ScopeProject, integration.ScopeGlobal}
}

func (a legacyHostAdapter) Detect(string) integration.Detection {
	return integration.Detection{
		Host:       a.name,
		Scopes:     a.Scopes(),
		Method:     "manual-snippet",
		Confidence: integration.ConfidenceNone,
		Reason:     "legacy snippet adapter does not perform host detection",
	}
}

func (a legacyHostAdapter) Plan(root string, scope integration.Scope) (integration.HostPlan, error) {
	return integration.HostPlan{
		Host:  a.name,
		Root:  root,
		Scope: scope,
		Actions: []integration.HostAction{{
			Kind:        "manual",
			Target:      a.entry.Dest,
			Description: "merge the generated MCP snippet into the host configuration",
			Args:        []string{},
		}},
		Warnings: []string{},
	}, nil
}

func (a legacyHostAdapter) Install(context.Context, integration.HostPlan) (integration.HostResult, error) {
	return integration.HostResult{
		Host:       a.name,
		Status:     "manual",
		Targets:    []string{a.entry.Dest},
		Backups:    []string{},
		Warnings:   []string{},
		NextAction: "merge the generated MCP snippet into the host configuration",
	}, nil
}

func (a legacyHostAdapter) Inspect(string, integration.Scope) (integration.HostState, error) {
	return integration.HostState{
		Host:   a.name,
		Reason: "manual snippet adapter cannot inspect host configuration",
	}, nil
}

func (a legacyHostAdapter) Verify(string) integration.Verification {
	return integration.Verification{
		Host:   a.name,
		Status: "manual",
		Reason: "manual snippet adapter cannot verify host registration",
	}
}

var adapterRegistry = newLegacyAdapterRegistry()

func newLegacyAdapterRegistry() *integration.Registry {
	adapters := make([]integration.HostAdapter, 0, len(hostRegistry))
	for name, entry := range hostRegistry {
		adapters = append(adapters, legacyHostAdapter{name: name, entry: entry})
	}
	return integration.MustRegistry(adapters...)
}

// HostConfig returns the dest hint and config content for the named MCP host.
// The content string has /path/to/your/project replaced with root when root is
// non-empty. Returns ("", "", false) for an unknown host name.
func HostConfig(name, root string) (dest, content string, ok bool) {
	entry, ok := hostRegistry[name]
	if !ok {
		return "", "", false
	}
	b, err := hostsFS.ReadFile(entry.file)
	if err != nil {
		panic("mcp: missing embedded host config " + entry.file)
	}
	content = strings.TrimRight(string(b), "\n")
	if root != "" {
		content = strings.ReplaceAll(content, "/path/to/your/project", root)
	}
	return entry.Dest, content, true
}

// HostNames returns the sorted list of supported MCP host names.
func HostNames() []string {
	return adapterRegistry.Names()
}
