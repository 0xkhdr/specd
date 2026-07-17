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

// TestSteeringInManifestMissingSilentBaseline (W0 T02, R8/R2.3) pins the current
// gap: when no steering/memory exists it is silently skipped rather than failing
// the build with item identity. W1/W3 make *required* knowledge fail closed;
// optional steering legitimately stays optional. This baseline flips when the
// required-lane resolution lands.
func TestSteeringInManifestMissingSilentBaseline(t *testing.T) {
	root := t.TempDir() // no .specd/steering dir at all
	m, err := BuildManifest(root, "demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1", 0)
	if err != nil {
		t.Fatalf("baseline expected silent skip, got error: %v — update this baseline in W1/W3", err)
	}
	for _, it := range m.Items {
		if it.Kind == "steering" || it.Kind == "memory" {
			t.Fatalf("expected no steering/memory when dir absent, got %+v", it)
		}
	}
}

func TestManifestProgressiveStaticLanes(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".specd/specs/demo/requirements.md", "# Requirements\n")
	write(".specd/specs/demo/design.md", "# Design\n")
	write(".specd/roles/craftsman.md", "# Role\n")
	write("internal/x.go", "package internal\n")
	write(".specd/steering/go.md", "<!-- specd-context\nphases: execute\nroles: craftsman\nfiles: **/*.go\n-->\n# Go\n")
	write(".specd/examples/go.md", "<!-- specd-example\nid: go\nversion: 1\nphases: execute\nroles: craftsman\n-->\n# Example\n")
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"internal/x.go"}, Acceptance: "R6"}}
	hs := core.Handshake{ConfigDigest: "config", PaletteDigest: "palette"}
	m, err := BuildMachineManifest(root, "demo", tasks, "T1", "execute", "execute", 0, hs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"instructions": false, "examples": false}
	for _, item := range m.Items {
		if _, ok := want[item.Kind]; ok {
			want[item.Kind] = true
		}
	}
	for kind, found := range want {
		if !found {
			t.Fatalf("missing %s lane: %+v", kind, m.Items)
		}
	}
}
