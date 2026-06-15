# tasks.md — Open Spec Format execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Canonical type recon

- [ ] **T1 — Inventory the canonical artifact types**
  - role: investigator · depends: — · requirements: R1,R2
  - Map the Go types/parsers defining EARS, the 7-key task + DAG, the evidence
    record, and `state.json`. These are the schema's source of truth. file:line.
  - verify: N/A — complete with `--unverified --evidence "<type inventory>"`

## Wave 2 — Schema + conformance

- [ ] **T2 — Author versioned JSON Schema (v1) for all artifacts**
  - role: builder · depends: T1 · requirements: R1,R4
  - Embed via `embed.go`; explicit version id.
  - verify: `go test ./internal/core/ -run TestSchemaParse -race -count=2`

- [ ] **T3 — Conformance test: schema ↔ Go types (drift fails CI)**
  - role: verifier · depends: T2 · requirements: R2
  - Round-trip real artifacts; a struct change without schema update fails.
  - verify: `go test ./internal/core/ -run TestSchemaConformance -race -count=2`

## Wave 3 — Commands + docs

- [ ] **T4 — `specd schema [--version]` emits embedded schema**
  - role: builder · depends: T2 · requirements: R3
  - verify: `go test ./internal/cmd/ -run TestSchemaCmd -race -count=1`

- [ ] **T5 — `specd validate --schema` format conformance mode**
  - role: builder · depends: T2 · requirements: R5
  - Independent of the 7 semantic gates.
  - verify: `go test ./internal/cmd/ -run TestValidateSchema -race -count=1`

- [ ] **T6 — `docs/spec-format.md` versioned prose standard**
  - role: builder · depends: T4 · requirements: R4,R6
  - verify: N/A — complete with `--unverified --evidence "<format doc>"`

- [ ] **T7 — Review: schema is single source of truth, v1 non-breaking**
  - role: reviewer · depends: T3,T5,T6 · requirements: R2,R4
  - verify: N/A — complete with `--unverified --evidence "<standard review>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R2 |
| 2 | T2–T3 | R1, R2, R4 |
| 3 | T4–T7 | R3, R4, R5, R6 |
