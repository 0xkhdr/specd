package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestManifestAtomicStableOrderingAndOwnership(t *testing.T) {
	root := t.TempDir()
	path := core.IntegrationsPath(root)
	content := []byte(`{"command":"specd"}`)
	manifest := Manifest{
		Version: ManifestVersion,
		Entries: []ManifestEntry{
			{
				Host: "zeta", Scope: ScopeProject, ServerName: "specd",
				Root: ".", RootStrategy: "relative", Method: "project-file",
				Target: ".zeta/mcp.json", Fingerprint: Fingerprint(content),
			},
			{
				Host: "alpha", Scope: ScopeProject, ServerName: "specd",
				Root: ".", RootStrategy: "relative", Method: "native-cli",
				Target: ".alpha/mcp.json", Fingerprint: Fingerprint(content),
			},
		},
	}
	if err := SaveManifest(path, manifest); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(path, manifest); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("manifest serialization is unstable:\n%s\n%s", first, second)
	}
	if strings.Index(string(first), `"alpha"`) > strings.Index(string(first), `"zeta"`) {
		t.Fatalf("manifest entries not sorted:\n%s", first)
	}
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("entries = %d", len(loaded.Entries))
	}
	if err := VerifyOwnership(loaded.Entries[0], content); err != nil {
		t.Fatal(err)
	}
	if err := VerifyOwnership(loaded.Entries[0], []byte("changed")); err == nil {
		t.Fatal("fingerprint mismatch accepted")
	}
	if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".*.tmp")); len(matches) != 0 {
		t.Fatalf("atomic write left temp files: %v", matches)
	}
}

func TestManifestMigrationGuardAndNoSecretFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "integrations.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"entries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("newer manifest version accepted")
	}
	entry := ManifestEntry{}
	if strings.Contains(strings.ToLower(strings.Join([]string{
		entry.Host, entry.ServerName, entry.Root, entry.Method, entry.Target,
	}, " ")), "secret") {
		t.Fatal("manifest exposes secret storage")
	}
}
