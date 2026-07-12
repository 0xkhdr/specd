package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryBlock(t *testing.T) {
	// RenderMemBlock is byte-stable and formats --related into wikilinks.
	got := RenderMemBlock(MemFields{
		Key: "atomic-writes", Pattern: "write temp then rename", Detail: "fsync dir after",
		Source: "io.go", Criticality: "important", Related: "cas, lock",
	})
	want := "## atomic-writes\n**Pattern:** write temp then rename\n**Detail:** fsync dir after\n**Source:** io.go\n**Criticality:** important\n**Related:** [[cas]], [[lock]]\n"
	if got != want {
		t.Fatalf("RenderMemBlock mismatch:\n got %q\nwant %q", got, want)
	}

	// Absent --related renders as an em dash.
	if b := RenderMemBlock(MemFields{Key: "k"}); !strings.Contains(b, "**Related:** —\n") {
		t.Fatalf("empty related should render —, got %q", b)
	}

	// ExtractMemBlock reads one block up to the next heading.
	doc := "# title\n\n" + want + "\n## other\n**Pattern:** nope\n"
	block := ExtractMemBlock(doc, "atomic-writes")
	if block != "## atomic-writes\n**Pattern:** write temp then rename\n**Detail:** fsync dir after\n**Source:** io.go\n**Criticality:** important\n**Related:** [[cas]], [[lock]]" {
		t.Fatalf("ExtractMemBlock did not stop at next heading:\n%q", block)
	}
	if ExtractMemBlock(doc, "missing") != "" {
		t.Fatal("ExtractMemBlock should return empty for a missing key")
	}
}

func TestIndexMemBlocks(t *testing.T) {
	doc := "# Memory\n\n## beta\n**Pattern:** B\n**Criticality:** important\n**Applies-To:** tags=go; phases=execute\n\n## alpha\n**Pattern:** A\n**Criticality:** critical\n**Applies-To:** tags=core\n"
	blocks, err := IndexMemBlocks(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || blocks[0].Key != "alpha" || blocks[1].Key != "beta" {
		t.Fatalf("blocks = %+v", blocks)
	}
	if blocks[0].Criticality != "critical" || blocks[0].Digest == "" || blocks[1].AppliesTo != "tags=go; phases=execute" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestSpecMemoryPath(t *testing.T) {
	root := "/tmp/proj"
	if got := SpecMemoryPath(root, "demo"); got != filepath.Join(root, ".specd/specs/demo/memory.md") {
		t.Fatalf("SpecMemoryPath = %q", got)
	}
	if got := SteeringMemoryPath(root); got != filepath.Join(root, ".specd/steering/memory.md") {
		t.Fatalf("SteeringMemoryPath = %q", got)
	}
}

func TestListSpecs(t *testing.T) {
	root := t.TempDir()
	if got := ListSpecs(root); got != nil {
		t.Fatalf("missing specs dir should yield nil, got %v", got)
	}
	for _, s := range []string{"beta", "alpha"} {
		if err := os.MkdirAll(filepath.Join(root, ".specd/specs", s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := ListSpecs(root)
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("ListSpecs = %v, want [alpha beta] sorted", got)
	}
}

func TestPromotionThreshold(t *testing.T) {
	if DefaultConfig.PromotionThreshold != 3 {
		t.Fatalf("default PromotionThreshold = %d, want 3", DefaultConfig.PromotionThreshold)
	}
	// Env cascade override.
	cfg, diags := LoadConfig(ConfigPaths{}, map[string]string{"SPECD_PROMOTION_THRESHOLD": "5"})
	if len(diags) != 0 || cfg.PromotionThreshold != 5 {
		t.Fatalf("override failed: threshold=%d diags=%v", cfg.PromotionThreshold, diags)
	}
	// Invalid value keeps default and reports a diagnostic.
	cfg, diags = LoadConfig(ConfigPaths{}, map[string]string{"SPECD_PROMOTION_THRESHOLD": "0"})
	if cfg.PromotionThreshold != 3 || len(diags) == 0 {
		t.Fatalf("threshold 0 should be rejected: threshold=%d diags=%v", cfg.PromotionThreshold, diags)
	}
}
