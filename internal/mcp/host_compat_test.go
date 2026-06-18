package mcp_test

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestHostCompatibilityMatrix verifies every documented host integration path:
// (a) `--config <host>` produces a non-empty snippet with the project root
// substituted, and (b) the transport those snippets target (stdio) yields a
// working tool call. The registry must equal the documented set — no silent
// drift in either direction. R3.1 / R3.3.
func TestHostCompatibilityMatrix(t *testing.T) {
	documentedHosts := documentedCompatibilityHosts(t)
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

func documentedCompatibilityHosts(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate host compatibility test")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "docs", "agent-harness-compat.md")
	handle, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	var hosts []string
	scanner := bufio.NewScanner(handle)
	inHosts := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "## Hosts" {
			inHosts = true
			continue
		}
		if inHosts && strings.HasPrefix(line, "## ") {
			break
		}
		if !inHosts || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) == 0 {
			continue
		}
		host := strings.TrimSpace(cells[0])
		if host == "Host" || strings.HasPrefix(host, "---") {
			continue
		}
		hosts = append(hosts, host)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(hosts)
	return hosts
}
