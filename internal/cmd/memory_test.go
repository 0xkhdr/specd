package cmd

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// TestMemoryPromoteFlywheel exercises the add→promote flywheel: promotion is a
// pure count of on-disk specs, gated by the threshold, overridable with --force,
// with byte-deterministic provenance from an injected clock (RM.3–RM.7).
func TestMemoryPromoteFlywheel(t *testing.T) {
	root := t.TempDir()

	fixed := time.Date(2020, 1, 2, 15, 4, 5, 0, time.UTC)
	orig := memoryNow
	memoryNow = func() time.Time { return fixed }
	t.Cleanup(func() { memoryNow = orig })

	add := func(slug string) {
		t.Helper()
		if err := runNew(root, []string{slug}, nil); err != nil {
			t.Fatalf("new %s: %v", slug, err)
		}
		flags := map[string]string{"key": "kx", "pattern": "p", "body": "b", "source": "s", "criticality": "minor"}
		if err := runMemory(root, []string{slug, "add"}, flags); err != nil {
			t.Fatalf("add %s: %v", slug, err)
		}
	}
	add("alpha")
	add("beta")

	steering := core.SteeringMemoryPath(root)

	// Below threshold (2 < default 3): refuse, and write nothing.
	err := runMemory(root, []string{"alpha", "promote"}, map[string]string{"key": "kx"})
	if err == nil || !strings.Contains(err.Error(), "seen in 2 spec(s)") || !strings.Contains(err.Error(), "threshold is 3") {
		t.Fatalf("below-threshold promote should report count+threshold, got %v", err)
	}
	if core.ReadOrNull(steering) != nil {
		t.Fatal("refused promotion must not touch steering store")
	}

	// Missing key fails loud (RM.5).
	if err := runMemory(root, []string{"alpha", "promote"}, map[string]string{"key": "nope"}); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing-key promote should fail loud, got %v", err)
	}

	// At threshold: promote appends block + deterministic provenance.
	add("gamma")
	if err := runMemory(root, []string{"alpha", "promote"}, map[string]string{"key": "kx"}); err != nil {
		t.Fatalf("at-threshold promote: %v", err)
	}
	got := readFile(t, steering)
	want := "\n## kx\n**Pattern:** p\n**Detail:** b\n**Source:** s\n**Criticality:** minor\n**Related:** —\n**Promoted:** from spec 'alpha' on 2020-01-02 (seen in 3 spec(s))\n"
	if got != want {
		t.Fatalf("promotion output not byte-stable:\n got %q\nwant %q", got, want)
	}

	// --force promotes past the threshold for a fresh, single-spec pattern.
	force := map[string]string{"key": "ky", "pattern": "p", "body": "b", "source": "s", "criticality": "minor"}
	if err := runMemory(root, []string{"alpha", "add"}, force); err != nil {
		t.Fatalf("add ky: %v", err)
	}
	if err := runMemory(root, []string{"alpha", "promote"}, map[string]string{"key": "ky"}); err == nil {
		t.Fatal("ky seen in 1 spec should refuse without --force")
	}
	if err := runMemory(root, []string{"alpha", "promote"}, map[string]string{"key": "ky", "force": "true"}); err != nil {
		t.Fatalf("--force promote: %v", err)
	}
	if !strings.Contains(readFile(t, steering), "## ky") {
		t.Fatal("--force promotion should append the block")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}
