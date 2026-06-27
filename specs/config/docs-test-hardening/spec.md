# Spec — Documentation and Test Hardening

**Priority:** P1 · **Wave:** 3 · **Domain:** release confidence, docs, and regression protection.

## Introduction

Config architecture changes touch bootstrap, policy, doctor, migration, and many tests. This spec ensures the implementation is documented and guarded by regression tests so future changes do not break YAML, legacy JSON, cascade precedence, or machine JSON state invariants.

## Requirements

### Requirement 1 — User documentation
**Acceptance criteria:**
1. `docs/command-reference.md` SHALL document config file locations, lookup order, schema versions, env precedence, and `specd migrate config`.
2. `docs/user-guide.md` SHALL include a "Global vs Project Config" section with YAML examples.
3. Documentation SHALL state that state/runtime files remain JSON.
4. Docs SHALL include a migration path from `.specd/config.json`.

### Requirement 2 — Contributor documentation
**Acceptance criteria:**
1. `docs/contributor-guide.md` SHALL explain config loader architecture, schema translation, merge semantics, and validation strategy.
2. Security notes SHALL cover untrusted config values, env precedence, and secret-bearing orchestration rejection.
3. Comments in code SHALL identify compatibility wrappers and deprecation paths.

### Requirement 3 — Compatibility and round-trip tests
**Acceptance criteria:**
1. YAML samples from docs SHALL parse successfully in tests.
2. Equivalent v1 JSON and v2 YAML configs SHALL produce identical effective runtime behavior.
3. Migration output SHALL be deterministic and parse-validated.
4. Existing configs using resilience, MCP exposure, custom gates, and orchestration fields SHALL remain supported.

### Requirement 4 — End-to-end init/doctor/migrate coverage
**Acceptance criteria:**
1. Fresh init E2E SHALL create project YAML and, in isolated HOME/XDG, global YAML.
2. Doctor SHALL pass for fresh YAML projects and report invalid global/project config.
3. Migrate E2E SHALL convert legacy project JSON and leave a backup.
4. JSON command output SHALL remain ANSI-free and schema-stable.

### Requirement 5 — CI and quality gates
**Acceptance criteria:**
1. `make test` SHALL pass with race detector expectations intact.
2. New tests SHALL avoid dependence on real user HOME/XDG.
3. Coverage SHALL not regress below project policy.
4. Full `make ci` SHOULD pass before the program is marked complete.

## Design

- Add focused unit tests beside config loader/merge/schema code, plus command tests for init/doctor/migrate.
- Prefer table-driven tests with isolated temp roots and `t.Setenv`.
- Reuse documentation examples as test fixtures where practical.
- Keep docs concise but explicit about precedence and compatibility.

## Out of scope

- Implementing loader/migration behavior directly; this spec validates and documents it.
- Removing legacy JSON docs; legacy support remains documented as deprecated but supported.

## Risks

- **Docs drift:** Test YAML snippets or maintain fixtures mirrored from docs.
- **Real HOME pollution:** All tests must isolate environment and never write to a developer's actual config directory.
