package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestCompatibilityInventory exercises the registry, doctor projection, and
// derived workflow metrics against the five acceptance checks: stable ordering,
// read-only no-write reporting, secret/source redaction, migrated suppression,
// and unmet removal-window eligibility.
func TestCompatibilityInventory(t *testing.T) {
	t.Run("stable-order", func(t *testing.T) {
		facts := CompatFacts{
			LegacyConfigSource: "project.yml",
			LegacyStateSchema:  true,
			UnknownActors:      []string{"zeta", "alpha", "mid"},
		}
		first := CompatInventory(facts, "1.4.0", "2026-07-01")
		second := CompatInventory(facts, "1.4.0", "2026-07-01")
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("inventory not deterministic:\n%+v\n%+v", first, second)
		}
		for i := 1; i < len(first); i++ {
			a, b := first[i-1], first[i]
			if a.Code > b.Code || (a.Code == b.Code && a.Entity > b.Entity) {
				t.Fatalf("inventory not sorted by code+entity at %d: %q/%q then %q/%q", i, a.Code, a.Entity, b.Code, b.Entity)
			}
		}
	})

	t.Run("read-only-fs", func(t *testing.T) {
		root := t.TempDir()
		specDir := filepath.Join(root, ".specd", "specs", "demo")
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// A legacy (schema 1) state file is an active deprecated surface.
		if err := os.WriteFile(filepath.Join(specDir, "state.json"), []byte(`{"schema_version":1,"slug":"demo","revision":0}`), 0o444); err != nil {
			t.Fatal(err)
		}
		before := dirEntries(t, specDir)
		if err := os.Chmod(specDir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(specDir, 0o700) })

		facts := LoadCompatFacts(root, "demo")
		inv := CompatInventory(facts, "dev", "2026-07-01")
		if len(inv) != len(CompatRegistry()) {
			t.Fatalf("inventory dropped rows on read-only fs: got %d want %d", len(inv), len(CompatRegistry()))
		}
		if !hasActive(inv, "LEGACY_STATE_SCHEMA") {
			t.Fatalf("read-only inventory did not report the legacy state schema: %+v", inv)
		}
		if err := os.Chmod(specDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if after := dirEntries(t, specDir); !reflect.DeepEqual(before, after) {
			t.Fatalf("inventory wrote to disk: before %v after %v", before, after)
		}
	})

	t.Run("secret-redaction", func(t *testing.T) {
		const secret = "SECRET-TOKEN-abc123"
		events := []WorkflowEventV1{
			{Transition: "approve_requirements", Actor: "agent", Reason: secret},
			{Transition: "approve_requirements", Actor: "agent", Reason: secret},
			{Transition: "reopened", Actor: "agent", Reason: secret},
		}
		facts := CompatFacts{UnknownActors: []string{"agent-x"}}
		metrics := DeriveWorkflowMetrics(events, CompatInventory(facts, "1.4.0", "2026-07-01"))
		out := RenderWorkflowMetrics("demo", metrics)
		if strings.Contains(out, secret) {
			t.Fatalf("secret leaked into derived metrics:\n%s", out)
		}
		if metrics.ByTransition["approve_requirements"] != 2 || metrics.TransitionAttempts != 3 {
			t.Fatalf("metrics miscounted transitions: %+v", metrics)
		}
	})

	t.Run("migrated-suppression", func(t *testing.T) {
		facts := CompatFacts{
			LegacyStateSchema:  true,
			LegacyStatusWrites: true,
			Migrated:           map[string]bool{"LEGACY_STATE_SCHEMA": true},
		}
		inv := CompatInventory(facts, "1.4.0", "2026-07-01")
		if hasActive(inv, "LEGACY_STATE_SCHEMA") {
			t.Fatalf("migrated surface still reported active: %+v", inv)
		}
		if row := find(inv, "LEGACY_STATE_SCHEMA"); row == nil || !row.Migrated {
			t.Fatalf("migrated surface dropped from history: %+v", inv)
		}
		if !hasActive(inv, "LEGACY_STATUS_PROJECTION") {
			t.Fatalf("non-migrated active surface suppressed: %+v", inv)
		}
		doc := DoctorCompat(facts, "1.4.0", "2026-07-01")
		if doc.Healthy {
			t.Fatalf("doctor healthy despite an active surface: %+v", doc)
		}
		for _, f := range doc.Findings {
			if f.Code == "LEGACY_STATE_SCHEMA" {
				t.Fatalf("doctor surfaced a migrated code: %+v", doc.Findings)
			}
		}
	})

	t.Run("unmet-window", func(t *testing.T) {
		// An inactive surface is only removal-eligible once BOTH the version and
		// the date thresholds are met; either unmet keeps the route and names the
		// gate. Time alone never deletes support.
		belowVersion := find(CompatInventory(CompatFacts{}, "1.0.0", "2026-07-01"), "LEGACY_CONFIG_SOURCE")
		if belowVersion.RemovalEligible || belowVersion.UnmetGate != "unmet-window-version" {
			t.Fatalf("version gate not enforced: %+v", belowVersion)
		}
		beforeDate := find(CompatInventory(CompatFacts{}, "1.4.0", "2026-01-01"), "LEGACY_CONFIG_SOURCE")
		if beforeDate.RemovalEligible || beforeDate.UnmetGate != "unmet-window-date" {
			t.Fatalf("date gate not enforced: %+v", beforeDate)
		}
		met := find(CompatInventory(CompatFacts{}, "1.4.0", "2026-07-01"), "LEGACY_CONFIG_SOURCE")
		if !met.RemovalEligible || met.UnmetGate != "" {
			t.Fatalf("removal not eligible once window met: %+v", met)
		}
		// Active use always blocks removal regardless of the window.
		active := find(CompatInventory(CompatFacts{LegacyConfigSource: "project.yml"}, "9.9.9", "2030-01-01"), "LEGACY_CONFIG_SOURCE")
		if active.RemovalEligible || active.UnmetGate != "active-use" {
			t.Fatalf("active use did not block removal: %+v", active)
		}
	})
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func find(inv []CompatDiagnostic, code string) *CompatDiagnostic {
	for i := range inv {
		if inv[i].Code == code {
			return &inv[i]
		}
	}
	return nil
}

func hasActive(inv []CompatDiagnostic, code string) bool {
	for _, d := range inv {
		if d.Code == code && d.Active {
			return true
		}
	}
	return false
}
