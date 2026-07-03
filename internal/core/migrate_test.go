package core

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// downgradeStateSchema rewrites a spec's on-disk schemaVersion to simulate a
// v0.1.x-era state.json that predates the current schema.
func downgradeStateSchema(t *testing.T, root, slug string, to int) {
	t.Helper()
	p := statePath(root, slug)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// The seeded state is written at the current SchemaVersion; rewrite that one
	// field textually so the file is otherwise identical to a real old state.
	old := `"schemaVersion": ` + strconv.Itoa(SchemaVersion)
	next := `"schemaVersion": ` + strconv.Itoa(to)
	s := strings.Replace(string(raw), old, next, 1)
	if s == string(raw) {
		t.Fatalf("could not find %q to downgrade", old)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateProjectBumpsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	seedDashboardSpec(t, root, "billing", nil)
	downgradeStateSchema(t, root, "billing", 5)

	rep, err := MigrateProject(root)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if rep.SchemaVersion != SchemaVersion {
		t.Fatalf("report schema = %d, want %d", rep.SchemaVersion, SchemaVersion)
	}
	if len(rep.Specs) != 1 || !rep.Specs[0].Migrated || rep.Specs[0].FromVersion != 5 || rep.Specs[0].ToVersion != SchemaVersion {
		t.Fatalf("migration record wrong: %+v", rep.Specs)
	}
	// The state is now persisted at the current version.
	if v, _ := onDiskSchemaVersion(root, "billing"); v != SchemaVersion {
		t.Fatalf("on-disk schema = %d, want %d", v, SchemaVersion)
	}

	// Second run is a no-op: nothing migrated.
	rep2, err := MigrateProject(root)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if rep2.Specs[0].Migrated {
		t.Fatal("second migrate re-migrated an already-current spec")
	}
}

func TestMigrateProjectReportsConfigHints(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".specd/guardrails.json", `{"rules":[]}`)

	rep, err := MigrateProject(root)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]ConfigHint{}
	for _, h := range rep.Hints {
		byName[h.Name] = h
	}
	if !byName["guardrails"].Present {
		t.Fatal("guardrails hint should report present")
	}
	if byName["routing"].Present {
		t.Fatal("routing hint should report absent")
	}
	// Adopt strings are advisory and non-empty.
	for _, h := range rep.Hints {
		if strings.TrimSpace(h.Adopt) == "" {
			t.Fatalf("hint %q missing adopt guidance", h.Name)
		}
	}
}
