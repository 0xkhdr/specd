package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// seedRunnableSpec writes a spec on disk with one pending, dependency-free task,
// so its runnable frontier is non-empty. It avoids the test harness (which would
// create an import cycle: testharness imports cmd) by going straight to core.
func seedRunnableSpec(t *testing.T, root, slug string) {
	t.Helper()
	if err := os.MkdirAll(core.SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	st := core.InitialState(slug, slug)
	st.Status = core.StatusExecuting
	st.Tasks = map[string]core.TaskState{
		"T1": {ID: "T1", Title: "build", Role: "craftsman", Wave: 1, Status: core.TaskPending, Depends: []string{}, Requirements: []int{1}},
	}
	b := core.DefaultBackend()
	if err := b.WithLock(root, slug, func() error { return b.Save(root, slug, &st) }); err != nil {
		t.Fatal(err)
	}
}

// TestWatchSSE drives the SSE handler over httptest and confirms it streams a
// well-formed `data:` frame carrying the current frontier on connect.
func TestWatchSSE(t *testing.T) {
	root := t.TempDir()
	seedRunnableSpec(t, root, "alpha")

	srv := httptest.NewServer(sseHandler(root, ""))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	sc := bufio.NewScanner(resp.Body)
	var data string
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	cancel() // disconnect → handler returns

	var ev core.FrontierEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		t.Fatalf("SSE frame not valid JSON: %q (%v)", data, err)
	}
	if ev.Spec != "alpha" || len(ev.Frontier) == 0 {
		t.Errorf("unexpected SSE event: %+v", ev)
	}
}

// TestWatchWebhook confirms events are delivered by POST, that Close drains the
// queue, and that Emit is non-blocking against a hung endpoint.
func TestWatchWebhook(t *testing.T) {
	var got int32
	var mu sync.Mutex
	var bodies []core.FrontierEvent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev core.FrontierEvent
		_ = json.NewDecoder(r.Body).Decode(&ev)
		mu.Lock()
		bodies = append(bodies, ev)
		mu.Unlock()
		atomic.AddInt32(&got, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := newWebhookSink(srv.URL)
	sink.Emit(core.FrontierEvent{Spec: "alpha", Frontier: []string{"T1"}})
	sink.Emit(core.FrontierEvent{Spec: "beta", Frontier: []string{"T2"}})
	sink.Close() // drains before returning

	if n := atomic.LoadInt32(&got); n != 2 {
		t.Fatalf("webhook received %d events, want 2", n)
	}
	mu.Lock()
	if len(bodies) == 0 || bodies[0].Spec == "" {
		t.Errorf("webhook body not decoded: %+v", bodies)
	}
	mu.Unlock()

	// Non-blocking against a hung endpoint.
	hungDone := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-hungDone:
		case <-r.Context().Done():
		}
	}))
	defer hung.Close()
	defer close(hungDone)
	slow := newWebhookSink(hung.URL)
	defer slow.abort() // immediate shutdown: never drains against the hung endpoint
	done := make(chan struct{})
	go func() {
		for i := 0; i < webhookQueueSize+50; i++ {
			slow.Emit(core.FrontierEvent{Spec: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Emit blocked on a hung endpoint")
	}
}

// TestWatchShutdown confirms the polling loop returns cleanly when its context
// is cancelled (the signal path).
func TestWatchShutdown(t *testing.T) {
	root := t.TempDir()
	seedRunnableSpec(t, root, "alpha")

	ctx, cancel := context.WithCancel(context.Background())
	det := core.NewFrontierDetector()
	rc := make(chan int, 1)
	go func() {
		rc <- watchLoop(ctx, root, "", det, 10*time.Millisecond, []eventSink{ndjsonSink{discardWriter{}}})
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case code := <-rc:
		if code != core.ExitOK {
			t.Errorf("watchLoop exit = %d, want ExitOK", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watchLoop did not return after context cancel")
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
