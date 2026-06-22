# specd Level-Up — Master Progress Tracker

> Single source of truth for the level-up program defined in `LEVEL_UP_PLAN.md`.
> Tracks all domain specs as **waves**. Each wave is a coherent, shippable
> increment with its own exit gate. Do not start a wave until its predecessor's
> exit gate is green.
>
> Baseline commit: `c2cc4af` · Branch: `level-up`

---

## How to use this file

- Each spec lives under `specs/<NN-domain>/` with `spec.md` (the *what & why*)
  and `tasks.md` (the *how*, in waves).
- The coding agent works **one wave at a time, top to bottom**, within the
  active spec(s) for the current global wave.
- After finishing a task, set its checkbox here and in the spec's `tasks.md`.
- A wave is **Done** only when every task is checked AND its exit gate passes.
- Never lower a coverage floor or disable a CI gate to make a wave pass.

Status legend: `⬜ not started` · `🟡 in progress` · `✅ done` · `🔴 blocked`

---

## Global wave map

| Wave | Specs | Theme | Gate |
|---|---|---|---|
| **W1** | `01-worker-package`, `02-crash-recovery` | Make autonomous core trustworthy (P0) | `cmd/brain.go` ≥80%, `internal/worker` ≥90%, recovery stress job green |
| **W2** | `03-observability`, `04-coverage-ratchet` | Observability + ratchet (P1) | slog on stderr, stdout byte-stable, floors raised |
| **W3** | `05-lint-hardening`, `06-orchestration-split`, `07-windows-orchestration` | Hardening + maintainability (P2) | new linters green, no non-test file >700 LOC, Windows works-or-fails-fast |
| **W4** | `08-harness-features` | World-wide harness features (P3) | resume UX, metrics export, cost brake — all tested |

Sequencing (from plan §3):
```
W1 (P0)  ──►  W2 (P1)  ──►  W3 (P2)  ──►  W4 (P3)
```
W1+W2 are the **production-grade gate**. W3 is debt paydown that gets cheaper
after W1's `internal/worker` seam exists. W4 must not start until the core is
observable and tested.

---

## Wave 1 — Autonomous core (P0) · Status: ⬜

### 01-worker-package
- [x] W1.1 Extract `internal/worker` package with `Runner` seam
- [x] W1.2 Move exec/worker funcs out of `cmd/brain.go`; CLI keeps arg-parsing
- [x] W1.3 Test deadline → SIGKILL-of-process-group
- [x] W1.4 Test pipe-drain (post-signal child write, no hang)
- [x] W1.5 Test line-writer prefixing across partial/no-newline chunks
- [x] W1.6 Test mission env propagation
- [ ] W1.7 Reach `internal/worker` ≥90%, `cmd/brain.go` ≥80%

### 02-crash-recovery
- [ ] W1.8 Driver-level kill-mid-wave → resume reclaims leases test
- [ ] W1.9 Assert no double-dispatch after resume
- [x] W1.10 `make stress-brain-recovery` CI job

**Exit gate W1:** ⬜ `cmd/brain.go` ≥80% · `internal/worker` ≥90% · recovery
stress job green in CI.

---

## Wave 2 — Observability + ratchet (P1) · Status: ⬜

### 03-observability
- [ ] W2.1 `log/slog` JSON handler, `SPECD_LOG` env, default `warn`
- [ ] W2.2 Per-session log file `.specd/sessions/<id>/brain.log`
- [ ] W2.3 Structured events (dispatch/reclaim/retry/escalate/timeout/complete)
- [ ] W2.4 Stdout byte-stability test with logging on
- [ ] W2.5 `brain why`/`status` reads structured timeline

### 04-coverage-ratchet
- [ ] W2.6 Raise floors to measured-minus-1 (overall 78, core 80, cmd 75)
- [ ] W2.7 Document 85/95 target + no-floor-lowering rule in script

**Exit gate W2:** ⬜ logs stderr-only · stdout byte-unchanged test green ·
floors raised, CI green.

---

## Wave 3 — Hardening + maintainability (P2) · Status: ⬜

### 05-lint-hardening
- [ ] W3.1 Add `errorlint, gosec, bodyclose, gocritic, unconvert, misspell`
- [ ] W3.2 Triage + fix first run, CI green

### 06-orchestration-split
- [ ] W3.3 Split `program_orchestration.go` into ≤~400-LOC files
- [ ] W3.4 Tests stay green (mechanical move)

### 07-windows-orchestration
- [ ] W3.5 Decide: non-`sh` Runner OR fail-fast on Windows
- [ ] W3.6 Implement chosen path behind `Runner` interface

**Exit gate W3:** ⬜ new linters green · no non-test file >700 LOC · Windows
works-or-fails-fast with clear message.

---

## Wave 4 — Harness features (P3) · Status: ⬜

### 08-harness-features
- [ ] W4.1 `specd brain resume --session <id>` first-class + tested
- [ ] W4.2 New host adapters as agents ship (keep `--config` fallback)
- [ ] W4.3 Metrics export (Prometheus-textfile / OTLP-JSON output format)
- [ ] W4.4 Cost-brake enforcement test + soft (80% warn) / hard (100% halt)

**Exit gate W4:** ⬜ resume tested · metrics format emitted · cost brake
deterministic test green.

---

## Program exit criteria (definition of "production-grade")

From plan §4 — all must be ✅ before the program is complete:

- [ ] `cmd/brain.go` ≥80%; `internal/worker` ≥90%
- [ ] Overall coverage ≥85%, `internal/core` ≥90% (toward 95%), floors raised
- [ ] Deadline/kill/pipe-drain/crash-recovery have explicit tests + CI stress job
- [ ] slog tracing on stderr, per-session log file, stdout byte-unchanged test
- [ ] `.golangci.yml` includes `errorlint`+`gosec`+`bodyclose`; CI green
- [ ] No single non-test file >~700 LOC (`program_orchestration.go` split)
- [ ] Orchestration on Windows works or fails fast with a clear message
- [ ] Stdlib-only runtime invariant preserved throughout
