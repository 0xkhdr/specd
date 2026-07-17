package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/adapter"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
)

// mangleFirstManagedAsset corrupts the managed region of the first role/steering
// asset and wraps it in user content outside the markers, returning the asset and
// the user sentinels that must survive a repair.
func mangleFirstManagedAsset(t *testing.T, root string) (core.ManagedAsset, string, string) {
	t.Helper()
	assets, err := core.ManagedAssets()
	if err != nil || len(assets) == 0 {
		t.Fatalf("managed assets: %v", err)
	}
	asset := assets[0]
	begin := "<!-- specd:managed:" + asset.Name + ":v1 begin -->"
	end := "<!-- specd:managed:" + asset.Name + ":v1 end -->"
	head, tail := "USER-HEADER-KEEP", "USER-FOOTER-KEEP"
	corrupted := head + "\n\n" + begin + "\nMANGLED CONTENT\n" + end + "\n\n" + tail + "\n"
	if err := os.WriteFile(filepath.Join(root, asset.RelPath), []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}
	return asset, head, tail
}

func TestInitRepair(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "init", nil, nil); err != nil {
		t.Fatalf("init: %v", err)
	}
	asset, head, tail := mangleFirstManagedAsset(t, root)

	out, err := captureStdout(t, func() error { return Run(root, "init", nil, map[string]string{"repair": ""}) })
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if !strings.Contains(out, "repaired") {
		t.Fatalf("repair did not report the file: %q", out)
	}

	raw, err := os.ReadFile(filepath.Join(root, asset.RelPath))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if strings.Contains(got, "MANGLED CONTENT") {
		t.Fatal("repair did not restore the managed region")
	}
	if !strings.Contains(got, strings.TrimSpace(strings.Split(asset.Template, "\n")[0])) {
		t.Fatal("repair did not restore template content")
	}
	// Content outside the markers is byte-preserved (R3).
	if !strings.Contains(got, head) || !strings.Contains(got, tail) {
		t.Fatalf("repair clobbered user content outside markers:\n%s", got)
	}
}

func TestInitDryRun(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "init", nil, nil); err != nil {
		t.Fatalf("init: %v", err)
	}
	asset, _, _ := mangleFirstManagedAsset(t, root)
	before, err := os.ReadFile(filepath.Join(root, asset.RelPath))
	if err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return Run(root, "init", nil, map[string]string{"repair": "", "dry-run": ""})
	})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out, "--- "+asset.RelPath) || !strings.Contains(out, "+ ") {
		t.Fatalf("dry-run did not show a diff preview:\n%s", out)
	}
	// Nothing was written.
	after, err := os.ReadFile(filepath.Join(root, asset.RelPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dry-run wrote to disk")
	}
}

func TestMCPConfig(t *testing.T) {
	root := t.TempDir()

	out, err := captureStdout(t, func() error {
		return Run(root, "mcp", nil, map[string]string{"config": "claude-code", "spec": "demo", "root": "/proj"})
	})
	if err != nil {
		t.Fatalf("mcp --config: %v", err)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("snippet is not valid JSON:\n%s", out)
	}
	for _, want := range []string{`"mcpServers"`, `"specd"`, `"mcp"`, "/proj"} {
		if !strings.Contains(out, want) {
			t.Fatalf("snippet missing %q:\n%s", want, out)
		}
	}

	// Unknown host fails closed, listing known hosts (exit 2 semantics via ErrUsage).
	err = Run(root, "mcp", nil, map[string]string{"config": "emacs"})
	if err == nil || !strings.Contains(err.Error(), "known hosts") {
		t.Fatalf("unknown host should fail listing known hosts, got %v", err)
	}
}

func TestHandshakeDigest(t *testing.T) {
	root := t.TempDir()

	out, err := captureStdout(t, func() error {
		return Run(root, "handshake", []string{"bootstrap"}, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	var hs struct {
		PaletteDigest string `json:"palette_digest"`
		ConfigDigest  string `json:"config_digest"`
	}
	if err := json.Unmarshal([]byte(out), &hs); err != nil {
		t.Fatalf("handshake json: %v", err)
	}
	if hs.PaletteDigest == "" || hs.ConfigDigest == "" {
		t.Fatalf("digests missing: %+v", hs)
	}

	// Matching expectation passes; drift fails with exit 1 (R6).
	if err := Run(root, "handshake", []string{"bootstrap"}, map[string]string{"expect-palette-digest": hs.PaletteDigest}); err != nil {
		t.Fatalf("matching palette digest should pass: %v", err)
	}
	err = Run(root, "handshake", []string{"bootstrap"}, map[string]string{"expect-palette-digest": "deadbeef"})
	if err == nil || !strings.Contains(err.Error(), "palette digest drift") {
		t.Fatalf("stale palette digest should be caught, got %v", err)
	}
	err = Run(root, "handshake", []string{"bootstrap"}, map[string]string{"expect-config-digest": "deadbeef"})
	if err == nil || !strings.Contains(err.Error(), "config digest drift") {
		t.Fatalf("stale config digest should be caught, got %v", err)
	}
}

func TestHandshakeMismatchExits(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "init", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "new", []string{"demo"}, map[string]string{"title": "Demo"}); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return Run(root, "handshake", []string{"bootstrap", "demo"}, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatal(err)
	}
	var hs core.Handshake
	if err := json.Unmarshal([]byte(out), &hs); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"expect-binary-version":  "wrong",
		"expect-state-schema":    "999",
		"expect-context-schema":  "999",
		"expect-template-schema": "999",
		"expect-root":            filepath.Join(root, "wrong"),
		"expect-spec":            "wrong",
		"expect-revision":        "999",
		"expect-palette-digest":  "wrong",
		"expect-config-digest":   "wrong",
		"expect-managed-digest":  "wrong",
	}
	for flag, value := range cases {
		err := Run(root, "handshake", []string{"bootstrap", "demo"}, map[string]string{flag: value})
		if err == nil || !strings.Contains(err.Error(), "precondition "+strings.TrimPrefix(flag, "expect-")) || !strings.Contains(err.Error(), "current") {
			t.Fatalf("%s mismatch not actionable: %v", flag, err)
		}
		after, readErr := os.ReadFile(core.StatePath(root, "demo"))
		if readErr != nil || string(after) != string(before) {
			t.Fatalf("%s mismatch mutated state: %v", flag, readErr)
		}
	}
	if err := Run(root, "handshake", []string{"bootstrap", "demo"}, map[string]string{"expect-managed-digest": hs.ManagedDigest}); err != nil {
		t.Fatalf("matching managed digest: %v", err)
	}
}

func TestIntegrationApproveHandoffParity(t *testing.T) {
	root := newDemoSpec(t)
	out, err := captureStdout(t, func() error {
		return Run(root, "agents", []string{"guide", "demo"}, map[string]string{"json": "true"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var guide core.DriverGuideV1
	if err := json.Unmarshal([]byte(out), &guide); err != nil {
		t.Fatal(err)
	}
	var approve core.NextAction
	foundApprove := false
	for _, action := range guide.NextActions {
		if action.Command == "approve" {
			approve, foundApprove = action, true
			break
		}
	}
	if !foundApprove {
		t.Fatal("driver guide omitted approve handoff")
	}
	raw, err := json.Marshal(map[string]any{"name": "approve", "arguments": map[string]any{"args": approve.Args}})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	resp := mcp.Dispatch(mcp.Request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: raw}, mcp.CoreTools(), func(string, []string, map[string]string, *core.AuthorityV1, time.Time) (string, error) {
		called = true
		return "", nil
	})
	if called {
		t.Fatal("MCP human handoff executed approval")
	}
	if resp.Error == nil || resp.Error.Code != mcp.MCPHandoffRequiredCode {
		t.Fatalf("MCP response = %+v", resp)
	}
	handoff, ok := resp.Error.Data.(mcp.Handoff)
	if !ok || handoff.Command != "specd approve demo" || handoff.Actor != approve.Actor {
		t.Fatalf("driver/MCP handoff mismatch: action=%+v handoff=%+v", approve, handoff)
	}
}

func TestIntegrationMachineContextCarriesDriverContract(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".specd/specs/demo", ".specd/roles", "internal"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, ".specd/specs/demo/tasks.md"), "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | internal/a.go | - | go test ./... | R2.1 |\n")
	writeFile(t, filepath.Join(root, ".specd/specs/demo/requirements.md"), "# Requirements\n")
	writeFile(t, filepath.Join(root, ".specd/specs/demo/design.md"), "# Design\n")
	writeFile(t, filepath.Join(root, ".specd/roles/craftsman.md"), "# Craftsman\n")
	writeFile(t, filepath.Join(root, "internal/a.go"), "package internal\n")
	out, err := captureStdout(t, func() error { return Run(root, "context", []string{"demo", "T1"}, map[string]string{"json": ""}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"schema_version": "1"`, `"selected_task"`, `"route": "cli:`, `"palette_digest"`, `"config_digest"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("machine context missing %s:\n%s", want, out)
		}
	}
}

func TestIntegrationMachineContextRejectsRouteIdentityMismatch(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".specd/specs/demo", ".specd/roles", "internal"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, ".specd/specs/demo/tasks.md"), "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | internal/a.go | - | go test ./... | R2.1 |\n")
	writeFile(t, filepath.Join(root, ".specd/specs/demo/requirements.md"), "# Requirements\n")
	writeFile(t, filepath.Join(root, ".specd/specs/demo/design.md"), "# Design\n")
	writeFile(t, filepath.Join(root, ".specd/roles/craftsman.md"), "# Craftsman\n")
	writeFile(t, filepath.Join(root, "internal/a.go"), "package internal\n")
	out, err := captureStdout(t, func() error { return Run(root, "context", []string{"demo", "T1"}, map[string]string{"json": ""}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"route": "cli:`) || strings.Contains(out, `"route": "mcp:`) {
		t.Fatalf("context route identity mismatch: %s", out)
	}
}

// Domain 03 W0 baselines: later waves flip each assertion when common spec
// resolution and truthful plain-path migration land.
func TestIntegrationDriverGapBaseline(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "mcp", nil, map[string]string{"config": "claude-code", "spec": "demo"}); err != nil {
		t.Fatal(err)
	}
	// Until MCP consumes common resolver, generated config must not emit inert pin.
	out, _ := captureStdout(t, func() error { return Run(root, "mcp", nil, map[string]string{"config": "claude-code", "spec": "demo"}) })
	if strings.Contains(out, "SPECD_SPEC") {
		t.Fatal("generated inert host pin")
	}
}

// TestOfflineProviderOutageContinuesPlanning pins R8.2 at the command layer: a
// configured-but-unreachable adapter is projected as `missing` with an exact
// external cause, and the read-only command still succeeds. The outage becomes
// visible blocked evidence, never an implicit success (`configured`) and never a
// command failure that would stall local planning.
func TestOfflineProviderOutageContinuesPlanning(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "unreachable-provider")
	writeManifest(t, root, []adapter.Adapter{
		{Name: "provider", Path: missing, SchemaVersion: adapter.SchemaVersion, Enabled: true},
	})

	out, err := captureStdout(t, func() error {
		return Run(root, "adapters", nil, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatalf("outage must not fail the command (planning continues): %v", err)
	}
	var report adaptersReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if len(report.Adapters) != 1 {
		t.Fatalf("want one adapter, got %d", len(report.Adapters))
	}
	got := report.Adapters[0]
	if got.State == "configured" {
		t.Fatal("an unreachable provider must never project as configured (implicit success)")
	}
	if got.State != "missing" {
		t.Fatalf("state=%q, want missing", got.State)
	}
	if !strings.Contains(got.Detail, missing) {
		t.Fatalf("missing detail must name the exact external cause %q, got %q", missing, got.Detail)
	}
}
