# tasks.md — Coverage & Diff-scope Evidence execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Record & contract recon

- [x] **T1 — Map verify record + files-contract plumbing** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R2
  - Report `VerificationRecord` shape, where HEAD is captured in
    `RunVerify`, and how `files:` is parsed/stored. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<plumbing map>"`
  - **Evidence:** record shape — `VerificationRecord`
    `internal/core/state.go:52-62` (`Command, ExitCode, Verified, TimedOut,
    StdoutTail, StderrTail, DurationMs, RanAt, GitHead *string`). HEAD capture —
    `gitHead(cwd)` `internal/cmd/verify.go:48-58` (`git rev-parse HEAD`), called
    when building the record in `runVerifyCommand` `verify.go:254`; record
    persisted under the spec lock `ts.Verification = rec` `verify.go:107-111`.
    `files:` contract — `files` is one of the 7 `MandatoryKeys`
    `internal/core/tasksparser.go:12`, stored as raw text in `ParsedTask.Meta`
    `tasksparser.go:261`; **no gate currently reads/enforces it** (`CheckGates`
    `gates.go:26-34` has no scope gate). Gaps to fill: add `ChangedFiles` +
    `Coverage` to `VerificationRecord`, capture them in `runVerifyCommand`
    alongside `GitHead`, and add a `GateScope` that diffs changed files against
    the `files:` glob (`*`/unset = no-op).

## Wave 2 — Capture evidence

- [x] **T2 — Add `ChangedFiles` + `Coverage` to the record (back-compat)** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R1,R3
  - `omitempty` JSON; existing records still parse.
  - verify: `go test ./internal/core/ -run TestVerificationRecordCompat -race -count=2`
  - **Evidence:** `VerificationRecord.ChangedFiles []string` + `Coverage string`
    (both omitempty) `state.go`; schema mirror added. `TestVerificationRecordCompat`
    proves legacy records parse and empty fields stay omitted. Passes `-race -count=2`.

- [x] **T3 — Capture changed files + optional coverage in `RunVerify`** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R1,R3,R4
  - `git diff --name-only` vs base HEAD; parse coverage total if present, else
    "unavailable" (never fail on coverage).
  - verify: `go test ./internal/cmd/ -run TestVerifyCapture -race -count=1`
  - **Evidence:** `changedFiles` (diff vs HEAD + untracked, sorted, best-effort)
    and `parseCoverage` (`coverage: N%` → "N%" else "unavailable") in `verify.go`,
    wired into `runVerifyCommand`'s record. Coverage capture never fails verify.
    `TestVerifyCapture` + `TestVerifyCaptureNoCoverage` pass.

## Wave 3 — Scope gate + surface

- [ ] **T4 — `GateScope` (warn/error, `*`/unset = no-op)**
  - role: builder · depends: T3 · requirements: R2,R5
  - `filepath.Match` changed files vs task `files:` globs.
  - verify: `go test ./internal/core/ -run TestGateScope -race -count=2`

- [ ] **T5 — Report shows changed-file count + coverage**
  - role: builder · depends: T3 · requirements: R6
  - verify: `go test ./internal/cmd/ -run TestReportScope -race -count=1`

- [ ] **T6 — Review: coverage is evidence, not a binary floor**
  - role: reviewer · depends: T4,T5 · requirements: R4
  - verify: N/A — complete with `--unverified --evidence "<no hardcoded floor>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R2 |
| 2 | T2–T3 | R1, R3, R4 |
| 3 | T4–T6 | R2, R4, R5, R6 |
