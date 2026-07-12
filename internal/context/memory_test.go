package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemorySelection(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "# Memory\n\n## atomic\n**Pattern:** atomic writes\n**Detail:** rename\n**Source:** io.go\n**Criticality:** critical\n**Related:** [[V1]]\n**Applies-To:** tags=go,io; phases=execute; roles=craftsman; files=internal/*.go\n\n## css\n**Pattern:** css\n**Criticality:** important\n**Applies-To:** tags=css\n"
	path := filepath.Join(dir, "memory.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	items, omissions, err := SelectMemory(root, "demo", SelectionContext{Phase: "execute", Role: "craftsman", Tags: []string{"go"}, Files: []string{"internal/io.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Selector != "memory:atomic" || items[0].Priority >= ExamplePriority {
		t.Fatalf("selected = %+v", items)
	}
	if len(omissions) != 1 || omissions[0].Source == "" {
		t.Fatalf("omissions = %+v", omissions)
	}
}
