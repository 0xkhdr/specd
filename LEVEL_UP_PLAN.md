# specd — Level-Up Plan

> Analysis + action plan to take `specd` from "mature and well-engineered" to
> **production-grade, world-wide coding-agent harness**.
>
> Date: 2026-06-23 · Branch: `level-up` · Baseline commit: `c2cc4af`

---

## 0. Executive Summary

`specd` is already a **high-quality** codebase, not a rough prototype:

- ~25.8k LOC of non-test Go, **stdlib-only** (zero external runtime deps) — a
  deliberate, defensible supply-chain posture.
- **1154 tests pass**, 120 test files, co-located per package.
- Strong CI: `gofmt` gate, `go vet`, `golangci-lint`/`staticcheck`,
  `govulncheck`, `-race` + `-count=2` order-dependence run, a **coverage floor
  ratchet**, cross-OS build matrix (linux/macOS/windows), and **four** stress
  jobs (acp / orchestration / program / cross-process contention).
- Deterministic-by-design: exit-code contract (`0/1/2/3`), `SPECD_JSON=1`
  structured output, atomic+locked state mutations, byte-stable init receipts.
- Rich, coherent command surface: lifecycle, spec workflow, DAG waves,
  evidence-gated verify, Brain/Pinky orchestration, MCP host auto-setup,
  reporting/serve/watch/replay, packs, and a versioned JSON Schema.

So this is **not** a rewrite. Leveling up means closing a focused set of gaps
where the *newest, highest-risk subsystem (autonomous orchestration)* outran its
test and observability coverage, and adding the operational features a
world-wide harness needs in the field.

### The single biggest risk

`internal/cmd/brain.go` — **692 LOC, 0% test coverage**. This is the autonomous
orchestration driver: it spawns subagent workers (`sh -c <worker-cmd>`), manages
process groups, deadlines, SIGKILL escalation, pipe draining, and line-prefixed
output. The deterministic *engine* underneath (`orchestration_engine.go`) is
tested; the **process-orchestration layer that actually runs in the field is
not**. This is exactly the code that produces zombie processes, hung pipes, and
deadline races in production. **P0.**

---

## 1. Gap Analysis (evidence-backed)

### 1.1 Test coverage

| Package | Coverage | Floor | Notes |
|---|---|---|---|
| overall | **72.5%** | 70% | Documented target: 85% |
| internal/core | 74.9% | 70% | Documented target: 95% |
| internal/cmd | **61.9%** | — | Lowest; driver/exec paths uncovered |
| internal/mcp | 86.6% | 70% | Healthy |
| internal/integration | 69.3% | — | Host-adapter exec paths |
| internal/cli | 100% | — | — |
| internal/testharness | 80.0% | 80% | At floor |

Concentrated zero-coverage funcs (field-critical):

- `cmd/brain.go`: `brainRun`, `brainRunProgram`, `brainRunWorker`,
  `brainRunProgramWorker`, `brainStep`, `brainRunPolicy`, `workerDeadlineContext`,
  `newWorkerLineWriter`/`Write`/`Flush`, `bootstrapHint`, `brainWhy`,
  `brainDirective` — **17 uncovered funcs**.
- `cmd/serve.go:RunServe`, `cmd/watch*.go:watchPass/runWatchSSE`,
  `cmd/update.go:RunUpdate/fetchLatestTag`, `cmd/mcp.go:RunMCP` — long-running /
  network entrypoints with no harnessed test.

**Why it matters:** the coverage *floor* (70) sits ~2.5pts under measured and is
a regression ratchet, not the goal. The **gap to the stated target (85/95) is
real**, and it is unevenly distributed onto the most dangerous code.

### 1.2 Observability — the biggest *operational* gap

- **No structured logging anywhere.** `grep` for `log/slog` in non-test code: a
  single trivial `report.go` hit. Only **8** `fmt.Fprintf(os.Stderr, …)` sites in
  the whole tree.
- An autonomous harness that spawns child processes across waves has **no
  field-debuggable trace**: when a worker hangs or a session stalls, an operator
  has stdout line-prefixes and nothing else. No leveled log, no per-session log
  file, no structured event with timestamps/durations/exit codes.
- `telemetry.go` is roll-up only and cost is operator-annotated — fine by
  design, but it is *accounting*, not *operational tracing*.

### 1.3 Code cohesion / maintainability

- `internal/core/program_orchestration.go` — **1129 LOC single file**. The
  orchestration subsystem is spread across ~12 files but this one is a god-file.
  High-churn area (last 5 commits all touch orchestration/context) — splitting by
  responsibility (plan / decide / dispatch / reconcile) lowers merge-conflict and
  review cost.
- `cmd/brain.go` mixes CLI arg parsing, policy, and OS process management in one
  692-LOC file. The exec/worker layer wants to be its own package
  (`internal/worker`) — both for testability (inject a fake runner) and clarity.

### 1.4 Lint / static analysis depth

`.golangci.yml` enables only `staticcheck, errcheck, govet, ineffassign,
unused`. For code that does `exec.Command`, env propagation, and file I/O,
missing high-signal linters:

- `errorlint` — the codebase has 402 `fmt.Errorf` and 142 `errors.Is/As/%w`
  sites; `errorlint` enforces correct wrap/compare and catches the rest.
- `gosec` — flags subprocess/`os` mis-use on a tool whose job *is* running
  untrusted-ish commands.
- `bodyclose` — `update.go`/`watch_sse.go` do HTTP; guards leaked bodies.
- `gocritic`, `unconvert`, `misspell` — cheap polish.

### 1.5 Resilience / lifecycle gaps for autonomous mode

- **Crash recovery is unverified.** Sessions persist via
  `Save/LoadOrchestrationSession`, but there's no tested "brain process dies
  mid-wave → resume reclaims in-flight leases" path at the `cmd` layer (the ACP
  lease layer is stress-tested; the driver wrapper is not).
- **No live cost/token brake test.** `--cost-limit`/`--orchestration-cost-limit`
  exist but the brake depends on host-reported numbers with no harnessed
  enforcement test.
- **Worker exec is `sh -c`** (POSIX-only path). Windows worker dispatch is
  effectively unsupported for orchestration even though the binary builds there.

---

## 2. Action Plan (prioritized)

### P0 — Make the autonomous core trustworthy

1. **Extract `internal/worker` package** from `cmd/brain.go`.
   - Define a `Runner` interface: `Run(ctx, Mission) (Result, error)`.
   - Move `brainRunWorker`, `workerDeadlineContext`, `workerArtifactHint`,
     `workerLineWriter` into it. CLI keeps arg-parsing only.
   - Acceptance: `cmd/brain.go` drops below ~350 LOC; new package ≥ 90% covered.

2. **Test the process-orchestration layer.** With the `Runner` seam:
   - Deadline → SIGKILL-of-process-group (spawn a `sh` that forks a child that
     ignores SIGTERM; assert the group dies and `Wait` returns within
     `WaitDelay`).
   - Pipe-drain: child that writes after parent signal → no hang.
   - Line-writer prefixing across partial/no-trailing-newline chunks.
   - Mission env propagation (`SPECD_MISSION/SESSION/WORKER/SPEC/TASK/ROLE/ARTIFACT`).
   - Acceptance: `cmd/brain.go` from 0% → ≥ 80%.

3. **Crash-recovery test at the driver level.** Kill a `brain run` mid-wave;
   assert resume reclaims leases and does not double-dispatch a task.
   - Add a `make stress-brain-recovery` job to CI.

### P1 — Observability + operability

4. **Introduce leveled structured logging** (stdlib `log/slog`, JSON handler —
   no new dep).
   - `SPECD_LOG=debug|info|warn` env; default `warn` (keeps quiet UX).
   - Per-session log file under `.specd/sessions/<id>/brain.log` with
     `event=dispatch|reclaim|retry|escalate|timeout|complete`, `worker`, `task`,
     `dur_ms`, `exit`.
   - Critical: route logs to **stderr only**, never stdout — preserve the
     `SPECD_JSON=1` stdout contract and byte-stable receipts. Add a test asserting
     stdout is unchanged with logging on.

5. **`specd brain why` / `status` upgrade.** Surface the structured session
   timeline (waves, decisions, reclaims) from the log so operators can diagnose a
   stalled session without reading raw output. (`brainWhy` is currently 0%.)

6. **Coverage ratchet step.** Raise floors to measured-minus-1 as the P0/P1 tests
   land (overall → 78, core → 80, cmd → 75), with a written target of 85/95 and a
   per-PR "no-floor-lowering" rule already documented in `coverage-check.sh`.

### P2 — Hardening + maintainability

7. **Deepen lint config:** add `errorlint, gosec, bodyclose, gocritic,
   unconvert, misspell` to `.golangci.yml`; triage and fix the first run. Keep CI
   green (these are dev-only; stdlib-only runtime invariant preserved).

8. **Split `program_orchestration.go`** along its existing internal seams
   (resolve / decide / dispatch / reconcile) into ≤ ~400-LOC files. Pure
   mechanical move + tests stay green; lowers churn cost in the hottest file.

9. **Windows orchestration story.** Either (a) implement a non-`sh` worker
   runner behind the `Runner` interface (`cmd /c` / direct exec), or (b)
   explicitly document orchestration as POSIX-only and fail-fast with a clear
   message on Windows (today it silently relies on a bash-like `sh`).

### P3 — World-wide harness features (roadmap, post-hardening)

10. **Session resumability UX:** `specd brain resume --session <id>` as a
    first-class, tested command (pieces exist; promote + document).
11. **More host adapters:** the managed set is claude-code/codex/cursor/gemini/
    vscode + config-snippet antigravity/claude-desktop. Add adapters as new
    agents ship; keep the `--config` snippet fallback as the universal path.
12. **Metrics export** (optional, opt-in): emit the session event stream
    (already NDJSON via `watch`) in a Prometheus-textfile / OTLP-JSON shape — no
    runtime dep, just an output format — for fleet operators running many specs.
13. **Cost-brake enforcement test + soft/hard modes:** warn at 80%, halt at 100%,
    with a deterministic test feeding synthetic host cost reports.

---

## 3. Sequencing

```
P0 (1→2→3)  ──►  P1 (4→5→6)  ──►  P2 (7,8,9)  ──►  P3 (10-13)
worker pkg      slog + why       lint + split    harness features
+ tests         + ratchet        + windows
```

P0 and P1 are the production-grade gate. P2 is maintainability debt paydown that
gets cheaper *after* the worker package exists. P3 is feature growth that should
not start until the autonomous core is observable and tested.

---

## 4. Exit Criteria (definition of "production-grade")

- [ ] `cmd/brain.go` ≥ 80% coverage; new `internal/worker` ≥ 90%.
- [ ] Overall coverage ≥ 85%, `internal/core` ≥ 90% (toward 95%), floors raised.
- [ ] Deadline/kill/pipe-drain/crash-recovery paths have explicit tests + a CI
      stress job.
- [ ] Structured `slog` tracing on stderr, per-session log file, with a test
      proving the `SPECD_JSON=1` stdout contract is byte-unchanged.
- [ ] `.golangci.yml` includes `errorlint`+`gosec`+`bodyclose`; CI green.
- [ ] No single non-test file > ~700 LOC (`program_orchestration.go` split).
- [ ] Orchestration on Windows either works or fails fast with a clear message.
- [ ] Stdlib-only runtime invariant preserved throughout.

---

## 5. Explicitly *not* changing (and why)

- **Stdlib-only.** It's a feature, not a gap. All proposals above use `log/slog`,
  `os/exec`, and output *formats* — no new runtime deps.
- **Deterministic exit-code / JSON contract.** Untouched; logging is additive on
  stderr.
- **The spec-workflow + gates engine.** Already well-tested and coherent; no
  rework warranted.
- **CI breadth.** Already strong; we add jobs, not restructure.
