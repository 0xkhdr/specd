# Config Program — Progress & Wave Plan

Source plan: [`specd-config-analysis-action-plan.md`](../../specd-config-analysis-action-plan.md)

This program migrates specd from a single project-scoped JSON config to a dual-layer, format-agnostic configuration system: embedded defaults → global YAML → project YAML/legacy JSON → environment overrides. Human-authored config becomes YAML by default while machine state remains JSON.

Each child spec has its own `spec.md` (analysis, requirements, design) and `tasks.md` (wave DAG for a coding agent).

## Spec map

| Plan phase / focus | Title | Spec | Priority |
|---|---|---|---|
| Phase 1 + 3 loader foundation | YAML Loader and Config Cascade | [yaml-loader-cascade](yaml-loader-cascade/spec.md) | P0 |
| Phase 2 schema refactor | V2 Schema Namespacing and Compatibility | [schema-v2-namespacing](schema-v2-namespacing/spec.md) | P0 |
| Phase 4 + init global config | Scaffold and Global Init Migration | [scaffold-global-init](scaffold-global-init/spec.md) | P0 |
| Phase 5 migration command | Deterministic Config Migration Command | [migrate-config](migrate-config/spec.md) | P1 |
| Phase 6 env alignment | Environment Precedence and Format Control | [env-precedence](env-precedence/spec.md) | P1 |
| Phase 7 docs/tests/hardening | Documentation and Test Hardening | [docs-test-hardening](docs-test-hardening/spec.md) | P1 |

## Program waves

### Wave 1 — Safe parser foundation (P0) — **status: complete**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| yaml-loader-cascade | complete | — | Added format detection, project/global path candidates, cascade merge, and source-aware diagnostics foundation. |
| schema-v2-namespacing | complete | yaml-loader-cascade/T1-T3 | Added custom v2 YAML snake_case/defaults mapping while preserving v1 JSON callers. |

### Wave 2 — User-facing defaults and migration (P0/P1) — **status: complete**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| scaffold-global-init | complete | yaml-loader-cascade; schema-v2-namespacing | New projects scaffold `.specd/config.yml`; first init creates global config when absent. |
| migrate-config | complete | yaml-loader-cascade; schema-v2-namespacing; scaffold-global-init/T1 | Safe deterministic conversion from legacy JSON to YAML with dry-run and global mode. |

### Wave 3 — Precedence, docs, and regression net (P1) — **status: complete**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| env-precedence | complete | yaml-loader-cascade; schema-v2-namespacing | Applied supported config `SPECD_*` vars after merge with strict diagnostics, effective validation, `SPECD_CONFIG_FORMAT`, and precedence tests. |
| docs-test-hardening | complete | all Wave 1/2 tasks; env-precedence | Updated user/contributor/security docs, added docs parse/compat/migration/init-doctor-migrate/machine JSON invariant tests, and passed `make ci`. |

## Acceptance checklist

- [x] `specd init` scaffolds `.specd/config.yml` for new projects.
- [x] First `specd init` creates `~/.config/specd/config.yml` when no global config exists.
- [x] Existing `.specd/config.json` continues to parse and produce compatible runtime behavior.
- [x] Project config overrides global config on a field-by-field basis.
- [x] `specd migrate config` converts `.specd/config.json` to `.specd/config.yml` deterministically and backs up the JSON file.
- [x] `specd doctor` reports global and project config health.
- [x] `SPECD_*` env vars override merged config values.
- [x] YAML config supports comments and receives strict validation diagnostics.
- [x] `state.json`, `program.json`, `session.json`, and integration/runtime state remain JSON.
- [x] Docs describe locations, schema, migration, precedence, and env vars.

## Status legend

`not-started` → `in-progress` → `verifying` → `complete` / `blocked`

## Tracking guidance

Each child `tasks.md` owns implementation checkboxes. Mark progress through the project workflow/CLI, not by manually flipping task status in machine-owned state. Update the wave tables above as specs advance.

## Open program-level decisions

- **Dependency policy:** The action plan proposes `gopkg.in/yaml.v3`; confirm the repo's zero-runtime-dependencies stance is intentionally relaxed or implement a minimal internal YAML subset parser. Prefer an explicit architecture decision before adding the dependency.
- **Public API compatibility:** Existing `LoadConfig(root) Config` has many callers. If introducing diagnostics requires a new signature, provide wrappers (`LoadConfig`, `LoadConfigWithDiagnostics`, `LoadConfigStrict`) to avoid churn and preserve behavior.
- **Absent vs zero:** The cascade must distinguish absent fields from explicit zero/false values. Use pointer-backed partial structs or presence maps for every mergeable field.
