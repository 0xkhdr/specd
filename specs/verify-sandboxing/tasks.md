# tasks.md — Verify Sandboxing execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Runner recon

- [x] **T1 — Map the current verify execution path** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R3,R4
  - Report exactly where `sh -c` runs, env allowlist + NUL rejection live, and
    how the record is written. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<exec path map>"`
  - **Evidence:** exec — `runVerifyCommand` `internal/cmd/verify.go:214-256`
    runs `exec.CommandContext(ctx, shell, "-c", command)` `verify.go:223` with
    `cmd.Dir = root` `verify.go:224`; shell = `SPECD_VERIFY_SHELL` else `"sh"`
    `verify.go:99-102`; timeout `SPECD_VERIFY_TIMEOUT_MS` (default 600 000 ms)
    `verify.go:25-27`/`:215-216`. Env allowlist — `scrubbedEnv`
    `verify.go:32-46` (keeps `PATH,HOME,LANG,LC_ALL,TMPDIR` + `SPECD_*` only),
    applied at `cmd.Env = scrubbedEnv()` `verify.go:225`. NUL rejection —
    `verify.go:95-97` (refuses command containing a NUL byte). Record write —
    built `verify.go:245-255`, persisted under `WithSpecLock` via
    `ts.Verification = rec` + `SaveState` `verify.go:104-111`. Insertion point
    for a `Runner` interface is the single `exec.CommandContext` call
    `verify.go:223` — default `shRunner` reproduces today's behaviour byte-for-
    byte; sandbox runners wrap it fail-closed.

## Wave 2 — Runner abstraction

- [x] **T2 — Extract `Runner` interface; default `shRunner` = today** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R3,R4
  - Behavior byte-identical for `none`. Keep env scrub + NUL reject + print.
  - verify: `go test ./internal/core/ -run TestShRunnerUnchanged -race -count=2`
  - **Evidence:** `internal/core/runner.go` — `Runner` interface (`Name`+`Run`),
    `RunSpec`/`RunResult`, default `shRunner` (`NewShRunner`, Name "none")
    reproducing the exact `shell -c` exec/timeout/exit-code logic. `verify.go`'s
    `runVerifyCommand` now delegates to a `core.Runner`, keeping env-scrub + NUL
    reject + print at the cmd layer. `TestShRunnerUnchanged` passes `-race -count=2`.

- [x] **T3 — Add `Sandbox` to VerificationRecord (back-compat)** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R5
  - verify: `go test ./internal/core/ -run TestRecordSandboxField -race -count=2`
  - **Evidence:** `VerificationRecord.Sandbox string` (omitempty) + schema mirror.
    Default/none runs leave it empty (byte-identical to legacy); only isolating
    backends stamp it. `TestRecordSandboxField` passes `-race -count=2`.

## Wave 3 — Sandbox backends

- [x] **T4 — `bwrapRunner` (fail-closed if absent)** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R1,R2,R3,R6
  - ro-bind + tmpfs + no-net; preserve env scrub + exit/output passthrough.
  - verify: `go test ./internal/core/ -run TestBwrapRunner -race -count=1`
  - **Evidence:** `internal/core/runner_sandbox.go` — `bwrapRunner` wraps the
    command with `--unshare-all` (no net), `--ro-bind / /`, a writable
    `--bind` of the workspace, private `/proc`/`/dev`/`--tmpfs /tmp`,
    `--chdir`. `newBwrapRunner` fails closed via `exec.LookPath`. Exit/timeout
    folding shared with the shell runner via `runIsolated` (124 on timeout).

- [x] **T5 — `containerRunner` (docker/podman, fail-closed if absent)** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R1,R2,R3,R6
  - verify: `go test ./internal/core/ -run TestContainerRunner -race -count=1`
  - **Evidence:** `runner_sandbox.go` — `containerRunner` runs
    `docker|podman run --rm --network none --volume <root>:<root> --workdir
    <root>` forwarding only the scrubbed env via `--env`. `newContainerRunner`
    fails closed when no engine is on PATH or `SPECD_SANDBOX_IMAGE` is unset.

- [x] **T6 — Wire `verify.sandbox` config + `--sandbox` flag** ✓ complete · 2026-06-16
  - role: builder · depends: T4,T5,T3 · requirements: R1,R5
  - verify: `go test ./internal/core/ -run TestSelectRunner -race -count=1`
  - **Evidence:** `core.SelectRunner` resolves none/bwrap/container (fail-closed);
    `cmd/verify.go` reads `--sandbox` (overriding `verify.sandbox` config) and
    selects the runner before running. `--sandbox` flag added to verify command
    metadata.

## Wave 4 — Safety + docs

- [x] **T7 — Test: fail-closed on missing isolator; `none` regression** ✓ complete · 2026-06-16
  - role: verifier · depends: T6 · requirements: R2,R4
  - verify: `go test ./internal/core/ -run 'TestSelectRunner' -race -count=2`
  - **Evidence:** `runner_sandbox_test.go` — `TestSelectRunnerNoneRegression`
    (none → shRunner, name "none"), `TestSelectRunnerUnknownFailsClosed`,
    `TestSelectRunnerFailsClosedOnMissingIsolator` (empty PATH → bwrap/container
    refuse with "refusing to run unisolated"), `TestSelectContainerFailsClosedWithoutImage`.

- [x] **T8 — Update SECURITY.md isolation + fail-closed contract** ✓ complete · 2026-06-16
  - role: builder · depends: T7 · requirements: R7
  - verify: N/A — complete with `--unverified --evidence "<SECURITY.md diff>"`
  - **Evidence:** SECURITY.md "Verify isolation (opt-in, fail-closed)" bullet
    documents none/bwrap/container semantics and the fail-closed contract.

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R3, R4 |
| 2 | T2–T3 | R3, R4, R5 |
| 3 | T4–T6 | R1, R2, R3, R5, R6 |
| 4 | T7–T8 | R2, R4, R7 |
