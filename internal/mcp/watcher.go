package mcp

import (
	"context"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// phaseMode reports whether the loaded config selects expose:"phase" — the only
// mode that advertises a mutable tool list and runs the watcher (dynamic-tool-
// list spec R1/R6).
func phaseMode(cfg *core.Config) bool {
	return cfg != nil && cfg.MCP.Expose == "phase"
}

// phaseWatcher polls the project's specs for lifecycle-status changes and keeps a
// toolRegistry in sync with the active phase's subset, pushing one
// notifications/tools/list_changed per settled change (dynamic-tool-list §5.4).
// interval/debounce are fields so tests can drive transitions deterministically.
type phaseWatcher struct {
	registry *toolRegistry
	cfg      *core.Config
	// notify is called after a swap to tell the host to re-fetch tools/list. It is
	// nil on transports without a server→client push channel (HTTP/SSE), where the
	// registry still updates but the host must poll.
	notify   func()
	interval time.Duration
	debounce time.Duration
}

// defaultWatchInterval reuses the same env knob as `specd watch` so operators
// tune both pollers with one setting (spec §3). defaultDebounce collapses a burst
// of rapid transitions into a single notification (R7).
func defaultWatchInterval() time.Duration {
	ms := core.EnvInt("SPECD_WATCH_INTERVAL_MS", 1000, 50, 0)
	return time.Duration(ms) * time.Millisecond
}

const defaultPhaseDebounce = 250 * time.Millisecond

// startPhaseWatcher seeds the registry with the current phase subset and launches
// the polling goroutine. It returns immediately; the goroutine exits when ctx is
// cancelled (R5). Callers must only invoke it in phase mode (R6).
func startPhaseWatcher(ctx context.Context, registry *toolRegistry, cfg *core.Config, notify func()) {
	w := &phaseWatcher{
		registry: registry,
		cfg:      cfg,
		notify:   notify,
		interval: defaultWatchInterval(),
		debounce: defaultPhaseDebounce,
	}
	go w.run(ctx)
}

// run is the poll loop. It seeds the initial subset without notifying (the host
// fetched it via tools/list already), then on every detected status change waits
// out the debounce window, re-reads the settled status, swaps the registry, and
// notifies exactly once (AC2/AC3/R7). Cancellation returns cleanly (R5).
func (w *phaseWatcher) run(ctx context.Context) {
	last, hasLast := activeStatus()
	if hasLast {
		w.registry.swap(buildPhaseTools(w.cfg, last))
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, ok := activeStatus()
			if !ok || (hasLast && status == last) {
				continue
			}
			// Debounce: let a burst of transitions settle, then act on the final
			// status so the host sees one notification, not one per intermediate step.
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.debounce):
			}
			settled, ok := activeStatus()
			if !ok {
				continue
			}
			last, hasLast = settled, true
			w.registry.swap(buildPhaseTools(w.cfg, settled))
			if w.notify != nil {
				w.notify()
			}
		}
	}
}

// activeStatus resolves the project's representative lifecycle status: the
// furthest-along in-progress spec (executing dominates planning), tie-broken by
// the deterministic slug order ListSpecs returns. It returns false when no spec
// has loadable state, leaving the seeded list untouched. Read-only throughout.
func activeStatus() (core.SpecStatus, bool) {
	_, _, status, ok := activeSpec()
	return status, ok
}

// activeSpec resolves the project root plus the furthest-along in-progress spec
// — the same spec activeStatus/the phase watcher track (statusRank ordering,
// slug tie-break). It is the active context the context-manifest filter (C1)
// reads its per-spec tool policy from. Read-only; ok=false when no .specd root
// or no loadable spec state exists.
func activeSpec() (root, slug string, status core.SpecStatus, ok bool) {
	root, found := core.FindSpecdRoot("")
	if !found {
		return "", "", "", false
	}
	bestRank := -1
	for _, s := range core.ListSpecs(root) {
		state, err := core.LoadState(root, s)
		if err != nil || state == nil {
			continue
		}
		if r := statusRank(state.Status); r > bestRank {
			bestRank, slug, status = r, s, state.Status
		}
	}
	if bestRank < 0 {
		return "", "", "", false
	}
	return root, slug, status, true
}

// statusRank orders lifecycle statuses by how "active" the work is so the tool
// surface tracks the spec a user is most likely driving. Executing outranks
// planning; complete sits at the bottom.
func statusRank(s core.SpecStatus) int {
	switch s {
	case core.StatusExecuting:
		return 5
	case core.StatusVerifying, core.StatusBlocked:
		return 4
	case core.StatusTasks:
		return 3
	case core.StatusDesign:
		return 2
	case core.StatusRequirements:
		return 1
	default: // complete and any future status
		return 0
	}
}
