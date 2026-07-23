package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestShippedSteeringConformance(t *testing.T) {
	root := t.TempDir()
	if err := core.WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	items, omissions, err := SelectSteering(root, SelectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range omissions {
		if o.Reason == "missing explicit applicability metadata" {
			t.Fatalf("shipped steering template omitted for missing metadata: %s", o.Source)
		}
	}
	want := map[string]struct {
		id       string
		priority int
	}{
		"product.md":   {"product", 10},
		"reasoning.md": {"reasoning", 5},
		"structure.md": {"structure", 20},
		"tech.md":      {"tech", 20},
		"workflow.md":  {"workflow", 5},
	}
	if len(items) != len(want) {
		t.Fatalf("selected %d steering templates, want %d: %+v", len(items), len(want), items)
	}
	for _, it := range items {
		name := filepath.Base(it.Source)
		canonical, ok := want[name]
		if !ok {
			t.Fatalf("unexpected steering template selected: %s", it.Source)
		}
		raw, err := os.ReadFile(filepath.Join(root, it.Source))
		if err != nil {
			t.Fatal(err)
		}
		metadata, err := parseMetadata(raw, "specd-context")
		if err != nil {
			t.Fatalf("%s: %v", it.Source, err)
		}
		if metadata.ID != canonical.id || metadata.Version != "1" || metadata.Priority != canonical.priority {
			t.Fatalf("%s metadata = id:%q version:%q priority:%d", it.Source, metadata.ID, metadata.Version, metadata.Priority)
		}
	}
}
