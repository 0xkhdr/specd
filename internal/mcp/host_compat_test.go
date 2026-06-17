package mcp_test

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// documentedHosts is the set of MCP hosts docs/mcp-guide.md promises support for
// (the "Supported values:" line under Host Configuration). The matrix test below
// asserts the host registry matches this set exactly, so the docs never imply
// support specd does not ship, nor hide a host it does. R3.1.
var documentedHosts = []string{"antigravity", "claude-desktop", "codex", "cursor", "vscode"}

// TestHostCompatibilityMatrix verifies every documented host integration path:
// (a) `--config <host>` produces a non-empty snippet with the project root
// substituted, and (b) the transport those snippets target (stdio) yields a
// working tool call. The registry must equal the documented set — no silent
// drift in either direction. R3.1 / R3.3.
func TestHostCompatibilityMatrix(t *testing.T) {
	// Registry ↔ docs parity: honest support surface, no implied coverage.
	got := mcp.HostNames()
	if strings.Join(got, ",") != strings.Join(documentedHosts, ",") {
		t.Fatalf("host registry %v != documented hosts %v (docs/mcp-guide.md and hosts.go disagree)", got, documentedHosts)
	}

	const root = "/tmp/proj"
	for _, host := range documentedHosts {
		t.Run(host, func(t *testing.T) {
			dest, content, ok := mcp.HostConfig(host, root)
			if !ok {
				t.Fatalf("HostConfig(%q) not ok — documented but unsupported", host)
			}
			if strings.TrimSpace(content) == "" {
				t.Fatalf("HostConfig(%q) empty content", host)
			}
			if dest == "" {
				t.Errorf("HostConfig(%q) has no dest hint", host)
			}
			// The snippet must invoke the specd MCP server, else it is not a
			// working integration path.
			if !strings.Contains(content, "specd") || !strings.Contains(content, "mcp") {
				t.Errorf("HostConfig(%q) snippet does not invoke `specd mcp`:\n%s", host, content)
			}
			// Snippets carrying a project path must honor the root substitution.
			if strings.Contains(content, "/path/to/your/project") {
				t.Errorf("HostConfig(%q) left placeholder path unsubstituted", host)
			}
		})
	}

	// All registry hosts target the stdio transport (docs: "Use stdio (default)
	// for all local desktop hosts"). Prove that path yields a working tool call.
	h := th.New(t)
	h.Spec("hostcompat").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	const statusReq = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["hostcompat"]}}}`
	res := stdioResult(t, statusReq)
	if res["isError"] == true {
		t.Fatalf("stdio specd_status tool call returned error result: %v", res)
	}
	if _, ok := res["content"]; !ok {
		t.Fatalf("stdio tool call has no content payload: %v", res)
	}
}
