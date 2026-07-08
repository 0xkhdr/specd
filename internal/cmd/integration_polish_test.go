package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
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
	for _, want := range []string{`"mcpServers"`, `"specd"`, `"mcp"`, "SPECD_SPEC", "demo", "/proj"} {
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
