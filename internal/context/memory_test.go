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
	doc := "# Memory\n\n## atomic\n**Pattern:** atomic writes\n**Detail:** rename\n**Source:** evidence:sha256:abc\n**Criticality:** critical\n**Related:** [[V1]]\n**Status:** active\n**Applies-To:** tags=go,io; phases=execute; roles=craftsman; files=internal/*.go\n\n## css\n**Pattern:** css\n**Source:** review:review_report.md\n**Criticality:** important\n**Status:** active\n**Applies-To:** tags=css\n\n## old\n**Pattern:** old\n**Source:** exception:EX-1\n**Criticality:** critical\n**Status:** expired\n**Applies-To:** tags=go\n\n## replaced\n**Pattern:** replaced\n**Source:** evidence:sha256:def\n**Criticality:** critical\n**Status:** superseded\n**Superseded-By:** atomic\n**Applies-To:** tags=go\n"
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
	if len(omissions) != 3 || omissions[0].Source == "" {
		t.Fatalf("omissions = %+v", omissions)
	}
	for _, omission := range omissions {
		if omission.Source == ".specd/specs/demo/memory.md#old" && omission.Reason != "expired" {
			t.Fatalf("expired omission = %+v", omission)
		}
		if omission.Source == ".specd/specs/demo/memory.md#replaced" && omission.Reason != "superseded by atomic" {
			t.Fatalf("superseded omission = %+v", omission)
		}
	}
}
