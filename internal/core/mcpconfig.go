package core

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
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
func MCPConfigSnippet(host, root, spec string, resolutionRoots ...string) (string, error) {
	if host != "claude-code" {
		return "", fmt.Errorf("unknown host %q; known hosts: %s", host, strings.Join(MCPHosts(), ", "))
	}
	resolutionRoot := root
	if len(resolutionRoots) > 0 {
		resolutionRoot = resolutionRoots[0]
	}
	command, err := ResolveMCPCommand(resolutionRoot)
	if err != nil {
		return "", err
	}
	return claudeCodeSnippet(command, root, spec)
}

// ResolveMCPCommand prefers the repository binary and otherwise returns the
// installed binary's resolved path. Generated hosting never relies on a later,
// potentially different PATH lookup.
func ResolveMCPCommand(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if local, err := exec.LookPath(filepath.Join(absRoot, "specd")); err == nil {
		return local, nil
	}
	installed, err := exec.LookPath("specd")
	if err != nil {
		return "", fmt.Errorf("resolve specd executable for MCP hosting: %w", err)
	}
	return installed, nil
}

func claudeCodeSnippet(command, root, spec string) (string, error) {
	server := map[string]any{
		"command": command,
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

func mcpCommandFromCodexConfig(config string) (string, bool) {
	start := strings.Index(config, pinkyCodexBegin)
	if start < 0 {
		return "", false
	}
	end := strings.Index(config[start:], pinkyCodexEnd)
	if end < 0 {
		return "", false
	}
	for _, line := range strings.Split(config[start:start+end], "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.TrimSpace(key) != "command" {
			continue
		}
		command, err := strconv.Unquote(strings.TrimSpace(value))
		return command, err == nil && command != ""
	}
	return "", false
}
