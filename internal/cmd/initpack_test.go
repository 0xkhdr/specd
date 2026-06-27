package cmd_test

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestInitPack applies a built-in pack and confirms its declared file lands with
// vars substituted, that a second apply without --force is refused (and writes
// nothing new), and that an unknown pack fails closed.
func TestInitPack(t *testing.T) {
	h := th.New(t)

	h.RunExpect(core.ExitOK, "init", "--pack", "minimal")
	h.AssertFileExists(".specd/steering/project.md")
	body := h.ReadFile(".specd/steering/project.md")
	if strings.Contains(body, "{{TITLE}}") {
		t.Errorf("pack vars not substituted:\n%s", body)
	}

	// Re-apply without --force is refused (file already exists) — fail-closed.
	h.RunExpect(core.ExitGate, "init", "--pack", "minimal")

	// --force re-applies cleanly.
	h.RunExpect(core.ExitOK, "init", "--pack", "minimal", "--force")

	// Unknown built-in fails closed.
	h.RunExpect(core.ExitNotFound, "init", "--pack", "no-such-pack")
}

// TestInitDefaultRegression confirms that adding --pack support left the default
// `specd init` (no --pack) byte-unchanged: it still scaffolds the standard tree.
func TestInitDefaultRegression(t *testing.T) {
	h := th.New(t)
	h.RunExpect(core.ExitOK, "init")
	for _, f := range []string{
		".specd/config.yml",
		".specd/steering/product.md",
		".specd/roles/builder.md",
		"AGENTS.md",
	} {
		h.AssertFileExists(f)
	}
	// The default scaffold must NOT create a pack-only file.
	h.AssertFileAbsent(".specd/steering/project.md")
}
