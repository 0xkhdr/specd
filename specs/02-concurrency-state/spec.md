# Stage 02 — Concurrency & State Integrity

## Scope

The primitives every mutating command depends on: the advisory lock
(`internal/core/lock.go`), the state machine + CAS revision logic
(`internal/core/state.go`), and atomic file writes (`internal/core/io.go`).
Bugs here corrupt `state.json` or deadlock the CLI, so this stage precedes all
command refactors.

## Current state & findings

### F1 — [MEDIUM] CAS is a read-after-read TOCTOU, masked only by the lock
`state.go:198-218` `SaveState` re-reads disk, compares `onDisk.Revision` to
`state.Revision`, then writes. The compare and the write are **not atomic**;
correctness relies entirely on `WithSpecLock` being held. That coupling is
real but **undocumented and unenforced** — any future caller of `SaveState`
outside a lock silently loses the guarantee.

Additional: if `disk == nil` (file vanished mid-session) the revision check is
**skipped entirely** (`state.go:201`), so a concurrent delete+recreate is not
detected.

**Intent:**
- Document the invariant in `SaveState`'s doc comment: *must* be called inside
  `WithSpecLock` for the same `(root, slug)`.
- Add a debug-buildable assertion (or a `lockHeld(path)` check using the
  existing `locks` map) that panics in tests if `SaveState` runs without the
  in-process lock owner set — catches the footgun without runtime cost in prod.
- Treat a vanished file during an expected-update as a conflict, not a silent
  pass.

### F2 — [MEDIUM] `goID()` fragility
`lock.go:44-54` parses the goroutine id out of `runtime.Stack`. On a parse
failure it returns `0`. `0` is also the "unheld" sentinel for `ls.owner`
(`lock.go:21`). If `goID` ever returns `0` for two different goroutines, the
reentrancy fast-path (`lock.go:123`) could misfire. Probability is low (the
header format is stable) but the **failure mode is a silent lock bypass**, the
worst possible outcome for a concurrency primitive.

**Intent:** make `0` impossible to confuse: if parsing fails, fall back to a
process-unique monotonic id from `atomic.AddInt64` rather than `0`, OR keep
`goID` but change the unheld sentinel to `-1` so a real gid of `0` (or a parse
failure mapped to a sentinel) can never alias "unheld". Add a unit test that
asserts `goID()` is non-zero and stable within a goroutine and distinct across
goroutines.

### F3 — [LOW] Stale-lock reclamation race window
`lock.go:140-154`: process A finds the lock stale, `os.Remove`s it, loops, and
`tryAcquire`s. Two processes can both see stale, both remove, both create —
but `O_CREATE|O_EXCL` makes only one `tryAcquire` win, so this is **safe**.
However the loser then loops and may delete the *winner's fresh* lock if it
re-evaluates `isStale` on a file younger than `staleMs` — it won't, because a
fresh lock is not stale. Net: correct, but the reasoning is subtle and
untested.

**Intent:** add a concurrency test (extend `concurrency_test.go` /
`stress.sh`) that spawns N goroutines + simulated stale lock and asserts
exactly one writer at a time and no lost increments. No code change unless the
test reveals a defect.

### F4 — [LOW] Lock not released on panic in `fn`
`lock.go:161-168` uses `defer` to release, so a panic *does* unwind the defer
and release — **correct**. But `fn()`'s panic propagates with the lock state
already reset; verify `ls.mu.Unlock()` ordering is panic-safe (it is, via
defer). Document this; add a test that panics inside `WithSpecLock` and asserts
the next acquire succeeds.

### F5 — [LOW] `migrate()` ignores unmarshal errors
`state.go:143` `json.Unmarshal(v, &sv)` return value discarded. A corrupt
`schemaVersion` field would leave `sv=0` → treated as v1 → may misparse.
Low impact (outer unmarshal would usually fail first) but sloppy.

**Intent:** check the error; on failure return a `GateError("corrupt
schemaVersion")`.

## Non-goals
- Switching to an OS file-lock syscall (`flock`) — the O_EXCL + mutex design is
  portable and adequate; revisit only if cross-process stress fails.
- Changing `state.json` schema shape (Stage-wide constraint).

## Acceptance criteria
1. `SaveState` documents and (in test builds) asserts the lock-held invariant.
2. `goID()` can never return the unheld sentinel; covered by a unit test.
3. New concurrency test: N writers, no lost revision increments, exactly-one
   writer, panic-in-fn releases the lock.
4. `migrate()` surfaces a corrupt `schemaVersion` instead of silently
   defaulting.
5. `go test -race ./...` green; `scripts/stress.sh` passes.
