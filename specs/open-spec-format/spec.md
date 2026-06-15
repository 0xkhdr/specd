# spec.md — The EARS/DAG Spec Format as an Open Standard

**Status:** proposed
**Source:** specd-report.html §8 idea **E2** (impact: very high · effort: low · moat: very high) · §9 north-star item **#3**
**Date:** 2026-06-16
**Scope:** publish a versioned JSON Schema + format spec for specd's artifacts; `specd schema` command.

---

## 1. Objective

Publish the artifact schema (EARS requirements + 7-key task DAG + evidence
record) as a documented, versioned **open spec format** with a JSON Schema, so
other tools can read/write it. specd becomes the *reference implementation* of a
standard, not just a tool. Tools get replaced; standards endure. If competitors
interop with the specd format, specd has already won the category — this is the
deepest moat on the page.

> **Hard invariant:** stdlib-only, deterministic. The JSON Schema must be the
> **single source of truth, generated from / checked against the actual Go
> types** specd uses — a hand-written schema that drifts from the parser is
> worse than none. Versioned with explicit semver; backward-compatible reads.
> No network, no LLM.

## 2. Context

- The format today is implicit in the parsers/validators: EARS (`ears.go`), task
  schema + DAG (`tasksparser.go`, `dag.go`), evidence record
  (`VerificationRecord`, `state.go`), 7 design headers (`gates.go`).
- The report (E2) calls for "a documented, versioned open spec format with a
  JSON Schema" that "other tools read/write."

## 3. Requirements (EARS)

- **R1 (H)** THE SYSTEM SHALL publish a versioned JSON Schema covering the
  artifact set: EARS requirements, the 7-key task object + DAG, the evidence/
  verification record, and `state.json`.
- **R2 (H)** THE JSON Schema SHALL be derived from or conformance-tested against
  the actual Go types/parsers, so a schema-vs-implementation drift fails CI.
- **R3 (H)** WHEN `specd schema [--version N]` runs, THE SYSTEM SHALL emit the
  JSON Schema for the format version (embedded in the binary).
- **R4 (M)** THE format SHALL carry an explicit version identifier, and specd
  SHALL document its compatibility policy (backward-compatible reads across
  minor versions).
- **R5 (M)** THE SYSTEM SHALL provide a `specd validate --schema` mode that
  checks an artifact set against the published schema independently of the 7
  gates (format conformance vs. semantic gates).
- **R6 (L)** A human-readable format specification document SHALL accompany the
  schema (`docs/spec-format.md`), versioned alongside it.

## 4. Design / approach

1. **Schema source** — define/annotate the canonical Go structs
   (`VerificationRecord`, task object, state) and generate JSON Schema from them
   (codegen or a maintained schema with a conformance test asserting equality).
2. **Conformance test** — round-trip: marshal real specd artifacts, validate
   against the schema; mutate a struct field ⇒ test fails until schema updates
   (R2).
3. **`specd schema`** — emit the embedded schema; `--version` selects.
4. **`specd validate --schema`** — pure format check, separate from semantic
   gates.
5. **Docs** — `docs/spec-format.md` as the prose standard.

## 5. Non-goals

- No change to the 7 semantic gates (format conformance is a distinct concern).
- No external schema registry hosting in this spec (publish the artifact; host
  later).
- No breaking change to existing artifacts — the current format becomes v1.

## 6. Acceptance criteria

- `specd schema` emits a versioned JSON Schema for all artifacts; `--version`
  works.
- A conformance test ties the schema to the real Go types — changing a type
  without the schema fails CI.
- `specd validate --schema` checks an artifact set against the schema,
  independent of the 7 gates.
- `docs/spec-format.md` documents the versioned standard; `make ci` green;
  stdlib-only.
