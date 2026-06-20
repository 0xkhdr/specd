package mcp_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// driveCfg is drive() with an explicit project config so tools/list exercises
// the configured (non-passthrough) path the Wave 4 filters live on.
func driveCfg(t *testing.T, cfg *core.Config, requests ...string) []map[string]any {
	t.Helper()
	in := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var out bytes.Buffer
	if err := mcp.Serve(in, &out, cmd.Dispatch, cfg); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var resps []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("response not JSON: %q: %v", line, err)
		}
		resps = append(resps, m)
	}
	return resps
}

func listToolNames(t *testing.T, resp map[string]any) []string {
	t.Helper()
	r := result(t, resp)
	raw, ok := r["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list result missing tools array: %v", r)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		m, _ := item.(map[string]any)
		if n, ok := m["name"].(string); ok {
			out = append(out, n)
		}
	}
	return out
}

func writeManifest(t *testing.T, root, slug, body string) {
	t.Helper()
	path := core.SpecDir(root, slug) + "/manifest.json"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func has(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// TestManifestFilterIntegration: a configured server reading a spec's
// manifest.json restricts tools/list to its required/optional set and excludes
// forbidden tools (C1 AC1/AC2).
func TestManifestFilterIntegration(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")
	writeManifest(t, h.Root, "auth", `{"contextManifest":{
		"requiredTools":["specd_inspect","specd_verify"],
		"forbiddenTools":["specd_task"]}}`)

	cfg := &core.Config{MCP: core.MCPConfig{Expose: "all"}}
	resps := driveCfg(t, cfg, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	got := listToolNames(t, resps[0])

	if !has(got, "specd_inspect") || !has(got, "specd_verify") {
		t.Fatalf("required tools missing: %v", got)
	}
	if has(got, "specd_task") {
		t.Fatalf("forbidden specd_task present: %v", got)
	}
	if has(got, "specd_status") {
		t.Fatalf("non-allowlisted specd_status present: %v", got)
	}
}

// TestNoManifestUnchangedIntegration: with no manifest, a configured server's
// tools/list matches the config/phase plan (C1 AC4).
func TestNoManifestUnchangedIntegration(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "all"}}

	withManifestWritten := func(body string) int {
		if body != "" {
			writeManifest(t, h.Root, "auth", body)
		}
		resps := driveCfg(t, cfg, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		return len(listToolNames(t, resps[0]))
	}
	base := withManifestWritten("")
	// Adding an all-permitting (absent-field) manifest must not change the count.
	if got := withManifestWritten(`{"other":true}`); got != base {
		t.Fatalf("empty-policy manifest changed tool count: %d vs %d", got, base)
	}
}

// TestHostNegotiationMaxTools: capabilities.specd.maxTools caps tools/list, and
// the same session keeps applying it on re-fetch (C2 AC1/R5).
func TestHostNegotiationMaxTools(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "all"}}

	resps := driveCfg(t, cfg,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{"specd":{"maxTools":5}}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
	)
	for _, id := range []int{1, 2} { // resp index 1,2 are the two tools/list
		got := listToolNames(t, resps[id])
		if len(got) > 5 {
			t.Fatalf("maxTools=5 emitted %d tools: %v", len(got), got)
		}
	}
}

// TestHostNegotiationAbsentIdentical: a host that omits capabilities.specd sees
// the exact feature-off tool list (C2 AC4).
func TestHostNegotiationAbsentIdentical(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "all"}}

	plain := listToolNames(t, driveCfg(t, cfg, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)[0])
	negotiated := driveCfg(t, cfg,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	got := listToolNames(t, negotiated[1])
	if strings.Join(plain, ",") != strings.Join(got, ",") {
		t.Fatalf("absent capability changed output:\n plain=%v\n got  =%v", plain, got)
	}
}
