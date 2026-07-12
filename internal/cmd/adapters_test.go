package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/adapter"
)

func writeManifest(t *testing.T, root string, entries []adapter.Adapter) {
	t.Helper()
	dir := filepath.Join(root, ".specd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "adapters.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAdaptersProjection pins R7.2: the read-only projection classifies each
// adapter as configured, missing, incompatible, or disabled.
func TestAdaptersProjection(t *testing.T) {
	root := t.TempDir()
	present := filepath.Join(root, "present-bin")
	if err := os.WriteFile(present, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, root, []adapter.Adapter{
		{Name: "good", Path: present, SchemaVersion: adapter.SchemaVersion, Enabled: true},
		{Name: "gone", Path: filepath.Join(root, "nope"), Enabled: true},
		{Name: "old", Path: present, SchemaVersion: "adapter/v0", Enabled: true},
		{Name: "off", Path: present, Enabled: false},
	})

	out, err := captureStdout(t, func() error {
		return Run(root, "adapters", nil, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatalf("adapters --json: %v", err)
	}
	var report adaptersReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	want := map[string]string{"good": "configured", "gone": "missing", "old": "incompatible", "off": "disabled"}
	got := map[string]string{}
	for _, a := range report.Adapters {
		got[a.Name] = a.State
	}
	for name, state := range want {
		if got[name] != state {
			t.Errorf("adapter %q state=%q, want %q", name, got[name], state)
		}
	}
}

// TestAdaptersNoManifest pins R8.1: with no manifest the projection is empty and
// the command stays usable — core needs no adapters configured.
func TestAdaptersNoManifest(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return Run(t.TempDir(), "adapters", nil, map[string]string{})
	})
	if err != nil {
		t.Fatalf("adapters: %v", err)
	}
	if !strings.Contains(out, "no adapters configured") {
		t.Fatalf("expected empty-manifest notice, got %q", out)
	}
}

// TestAdaptersNoSecretLoad pins R7.2/R6.3: inspection never surfaces the values
// of the env vars an adapter is allowed to read — only their names live in the
// manifest, and inspection reads nothing from the environment.
func TestAdaptersNoSecretLoad(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SPECD_ADAPTER_SECRET", "super-secret-value")
	writeManifest(t, root, []adapter.Adapter{
		{Name: "svc", Path: filepath.Join(root, "svc"), EnvAllow: []string{"SPECD_ADAPTER_SECRET"}, Enabled: true},
	})
	out, err := captureStdout(t, func() error {
		return Run(root, "adapters", nil, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatalf("adapters --json: %v", err)
	}
	if strings.Contains(out, "super-secret-value") {
		t.Fatal("inspection must not load or emit secret values")
	}
}

// TestAdaptersMalformedManifestFailsClosed pins the fail-closed contract: a
// broken manifest is a usage error, not a silent empty projection.
func TestAdaptersMalformedManifestFailsClosed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "adapters.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error {
		return Run(root, "adapters", nil, map[string]string{"json": ""})
	}); err == nil {
		t.Fatal("malformed manifest must fail closed")
	}
}
