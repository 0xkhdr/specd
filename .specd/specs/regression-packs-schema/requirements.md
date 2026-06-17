# Requirements — Regression: Packs, Schema, Templates (apply/resolve/validate)

## Introduction
Packs, the embedded JSON Schema, and templates (`internal/core/pack*.go`, `schema/`,
`embed_templates/`, `embed_packs/`) are how specd ships opinionated, reusable scaffolding
and validates every artifact. Regression here guarantees that generated artifacts are
schema-valid, pack application is deterministic and safe (including remote packs), and
templates stay in sync with the schema. Value: scaffolding is trustworthy and reproducible.

## Requirement 1 — Schema validity of generated artifacts
**User story:** As an author, I want every artifact specd generates to validate against the
embedded schema, so that downstream tooling never chokes.

**Acceptance criteria:**
1. WHEN `specd new` generates artifacts THE SYSTEM SHALL produce a state.json that validates against the embedded schema
2. IF an artifact violates the schema THEN THE SYSTEM SHALL reject it during `specd check`
3. THE SYSTEM SHALL emit the schema via `specd schema` matching the version state files declare

## Requirement 2 — Deterministic pack application
**User story:** As a user applying a pack, I want identical output for identical input, so
that scaffolding is reproducible.

**Acceptance criteria:**
1. WHEN the same pack is applied twice to a clean target THE SYSTEM SHALL produce byte-identical output
2. THE SYSTEM SHALL list embedded packs via `--list-packs`
3. WHERE `--force` is absent THE SYSTEM SHALL NOT overwrite existing files

## Requirement 3 — Remote pack safety
**User story:** As a security-conscious user, I want remote packs pinned by digest, so that
supply-chain tampering is detectable.

**Acceptance criteria:**
1. WHEN a remote `--pack` URL is given with `--sha256` THE SYSTEM SHALL verify the digest before applying
2. IF the digest does not match THEN THE SYSTEM SHALL refuse to apply the pack
3. IF a remote pack URL is given without a pinned digest THEN THE SYSTEM SHALL require explicit opt-in

## Requirement 4 — Template/schema synchronization
**User story:** As a maintainer, I want templates and schema to stay in lockstep, so that a
schema bump cannot silently produce invalid templates.

**Acceptance criteria:**
1. THE SYSTEM SHALL validate every embedded template artifact against the current schema
2. IF a template references a field absent from the schema THEN THE SYSTEM SHALL fail a regression test
3. WHEN pack resolution encounters an unknown field THE SYSTEM SHALL report it rather than dropping it
