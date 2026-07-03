package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// webhookQueueSize bounds the in-flight event buffer. A slow or dead endpoint
// can never block the read-only polling loop: when the buffer is full, Emit
// drops the event with a warning rather than applying backpressure.
const webhookQueueSize = 256

// webhookSink POSTs each frontier event to a URL on a dedicated worker goroutine
// with bounded retry/backoff. Emit is non-blocking. Close stops the worker after
// draining any queued events, so a --once run still delivers what it produced.
type webhookSink struct {
	url    string
	ch     chan core.FrontierEvent
	stop   chan struct{}
	done   chan struct{}
	client *http.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func newWebhookSink(url string) *webhookSink {
	ctx, cancel := context.WithCancel(context.Background())
	s := &webhookSink{
		url:    url,
		ch:     make(chan core.FrontierEvent, webhookQueueSize),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		client: &http.Client{Timeout: 10 * time.Second},
		ctx:    ctx,
		cancel: cancel,
	}
	go s.run()
	return s
}

func (s *webhookSink) Emit(ev core.FrontierEvent) {
	select {
	case s.ch <- ev:
	default:
		errLine("watch: webhook queue full — dropping frontier event for %s", ev.Spec)
	}
}

// Close signals the worker to drain queued events and stop, then waits for it.
// Used on a healthy shutdown (e.g. --once) so produced events are delivered.
func (s *webhookSink) Close() {
	close(s.stop)
	<-s.done
}

// abort cancels any in-flight and pending posts immediately, then stops the
// worker without waiting on the network — the non-graceful shutdown path.
func (s *webhookSink) abort() {
	s.cancel()
	s.Close()
}

func (s *webhookSink) run() {
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			s.drain()
			return
		case ev := <-s.ch:
			s.post(ev)
		}
	}
}

// drain flushes whatever is currently queued (non-blocking) on shutdown.
func (s *webhookSink) drain() {
	for {
		select {
		case ev := <-s.ch:
			s.post(ev)
		default:
			return
		}
	}
}

// post delivers one event, retrying with exponential backoff up to a small,
// bounded number of attempts. It owns its own context so it cannot be wedged by
// a stuck endpoint beyond the client timeout × attempts.
func (s *webhookSink) post(ev core.FrontierEvent) {
	body, err := json.Marshal(ev)
	if err != nil {
		return
	}
	backoff := 100 * time.Millisecond
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if s.ctx.Err() != nil {
			return // aborted: do not keep trying during shutdown
		}
		req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, s.url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err == nil {
			ok := resp.StatusCode < 400
			resp.Body.Close()
			if ok {
				return
			}
		}
		if attempt < maxAttempts {
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
		}
	}
	errLine("watch: webhook POST to %s failed after %d attempts (spec %s)", s.url, maxAttempts, ev.Spec)
}
