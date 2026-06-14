# Stage 02 — Tasks

Branch: `refactor/02-concurrency-state`.

## T1 — Harden `goID()` sentinel (F2)
**File:** `internal/core/lock.go`.

1. Change the unheld sentinel from `0` to `-1`:
   - `lockState.owner` zero-value is `0`; add explicit init so a fresh
     `lockState{owner: -1}` is created in `lockStateFor` (`lock.go:35`).
   - Update the release defer (`lock.go:163`) to set `ls.owner = -1` (not `0`).
   - Update the reentrant check `lock.go:123` to `ls.owner == gid && gid != -1`.
2. In `goID()` (`lock.go:44-54`), on parse failure return a process-unique
   negative-free fallback:
   ```go
   id, err := strconv.ParseInt(string(b[:i]), 10, 64)
   if err != nil || id == 0 {
       return atomic.AddInt64(&goIDFallback, 1) + 1<<40
   }
   ```
   Add `var goIDFallback int64` and `import "sync/atomic"`. The `1<<40` offset
   keeps fallback ids from colliding with real (small) gids.
3. Add `lock_test.go` cases: `goID()` non-zero, stable within goroutine,
   distinct across two goroutines.

**Verify:** `go test -race ./internal/core/ -run Lock`

## T2 — Document + assert SaveState lock invariant (F1)
**Files:** `internal/core/state.go`, `internal/core/lock.go`.

1. Add an exported helper in `lock.go`:
   ```go
   // lockHeldBy reports whether the current goroutine owns the spec lock for
   // path. Used by SaveState's debug assertion.
   func lockHeldBy(root, slug string) bool {
       path := lockFilePath(root, slug)
       locksMu.Lock()
       defer locksMu.Unlock()
       ls, ok := locks[path]
       return ok && ls.owner == goID()
   }
   ```
2. In `SaveState` doc comment (`state.go:198`): state it MUST be called inside
   `WithSpecLock(root, slug, …)`.
3. Add a test-only guard. Prefer a package-level `var assertLocked = false` that
   tests flip on, so prod has zero overhead:
   ```go
   if assertLocked && !lockHeldBy(root, slug) {
       panic("SaveState called without spec lock: " + slug)
   }
   ```
   Set `assertLocked = true` in `state_cas_test.go`'s `TestMain` or each test.
4. Handle vanished-file conflict (`state.go:201`): if a prior load saw the file
   but `disk == nil` now, treat as conflict. Simplest: if `state.Revision > 0`
   and `disk == nil`, return `GateError("state.json for '<slug>' disappeared
   mid-session — concurrent delete detected")`.

**Verify:** `go test -race ./internal/core/ -run CAS`

## T3 — migrate() error checking (F5)
**File:** `internal/core/state.go:140-152`.

1. Replace `json.Unmarshal(v, &sv)` (line 143) with:
   ```go
   if err := json.Unmarshal(v, &sv); err != nil {
       return State{}, GateError("corrupt schemaVersion in state.json")
   }
   ```

**Verify:** `go test ./internal/core/ -run State`

## T4 — Concurrency + panic test (F3, F4)
**File:** `internal/core/concurrency_test.go` (extend).

1. Test `WithSpecLock` with N=20 goroutines each doing
   load→increment→`SaveState` under the lock; assert final revision == N and no
   CAS error.
2. Test: a goroutine that panics inside `fn`; recover; assert the lock file is
   gone and a subsequent `WithSpecLock` acquires immediately.
3. Test stale reclamation: pre-write a lock file with an old timestamp
   (`now - SPECD_LOCK_STALE_MS - 1`), assert `WithSpecLock` reclaims it.

**Verify:** `go test -race ./internal/core/ -run Concurren` and `bash scripts/stress.sh`

## Done-when
- `go vet ./... && gofmt -l . && go test -race ./...` green.
- `goID` sentinel collision impossible (T1) and tested.
- SaveState-without-lock panics in tests (T2).
