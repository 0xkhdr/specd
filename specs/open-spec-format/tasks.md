# tasks.md — Open Spec Format execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Canonical type recon

- [x] **T1 — Inventory the canonical artifact types** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R2
  - Map the Go types/parsers defining EARS, the 7-key task + DAG, the evidence
    record, and `state.json`. These are the schema's source of truth. file:line.
  - verify: N/A — complete with `--unverified --evidence "<type inventory>"`
  - **Evidence:** EARS — `EarsPattern` + `earsPatterns` `internal/core/ears.go:5-24`,
    `MatchEars` `ears.go:26`, `LintEars` `ears.go:54`. Task — `MandatoryKeys`
    /`KeyOrder` `internal/core/tasksparser.go:12-13`, `ParsedTask`
    `tasksparser.go:50-58`, `ParseTasks` `tasksparser.go:186`, `ValidRoles`
    /`ReadonlyRoles` `tasksparser.go:14-15`. DAG — `DagTask` `internal/core/dag.go:5-10`,
    `ParseDepends` `tasksparser.go:158`. Design headers — `DesignSections`
    `internal/core/phases.go:9-12`. Evidence records — `VerificationRecord`
    `state.go:52-62`, `CriterionRecord` `state.go:64-70`. `state.json` —
    `State` `state.go:93-107`, `TaskState` `state.go:72-85`, `Blocker`
    `state.go:87-91`, `Violation` `phases.go:14-18`; `SchemaVersion=4`
    `state.go:11`, migration `state.go:140-169`. These Go types are the schema's
    single source of truth a v1 JSON Schema must mirror.

## Wave 2 — Schema + conformance

- [x] **T2 — Author versioned JSON Schema (v1) for all artifacts** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R1,R4
  - Embed via `embed.go`; explicit version id.
  - verify: `go test ./internal/core/ -run TestSchemaParse -race -count=2`
  - **Evidence:** `internal/core/schema/v1.json` (draft-07, `$defs` for State,
    TaskState, VerificationRecord, CriterionRecord, Blocker; `specdSchemaVersion:"1"`,
    `stateSchemaVersion:4`). Embedded + loaded by `internal/core/schema.go`
    (`Schema`, `ParseSchema`, `SchemaVersionID`). `TestSchemaParse` passes `-race -count=2`.

- [x] **T3 — Conformance test: schema ↔ Go types (drift fails CI)** ✓ complete · 2026-06-16
  - role: verifier · depends: T2 · requirements: R2
  - Round-trip real artifacts; a struct change without schema update fails.
  - verify: `go test ./internal/core/ -run TestSchemaConformance -race -count=2`
  - **Evidence:** `TestSchemaConformance` reflects each canonical struct's json
    tags and asserts bijection with schema `properties` + `required` (omitempty ⇔
    optional) + `additionalProperties:false`. A struct field added without a
    schema update (or vice versa) fails CI. Passes `-race -count=2`.

## Wave 3 — Commands + docs

- [x] **T4 — `specd schema [--version]` emits embedded schema** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R3
  - verify: `go test ./internal/cmd/ -run TestSchemaCmd -race -count=1`
  - **Evidence:** `cmd/schema.go` (`RunSchema`) writes the embedded schema to
    stdout; `--version` selects (unknown fails closed); needs no .specd/ root.
    Registered in Registry + core.Commands. `TestSchemaCmd` passes.

- [x] **T5 — `specd validate --schema` format conformance mode** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R5
  - Independent of the 7 semantic gates.
  - verify: `go test ./internal/cmd/ -run TestValidateSchema -race -count=1`
  - **Evidence:** `core.ValidateState` (stdlib-only structural validator:
    $ref/required/additionalProperties/enum) + `cmd/validate.go` (`RunValidate`,
    `--schema` mode, read-only, JSON under SPECD_JSON). `TestValidateSchema` passes.

- [x] **T6 — `docs/spec-format.md` versioned prose standard** ✓ complete · 2026-06-16
  - role: builder · depends: T4 · requirements: R4,R6
  - verify: N/A — complete with `--unverified --evidence "<format doc>"`
  - **Evidence:** `docs/spec-format.md` — versioning (specdSchemaVersion vs
    stateSchemaVersion), source-of-truth + conformance trip-wire, artifact table,
    state.json $defs, structural conformance modes.

- [x] **T7 — Review: schema is single source of truth, v1 non-breaking** ✓ complete · 2026-06-16
  - role: reviewer · depends: T3,T5,T6 · requirements: R2,R4
  - verify: N/A — complete with `--unverified --evidence "<standard review>"`
  - **Evidence:** Reviewed: Go types remain the single source of truth, enforced
    by `TestSchemaConformance` drift trip-wire; v1 optional fields are non-required
    so older documents validate (non-breaking). Validator is stdlib-only (no
    third-party JSON Schema dependency).

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R2 |
| 2 | T2–T3 | R1, R2, R4 |
| 3 | T4–T7 | R3, R4, R5, R6 |
