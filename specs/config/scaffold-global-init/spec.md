# Spec — Scaffold and Global Init Migration

**Priority:** P0 · **Wave:** 2 · **Domain:** project bootstrap and global defaults.

## Introduction

New specd projects should be initialized with YAML project config, and users should get a commented global config once so static preferences can be shared across repositories. Existing projects with `.specd/config.json` must continue to work and receive a deprecation notice rather than destructive rewrites.

## Current-state grounding

- `specd init` currently scaffolds `.specd/config.json` from embedded templates and `DefaultConfig`.
- `DefaultScaffoldManifest()`/scaffold code controls which files init writes.
- Doctor already checks scaffold health and config-policy diagnostics.
- Current init must remain safe: no unexpected overwrite of user-authored config.

## Requirements

### Requirement 1 — YAML project config for new scaffolds
**Acceptance criteria:**
1. `specd init` SHALL write `.specd/config.yml` for new projects.
2. `DefaultScaffoldManifest()` SHALL include `config.yml` and SHALL NOT require `config.json` for newly initialized projects.
3. Existing `.specd/config.json` SHALL remain valid and SHALL NOT be deleted by init.
4. Init SHALL never overwrite an existing project config unless existing scaffold policy already permits safe creation/repair.

### Requirement 2 — First-run global config scaffold
**Acceptance criteria:**
1. On `specd init`, if no global config candidate exists, THE SYSTEM SHALL create the primary global YAML path with the embedded `config.yml` template.
2. THE SYSTEM SHALL create parent directories with secure/default user permissions.
3. If global config creation fails, init SHALL warn but project init SHALL remain usable unless the failure prevents required project scaffold creation.
4. If a global config already exists, init SHALL not modify it.

### Requirement 3 — Legacy deprecation notices
**Acceptance criteria:**
1. If `.specd/config.json` exists and `.specd/config.yml` does not, init SHALL print a concise deprecation notice and recommend `specd migrate config`.
2. If both YAML and JSON project configs exist, init/doctor SHALL identify YAML as active and JSON as ignored/deprecated.
3. Notices SHALL be human-readable in text mode and included as diagnostics in JSON mode where applicable.

### Requirement 4 — Doctor scaffold/config awareness
**Acceptance criteria:**
1. `specd doctor` SHALL recognize `config.yml` as the canonical scaffold file.
2. `specd doctor --fix` SHALL create missing canonical config from YAML template when safe.
3. Doctor SHALL validate global config parseability and surface deprecated global JSON if discovered.
4. Doctor SHALL not rewrite invalid user config except existing safe scaffold repair behavior.

## Design

- Update scaffold manifest to point at `config.yml` and embedded YAML content.
- Add global config creation helper in core or cmd that uses `GlobalConfigPaths()[0]` as the canonical write target.
- Keep legacy JSON detection read-only; migration happens in `migrate-config`.
- Reuse config loader diagnostics in doctor to avoid duplicate path precedence logic.

## Out of scope

- Schema decode implementation; covered by `schema-v2-namespacing`.
- Migration command implementation; covered by `migrate-config`.
- Environment override precedence; covered by `env-precedence`.

## Risks

- **Home directory side effect:** Init will now write outside the project. Print a clear notice and make failures non-fatal where possible.
- **Scaffold tests brittle:** Update tests to assert canonical YAML while retaining legacy JSON fixture support.
