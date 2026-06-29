package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// sseHandler returns an http.Handler that streams frontier changes as
// Server-Sent Events. Each connection gets its own FrontierDetector, so a newly
// connected client receives the current frontiers immediately, then deltas as
// they occur. The handler is read-only and ends when the client disconnects
// (request context cancelled). Exposed as a handler so it is testable over
// httptest without binding a real port.
func sseHandler(root, specFilter string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		// This stream is long-lived: clear any server WriteTimeout deadline so the
		// dashboard/watch write bound never severs it mid-stream (A1 R3).
		_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		det := core.NewFrontierDetector()
		interval := watchInterval()
		ctx := r.Context()

		emit := func() {
			for _, ev := range collectChanges(root, specFilter, det) {
				line, err := json.Marshal(ev)
				if err != nil {
					continue
				}
				// SSE frame: one `data:` line per event, terminated by a blank line.
				fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			}
		}

		emit() // initial snapshot
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				emit()
			}
		}
	}
}

// runWatchSSE serves the SSE stream at addr until SIGINT/SIGTERM, then shuts the
// server down gracefully.
func runWatchSSE(addr, root, specFilter string) int {
	mux := http.NewServeMux()
	mux.Handle("/events", sseHandler(root, specFilter))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: serveReadHeaderTimeout,
		WriteTimeout:      serveWriteTimeout,
		IdleTimeout:       serveIdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	core.Info(fmt.Sprintf("specd watch: SSE stream at http://%s/events (Ctrl-C to stop)", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return specdExit(core.GateError(fmt.Sprintf("watch SSE: %v", err)))
	}
	return core.ExitOK
}
