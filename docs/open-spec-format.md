# Open Spec Format — v1

A versioned, machine-readable description of the on-disk artifacts specd
produces and consumes. This document is the **prose standard**; the normative
contract is the JSON Schema embedded in the binary, emitted by `specd check --schema`.

```sh
specd check --schema                 # current version to stdout
specd check --schema --version 1     # an explicit version
specd check <slug> --schema-only   # check a spec's state.json against it
```

## Versioning

- **`specdSchemaVersion`** (`"1"`) versions *this format standard* — the
  published JSON Schema contract.
- **`stateSchemaVersion`** (`6`) versions the *on-disk migration shape* of
  `state.json` (see `internal/core/state.go`). The two are independent: the
  format standard can gain a new published version without changing the
  migration number, and vice versa.

v1 is **non-breaking**: every field the binary writes today is described, and
optional fields are marked optional so older documents continue to validate.

## Source of truth

The Go types in `internal/core` are the single source of truth. `schema/v1.json`
mirrors them, and `TestSchemaConformance` (`internal/core/schema_test.go`) is a
drift trip-wire: adding or removing a struct field without updating the schema
(or vice versa) fails CI. The schema is therefore never hand-maintained out of
sync with the code.

## Artifacts

A spec lives under `.specd/specs/<slug>/`:

| File | Purpose |
|------|---------|
| `requirements.md` | EARS-numbered acceptance criteria |
| `design.md` | Design sections (the design gate's required headers) |
| `tasks.md` | The 7-key task records + wave DAG |
| `decisions.md` | Architectural decision records (ADRs) |
| `memory.md` | Durable project learnings |
| `mid-requirements.md` | Mid-flight requirement-change log |
| `state.json` | The durable execution ledger — **the schema's subject** |

`requirements.md`, `design.md`, and `tasks.md` are Markdown with conventions
enforced by the validation gates (`docs/validation-gates.md`). `state.json` is
the structured ledger validated by `specd check --schema-only`.

## `state.json` — the validated document

The root is a `State` object. The schema defines five `$defs`:

- **`State`** — `schemaVersion`, `revision`, `spec`, `title`, `status`, `phase`,
  `gate`, `turn`, `createdAt`, `updatedAt`, `tasks` (map of `TaskState`),
  `blockers` (array of `Blocker`); optional `acceptance` (map of
  `CriterionRecord`), `prompt`, and the execution-mode pair `executionMode`
  (`simple` | `orchestrated`) and `modeOrigin` (`default` | `user` |
  `recommended-accepted`). All optional fields are `omitempty`, so a Simple spec
  that never opted into orchestration keeps a byte-identical `state.json`.
- **`TaskState`** — `id`, `title`, `role`, `wave`, `depends`, `requirements`,
  `status`; optional `startedAt`, `finishedAt`, `evidence`, `verification`,
  `blocker`, `telemetry`.
- **`VerificationRecord`** — the evidence of a verify run: `command`, `exitCode`,
  `verified`, `timedOut`, `stdoutTail`, `stderrTail`, `durationMs`, `ranAt`;
  optional `gitHead`, `changedFiles`, `coverage`, `sandbox`, `reverted`,
  `stashRef`.
- **`CriterionRecord`** — a per-acceptance-criterion proof: `requirement`,
  `criterion`, `status`, `evidence`, `ranAt`.
- **`Blocker`** — `task`, `reason`, `since`.

Every object is **closed** (`additionalProperties: false`): an unknown key is a
conformance violation. Enumerated fields (`status`, `phase`, `gate`, `role`) are
constrained to their allowed values.

## Conformance modes

`specd check --schema-only` performs **structural** conformance: object/array
shape, required keys, closed property sets, and enum membership. It is
intentionally independent of the seven semantic gates (`specd check`) and ships
no third-party JSON Schema validator — the binary stays stdlib-only. Numeric
bounds and string `format` keywords (`date-time`) are documented in the schema
for external tooling but are policy left to the semantic layer, not the format
check.

Exit codes: `0` conformant, `1` violations found, `2` usage error, `3` spec not
found. Add `SPECD_JSON=1` for a machine-readable report.
