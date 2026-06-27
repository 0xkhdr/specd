# Spec — Environment Precedence and Format Control

**Priority:** P1 · **Wave:** 3 · **Domain:** config override semantics.

## Introduction

The new cascade must preserve current `SPECD_*` environment override behavior and make precedence explicit: environment > project config > global config > embedded defaults. Overrides must apply after YAML/JSON parsing and deep merge, and diagnostics must make environment influence visible.

## Requirements

### Requirement 1 — Environment overrides after merge
**Acceptance criteria:**
1. THE SYSTEM SHALL apply all supported `SPECD_*` environment overrides after embedded/global/project config merge.
2. Env overrides SHALL win over explicit project and global values.
3. Existing env vars such as `SPECD_JSON`, verify timeout variables, and config-related orchestration vars SHALL retain behavior.
4. Env integer parsing SHALL continue to use `core.EnvInt` or equivalent clamp-and-warn behavior.

### Requirement 2 — Documented precedence diagnostics
**Acceptance criteria:**
1. Strict loader diagnostics/policy output SHALL be able to report that an effective value was overridden by env.
2. Doctor/fusion policy SHALL communicate the effective precedence ladder without leaking sensitive env values.
3. Env override diagnostics SHALL include the env var name and target field path.

### Requirement 3 — Optional format preference
**Acceptance criteria:**
1. `SPECD_CONFIG_FORMAT=yaml|json` MAY force preferred config selection for debugging if implemented.
2. Invalid values SHALL produce a warning/diagnostic and SHALL NOT silently select an unsafe path.
3. Format preference SHALL never change machine-owned JSON state file formats.
4. If omitted from implementation, docs/tests SHALL explicitly mark it unsupported rather than implying support.

### Requirement 4 — Security boundaries
**Acceptance criteria:**
1. Env vars SHALL NOT allow secret-bearing orchestration config that file validation would reject.
2. Verify command execution environment scrubbing SHALL remain unchanged.
3. No broad environment dump SHALL be added to JSON outputs.

## Design

- Centralize env override application in config loading, after cascade merge.
- Represent env overrides as a final config layer with source metadata.
- Keep existing command-specific env behavior if it is not truly config, but document it separately.
- Tests should set env with `t.Setenv` and isolated roots/HOME to avoid order dependence.

## Out of scope

- Adding new functional env vars beyond optional `SPECD_CONFIG_FORMAT`.
- Persisting env overrides into config files.

## Risks

- **Hidden order dependence:** Use table-driven precedence tests and `t.Setenv`.
- **Accidental secret exposure:** Diagnostics include var names and paths, not raw values unless already non-sensitive and necessary.
