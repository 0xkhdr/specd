package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MCPHosts is the set of agent hosts specd can emit a paste-ready MCP config
// snippet for. `claude-code` is the minimum (spec 11 R1); the list is designed to
// grow — add a case in MCPConfigSnippet and an entry here.
func MCPHosts() []string {
	return []string{"claude-code"}
}

// MCPConfigSnippet returns a ready-to-paste MCP server configuration wiring
// `specd mcp` for the named host. Optional root pins the server's working
// directory. spec is reserved until MCP consumes the common resolver; no inert
// SPECD_SPEC pin is emitted. An unknown host is an
// error naming the known hosts, which the caller maps to a fail-closed exit 2.
//
// The snippet is built from a typed structure and marshaled, so it is always
// valid JSON with stable (sorted) key order — golden-testable and never a
// hand-concatenated string that can drift into invalid JSON.
func MCPConfigSnippet(host, root, spec string) (string, error) {
	switch host {
	case "claude-code":
		return claudeCodeSnippet(root, spec)
	default:
		return "", fmt.Errorf("unknown host %q; known hosts: %s", host, strings.Join(MCPHosts(), ", "))
	}
}

func claudeCodeSnippet(root, spec string) (string, error) {
	server := map[string]any{
		"command": "specd",
		"args":    []string{"mcp"},
	}
	if root != "" {
		server["cwd"] = root
	}
	_ = spec
	cfg := map[string]any{
		"mcpServers": map[string]any{"specd": server},
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}
