# spec.md — Long-running Daemon + Event Stream (`specd watch`)

**Status:** proposed
**Source:** specd-report.html §8 idea **C1** (impact: high · effort: med · moat: med) · §9 north-star item **#5**
**Date:** 2026-06-16
**Scope:** new `specd watch` daemon emitting frontier-change events; `internal/cmd/watch.go`.

---

## 1. Objective

A daemon that watches the program/spec DAG and emits frontier-change events
(JSON-lines / SSE / webhook) so external orchestrators **react** instead of
polling `program status` in a loop. This turns specd into the scheduler core of
an agent fleet. Parallel orchestration today is poll-driven; an event stream is
what production multi-agent systems need to scale past a handful of tasks.

> **Hard invariant:** stdlib-only, deterministic, read-only. `watch` mutates no
> state — it observes `state.json`/program files and emits events. Event
> ordering is deterministic for a given sequence of on-disk changes. Transports
> are `net/http` (SSE) + JSON-lines on stdout + optional outbound webhook POST,
> all stdlib.

## 2. Context

- `specd program status` (`internal/cmd/program.go`, `internal/core/program.go`)
  computes which whole specs are runnable; `RunnableFrontier`
  (`internal/core/dag.go`) computes the task frontier.
- State lives in per-spec `state.json`; mutations are atomic + revision-versioned
  (`state.go` CAS) — a revision bump is a reliable change signal.

## 3. Requirements (EARS)

- **R1 (H)** WHEN `specd watch [--root path]` runs, THE SYSTEM SHALL observe the
  program + per-spec state and emit a `frontier-changed` event whenever the
  runnable task frontier or runnable-spec set changes.
- **R2 (H)** THE SYSTEM SHALL emit events as newline-delimited JSON on stdout by
  default, each event carrying the slug, the new frontier, and the state
  revision that produced it.
- **R3 (M)** WHERE `--sse [--port N]` is set, THE SYSTEM SHALL also serve the
  same events as Server-Sent Events over `net/http` (loopback default).
- **R4 (M)** WHERE `--webhook <url>` is set, THE SYSTEM SHALL POST each event as
  JSON to that URL, retrying with bounded backoff and never blocking the watch
  loop on a slow endpoint.
- **R5 (M)** THE SYSTEM SHALL detect changes via the state revision/CAS counter
  (not content diffing) so duplicate events are not emitted for no-op writes.
- **R6 (M)** THE SYSTEM SHALL be read-only: it never advances a phase, flips a
  task, or writes spec state.
- **R7 (L)** WHEN the daemon receives SIGINT/SIGTERM, it SHALL flush in-flight
  webhook posts (best effort) and exit cleanly.

## 4. Design / approach

1. **Change detection** — poll `state.json` revisions at a small interval (or
   `fsnotify`-free `os.Stat` mtime + revision read; no third-party watcher).
   Emit only when the computed frontier changes vs the last emitted frontier.
2. **Event model** — `core.FrontierEvent{Slug, Revision, Frontier, Runnable}`;
   compute via the existing `RunnableFrontier` / program builder.
3. **Transports** — JSON-lines writer (default), SSE handler (`net/http`),
   webhook POSTer with a bounded retry queue in a separate goroutine.
4. **Lifecycle** — signal handling for clean shutdown.

## 5. Non-goals

- No state mutation; this is an observer, not a scheduler that acts.
- No third-party file-watch or message-queue dependency.
- No distributed coordination (see C2 distributed-state-backend).

## 6. Acceptance criteria

- Advancing a task in one process makes `specd watch` emit exactly one
  `frontier-changed` event with the new frontier + revision (no duplicate on
  no-op writes).
- `--sse` streams the same events; `--webhook` POSTs them with bounded retry and
  never blocks the loop.
- The daemon writes no spec state (asserted).
- Clean shutdown on SIGINT; `make ci` green; stdlib-only.
