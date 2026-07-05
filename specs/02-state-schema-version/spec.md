# 02-state-schema-version — schemaVersion, migration hook, `check --schema`

Wave 0. FINDINGS refs: C.5, B.14, B.23, D-tier0 item 3, D-tier2 item 13
(schema half).

## Problem

`state.json` carries no schema version. The CAS revision counter guards
concurrency, not evolution: the first schema change after adoption becomes a
breaking event with no migration hook. v1 had schema versioning plus a
`migrate` verb (v5→v6 path, additive-blocks report, idempotent). FINDINGS
verdict: **port the discipline now, not the tool** — retrofitting versioning
after adoption is far more painful, and today there are no users. Pair with
v1's `check --schema` (embedded JSON Schema) so external tools get a
validation contract.

## Requirements (EARS)

- R1: THE SYSTEM SHALL write a top-level integer `schemaVersion` field
  (initially `1`) into every new `state.json`.
- R2: WHEN loading a `state.json` whose `schemaVersion` is missing, THE
  SYSTEM SHALL treat it as version 0 and migrate it forward to the current
  version at load time (legacy files keep working).
- R3: WHEN loading a `state.json` whose `schemaVersion` is greater than the
  binary's current version, THE SYSTEM SHALL fail closed (exit 2) with a
  message telling the user to upgrade the binary — never silently
  reinterpret newer state.
- R4: THE SYSTEM SHALL route every load through an ordered chain of pure
  migration functions (`v0→v1`, later `v1→v2`, …); each migration SHALL be
  idempotent, and the migrated state SHALL persist via the existing
  `core.AtomicWrite` + CAS path only when a write is otherwise occurring
  (read paths never mutate on disk).
- R5: WHEN a user runs `specd check --schema`, THE SYSTEM SHALL emit the
  embedded JSON Schema for the current `state.json` version; with
  `--schema-only` it SHALL validate the on-disk state against that schema
  and report violations without running other gates.
- R6: THE forward-migration policy SHALL be documented in
  `docs/contributor-guide.md`: any change to state shape bumps
  `schemaVersion`, adds a migration function, adds a fixture test.

## Design notes / best practice

- Migration chain: `[]func(map[string]any) (map[string]any, error)` indexed
  by from-version; run in order; test each with committed fixture files of
  every historical shape (golden-file tests). Determinism-first invariant:
  migrations are pure functions of the input bytes.
- JSON Schema: embed via `go:embed` next to the templates (existing
  precedent); schema file is the single source of truth, validated in tests
  against a freshly scaffolded spec's `state.json`. Stdlib-only constraint
  means the validator is a minimal internal checker (required fields, types,
  enums) — not a full draft-2020 implementation; document the subset at the
  top of the schema file.
- Version skew (R3) is the fail-closed twin of unknown verbs — same exit-2
  convention.
- Do NOT port the `migrate` verb yet: it earns existence only when two real
  schema versions exist. Record that in the code comment on the chain.

## Out of scope

- `specd migrate` verb (deferred until a second real version exists).
- Schema for artifacts other than `state.json` (tasks.md has the parser as
  its contract).

## Acceptance

- New specs get `schemaVersion: 1`; version-0 (missing-field) fixture loads
  and migrates; version-99 fixture fails closed with upgrade message.
- `specd check --schema` emits schema; `--schema-only` flags a corrupted
  fixture; both documented in command-reference + CHEATSHEET.
