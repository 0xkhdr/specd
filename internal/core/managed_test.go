package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestManagedMarkers(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	assets, err := ManagedAssets()
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) == 0 {
		t.Fatal("no managed assets discovered")
	}
	// Every managed region is wrapped in a versioned marker (R2).
	for _, asset := range assets {
		raw, err := os.ReadFile(filepath.Join(root, asset.RelPath))
		if err != nil {
			t.Fatalf("read %s: %v", asset.RelPath, err)
		}
		begin := "<!-- specd:managed:" + asset.Name + ":v" + strconv.Itoa(TemplateVersion) + " begin -->"
		if !strings.Contains(string(raw), begin) {
			t.Fatalf("%s missing begin marker:\n%s", asset.RelPath, raw)
		}
	}

	// A freshly scaffolded tree has no drift.
	changes, err := PlanManagedRepair(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("fresh scaffold reports drift: %+v", changes)
	}

	// Idempotence: scaffolding again is byte-identical (twice = once).
	snapshot := treeSnapshot(t, root)
	if err := WriteScaffold(root); err != nil {
		t.Fatalf("re-scaffold: %v", err)
	}
	if after := treeSnapshot(t, root); after != snapshot {
		t.Fatal("re-scaffold changed bytes; not idempotent")
	}
}

func treeSnapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.WriteString(path)
		b.WriteByte('\n')
		b.Write(raw)
		b.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
