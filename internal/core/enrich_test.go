package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// bootedRoot creates a temp repo with .specd/ + go.mod and a written boot.json,
// the precondition every enrich operation requires.
func bootedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if root2, err := filepath.EvalSymlinks(root); err == nil {
		root = root2
	}
	writeFiles(t, root, map[string]string{
		".specd/steering/.keep": "",
		"go.mod":                "module x\ngo 1.22\n",
	})
	res := AnalyzeBoot(root)
	b, _ := json.MarshalIndent(res, "", "  ")
	if err := os.WriteFile(filepath.Join(SpecdDir(root), "boot.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBuildEnrichBrief_Deterministic(t *testing.T) {
	root := bootedRoot(t)
	a := BuildEnrichBrief(root)
	b := BuildEnrichBrief(root)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("brief not deterministic:\n%s\n%s", ja, jb)
	}
	if len(a.Targets) != 3 {
		t.Fatalf("want 3 targets, got %d", len(a.Targets))
	}
	for _, tg := range a.Targets {
		if tg.State != "stub" {
			t.Fatalf("%s: state %q want stub", tg.Target, tg.State)
		}
	}
}

func TestApplyEnrichSection_Idempotent(t *testing.T) {
	root := bootedRoot(t)
	if err := ApplyEnrichSection(root, "product", "## Product\n\nApp.\n"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(SteeringDir(root), "product.md"))
	if err := ApplyEnrichSection(root, "product", "## Product\n\nApp.\n"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(SteeringDir(root), "product.md"))
	if string(first) != string(second) {
		t.Fatalf("not idempotent:\n%s\n---\n%s", first, second)
	}
	rec, ok := LoadEnrichRecord(root)
	if !ok || !contains(rec.Targets, "product") {
		t.Fatalf("record missing product: %+v", rec)
	}
}

func TestApplyEnrichSection_RequiresBoot(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(SpecdDir(root), 0o755)
	err := ApplyEnrichSection(root, "product", "## Product\n")
	if err == nil {
		t.Fatal("expected error without boot.json")
	}
}

func TestCheckEnrichFreshness_States(t *testing.T) {
	root := bootedRoot(t)

	// No enrich.json → NotFound.
	if _, err := CheckEnrichFreshness(root); err == nil {
		t.Fatal("want NotFound before any enrichment")
	}

	// Partial enrichment → stale (other targets still stubs).
	ApplyEnrichSection(root, "product", "## Product\n\nx.\n")
	f, err := CheckEnrichFreshness(root)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Stale {
		t.Fatal("want stale with only product enriched")
	}

	// Full enrichment → fresh.
	ApplyEnrichSection(root, "structure", "## Layout\n\nx.\n")
	ApplyEnrichSection(root, "tech", "## Conventions\n\nx.\n")
	f, _ = CheckEnrichFreshness(root)
	if f.Stale {
		t.Fatalf("want fresh, got issues: %v", f.Issues)
	}

	// Removing a recorded source makes it stale again.
	os.Remove(filepath.Join(root, "go.mod"))
	f, _ = CheckEnrichFreshness(root)
	if !f.Stale {
		t.Fatal("want stale after removing go.mod")
	}
}
