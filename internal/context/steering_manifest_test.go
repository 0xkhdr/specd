package context

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestSteeringInManifest (P4.3/R4.3): the manifest references the steering
// constitution (static-instructions) and memory (reference-if-needed) as items,
// never inlines their content, and drops memory before steering when over
// budget.
func TestSteeringInManifest(t *testing.T) {
	root := t.TempDir()
	steer := filepath.Join(root, ".specd", "steering")
	if err := os.MkdirAll(steer, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(steer, "product.md"), []byte("product constitution"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(steer, "memory.md"), []byte("durable memory"), 0o644); err != nil {
		t.Fatal(err)
	}

	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman"}}
	m, err := BuildManifest(root, "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	var steering, memory Item
	for _, it := range m.Items {
		switch it.Kind {
		case "steering":
			steering = it
		case "memory":
			memory = it
		}
	}
	if steering.Mode != "static-instructions" || steering.Path != ".specd/steering/product.md" {
		t.Fatalf("steering item = %+v", steering)
	}
	if memory.Mode != "reference-if-needed" || memory.Path != ".specd/steering/memory.md" {
		t.Fatalf("memory item = %+v", memory)
	}

	// References only — content never inlined.
	raw, _ := json.Marshal(m)
	if strings.Contains(string(raw), "durable memory") || strings.Contains(string(raw), "product constitution") {
		t.Fatal("manifest inlined steering/memory content")
	}

	// Budget: memory sheds before steering, deterministically.
	items := []Item{
		{Kind: "role", EstimatedTokens: 5},
		{Kind: "steering", Path: "s", EstimatedTokens: 5},
		{Kind: "memory", Path: "m", EstimatedTokens: 5},
	}
	kept, notes := enforceBudget(items, 10)
	if len(notes) != 1 || len(kept) != 2 {
		t.Fatalf("expected one drop, got kept=%+v notes=%v", kept, notes)
	}
	for _, it := range kept {
		if it.Kind == "memory" {
			t.Fatal("memory should drop before steering")
		}
	}
	kept2, _ := enforceBudget(items, 4)
	for _, it := range kept2 {
		if it.Kind == "memory" || it.Kind == "steering" {
			t.Fatalf("both memory and steering should drop under tight budget: %+v", kept2)
		}
	}
}
