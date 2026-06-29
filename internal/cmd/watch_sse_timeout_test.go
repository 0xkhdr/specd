package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// deadlineRecorder is a ResponseWriter that records http.ResponseController's
// SetWriteDeadline call so a test can prove the SSE handler clears the deadline.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	mu       sync.Mutex
	set      bool
	deadline time.Time
}

func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.set = true
	d.deadline = t
	return nil
}

func (d *deadlineRecorder) clearedZero() (bool, time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.set, d.deadline
}

// The dashboard/watch server carries a WriteTimeout (A1 R2.1) which would sever
// the long-lived /events SSE stream. The handler must clear its own write
// deadline so the bound never applies to the stream (R3.1). A zero deadline
// disables the bound — assert the handler sets exactly that.
func TestSSEHandlerClearsWriteDeadline(t *testing.T) {
	root := t.TempDir()
	rec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		sseHandler(root, "")(rec, req)
		close(done)
	}()

	// Spin until the handler has reached the deadline-clear, then stop it.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if set, _ := rec.clearedZero(); set {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("SSE handler never cleared its write deadline")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done

	set, dl := rec.clearedZero()
	if !set || !dl.IsZero() {
		t.Fatalf("write deadline cleared=%v value=%v; want cleared with zero time", set, dl)
	}
}
