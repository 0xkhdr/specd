# Spec — Deterministic Config Migration Command

**Priority:** P1 · **Wave:** 2 · **Domain:** legacy compatibility and migration UX.

## Introduction

Existing users have `.specd/config.json`. The new architecture keeps JSON readable but should offer a safe command to convert legacy config into YAML, preserve a backup, and preview changes. This spec defines `specd migrate config` and its registration/docs/test expectations.

## Requirements

### Requirement 1 — Project config migration
**Acceptance criteria:**
1. `specd migrate config` SHALL read the active project legacy `.specd/config.json` when no project YAML exists.
2. It SHALL write `.specd/config.yml` in v2 YAML schema with deterministic ordering and stable formatting.
3. It SHALL rename `.specd/config.json` to `.specd/config.json.bak` only after the YAML file is durably written and parse-validated.
4. If `.specd/config.yml` already exists, the command SHALL fail closed unless `--force` is explicitly implemented and documented.

### Requirement 2 — Dry run
**Acceptance criteria:**
1. `specd migrate config --dry-run` SHALL print the YAML that would be written and SHALL NOT mutate the filesystem.
2. Dry-run output SHALL be deterministic and parseable as YAML except for any clearly separated explanatory text.
3. Dry-run SHALL return non-zero for invalid source JSON.

### Requirement 3 — Global migration
**Acceptance criteria:**
1. `specd migrate config --global` SHALL migrate a legacy global JSON config to the canonical global YAML path.
2. It SHALL use the same validation, write, and backup safety as project migration.
3. It SHALL not require a `.specd` project root when `--global` is supplied.

### Requirement 4 — Command registration and help
**Acceptance criteria:**
1. `migrate` SHALL be registered in `cmd.Registry` and command metadata.
2. Usage SHALL include `specd migrate config [--dry-run] [--global]`.
3. Invalid subcommands/flag combinations SHALL return usage exit code 2.
4. JSON output MAY be added, but if omitted, text output must remain automation-friendly.

### Requirement 5 — Safety and validation
**Acceptance criteria:**
1. Migration SHALL reject invalid/secret-bearing/unsupported config before writing YAML.
2. Migration SHALL preserve effective runtime behavior of the source JSON.
3. YAML output SHALL include helpful comments from the template where practical, but comments are secondary to deterministic correct config.
4. Backup collision (`config.json.bak` exists) SHALL fail closed unless a documented unique backup strategy is implemented.

## Design

- Add `internal/cmd/migrate.go` with subcommand dispatch for `config`.
- Put conversion/rendering logic in core so it is testable without CLI harness.
- Decode legacy JSON through the same loader used by runtime, then render v2 YAML using a deterministic field order.
- Validate rendered YAML by re-parsing and comparing effective runtime config to the source effective config.
- Use atomic write for YAML and atomic/checked rename for backup.

## Out of scope

- Automatically migrating on `specd init` or `specd doctor --fix`.
- Preserving arbitrary JSON comments (not possible for JSON); template comments are best effort.
- Removing legacy JSON support after migration.

## Risks

- **Behavior drift in conversion:** Always parse the rendered YAML and compare effective config.
- **Partial migration:** Write and fsync YAML before backup rename; fail loudly if backup cannot be created.
