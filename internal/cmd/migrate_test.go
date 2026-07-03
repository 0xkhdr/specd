package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestMigrateUpgradeE2E simulates a v0.1.x-era project (a spec whose state.json
// predates the current schema), runs `specd migrate`, and confirms a
// representative set of v0.2.0 commands run correctly afterwards.
func TestMigrateUpgradeE2E(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	// Downgrade the on-disk schema to look like an older release.
	statePath := h.Path(".specd/specs/auth/state.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	downgraded := strings.Replace(string(raw), `"schemaVersion": 6`, `"schemaVersion": 5`, 1)
	if downgraded == string(raw) {
		t.Skip("current schema is not 6; adjust fixture")
	}
	if err := os.WriteFile(statePath, []byte(downgraded), 0o644); err != nil {
		t.Fatal(err)
	}

	res := h.RunExpect(core.ExitOK, "migrate")
	if !strings.Contains(res.Stdout, "auth") || !strings.Contains(res.Stdout, "config blocks") {
		t.Fatalf("migrate output unexpected:\n%s", res.Stdout)
	}

	// Idempotent second run.
	h.RunExpect(core.ExitOK, "migrate")

	// A sampling of v0.2.0 commands must run cleanly against the migrated repo.
	h.RunExpect(core.ExitOK, "status", "auth")
	h.RunExpect(core.ExitOK, "waves", "auth")
	h.RunExpect(core.ExitNotFound, "harness", "list") // no bundle yet, clean exit 3
}

// TestMigrateJSON confirms the machine-readable report shape.
func TestMigrateJSON(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	res := h.RunExpect(core.ExitOK, "migrate", "--json")
	if !strings.Contains(res.Stdout, `"schemaVersion"`) || !strings.Contains(res.Stdout, `"hints"`) {
		t.Fatalf("migrate --json missing fields:\n%s", res.Stdout)
	}
}
