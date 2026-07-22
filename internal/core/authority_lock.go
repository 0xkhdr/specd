package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const authorityLockFileName = "authority.lock"

var (
	authorityLocksMu sync.Mutex
	authorityLocks   = map[string]*lockState{}
)

// AuthorityDir is the project-wide delegation ledger directory. Grants are not
// per-spec — one grant can name several specs — so they live beside the specs
// rather than inside one of them.
func AuthorityDir(root string) string {
	return filepath.Join(SpecdDir(root), "authority")
}

func authorityLockPath(root string) string {
	return filepath.Join(AuthorityDir(root), authorityLockFileName)
}

// ErrLockOrder is the refusal raised when a caller asks for the authority lock
// while already holding the spec lock.
var ErrLockOrder = errors.New("lock order violation: acquire the authority lock before the spec lock")

// WithAuthorityLock serializes project-wide authority work (issuing, revoking,
// reserving, and consuming delegation grants) and permits reentry from the same
// goroutine, exactly as WithSpecLock does for one spec root.
//
// Lock order is total and documented: **authority lock, then spec lock.** A
// delegated approval needs both — it reserves a grant use and then commits a
// spec transition — and two orders would be a deadlock waiting for two
// concurrent approvals to interleave. Rather than trusting call sites to get it
// right, asking for the authority lock while already holding the spec lock
// fails with ErrLockOrder. A refusal is recoverable; a deadlock is not.
func WithAuthorityLock[T any](root string, fn func() (T, error)) (T, error) {
	var zero T
	root, err := filepath.Abs(root)
	if err != nil {
		return zero, err
	}
	state := authorityLockFor(root)
	gid := goroutineID()

	state.mu.Lock()
	reentrant := state.owner == gid
	if reentrant {
		state.depth++
	}
	state.mu.Unlock()
	if reentrant {
		defer authorityUnlock(root, state)
		return fn()
	}
	// Checked only on first acquisition: a reentrant call already holds the
	// authority lock, so it took both in the legal order by construction.
	if holdsSpecLock(gid) {
		return zero, ErrLockOrder
	}

	deadline := time.Now().Add(lockTimeout())
	for {
		acquired, err := acquireAuthorityLock(root)
		if err != nil {
			return zero, err
		}
		if acquired {
			break
		}
		if time.Now().After(deadline) {
			return zero, fmt.Errorf("authority lock timeout: %s", authorityLockPath(root))
		}
		time.Sleep(25 * time.Millisecond)
	}

	state.mu.Lock()
	state.owner, state.depth = gid, 1
	state.mu.Unlock()
	defer authorityUnlock(root, state)

	return fn()
}

// holdsSpecLock reports whether this goroutine already holds a spec lock for
// any root. Any root: the order is global, and a caller holding one root's spec
// lock while taking another root's authority lock is the same cycle.
func holdsSpecLock(gid uint64) bool {
	locksMu.Lock()
	states := make([]*lockState, 0, len(locks))
	for _, state := range locks {
		states = append(states, state)
	}
	locksMu.Unlock()
	for _, state := range states {
		state.mu.Lock()
		held := state.owner == gid && state.depth > 0
		state.mu.Unlock()
		if held {
			return true
		}
	}
	return false
}

func authorityLockFor(root string) *lockState {
	authorityLocksMu.Lock()
	defer authorityLocksMu.Unlock()
	if state := authorityLocks[root]; state != nil {
		return state
	}
	state := &lockState{}
	authorityLocks[root] = state
	return state
}

func authorityUnlock(root string, state *lockState) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.depth--
	if state.depth > 0 {
		return
	}
	state.owner = 0
	_ = os.Remove(authorityLockPath(root))
}

// acquireAuthorityLock mirrors acquireFileLock against the authority lock file,
// reusing the same stale-holder rules so an orphaned lock from a crashed
// process is reclaimed the same way.
func acquireAuthorityLock(root string) (bool, error) {
	if err := os.MkdirAll(AuthorityDir(root), 0o755); err != nil {
		return false, err
	}
	path := authorityLockPath(root)
	now := time.Now().UnixMilli()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err == nil {
		_, writeErr := fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), now)
		closeErr := f.Close()
		return writeErr == nil && closeErr == nil, errors.Join(writeErr, closeErr)
	}
	if !errors.Is(err, os.ErrExist) {
		return false, err
	}
	stale, err := lockIsStale(path, now)
	if err != nil {
		return false, err
	}
	if stale {
		_ = os.Remove(path)
	}
	return false, nil
}
