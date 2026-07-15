package core

import (
	"encoding/json"
	"fmt"
)

// `specd migrate` (V12/P6.4) is the documented, idempotent one-shot for moving a
// v0.1.x project onto v0.2.0. State migration itself is already silent and
// shape-compatible (a v5 state.json migrates to v6 on first load — see migrate()
// in state.go); this command makes that explicit and durable by rewriting each
// spec's state at the current SchemaVersion, and reports which additive v0.2.0
// policy blocks are available to adopt. It never writes policy content: adopting
// guardrails/routing/eval/review stays an explicit operator action, so a migrated
// repo keeps the new gates default-off (invariant 9).

// SpecMigration records one spec's state migration outcome.
type SpecMigration struct {
	Slug        string `json:"slug"`
	FromVersion int    `json:"fromVersion"`
	ToVersion   int    `json:"toVersion"`
	Migrated    bool   `json:"migrated"`
}

// ConfigHint names an additive v0.2.0 policy block that is available but not yet
// adopted, with the command that would adopt it. Reporting only — never applied.
type ConfigHint struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	Adopt   string `json:"adopt"`
}

// MigrateReport is the deterministic result of `specd migrate`.
type MigrateReport struct {
	SchemaVersion int             `json:"schemaVersion"`
	Specs         []SpecMigration `json:"specs"`
	Hints         []ConfigHint    `json:"hints"`
}

// onDiskSchemaVersion reads just the schemaVersion field of a spec's state.json
// without running a migration. A missing/blank version reads as 1 (the original
// schema); a missing file reports (0, false).
func onDiskSchemaVersion(root, slug string) (int, bool) {
	raw := ReadOrNull(statePath(root, slug))
	if raw == nil {
		return 0, false
	}
	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal([]byte(*raw), &probe); err != nil {
		return 0, true // present but unreadable version; let the migrate attempt surface the error
	}
	if probe.SchemaVersion == 0 {
		return 1, true
	}
	return probe.SchemaVersion, true
}

// MigrateProject migrates every spec's state to the current SchemaVersion and
// reports available config blocks. It is idempotent: a second run finds every
// spec already current and rewrites nothing. Each state rewrite takes the spec
// lock and goes through SaveState, so a concurrent writer is detected rather than
// clobbered.
func MigrateProject(root string) (MigrateReport, error) {
	rep := MigrateReport{SchemaVersion: SchemaVersion}
	for _, slug := range ListSpecs(root) {
		from, ok := onDiskSchemaVersion(root, slug)
		if !ok {
			continue
		}
		sm := SpecMigration{Slug: slug, FromVersion: from, ToVersion: SchemaVersion}
		if from < SchemaVersion {
			if _, err := WithSpecLock(root, slug, func() (struct{}, error) {
				st, err := LoadState(root, slug)
				if err != nil {
					return struct{}{}, err
				}
				if st == nil {
					return struct{}{}, nil
				}
				return struct{}{}, SaveState(root, slug, st)
			}); err != nil {
				return MigrateReport{}, GateError(fmt.Sprintf("migrate %s: %v", slug, err))
			}
			sm.Migrated = true
		}
		rep.Specs = append(rep.Specs, sm)
	}
	rep.Hints = migrateConfigHints(root)
	return rep, nil
}

// migrateConfigHints reports the additive v0.2.0 policy blocks and whether each
// is present. These are informational: `specd migrate` never writes policy.
func migrateConfigHints(root string) []ConfigHint {
	specd := func(rel string) string { return root + "/.specd/" + rel }
	return []ConfigHint{
		{Name: "guardrails", Present: FileExists(specd("guardrails.json")), Adopt: "specd init --guardrails"},
		{Name: "routing", Present: FileExists(specd("routing.json")), Adopt: "author .specd/routing.json (see docs/validation-gates.md)"},
		{Name: "eval-gate", Present: FileExists(specd("config.yml")) || FileExists(specd("config.yaml")), Adopt: "set gates in .specd/config.yml (default-off for migrated repos)"},
		{Name: "review-gate", Present: FileExists(specd("config.yml")) || FileExists(specd("config.yaml")), Adopt: "set gates in .specd/config.yml (default-off for migrated repos)"},
	}
}
