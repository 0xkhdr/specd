# Tasks — Close the Level-Up Production Gate

> Work top to bottom. Tasks 1–4 are P0 (unblock W1) and **must** land in order —
> each depends on the previous. Tasks 5–7 are verification/lock-in.
> After each task: `go build ./...` and `go test ./...` must stay green.
> Stdlib-only: add no new runtime imports outside the stdlib.

Status legend: `⬜ not started` · `🟡 in progress` · `✅ done`

---

## Wave P0 — Unblock W1 (do first, in order)

### ✅ T1 — Add injectable `worker.Runner` seam at the cmd layer

**Why:** `internal/cmd/brain.go:404` and `:323` hardcode `worker.ShellRunner{}`,
making the entire driver path untestable without spawning real `sh`. This is the
single root cause behind W1.7/W1.8/W1.9. Fix unblocks T2–T4.

**Where:**
- `internal/cmd/brain.go:400` `brainRunWorker` — line `runner := worker.ShellRunner{}`.
- `internal/cmd/brain.go:323` `brainRunProgramWorker` (wraps `brainRunWorker` via `inner`).
- Call sites: `brainRunSession` (builds opts at `:251`), program drive (`:301`).

**Do:**
1. Add a `worker.Runner` parameter to `brainRunWorker` (and thread through
   `brainRunProgramWorker`). Replace the hardcoded `runner := worker.ShellRunner{}`
   with the passed-in runner.
2. Prefer a single seam: introduce a small struct (e.g. `brainDeps{ Runner
   worker.Runner }`) or a package-level default, so production callers don't
   each repeat `worker.ShellRunner{}`. Production default **must** be
   `worker.ShellRunner{}` — byte-identical behavior.
3. Update all production call sites to pass the default runner.
4. Keep `worker.Mission{...}` construction and the `obs.LogEvent("complete"/"timeout", ...)`
   calls exactly as they are — only the runner instance is injected.

**Done when:**
- `go build ./...` green; `go test ./...` green.
- No hardcoded `worker.ShellRunner{}` inside `brainRunWorker`/`brainRunProgramWorker`.
- A test can construct the driver callback with a fake `worker.Runner`.
- Live `brain run` output is unchanged (spot-check stdout byte-stability).

---

### ✅ T2 — Driver-level recovery tests (W1.8 + W1.9)

**Why:** Spec 02 §3.1 requires driver-level kill-mid-wave + no-double-dispatch
across kill+resume + idempotent resume. Existing tests only cover the engine
(`internal/core/orchestration_recovery_test.go`) and the cooperative
pause→resume path (`internal/cmd/brain_resume_test.go`) — **not a crash** where
the process dies holding an in-flight lease.

**Depends on:** T1.

**Do:**
1. New file `internal/cmd/brain_recovery_test.go`.
2. Add a `recordingRunner` fake implementing `worker.Runner`:
   - Records every `Mission.TaskID` passed to `Run` (slice/map, mutex-guarded).
   - On **first** dispatch of a designated target task: persist the lease as
     in-flight (as a real worker would mid-run), then return a sentinel error
     that stops the drive **without releasing** the lease (simulates death).
   - On subsequent runs (post-resume): behave normally and complete.
3. Test `TestBrainDriverKillReclaimsLease` (W1.8): drive once → dies holding
   lease; reload session from a **fresh** driver; assert the stale lease is
   reclaimed (released or re-leased, never dangling).
4. Test `TestBrainDriverNoDoubleDispatchAcrossKillResume` (W1.9): run to
   completion across the kill+resume boundary; assert each `TaskID` appears in
   the recording **at most once**.
5. Test `TestBrainResumeIdempotent`: resume twice → identical reconciled session
   state (compare serialized state / event log).
6. **Hygiene:** seed all randomness explicitly; tests must pass under
   `go test -race -count=2 ./internal/cmd/`.

**Done when:**
- All three tests green under `-race -count=2`.
- Fake runner injected via the T1 seam (no real `sh` spawned).

---

### 🟡 T3 — Cover the brain driver path → `cmd/brain.go` ≥80% (W1.7)

> **Status (2026-06-23):** The 11 formerly-0% driver funcs are now all covered
> (none at 0%) via the recording-fake seam — the core W1.7 objective. After the
> T4 split those funcs live across `brain.go` (74.6%), `brain_worker.go`
> (72.5%), `brain_policy.go` (89%), `brain_commands.go` (81%); aggregate brain
> driver code ≈78.5%, just under the literal 80% bar. The remaining gap is in
> `brainRunBootstrap`'s spec-creation success branch, which isn't reachable
> through `brain run` in the current flow (the spec is loaded before preflight
> bootstrap fires) — closing it needs a flow change, not just a test.

**Why:** brain.go is at **41.8%**; 11 funcs at 0% — the whole live driver path.
Gate is ≥80%.

**Depends on:** T1, T2.

**Do — exercise these 0% functions with the fake runner:**
`brainRun, brainRunProgram, brainRunProgramWorker, brainRunSession,
brainRunBootstrap, bootstrapHint, brainRunWorker, brainRunPolicy, workerExitCode,
brainStep, brainDirective`.
- Reuse the `recordingRunner` and resume helpers from T2.
- Cover `workerExitCode` mapping (success, timeout, generic error → exit codes).
- Cover `brainRunBootstrap` + `bootstrapHint` (spec vs non-spec preflight item).
- Cover `brainRunPolicy` defaults and flag overrides.

**Done when:**
- `go test ./... -coverprofile=cov.out && go tool cover -func=cov.out | grep 'cmd/brain.go'`
  shows ≥80% and none of the 11 funcs at 0%.

---

### ✅ T4 — Slim `cmd/brain.go` to ≤~350 LOC (spec 01 §4)

**Why:** spec 01 §4 requires brain.go drop from 692 → ≤~350. Still **629**.
W3's 700-LOC gate passes but spec 01's own criterion fails.

**Depends on:** T1–T3 (do after tests exist so refactor is safe).

**Do:**
- Move mechanism out of the CLI file into a helper file (e.g.
  `internal/cmd/brain_worker.go`): mission-build, `workerExitCode`,
  `bootstrapHint`, the worker-callback construction.
- CLI file keeps: command routing (`RunBrain`), policy defaults
  (`brainRunPolicy`/`brainPolicy`), flag parsing helpers.
- Pure move/extract — no behavior change. Tests from T2/T3 must stay green.

**Done when:**
- `wc -l internal/cmd/brain.go` ≤ ~350.
- `go test ./... ` green; coverage still ≥80% for the brain code.

---

## Wave P1 — Close W2/W3 verification

### ✅ T5 — Install + run `golangci-lint`, triage (W3.2)

**Why:** `.golangci.yml` enables 11 linters (W3.1 done) but the tool isn't
installed → "first run triaged, CI green" unverified. Only one `//nolint` exists.

**Do:**
1. Install `golangci-lint` (pinned version matching CI if specified).
2. `golangci-lint run ./...`; fix real findings.
3. Add narrow commented `//nolint:<linter> // reason` **only** where intentional —
   notably gosec on the `sh -c` worker path (`internal/worker/shell_runner.go`,
   tie reason to `SECURITY.md`).
4. Confirm `bodyclose` clean on `update.go` / `watch_sse.go`; confirm `errorlint`
   `%w` / `errors.Is` correctness across the 402 `fmt.Errorf` sites.

**Done when:** `golangci-lint run ./...` exits 0.

---

### 🟡 T6 — Ratchet coverage toward program targets (W2.6)

> **Status (2026-06-23):** Floors ratcheted **up** to locked measured values
> (overall 71→77, core 73→79, cmd 61→69, mcp 87→88; worker 90, harness 80
> unchanged). All gains locked — no floor lowered. The aspirational program
> targets (overall ≥85, core ≥90) are **not yet met**: reaching them requires
> error-branch fixtures across the program-orchestration runtime
> (`program_lease.go`, `program_session.go`, `program_step.go`,
> `orchestration_engine.go`), which is the bulk of the remaining ~580 core /
> ~850 overall uncovered statements. Cheap pure/0%-func coverage is exhausted.

**Why:** floors are *measured-minus-1*, far below program targets. Ratchet exists
but never ratcheted up. Targets: overall ≥85, core ≥90.

**Depends on:** T3 (driver coverage lands first).

**Do:**
1. Measure post-T3: `go test ./... -coverprofile=cov.out && go tool cover -func=cov.out`.
2. Add tests for the largest remaining 0%/low funcs in `internal/core` until
   core ≥90 and overall ≥85.
3. Bump floors in `scripts/coverage-check.sh` to the new measured values.
   **Never lower a floor.**

**Done when:** overall ≥85%, `internal/core` ≥90%; floors updated to match.

---

## Wave P2 — Confirm gates in CI

### 🟡 T7 — `make ci` end-to-end + flip `progress.md`

> **Status (2026-06-23):** `make ci` exits 0 end-to-end (lint, test, test-order
> `-count=2`, cover-check with raised floors, perf-gate, and all four stress
> jobs incl. `stress-brain-recovery`). `golangci-lint run ./...` exits 0
> separately. W1/W3 exit gates are green; W2 stays 🟡 until the coverage stretch
> targets in T6 land — `progress.md` flipped accordingly, not blanket-green.

**Why:** capture green output as W1/W2/W3 exit-gate evidence.

**Depends on:** T1–T6.

**Do:**
1. Run `make ci` (lint + test + test-order + cover-check + perf-gate + all four
   stress jobs incl. `make stress-brain-recovery`).
2. Capture green output as exit-gate evidence.
3. Flip wave statuses in `specs/progress.md` (W1 🔴→✅, W2/W3 🟡→✅) and check the
   corresponding tasks in each spec's `tasks.md`.

**Done when:** `make ci` exits 0; `progress.md` reflects green W1–W4.

---

## Verification commands

```bash
# coverage by package
go test ./... -coverprofile=cov.out && go tool cover -func=cov.out | tail -1
go tool cover -func=cov.out | grep 'cmd/brain.go'   # the formerly-0% funcs

# recovery tests under race
go test -race -count=2 ./internal/cmd/

# lint
golangci-lint run ./...

# recovery stress + full gate
make stress-brain-recovery
make ci
```
