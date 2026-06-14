package cmd_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// bootedRepo returns a harness with a Go repo that has been init'd and booted,
// so boot.json exists and the enrich brief is buildable.
func bootedRepo(t *testing.T) *th.Harness {
	t.Helper()
	h := th.New(t)
	h.Init()
	os.WriteFile(h.Path("go.mod"), []byte("module x\ngo 1.22\n"), 0o644)
	h.RunExpect(core.ExitOK, "boot")
	return h
}

// applyTarget writes content to a temp file and applies it to a target.
func applyTarget(t *testing.T, h *th.Harness, target, content string) {
	t.Helper()
	f := h.Path(target + "-section.md")
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	h.RunExpect(core.ExitOK, "enrich", "apply", "--target", target, "--content-file", f)
}

func TestEnrich_PlanRequiresBoot(t *testing.T) {
	h := th.New(t)
	h.Init()
	// No boot.json yet → plan cannot build a brief.
	h.RunExpect(core.ExitNotFound, "enrich", "plan")
}

func TestEnrich_PlanBrief(t *testing.T) {
	h := bootedRepo(t)

	res := h.RunExpect(core.ExitOK, "enrich", "plan", "--json")
	var brief core.EnrichBrief
	if err := json.Unmarshal([]byte(res.Stdout), &brief); err != nil {
		t.Fatalf("invalid brief JSON: %v\n%s", err, res.Stdout)
	}
	if len(brief.Targets) != 3 {
		t.Fatalf("want 3 targets, got %d", len(brief.Targets))
	}
	for _, tg := range brief.Targets {
		if tg.State != "stub" {
			t.Fatalf("target %s state = %q, want stub", tg.Target, tg.State)
		}
	}
	// Evidence should cite the go.mod manifest recorded by boot.
	found := false
	for _, e := range brief.Evidence {
		if e.Path == "go.mod" {
			found = true
		}
	}
	if !found {
		t.Fatalf("brief missing go.mod evidence: %+v", brief.Evidence)
	}
}

func TestEnrich_PlanDeterministic(t *testing.T) {
	h := bootedRepo(t)
	a := h.RunExpect(core.ExitOK, "enrich", "plan", "--json")
	b := h.RunExpect(core.ExitOK, "enrich", "plan", "--json")
	if a.Stdout != b.Stdout {
		t.Fatalf("brief not deterministic:\n--- a ---\n%s\n--- b ---\n%s", a.Stdout, b.Stdout)
	}
}

func TestEnrich_ApplyAndGate(t *testing.T) {
	h := bootedRepo(t)

	// Status before any enrichment: not-found (no enrich.json yet).
	h.RunExpect(core.ExitNotFound, "enrich", "status")

	applyTarget(t, h, "product", "## Product\n\nA test app.\n")

	// Managed block landed in the file.
	prod, _ := os.ReadFile(h.Path(".specd/steering/product.md"))
	if !strings.Contains(string(prod), "SPECD ENRICH: BEGIN") {
		t.Fatalf("product.md missing ENRICH block:\n%s", prod)
	}
	if !strings.Contains(string(prod), "A test app.") {
		t.Fatalf("product.md missing authored content")
	}
	// Sidecar recorded the write.
	if !core.FileExists(h.Path(".specd/enrich.json")) {
		t.Fatal("enrich.json not written")
	}

	// Gate is still stale — structure + tech remain stubs.
	h.RunExpect(core.ExitGate, "enrich", "status")

	applyTarget(t, h, "structure", "## Layout\n\nFlat.\n")
	applyTarget(t, h, "tech", "## Conventions\n\ngofmt.\n")

	// All three enriched → gate green, via both status and check --enrich.
	h.RunExpect(core.ExitOK, "enrich", "status")
	res := h.RunExpect(core.ExitOK, "check", "--enrich")
	if !strings.Contains(res.Stdout, "enrich-freshness") {
		t.Fatalf("missing gate output:\n%s", res.Stdout)
	}
}

func TestEnrich_Idempotent(t *testing.T) {
	h := bootedRepo(t)
	applyTarget(t, h, "product", "## Product\n\nFirst.\n")
	first, _ := os.ReadFile(h.Path(".specd/steering/product.md"))
	applyTarget(t, h, "product", "## Product\n\nFirst.\n")
	second, _ := os.ReadFile(h.Path(".specd/steering/product.md"))
	if string(first) != string(second) {
		t.Fatalf("apply not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestEnrich_ApplyEmptyRejected(t *testing.T) {
	h := bootedRepo(t)
	f := h.Path("empty.md")
	os.WriteFile(f, []byte("   \n"), 0o644)
	h.RunExpect(core.ExitUsage, "enrich", "apply", "--target", "product", "--content-file", f)
}

func TestEnrich_ApplyUnknownTarget(t *testing.T) {
	h := bootedRepo(t)
	f := h.Path("x.md")
	os.WriteFile(f, []byte("## X\n"), 0o644)
	h.RunExpect(core.ExitUsage, "enrich", "apply", "--target", "bogus", "--content-file", f)
}

func TestEnrich_StaleOnBootDrift(t *testing.T) {
	h := bootedRepo(t)
	applyTarget(t, h, "product", "## Product\n\nx.\n")
	applyTarget(t, h, "structure", "## Layout\n\nx.\n")
	applyTarget(t, h, "tech", "## Conventions\n\nx.\n")
	h.RunExpect(core.ExitOK, "enrich", "status")

	// Change detection and regenerate boot.json → recorded boot hash drifts.
	os.WriteFile(h.Path("package.json"), []byte(`{"name":"w"}`), 0o644)
	h.RunExpect(core.ExitOK, "boot", "--force")

	res := h.RunExpect(core.ExitGate, "check", "--enrich")
	if !strings.Contains(res.Out(), "drift") {
		t.Fatalf("expected drift issue:\n%s", res.Out())
	}
}

func TestEnrich_StaleOnMissingSource(t *testing.T) {
	h := bootedRepo(t)
	applyTarget(t, h, "product", "## Product\n\nx.\n")
	applyTarget(t, h, "structure", "## Layout\n\nx.\n")
	applyTarget(t, h, "tech", "## Conventions\n\nx.\n")
	h.RunExpect(core.ExitOK, "enrich", "status")

	// Remove a recorded evidence source.
	os.Remove(h.Path("go.mod"))
	h.RunExpect(core.ExitGate, "enrich", "status")
}
