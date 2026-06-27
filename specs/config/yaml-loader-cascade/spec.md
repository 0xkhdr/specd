# Spec — YAML Loader and Config Cascade

**Priority:** P0 · **Wave:** 1 · **Domain:** configuration loading foundation.

## Introduction

specd currently reads one project-scoped JSON file at `.specd/config.json` through `ConfigPath(root)` and `LoadConfig(root)`. Missing or invalid config falls back to `DefaultConfig`; `LoadConfigStrict` adds diagnostics for the JSON path only. The action plan requires a format-agnostic loader with YAML-first project config, legacy JSON compatibility, global user defaults, and source-aware diagnostics.

This spec creates the loading substrate without requiring every caller to understand YAML, XDG paths, or merge rules.

## Current-state grounding

- `internal/core/specfiles.go` owns `Config`, `DefaultConfig`, and permissive `LoadConfig(root) Config`.
- `internal/core/config_validate.go` owns `LoadConfigStrict(root)` for `.specd/config.json` diagnostics.
- `internal/core/paths.go` exposes `ConfigPath(root)` hardcoded to `.specd/config.json`.
- `internal/core/fusion.go` computes config digests using `ConfigPath(root)`.
- Many tests and commands write `core.ConfigPath(root)` directly.

## Requirements

### Requirement 1 — Format-aware config file loading
**User story:** As a user, I want specd to read YAML config by default while continuing to accept existing JSON config.

**Acceptance criteria:**
1. THE SYSTEM SHALL load `.yml` and `.yaml` config files as YAML and `.json` config files as JSON.
2. THE SYSTEM SHALL reject unsupported config extensions with diagnostics instead of guessing.
3. THE SYSTEM SHALL retain legacy `.specd/config.json` support indefinitely.
4. THE SYSTEM SHALL keep machine-owned files such as `state.json`, `program.json`, and runtime session files on the existing JSON paths.

### Requirement 2 — Project path candidates
**User story:** As a maintainer, I want one path resolver that defines config lookup order.

**Acceptance criteria:**
1. THE SYSTEM SHALL add `ConfigPaths(root) []string` in priority order: `.specd/config.yml`, `.specd/config.yaml`, `.specd/config.json`.
2. THE SYSTEM SHALL preserve a legacy helper for code/tests needing the JSON path, e.g. `LegacyConfigPath(root)`.
3. THE SYSTEM SHOULD keep `ConfigPath(root)` temporarily as an alias or deprecated wrapper if needed to reduce breakage.
4. If more than one project config exists, THE SYSTEM SHALL choose the highest-priority file and report lower-priority files as ignored/deprecated diagnostics.

### Requirement 3 — Global config path candidates
**User story:** As a user with multiple projects, I want shared defaults in a user-level config.

**Acceptance criteria:**
1. THE SYSTEM SHALL add `GlobalConfigPaths() []string` using `os.UserConfigDir()`/`XDG_CONFIG_HOME` and fallbacks.
2. Lookup order SHALL include `~/.config/specd/config.yml`, `~/.config/specd/config.yaml`, and `~/.specd.yml`; legacy global JSON MAY be included for migration/doctor warnings.
3. Path resolution SHALL not create directories or files; creation belongs to init/scaffold specs.
4. Errors discovering the home/config directory SHALL produce warnings and continue with embedded defaults.

### Requirement 4 — Cascading merge
**User story:** As a user, I want project settings to override global defaults without duplicating the full config.

**Acceptance criteria:**
1. THE SYSTEM SHALL load in this order: embedded defaults, first global config candidate, first project config candidate.
2. THE SYSTEM SHALL deep-merge config where project values override global values and absent project values preserve global values.
3. Explicit false/zero values SHALL be treated as present and SHALL override lower layers.
4. Lists SHALL use replace semantics, not append semantics, unless a field explicitly documents additive behavior.
5. The effective config returned to existing callers SHALL be fully populated with defaults.

### Requirement 5 — Source-aware strict diagnostics
**User story:** As an agent, I want diagnostics to say which config file and field produced a problem.

**Acceptance criteria:**
1. `LoadConfigStrict` or its replacement SHALL return diagnostics with source path, layer (`embedded`, `global`, `project`, `env` when applicable), severity, field path, and message.
2. Missing config SHALL be `info`/`defaulted`, not invalid.
3. Invalid syntax SHALL be fail-closed for strict loaders and permissively defaulted for legacy `LoadConfig` behavior.
4. Fusion policy and doctor config checks SHALL use effective digest/source information instead of assuming one JSON path.

## Design

- Add `internal/core/config_loader.go` for format detection, per-file decode, candidate selection, cascade loading, and effective source metadata.
- Add `internal/core/config_merge.go` with presence-aware merge helpers. Avoid using zero values alone to infer absence.
- Add YAML support through `gopkg.in/yaml.v3` only if the dependency policy is approved; otherwise implement the limited YAML subset needed by the shipped schema.
- Keep `LoadConfig(root) Config` as a compatibility wrapper around the new permissive cascade loader.
- Add a richer loader such as `LoadConfigWithDiagnostics(root) (Config, ConfigLoadResult)` and make `LoadConfigStrict(root)` validate the merged result plus individual file syntax.
- Update digest helpers to digest the selected project/global files or expose a deterministic effective config digest.

## Out of scope

- Renaming the schema fields; that belongs to `schema-v2-namespacing`.
- Writing global or project config files; that belongs to `scaffold-global-init`.
- CLI migration commands; that belongs to `migrate-config`.

## Risks

- **Dependency conflict with project goals:** The root `AGENTS.md` says runtime dependencies are stdlib-only. Resolve before adding `yaml.v3`.
- **Silent precedence bugs:** Use table-driven tests for absent vs explicit zero/false.
- **Caller churn:** Preserve wrappers and update call sites incrementally.
