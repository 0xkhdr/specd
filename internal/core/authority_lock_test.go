package core

import (
	"os"
	"sync"
	"testing"
	"time"
)

// TestDelegationGrantLockOrder pins the total lock order (design: authority
// lock before spec lock). Both locks are reentrant and each is project-wide, so
// without an order the first delegated approval to interleave with ordinary
// spec work would deadlock — and a deadlock in a harness that holds a file lock
// is not something a retry clears.
func TestDelegationGrantLockOrder(t *testing.T) {
	root := t.TempDir()

	t.Run("authoritythenspecisallowed", func(t *testing.T) {
		_, err := WithAuthorityLock(root, func() (struct{}, error) {
			return WithSpecLock(root, func() (struct{}, error) { return struct{}{}, nil })
		})
		if err != nil {
			t.Fatalf("legal order refused: %v", err)
		}
	})

	t.Run("specthenauthorityisrefused", func(t *testing.T) {
		_, err := WithSpecLock(root, func() (struct{}, error) {
			return WithAuthorityLock(root, func() (struct{}, error) { return struct{}{}, nil })
		})
		if err != ErrLockOrder {
			t.Fatalf("reversed order = %v, want ErrLockOrder", err)
		}
	})

	t.Run("authorityisreentrant", func(t *testing.T) {
		depth := 0
		_, err := WithAuthorityLock(root, func() (struct{}, error) {
			depth++
			return WithAuthorityLock(root, func() (struct{}, error) {
				depth++
				// Reentry already holds the lock in the legal order, so the
				// order guard must not fire on a nested acquisition.
				return struct{}{}, nil
			})
		})
		if err != nil || depth != 2 {
			t.Fatalf("reentrant acquisition: depth=%d err=%v", depth, err)
		}
		if _, err := os.Stat(authorityLockPath(root)); !os.IsNotExist(err) {
			t.Fatal("authority lock file survived the outermost release")
		}
	})

	t.Run("authorityexcludesothergoroutines", func(t *testing.T) {
		var mu sync.Mutex
		inside, max := 0, 0
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := WithAuthorityLock(root, func() (struct{}, error) {
					mu.Lock()
					inside++
					if inside > max {
						max = inside
					}
					mu.Unlock()
					time.Sleep(5 * time.Millisecond)
					mu.Lock()
					inside--
					mu.Unlock()
					return struct{}{}, nil
				})
				if err != nil {
					t.Errorf("concurrent acquisition: %v", err)
				}
			}()
		}
		wg.Wait()
		if max != 1 {
			t.Fatalf("%d goroutines held the authority lock at once", max)
		}
	})
}

// TestDelegationGrantConcurrentUseHasOneWinner pins R2.4: use accounting is
// serialized by the authority lock, so the last use of a grant has exactly one
// winner however many consumers race for it.
func TestDelegationGrantConcurrentUseHasOneWinner(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	grant, token := issueSampleGrant(t, root, now)

	var wg sync.WaitGroup
	results := make([]error, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := sampleRequest(grant, token, "req-"+string(rune('a'+i)))
			results[i] = ReserveGrantUse(root, delegationConfig(), req, grant.ID, now)
		}()
	}
	wg.Wait()

	winners := 0
	for _, err := range results {
		if err == nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("%d consumers reserved the single available use", winners)
	}
	projection, err := LoadGrant(root, grant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Uses() != 1 {
		t.Fatalf("uses = %d after a race, want 1", projection.Uses())
	}
}
