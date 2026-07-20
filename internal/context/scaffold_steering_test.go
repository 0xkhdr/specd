package context

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestScaffoldedSteeringSelects pins the steering-template → SelectSteering
// contract (R1.1, R5.1): a freshly scaffolded project must load every shipped
// steering file into the machine manifest with zero "missing explicit
// applicability metadata" omissions. It asserts through the real consumer, not a
// hand-copied expected string, so a template that drops its specd-context block
// fails here naming both the template and this consumer.
func TestScaffoldedSteeringSelects(t *testing.T) {
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
	if len(items) == 0 {
		t.Fatal("no steering items selected from a freshly scaffolded root")
	}
	for _, it := range items {
		if it.Source == ".specd/steering/memory.md" {
			t.Fatalf("memory.md must be excluded from SelectSteering (R1.1), got %s", it.Source)
		}
	}
}
