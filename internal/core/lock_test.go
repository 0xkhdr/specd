package core

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestReentrantLock(t *testing.T) {
	root := t.TempDir()
	_, err := WithSpecLock(root, func() (struct{}, error) {
		if _, err := os.Stat(lockPath(root)); err != nil {
			t.Fatalf("lock file missing: %v", err)
		}
		_, err := WithSpecLock(root, func() (struct{}, error) {
			return struct{}{}, nil
		})
		return struct{}{}, err
	})
	if err != nil {
		t.Fatalf("WithSpecLock reentry: %v", err)
	}
	if _, err := os.Stat(lockPath(root)); !os.IsNotExist(err) {
		t.Fatalf("lock file after release err=%v", err)
	}

	t.Setenv("SPECD_LOCK_STALE_MS", "1")
	t.Setenv("SPECD_LOCK_TIMEOUT_MS", "250")
	if err := os.MkdirAll(SpecdDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-time.Second).UnixMilli()
	if err := os.WriteFile(filepath.Join(SpecdDir(root), lockFileName), []byte("999999\n"+strconv.FormatInt(stale, 10)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WithSpecLock(root, func() (struct{}, error) { return struct{}{}, nil }); err != nil {
		t.Fatalf("stale reclaim: %v", err)
	}
}
