# tasks.md — Spec-pack Registry execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Init + verify recon

- [x] **T1 — Map init scaffolding + SHA256 verify pattern** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R3
  - Report how `init.go` renders embedded assets (`embed.go`) and how
    `update.go`/`install.sh` do fail-closed SHA256. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<init+verify map>"`
  - **Evidence:** init render — `RunInit` `internal/cmd/init.go:19`; `place()`
    `init.go:28-43` reads embedded templates via `core.ReadTemplate`
    (`internal/core/embed.go:11`, backed by `//go:embed embed_templates`
    `embed.go:8-9`) then `core.AtomicWrite` `init.go:38`; asset lists
    `steeringFiles`/`roleFiles`/`skillFiles` `init.go:12-17`; `ApplyVars`
    substitution `embed.go:19-24`; AGENTS.md marker-merge `init.go:57-67`.
    Fail-closed SHA256 — Go: `downloadBinary` `internal/cmd/update.go:87-134`,
    `fetchChecksums` `update.go:58-85` (no SHA256SUMS ⇒ abort `update.go:66`,
    no entry ⇒ abort `update.go:95-98`), stream-hash via `io.TeeReader`
    `update.go:118-119`, mismatch abort `update.go:128-130`. Shell:
    `install.sh` `verify_checksum` `scripts/install.sh:48-69` (die on missing
    SHA256SUMS `:57`, no hasher `:66`, checksum fail `:68`; `--no-verify` escape
    `:51`). Pattern to reuse for pinned-pack resolve: download → hash → compare →
    refuse on any gap.

## Wave 2 — Pack format + built-ins

- [x] **T2 — `pack.json` manifest format + parser (declarative only)** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R4,R5,R7
  - Reject manifests referencing executable hooks.
  - verify: `go test ./internal/core/ -run TestPackManifest -race -count=2`
  - **Evidence:** `internal/core/pack.go` — `Pack`/`PackFile` + `ParsePack`
    (DisallowUnknownFields + explicit forbidden executable-key list, case-insensitive;
    path-safety via `validatePackPath`: rejects abs/`..`/non-canonical/dup).
    Fails closed, no partial pack. `TestPackManifest` passes `-race -count=2`.

- [x] **T3 — Embed built-in packs + `--list-packs`** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R2
  - verify: `go test ./internal/cmd/ -run TestListPacks -race -count=1`
  - **Evidence:** `//go:embed embed_packs` + `BuiltinPacks`/`BuiltinPack` in
    `pack.go`; ship `minimal` + `go-service`. `specd init --list-packs` (text +
    `SPECD_JSON`) added in `init.go`, scaffolds nothing. `TestListPacks` passes.

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
