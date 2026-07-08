package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// lockUnheld is the owner sentinel for a lock not held by any goroutine. It is
// -1 (not 0) so a real goroutine id of 0 — or a goID parse failure — can never
// alias "unheld" and silently bypass the lock. See goID.
const lockUnheld int64 = -1

// lockState tracks both the in-process serialization mutex and the reentrancy
// bookkeeping for one spec lock path. owner/depth are guarded by locksMu, not
// by mu, so a reentrant call by the owning goroutine can update depth without
// taking mu (which it already holds).
type lockState struct {
	mu    sync.Mutex // serializes goroutines within this process
	owner int64      // goroutine id currently holding mu (lockUnheld = unheld)
	depth int        // reentrancy depth for owner
}

var (
	locksMu sync.Mutex
	locks   = map[string]*lockState{}

	// goIDFallback feeds a process-unique id when goID cannot parse the stack
	// header. Offset by 1<<40 so fallback ids never collide with real (small)
	// goroutine ids.
	goIDFallback int64
)

func lockStateFor(path string) *lockState {
	locksMu.Lock()
	defer locksMu.Unlock()
	ls, ok := locks[path]
	if !ok {
		ls = &lockState{owner: lockUnheld}
		locks[path] = ls
	}
	return ls
}

// goID returns the current goroutine's id by parsing the runtime stack header
// ("goroutine N [..."). Used only to make WithSpecLock reentrant for the same
// goroutine while still excluding other goroutines.
//
// It never returns 0 or lockUnheld: on a parse failure it falls back to a
// process-unique positive id, so a degraded goID can never be mistaken for the
// "unheld" sentinel and bypass the lock.
func goID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	b := bytes.TrimPrefix(buf[:n], []byte("goroutine "))
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		return atomic.AddInt64(&goIDFallback, 1) + 1<<40
	}
	id, err := strconv.ParseInt(string(b[:i]), 10, 64)
	if err != nil || id == 0 {
		return atomic.AddInt64(&goIDFallback, 1) + 1<<40
	}
	return id
}

func staleMs() time.Duration {
	return time.Duration(EnvInt("SPECD_LOCK_STALE_MS", 30_000, 0, 0)) * time.Millisecond
}
func timeoutMs() time.Duration {
	return time.Duration(EnvInt("SPECD_LOCK_TIMEOUT_MS", 5_000, 0, 0)) * time.Millisecond
}

const retryInterval = 25 * time.Millisecond

func lockFilePath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), ".lock")
}

func tryAcquire(path string) bool {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // advisory lock metadata is non-secret; group/other-readable by design
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	fmt.Fprintf(f, "%d %d\n", os.Getpid(), time.Now().UnixMilli())
	return true
}

func isStale(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	parts := strings.Fields(strings.TrimSpace(string(data)))
	if len(parts) < 2 {
		return false
	}
	ms, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false
	}
	age := time.Now().UnixMilli() - ms
	return age > staleMs().Milliseconds()
}

// WithSpecLock runs fn while holding the spec's advisory lock. It provides three
// guarantees:
//   - Cross-process exclusion via an O_EXCL lock file (with stale reclamation).
//   - In-process exclusion via a per-path mutex, so concurrent goroutines in the
//     same process serialize rather than racing the lock file.
//   - Reentrancy for the owning goroutine, so a locked section may call another
//     locking helper (e.g. LoadSpec inside an already-locked command) without
//     deadlocking.
func WithSpecLock[T any](root, slug string, fn func() (T, error)) (T, error) {
	path := lockFilePath(root, slug)
	ls := lockStateFor(path)
	gid := goID()

	// Reentrant fast path: this goroutine already owns the lock.
	locksMu.Lock()
	if gid != lockUnheld && ls.owner == gid && ls.depth > 0 {
		ls.depth++
		locksMu.Unlock()
		defer func() {
			locksMu.Lock()
			ls.depth--
			locksMu.Unlock()
		}()
		return fn()
	}
	locksMu.Unlock()

	// Serialize against other goroutines in this process.
	ls.mu.Lock()

	// Acquire the cross-process lock file, reclaiming stale ones.
	deadline := time.Now().Add(timeoutMs())
	for !tryAcquire(path) {
		if isStale(path) {
			os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			ls.mu.Unlock()
			var zero T
			return zero, GateError(fmt.Sprintf("spec '%s' is locked by another specd process — retry shortly, or remove %s if it is stale", slug, path))
		}
		time.Sleep(retryInterval)
	}

	locksMu.Lock()
	ls.owner = gid
	ls.depth = 1
	locksMu.Unlock()

	// Release on return OR panic: defer unwinds even when fn panics, so the
	// lock file is removed and ls.mu is unlocked before the panic propagates,
	// leaving the next acquirer free.
	defer func() {
		locksMu.Lock()
		ls.owner = lockUnheld
		ls.depth = 0
		locksMu.Unlock()
		os.Remove(path)
		ls.mu.Unlock()
	}()
	return fn()
}

// WithProgramLock runs fn while holding the repo-wide program lock, using the
// same cross-process O_EXCL primitive (with stale reclamation) as WithSpecLock.
// It serializes program.json mutations — notably the maintenance-schedule
// claim/tick path (P3.5) — so a double-invoked `specd program tick` cannot run a
// due schedule twice. Unlike WithSpecLock it is not reentrant and is keyed on a
// single fixed path, since program-level operations never nest under a spec lock.
func WithProgramLock[T any](root string, fn func() (T, error)) (T, error) {
	if err := os.MkdirAll(SpecdDir(root), 0o755); err != nil { //nolint:gosec // .specd holds non-secret project artifacts (see SECURITY.md)
		var zero T
		return zero, err
	}
	path := filepath.Join(SpecdDir(root), ".program.lock")
	deadline := time.Now().Add(timeoutMs())
	for !tryAcquire(path) {
		if isStale(path) {
			os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			var zero T
			return zero, GateError(fmt.Sprintf("program is locked by another specd process — retry shortly, or remove %s if it is stale", path))
		}
		time.Sleep(retryInterval)
	}
	defer os.Remove(path)
	return fn()
}

// lockHeldBy reports whether the current goroutine owns the spec lock for
// (root, slug). Used by SaveState's debug assertion to catch callers that
// mutate state outside WithSpecLock.
func lockHeldBy(root, slug string) bool {
	path := lockFilePath(root, slug)
	locksMu.Lock()
	defer locksMu.Unlock()
	ls, ok := locks[path]
	return ok && ls.depth > 0 && ls.owner == goID()
}
