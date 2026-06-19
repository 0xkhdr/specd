# Pinky & The Brain — Production-Grade Review

**Date:** 2026-06-19
**Branch:** `pinky-and-the-brain`
**Scope:** Full review of the Pinky & The Brain orchestration implementation in `specd`, after the first-implementation gap closure (GAP-1 … GAP-10).
**Method:** Read of core orchestration sources (`internal/core/orchestration_*.go`, `acp*.go`, `pinky*.go`), MCP intent layer (`internal/mcp/intent.go`), CLI wiring (`internal/cmd/brain.go`); cross-checked against the design intent in `pinky-and-the-brain-analysis.md` and the closed-gap claims in `pinky-and-the-brain-gaps.md`.

---

## 0. Health snapshot

| Check | Result |
|-------|--------|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `go test ./internal/core/ ./internal/mcp/` | 553 passed |
| GAP-1 … GAP-10 | genuinely implemented in code (verified, not just claimed) |

The control plane is excellent and genuinely production-grade. ACP envelope validation, the lease store, the deterministic `DecideOrchestration` engine, and the evidence gates are rigorous. Durability is well-engineered: fsync + hardlink immutable event writes, the store as sole sequence authority, duplicate-messageId detection, file-permission hardening, and fail-soft handling of untrusted host telemetry.

**The harness loop around that control plane under-delivers against the tool's own stated design.** The gaps below are *not* tracked in `pinky-and-the-brain-gaps.md` (which marks everything DONE). They are the delta between the shipped reference driver and the concurrency / safety model the design promises.

---

## 1. Findings

### GAP-11 (P0) — Reference driver is serial; design and README promise concurrency

**Design intent.** `pinky-and-the-brain-analysis.md` specs `concurrent_pinkies: 4` and `max_concurrent_specs: 2`. `README.md` headlines "Waves, Not Lines", "Computes the concurrent runnable frontier of waves", and "Frontier Dispatch … emits ready-to-run packets for parallel subagents".

**Reality.** `DriveOrchestration` dispatches exactly one mission, then **blocks** on the worker callback before it steps again:

- `internal/core/orchestration_driver.go:84` — `opts.Worker(DriverDispatch{...})` is called synchronously; the loop cannot step the next slot until it returns.
- `internal/cmd/brain.go:370` — the shipped worker callback runs `cmd := exec.Command("sh", "-c", workerCmd)` then `cmd.Run()`, which blocks until the worker process exits.

So the loop never holds more than one active lease at a time.

**Consequence.** `MaxWorkers > 1` is **dead configuration** in the shipped loop. The decision `case len(snapshot.ActiveLeases) >= policy.MaxWorkers → Wait` (`internal/core/orchestration_decide.go:50`) can never fire, because the serial loop never accumulates a second lease. The lease store (`acp_lease.go`), the runnable frontier, and the wave math all *support* concurrency — only the driver fails to exercise it. The same applies one level up: `DriveProgramOrchestration` hands each child dispatch to the worker synchronously (`orchestration_driver.go:189`), so `max_concurrent_specs` is equally inert.

**This is the headline production gap:** the project's primary differentiator (concurrent DAG waves of parallel subagents) exists in the architecture but not in observable behavior.

**Fix direction.** Make the driver asynchronous against `MaxWorkers`:
- Spawn each dispatch's worker in a goroutine; keep stepping while `activeLeases < MaxWorkers` and the frontier yields unleased runnable work.
- Block (select) only when the frontier is empty *or* the worker pool is saturated.
- Reap on worker-report or lease-expiry; on either, re-step.
- The existing "dispatch→spawn contract" comment (`orchestration_driver.go:82`) already describes the correct per-slot semantics — generalize it from one slot to N.
- Keep the no-LLM-in-core invariant: concurrency lives in the driver glue, not in `DecideOrchestration` (which stays one-bounded-decision-per-step and is already concurrency-correct via the lease snapshot).

---

### GAP-12 (P0) — No host-side worker timeout

**Design intent.** `pinky-and-the-brain-analysis.md` specs `default_timeout_minutes: 30`. Every mission already carries a `Deadline` (`pinky.go:58`) and the lease carries a TTL.

**Reality.** The worker is launched with `exec.Command("sh", "-c", workerCmd)` and a bare `cmd.Run()` (`internal/cmd/brain.go:370`) — no `context`, no deadline. A hung or runaway worker hangs the entire drive indefinitely. The lease TTL is currently advisory only: because the driver blocks on `cmd.Run()`, it never reaches the lease-expiry branch its own contract comment describes for an in-process worker.

**Consequence.** A single stuck Pinky stalls Brain forever. No fail-closed behavior for the most common real-world failure (worker wedged on a network call, an interactive prompt, an infinite loop).

**Fix direction.**
- Launch with `exec.CommandContext(ctx, ...)` where `ctx` deadline derives from `mission.Deadline` (or a configured `worker_timeout`).
- Set `cmd.SysProcAttr` with a process group and kill the *group* on timeout (`sh -c` spawns children; killing only the shell orphans them).
- On timeout: release the lease and return a retryable failure so the next step sees an expired lease and applies the retry/escalate policy already present in `DecideOrchestration` (`orchestration_decide.go:31`).

---

### GAP-13 (P1) — File-based ACP store is O(n²) per session

**Reality.** `readAllEvents` reads the whole events directory, then reads, parses, and runs full `ValidateACPEnvelope` on **every** event file (`internal/core/acp_store.go:112-137`). It is called on:
- every `WriteEvent` (to allocate the next sequence + dup check) — `acp_store.go:56`
- every `ReadEvents` — `acp_store.go:93`
- every `senseHostReportedCost` — `orchestration_limits.go:22`

A session that accumulates N events (missions + accepted + heartbeats + progress + evidence + queries/directives, across a multi-spec program) pays O(N) per write → **O(N²)** over the session, with full JSON re-validation each time.

**Design intent.** `pinky-and-the-brain-analysis.md` risk #5 already flags "file-based ACP might not scale" — acknowledged, not yet mitigated.

**Fix direction.**
- Sequence allocation needs only the max existing sequence — derivable from filenames (`parseACPEventFilename`) without reading payloads.
- The duplicate-messageId check needs only the set of prior message IDs — also in the filename. Cache that set per session under the existing session lock.
- Reserve full parse+validate for `ReadEvents` consumers that actually need payloads, and consider a cached parsed tail keyed by max-sequence.

---

### GAP-14 (P2) — Minor / hardening

| # | Location | Issue | Note |
|---|----------|-------|------|
| 14a | `internal/core/acp_store.go:30` | `acpStoreLocks` map is never pruned — one entry per session lock path, unbounded growth. | Harmless for a short-lived CLI; a leak for a long-running `specd serve`/daemon. |
| 14b | `internal/cmd/brain.go` (worker stdout/stderr → `os.Stdout`) | Fine while serial; output interleaves illegibly once workers run concurrently. | Resolve together with GAP-11 (per-worker buffered/prefixed output). |
| 14c | `SPECD_ARTIFACT` env hint | Built via `filepath.Base` over a space-joined file list → garbage for multi-file missions. | Cosmetic; affects only the worker's env hint. Confirm exact construction before fixing. |

---

## 2. Invariants any fix must preserve

(From `pinky-and-the-brain-gaps.md` §4 — crown jewels. No fix below may violate these.)

1. **Core stays deterministic.** Zero LLM/provider-SDK/network calls in `internal/core`. Concurrency and timeouts live in the *driver glue* (`orchestration_driver.go`, `cmd/brain.go`), never in `DecideOrchestration`.
2. **Evidence gates completion.** No mission completes without a `specd verify` record bound to the task scope. Host telemetry stays untrusted.
3. **Fail closed.** Timeouts, conflicting leases, saturation → escalate/retry per policy, never guess.
4. **Human-only gates remain human-only.**
5. **One bounded action per step.** The driver may run N workers concurrently, but each `StepOrchestration` stays single-decision/single-action and replayable.

---

## 3. Implementation tasks

### Milestone D — Realize the concurrency model (P0)

- [x] **T1 — Async worker pool in `DriveOrchestration`.**
  Refactor the single-spec loop so it spawns each dispatch in a goroutine and keeps stepping while `activeLeases < policy.MaxWorkers` and the frontier yields unleased runnable work. Block only on pool saturation or empty frontier. Reap on worker-report / lease-expiry, then re-step. Preserve terminal outcomes (`complete | escalated | awaiting-approval | worker-stop | max-steps | stalled`).
  *Files:* `internal/core/orchestration_driver.go`, `internal/cmd/brain.go`.

- [x] **T2 — Async worker pool in `DriveProgramOrchestration`.**
  Same treatment at the program level so `max_concurrent_specs` becomes live; cap concurrent child specs and concurrent workers within each child.
  *Files:* `internal/core/orchestration_driver.go`, `internal/cmd/brain.go`.

- [x] **T3 — Concurrency golden tests.**
  Add tests asserting: (a) with `MaxWorkers=N` and ≥N runnable tasks in a wave, N leases are held simultaneously; (b) the `ActiveLeases >= MaxWorkers → Wait` branch actually fires; (c) deterministic completion regardless of worker finish order; (d) program-level concurrent-spec dispatch.
  *Files:* `internal/core/orchestration_driver_test.go`, `internal/integration/orchestration_test.go`.

### Milestone E — Fail-closed worker safety (P0)

- [x] **T4 — Per-worker timeout via `exec.CommandContext`.**
  Derive deadline from `mission.Deadline` (or a new `worker_timeout` config/flag). Launch the worker in its own process group; on timeout kill the *group*, release the lease, return a retryable failure so the next step applies retry/escalate.
  *Files:* `internal/cmd/brain.go`, config plumbing in `internal/core/orchestration.go`.

- [x] **T5 — Timeout tests.**
  Stub worker that sleeps past its deadline → assert lease released, failure recorded retryable, drive escalates after `MaxRetries`, no orphaned child processes.
  *Files:* `internal/cmd/brain_test.go` (or integration).

### Milestone F — Scale the ACP store (P1)

- [x] **T6 — O(1)-amortized sequence + dup check.**
  Replace the per-write `readAllEvents` with filename-derived max-sequence and a cached prior-messageId set (under the existing session lock). Keep full parse+validate only where payloads are consumed.
  *Files:* `internal/core/acp_store.go`.

- [x] **T7 — Store-scale benchmark + regression test.**
  Add a benchmark over a large synthetic session asserting write cost stays ~constant per event; assert no behavior change in event ordering, dup rejection, or sequence-gap detection.
  *Files:* `internal/core/acp_store_test.go`.

### Milestone G — Hardening (P2)

- [x] **T8 — Prune `acpStoreLocks` map** (or bound it / drop entries on session close).
  *Files:* `internal/core/acp_store.go`.
- [x] **T9 — Per-worker prefixed/buffered output** so concurrent worker logs stay legible (depends on T1).
  *Files:* `internal/cmd/brain.go`.
- [x] **T10 — Fix `SPECD_ARTIFACT` multi-file env hint.**
  *Files:* `internal/cmd/brain.go`.

---

## 4. One-line summary

> The control plane is production-grade and the first-implementation gaps (GAP-1…10) are genuinely closed. What remains is that the tool's headline promise — concurrent DAG waves of parallel subagents — lives in the architecture but not in the shipped driver, which runs strictly serial with no worker timeout. Close Milestones D and E and the behavior finally matches the design; Milestone F removes the next scaling wall.
