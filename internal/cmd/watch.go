package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// watchPass loads every spec under root (optionally filtered to one slug),
// feeds each state through the detector, and writes a compact NDJSON line for
// every spec whose runnable frontier changed. It is read-only — it never calls
// SaveState — and returns the number of events emitted. A spec that fails to
// load is skipped with a stderr warning rather than aborting the whole pass, so
// one corrupt spec cannot silence the stream for the rest.
func watchPass(w io.Writer, root, specFilter string, det *core.FrontierDetector) (int, error) {
	slugs := core.ListSpecs(root)
	emitted := 0
	for _, slug := range slugs {
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
		ev, changed := det.Observe(state)
		if !changed {
			continue
		}
		line, err := json.Marshal(ev)
		if err != nil {
			return emitted, err
		}
		if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
			return emitted, err
		}
		emitted++
	}
	return emitted, nil
}

func watchInterval() time.Duration {
	ms := core.EnvInt("SPECD_WATCH_INTERVAL_MS", 1000, 50, 0)
	return time.Duration(ms) * time.Millisecond
}

// RunWatch streams runnable-frontier changes as NDJSON on stdout. With --once it
// performs a single pass over current state and exits (the deterministic mode
// used in tests and CI); without it, it polls at SPECD_WATCH_INTERVAL_MS. SSE /
// webhook transports and graceful signal shutdown land in later waves; this
// wave provides the read-only core loop + NDJSON emitter.
func RunWatch(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	specFilter := args.Str("spec")
	det := core.NewFrontierDetector()

	if args.Bool("once") {
		if _, err := watchPass(os.Stdout, root, specFilter, det); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	interval := watchInterval()
	for {
		if _, err := watchPass(os.Stdout, root, specFilter, det); err != nil {
			return specdExit(err)
		}
		time.Sleep(interval)
	}
}
