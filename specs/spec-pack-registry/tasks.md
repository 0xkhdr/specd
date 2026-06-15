# tasks.md — Spec-pack Registry execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Init + verify recon

- [ ] **T1 — Map init scaffolding + SHA256 verify pattern**
  - role: investigator · depends: — · requirements: R1,R3
  - Report how `init.go` renders embedded assets (`embed.go`) and how
    `update.go`/`install.sh` do fail-closed SHA256. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<init+verify map>"`

## Wave 2 — Pack format + built-ins

- [ ] **T2 — `pack.json` manifest format + parser (declarative only)**
  - role: builder · depends: T1 · requirements: R4,R5,R7
  - Reject manifests referencing executable hooks.
  - verify: `go test ./internal/core/ -run TestPackManifest -race -count=2`

- [ ] **T3 — Embed built-in packs + `--list-packs`**
  - role: builder · depends: T2 · requirements: R2
  - verify: `go test ./internal/cmd/ -run TestListPacks -race -count=1`

## Wave 3 — Resolve + apply

- [ ] **T4 — Pack resolver: embedded + remote (pinned SHA256, fail-closed)**
  - role: builder · depends: T2 · requirements: R1,R3,R5
  - Reuse update.go verify helper; mismatch ⇒ nothing written.
  - verify: `go test ./internal/core/ -run TestPackResolve -race -count=2`

- [ ] **T5 — `specd init --pack` transactional apply (no partial scaffold)**
  - role: builder · depends: T3,T4 · requirements: R1,R5,R6
  - `--pack` omitted ⇒ unchanged.
  - verify: `go test ./internal/cmd/ -run TestInitPack -race -count=1`

- [ ] **T6 — Test: fail-closed remote + default regression**
  - role: verifier · depends: T5 · requirements: R3,R6
  - verify: `go test ./... -run 'TestPackFailClosed|TestInitDefaultRegression' -race -count=2`

- [ ] **T7 — Document the pack manifest contract**
  - role: builder · depends: T5 · requirements: R7
  - verify: N/A — complete with `--unverified --evidence "<pack format docs>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R3 |
| 2 | T2–T3 | R2, R4, R5, R7 |
| 3 | T4–T7 | R1, R3, R5, R6, R7 |
