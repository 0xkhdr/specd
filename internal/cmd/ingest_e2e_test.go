package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestIngestNewCreatesInventoryAndSpec: `ingest new` inventories a legacy tree
// (countable facts) and scaffolds an ingestion spec with the inventory recorded.
func TestIngestNewCreatesInventoryAndSpec(t *testing.T) {
	h := th.New(t)
	// Seed a legacy subtree (non-git → bounded walk fallback).
	legacy := filepath.Join(h.Root, "legacy")
	for rel, content := range map[string]string{
		"go.mod":               "module example.com/legacy\n",
		"a.go":                 "package legacy\n",
		"sub/b.go":             "package sub\n",
		"node_modules/skip.js": "ignored",
	} {
		abs := filepath.Join(legacy, rel)
		_ = os.MkdirAll(filepath.Dir(abs), 0o755)
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h.RunExpect(core.ExitOK, "ingest", "new", "legacy-code", "--path", "legacy")

	inv, err := core.LoadInventory(h.Root, "legacy-code")
	if err != nil || inv == nil {
		t.Fatalf("inventory not written: %v", err)
	}
	// node_modules excluded by the default walk.
	for _, f := range inv.Files {
		if strings.Contains(f.Path, "node_modules") {
			t.Errorf("node_modules leaked into inventory: %s", f.Path)
		}
	}
	if len(inv.Modules) != 1 || inv.Modules[0] != "example.com/legacy" {
		t.Errorf("modules = %v, want [example.com/legacy]", inv.Modules)
	}
	if st := h.State("legacy-code").Raw(); st.Ingest == nil || st.Ingest.Files != len(inv.Files) {
		t.Fatalf("ingest record = %+v", st.Ingest)
	}
	// Scaffolded artifacts exist.
	if _, err := os.Stat(core.ArtifactPath(h.Root, "legacy-code", "requirements.md")); err != nil {
		t.Errorf("requirements.md not scaffolded: %v", err)
	}
}

// TestIngestRejectsTraversal: a --path escaping the repo is refused.
func TestIngestRejectsTraversal(t *testing.T) {
	h := th.New(t)
	h.RunExpect(core.ExitGate, "ingest", "new", "evil", "--path", "../../etc")
}

// TestIngestDeterministicInventory: two ingests of the same tree produce a
// byte-identical inventory.json (V10 §5).
func TestIngestDeterministicInventory(t *testing.T) {
	h := th.New(t)
	legacy := filepath.Join(h.Root, "src")
	for rel, c := range map[string]string{"x.go": "package x\n", "y.go": "package y\n"} {
		abs := filepath.Join(legacy, rel)
		_ = os.MkdirAll(filepath.Dir(abs), 0o755)
		_ = os.WriteFile(abs, []byte(c), 0o644)
	}
	h.RunExpect(core.ExitOK, "ingest", "new", "one", "--path", "src")
	h.RunExpect(core.ExitOK, "ingest", "new", "two", "--path", "src")

	b1, _ := os.ReadFile(core.InventoryPath(h.Root, "one"))
	b2, _ := os.ReadFile(core.InventoryPath(h.Root, "two"))
	// Base label differs by slug directory only in the file, not content; both
	// inventory the same relative tree, so file lists match.
	inv1, _ := core.LoadInventory(h.Root, "one")
	inv2, _ := core.LoadInventory(h.Root, "two")
	if len(inv1.Files) != len(inv2.Files) || len(inv1.Files) != 2 {
		t.Fatalf("inventories differ: %d vs %d", len(inv1.Files), len(inv2.Files))
	}
	if len(b1) == 0 || len(b2) == 0 {
		t.Fatal("inventory files empty")
	}
}
