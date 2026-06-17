package mcp

import (
	"embed"
	"sort"
	"strings"
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
	names := make([]string, 0, len(hostRegistry))
	for k := range hostRegistry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
