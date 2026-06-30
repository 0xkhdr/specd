package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// collectChanges runs one read-only pass over every spec under root (optionally
// filtered to one slug) and returns the FrontierEvents for specs whose runnable
// frontier changed since the detector last observed them. It never writes state.
// A spec that fails to load is skipped with a stderr warning so one corrupt spec
// cannot silence the stream for the rest.
func collectChanges(root, specFilter string, det *core.FrontierDetector) []core.FrontierEvent {
	var events []core.FrontierEvent
	for _, slug := range core.ListSpecs(root) {
		if specFilter != "" && slug != specFilter {
			continue
		}
		state, err := core.LoadState(root, slug)
		if err != nil {
			errLine("watch: skipping %s: %v", slug, err)
			continue
		}
		if state == nil {
			continue
		}
		if ev, changed := det.Observe(state); changed {
			events = append(events, ev)
		}
	}
	return events
}

// watchPass writes a compact NDJSON line for every changed frontier and returns
// the count emitted. Retained for the NDJSON-on-stdout path.
//
//nolint:unused // retained NDJSON-on-stdout helper, not yet wired to a command.
func watchPass(w io.Writer, root, specFilter string, det *core.FrontierDetector) (int, error) {
	events := collectChanges(root, specFilter, det)
	for _, ev := range events {
		if err := writeNDJSON(w, ev); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}

func writeNDJSON(w io.Writer, ev core.FrontierEvent) error {
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", line)
	return err
}

func watchInterval() time.Duration {
	ms := core.EnvInt("SPECD_WATCH_INTERVAL_MS", 1000, 50, 0)
	return time.Duration(ms) * time.Millisecond
}

// eventSink consumes frontier events. The stdout NDJSON writer and the webhook
// poster are both sinks, so the polling loop is transport-agnostic.
type eventSink interface{ Emit(core.FrontierEvent) }

type ndjsonSink struct{ w io.Writer }

func (s ndjsonSink) Emit(ev core.FrontierEvent) { _ = writeNDJSON(s.w, ev) }

// watchLoop polls at interval, emitting changed frontiers to every sink, until
// ctx is cancelled (a signal), at which point it returns cleanly. The initial
// pass runs immediately so a fresh watcher reports current frontiers at once.
func watchLoop(ctx context.Context, root, specFilter string, det *core.FrontierDetector, interval time.Duration, sinks []eventSink) int {
	for {
		for _, ev := range collectChanges(root, specFilter, det) {
			for _, s := range sinks {
				s.Emit(ev)
			}
		}
		select {
		case <-ctx.Done():
			return core.ExitOK
		case <-time.After(interval):
		}
	}
}

// runWatch streams runnable-frontier changes. Transports (highest precedence
// first): --sse serves Server-Sent Events over net/http; otherwise it emits
// NDJSON on stdout and, with --webhook, POSTs each event to a URL on a
// non-blocking background worker. --once does a single pass and exits. The
// long-running modes shut down cleanly on SIGINT/SIGTERM. Read-only throughout.
func runWatch(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	specFilter := args.Str("spec")

	if addr := args.Str("sse"); addr != "" {
		return runWatchSSE(addr, root, specFilter)
	}

	det := core.NewFrontierDetector()

	// Build sinks: stdout always; webhook optionally.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sinks := []eventSink{ndjsonSink{os.Stdout}}
	if url := args.Str("webhook"); url != "" {
		ws := newWebhookSink(url)
		defer ws.Close() // drains queued events before returning
		sinks = append(sinks, ws)
	}

	if args.Bool("once") {
		for _, ev := range collectChanges(root, specFilter, det) {
			for _, s := range sinks {
				s.Emit(ev)
			}
		}
		return core.ExitOK
	}

	return watchLoop(ctx, root, specFilter, det, watchInterval(), sinks)
}
