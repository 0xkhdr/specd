package core

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestWithSpecLockReleasesOnReturn(t *testing.T) {
	// Arrange
	root := specRoot(t, "s")

	// Act
	_, err := WithSpecLock[int](root, "s", func() (int, error) { return 1, nil })
	if err != nil {
		t.Fatalf("WithSpecLock: %v", err)
	}

	// Assert: lock file removed after the critical section.
	if _, err := os.Stat(lockFilePath(root, "s")); !os.IsNotExist(err) {
		t.Errorf("lock file still present after release: %v", err)
	}
}

func TestWithSpecLockReentrant(t *testing.T) {
	// Arrange
	root := specRoot(t, "s")

	// Act: nested acquisition of the same lock must not deadlock.
	got, err := WithSpecLock[int](root, "s", func() (int, error) {
		return WithSpecLock[int](root, "s", func() (int, error) {
			return 42, nil
		})
	})

	// Assert
	if err != nil {
		t.Fatalf("reentrant lock: %v", err)
	}
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	if _, err := os.Stat(lockFilePath(root, "s")); !os.IsNotExist(err) {
		t.Errorf("lock leaked after reentrant release: %v", err)
	}
}

func TestWithSpecLockTimesOutWhenHeld(t *testing.T) {
	// Arrange: a fresh (non-stale) lock held by a "different process".
	root := specRoot(t, "s")
	t.Setenv("SPECD_LOCK_TIMEOUT_MS", "60")
	t.Setenv("SPECD_LOCK_STALE_MS", "60000")
	held := lockFilePath(root, "s")
	if err := os.WriteFile(held, []byte(fmt.Sprintf("99999 %d\n", time.Now().UnixMilli())), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := WithSpecLock[int](root, "s", func() (int, error) { return 1, nil })

	// Assert: blocked acquisition surfaces a gate error within the timeout.
	if se, ok := IsSpecdError(err); !ok || se.Code != ExitGate {
		t.Errorf("err = %v, want gate error on lock timeout", err)
	}
}

func TestWithSpecLockReclaimsStaleLock(t *testing.T) {
	// Arrange: a lock whose timestamp is far older than the stale threshold.
	root := specRoot(t, "s")
	t.Setenv("SPECD_LOCK_STALE_MS", "50")
	t.Setenv("SPECD_LOCK_TIMEOUT_MS", "2000")
	old := time.Now().Add(-time.Hour).UnixMilli()
	if err := os.WriteFile(lockFilePath(root, "s"), []byte(fmt.Sprintf("88888 %d\n", old)), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act: a live process should reclaim the abandoned lock.
	got, err := WithSpecLock[int](root, "s", func() (int, error) { return 7, nil })

	// Assert
	if err != nil {
		t.Fatalf("stale reclaim failed: %v", err)
	}
	if got != 7 {
		t.Errorf("got %d, want 7", got)
	}
}
