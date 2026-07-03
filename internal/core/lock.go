package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const lockFileName = "specd.lock"

var (
	locksMu sync.Mutex
	locks   = map[string]*lockState{}
)

type lockState struct {
	mu    sync.Mutex
	owner uint64
	depth int
}

// WithSpecLock serializes harness work for one spec root and permits reentry
// from the same goroutine.
func WithSpecLock[T any](root string, fn func() (T, error)) (T, error) {
	var zero T
	root, err := filepath.Abs(root)
	if err != nil {
		return zero, err
	}
	state := lockFor(root)
	gid := goroutineID()

	state.mu.Lock()
	if state.owner == gid {
		state.depth++
		state.mu.Unlock()
		defer unlock(root, state)
		return fn()
	}
	state.mu.Unlock()

	deadline := time.Now().Add(lockTimeout())
	for {
		acquired, err := acquireFileLock(root)
		if err != nil {
			return zero, err
		}
		if acquired {
			break
		}
		if time.Now().After(deadline) {
			return zero, fmt.Errorf("spec lock timeout: %s", lockPath(root))
		}
		time.Sleep(25 * time.Millisecond)
	}

	state.mu.Lock()
	state.owner = gid
	state.depth = 1
	state.mu.Unlock()
	defer unlock(root, state)

	return fn()
}

func lockFor(root string) *lockState {
	locksMu.Lock()
	defer locksMu.Unlock()
	if state := locks[root]; state != nil {
		return state
	}
	state := &lockState{}
	locks[root] = state
	return state
}

func unlock(root string, state *lockState) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.depth--
	if state.depth > 0 {
		return
	}
	state.owner = 0
	_ = os.Remove(lockPath(root))
}

func acquireFileLock(root string) (bool, error) {
	if err := os.MkdirAll(SpecdDir(root), 0o755); err != nil {
		return false, err
	}
	path := lockPath(root)
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

func lockIsStale(path string, now int64) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return true, nil
	}
	then, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return true, nil
	}
	return time.Duration(now-then)*time.Millisecond > lockStale(), nil
}

func lockPath(root string) string {
	return filepath.Join(SpecdDir(root), lockFileName)
}

func lockTimeout() time.Duration {
	return envDuration("SPECD_LOCK_TIMEOUT_MS", 5000)
}

func lockStale() time.Duration {
	return envDuration("SPECD_LOCK_STALE_MS", 30000)
}

func envDuration(key string, def int64) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return time.Duration(def) * time.Millisecond
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return time.Duration(def) * time.Millisecond
	}
	return time.Duration(n) * time.Millisecond
}

func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	line := bytes.TrimPrefix(buf[:n], []byte("goroutine "))
	idField, _, _ := bytes.Cut(line, []byte(" "))
	id, _ := strconv.ParseUint(string(idField), 10, 64)
	return id
}
