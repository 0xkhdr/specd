package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var lockDepths sync.Map // map[string]int

func numEnv(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func staleMs() time.Duration  { return time.Duration(numEnv("SPECD_LOCK_STALE_MS", 30_000)) * time.Millisecond }
func timeoutMs() time.Duration { return time.Duration(numEnv("SPECD_LOCK_TIMEOUT_MS", 5_000)) * time.Millisecond }

const retryInterval = 25 * time.Millisecond

func lockFilePath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), ".lock")
}

func tryAcquire(path string) bool {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	defer f.Close()
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

func WithSpecLock[T any](root, slug string, fn func() (T, error)) (T, error) {
	path := lockFilePath(root, slug)

	if v, ok := lockDepths.Load(path); ok {
		lockDepths.Store(path, v.(int)+1)
		defer func() {
			d, _ := lockDepths.Load(path)
			depth := d.(int) - 1
			if depth == 0 {
				lockDepths.Delete(path)
			} else {
				lockDepths.Store(path, depth)
			}
		}()
		return fn()
	}

	deadline := time.Now().Add(timeoutMs())
	for {
		if tryAcquire(path) {
			break
		}
		if isStale(path) {
			os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			var zero T
			return zero, GateError(fmt.Sprintf("spec '%s' is locked by another specd process — retry shortly, or remove %s if it is stale", slug, path))
		}
		time.Sleep(retryInterval)
	}

	lockDepths.Store(path, 1)
	defer func() {
		lockDepths.Delete(path)
		os.Remove(path)
	}()
	return fn()
}
