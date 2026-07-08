# SPEC-01: CI/CD & Build Tooling

## Overview
- **Domain:** CI/CD & Build Tooling (Analysis Plan Domain 1)
- **Risk Level:** High (a production release is impossible while CI is red)
- **Priority:** P0
- **Dependencies:** None. **Blocks** SPEC-02 … SPEC-07 (they all verify their work through CI).

## Current State

`.github/workflows/ci.yml` is the declared CI contract but references tooling that is not present
in the repository. Confirmed missing paths that CI invokes:

| ci.yml line | Invocation | Status |
|-------------|-----------|--------|
| 109 | `make perf-gate` | **No root Makefile exists** (the only Makefile is under `reference/`, which is off-limits) |
| 130 | `./scripts/coverage-check.sh` | missing |
| 144 | `./scripts/stress.sh` | missing |
| 158 | `./scripts/stress-acp.sh` | missing |
| 172 | `./scripts/stress-orchestration.sh` | missing |
| 186 | `./scripts/stress-program.sh` | missing |
| 200 | `./scripts/stress-brain-recovery.sh` | missing |
| 214 | `./scripts/stress-checkpoint-fault.sh` | missing |

Additional confirmed defects:

- **Version-floor mismatch (B4):** `go.mod` declares `go 1.26`; the CI test matrix
  (`ci.yml:90`) runs `go: ["1.22", "stable"]`. A `go 1.26` directive **cannot be built by the
  Go 1.22 toolchain**, so the `1.22` matrix leg fails to compile before any test runs. `README.md`
  and `CLAUDE.md` also claim a "1.22+" minimum, contradicting the `1.26` floor.
- **Unpinned security scanner (overlaps SPEC-04):** `ci.yml:80` installs
  `golang.org/x/vuln/cmd/govulncheck@latest`. An unpinned scanner is itself a supply-chain hole.
- **Orphan scripts:** `scripts/stress-brain.sh` and `scripts/verify-progress.sh` exist but no
  workflow calls them (note ci.yml calls `stress-brain-recovery.sh`, a *different* file). These
  are dead references in the other direction — flagged here, swept in SPEC-07.

Scripts that **do** exist and are correctly wired: `test-lint.sh` (ci.yml:42),
`docs-lint.sh` (ci.yml:53). Actions are pinned (checkout@v5, setup-go@v6, golangci-lint@v6,
shellcheck@2.0.0). `go build` and `go test ./... -count=1` pass locally today.

## Target State

A green CI run on a real push/PR against a clean checkout. Every path `ci.yml` invokes either
exists and passes, or has been deliberately removed from `ci.yml` with the decision recorded.
The Go version floor is internally consistent across `go.mod`, the CI matrix, and docs.

## Scope Boundaries

- **In Scope:** `.github/workflows/ci.yml`; new `scripts/*.sh` (or their deliberate deletion from
  ci.yml); a root `Makefile` **only if** `perf-gate` is kept as a make target (a `scripts/perf-gate.sh`
  is the preferred single-mechanism alternative — decide in T-01-02); the Go version reconciliation
  in `go.mod` + `ci.yml`; pinning `govulncheck`.
- **Out of Scope:** the *content* of the perf assertion beyond a minimal runnable gate (SPEC-03
  owns the real perf envelope); coverage floor *value* and per-package gaps (SPEC-05 owns the
  coverage policy — SPEC-01 only makes `coverage-check.sh` exist and pass); security scanner
  behavior (SPEC-04); doc prose about versioning (SPEC-07). `release.yml` internals (SPEC-03).
  Anything under `reference/`.

## Technical Requirements

1. **Decide keep-vs-delete per broken job.** For each of the 8 missing invocations, make an
   explicit decision: author the missing script/target, or delete the job from `ci.yml`. Record
   the decision inline (a comment in ci.yml or a short `scripts/README` note). The analysis plan's
   working assumption is that these represent *drift to reconcile* (author them), not an
   intentional teardown — but SPEC-01 owns the final call.
2. **`perf-gate`:** prefer replacing `make perf-gate` with `scripts/perf-gate.sh` (single
   script mechanism, no Makefile needed). The gate must be *runnable and minimal*: assert the
   documented A4 invariant that disabled-mode context-manifest build does no work. Deeper perf
   work is SPEC-03.
3. **`coverage-check.sh`:** author a script that produces `coverage.out` and enforces a floor.
   SPEC-01 sets a **provisional floor at the current measured coverage** (so the job is green);
   SPEC-05 ratchets it to a policy target. The script must fail (non-zero exit) if coverage drops
   below the floor.
4. **Stress scripts:** for each stress job kept, author a script that exercises the named subsystem
   (`stress.sh` general, `stress-acp.sh` ACP ledger, `stress-orchestration.sh`, `stress-program.sh`,
   `stress-brain-recovery.sh`, `stress-checkpoint-fault.sh`). Crash-safety semantics of ACP/checkpoint
   are detailed by SPEC-06; SPEC-01 only needs the scripts to exist and pass deterministically.
   Any job deemed redundant is deleted instead, with the reason recorded.
5. **Version reconciliation:** pick one floor. Recommended: keep `go 1.26` and change the CI matrix
   to a buildable set (e.g. `["1.26", "stable"]` or `["1.26.x", "stable"]`). Update `go.mod` only if
   the floor changes. Doc string fixes (README/CLAUDE.md "1.22+") are handed to SPEC-07 but the
   *matrix vs go.mod* consistency is fixed here.
6. **Pin `govulncheck`** to a specific released version at ci.yml:80 (coordinate the exact pin with
   SPEC-04, which owns the security rationale).
7. All new shell scripts must pass `shellcheck` (CI already runs shellcheck@2.0.0) and be
   `chmod +x`.

## Verification Strategy

- Local: `go build -o specd .` (exit 0); `go test ./... -race -count=1`; `gofmt -l .` empty;
  `go vet ./...`; `go mod tidy` produces no diff; `shellcheck scripts/*.sh` clean.
- Every script referenced by `ci.yml` resolves to an existing, executable file (or is absent from
  ci.yml). Assert with: for each `run:` path in ci.yml, `test -x` (or `make -n <target>`) succeeds.
- **Definitive:** a real push/PR produces an all-green CI run, including every matrix leg (no
  compile failure on the version-floor leg).
- No LLM introduced into any gate/DAG/report path; no evidence-bypass flag; `reference/` untouched.

## Blockers Discovered

### BD-01: double-dispatch race in `brain resume` (blocks T-01-04, T-01-07)

Authoring the stress scripts (T-01-04) surfaced a genuine crash-safety concurrency defect. The
five brain-resume-based scripts flake ~7% (measured 1/15 runs each) with a **double dispatch** —
two concurrent `brain resume` processes both re-issue the same crashed mission, writing two
`dispatch` lines to `acp.jsonl` for one mission.

- **Root cause:** `internal/cmd/brain_run.go:brainResume` calls
  `orchestration.AppendDispatch` (line ~242) **outside** any spec lock. `AppendDispatch`'s own
  docstring requires "the read-then-append runs under the caller's spec lock so the duplicate
  check is race-free" — that precondition is violated at the resume call site. The session
  CAS (`SaveSessionCAS`) is correctly locked, but the ledger read (`ReadACP`, line ~218),
  the `PlanResume` decision, and the `AppendDispatch` are not one atomic transaction. In the
  window between one resume's winning session CAS and its ledger append, a second resume can
  read a stale-empty ledger, win its own CAS (revision already advanced), and dispatch again.
  `AppendDispatch`'s `HasMission` guard also reads the ledger unlocked, so it TOCTOU-races too.
- **Pre-existing:** the orphan `scripts/stress-brain.sh` (never wired into CI — ci.yml
  referenced a *different*, nonexistent `stress-brain-recovery.sh`) flakes identically. The
  master CI-drift defect (Cross-Cutting Concern 1) is exactly what let this go uncaught.
- **Proposed fix (SPEC-06 scope):** hold `core.WithSpecLock` across the resume critical
  section — re-read the ledger, re-run `PlanResume`, and `AppendDispatch` inside one lock so the
  "already dispatched?" check and the append are atomic w.r.t. other resumes.
- **Scope:** SPEC-01 explicitly delegates ACP/checkpoint crash-safety semantics to SPEC-06 and
  its own remit is only that the scripts "exist and pass deterministically." The fix touches
  `internal/orchestration` + `internal/cmd/brain_run.go` — SPEC-06's domain. **Paused for
  direction:** fix now within SPEC-01, or defer to SPEC-06 (and quarantine the 5 flaky stress
  jobs meanwhile).

## References
- Analysis Plan: Domain 1; Cross-Cutting Concern 1 ("CI/tooling drift is the master defect");
  Blockers B1–B4; Recommended Spec Breakdown row SPEC-01.
- Related Specs: SPEC-03 (perf envelope, release.yml), SPEC-04 (govulncheck pin rationale),
  SPEC-05 (coverage floor policy), SPEC-06 (ACP/checkpoint crash-safety), SPEC-07 (version doc
  strings, dead-script sweep).
- Source Files: `.github/workflows/ci.yml`, `go.mod`, `scripts/`, (possibly new) root `Makefile`.
