package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunDashboard implements `specd dashboard` (V11/P6.2): the unified, read-only
// project dashboard. It renders every spec's status, orchestrator waves,
// conductor sessions, eval trends, cost, and escalations — plus the shared
// harness bundle — from local state and ledgers only, with zero outbound
// network. It is a project-wide alias over the `serve` machinery: the home page
// is the aggregate dashboard rather than a bare spec index, and it reuses the
// same SSE live-update stream, per-spec report routes, and loopback bind.
func RunDashboard(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	mode := args.Str("mode")
	if _, ok := core.NormalizeDashboardMode(mode); !ok {
		return specdExit(core.UsageError(fmt.Sprintf("dashboard: unknown --mode %q (want all|conductor|orchestrator|cost|eval)", mode)))
	}
	// A positional slug is the default per-spec report target; optional.
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	addr := args.Str("addr")
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	norm, _ := core.NormalizeDashboardMode(mode)
	handler := NewDashboardHandler(root, slug, mode)
	fmt.Printf("specd dashboard: read-only unified view (mode=%s) on http://%s/ (Ctrl-C to stop)\n", norm, addr)
	return serveHandler(addr, handler)
}

// NewDashboardHandler builds the unified dashboard handler. GET `/` renders the
// aggregate dashboard; GET `/s/<slug>` renders one spec's report (reusing the
// serve markup); GET `/api/dashboard` returns the JSON projection; `/events`
// mounts the shared SSE stream. Every response is rebuilt from disk per request
// and there are no mutating routes: non-GET → 405, missing spec → 404.
func NewDashboardHandler(root, slug, mode string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		// A per-request ?mode= override lets the single running server switch
		// panels without a restart; it defaults to the launch mode.
		m := r.URL.Query().Get("mode")
		if m == "" {
			m = mode
		}
		data, err := core.BuildDashboard(root, m)
		if err != nil {
			http.Error(w, "bad mode", http.StatusBadRequest)
			return
		}
		cfg := core.LoadConfig(root)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, core.RenderDashboardHTML(data, cfg.Report.AutoRefreshSeconds))
	})

	// GET /s/<slug> — per-spec live report HTML (same as serve).
	mux.HandleFunc("/s/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		want := strings.TrimPrefix(r.URL.Path, "/s/")
		if want == "" {
			http.Error(w, "spec not found", http.StatusNotFound)
			return
		}
		data, ok := serveReportData(w, root, want)
		if !ok {
			return
		}
		cfg := core.LoadConfig(root)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, core.RenderHTML(data, cfg.Report.AutoRefreshSeconds))
	})

	// GET /api/dashboard — JSON projection (deterministic, read-only).
	mux.HandleFunc("/api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		m := r.URL.Query().Get("mode")
		if m == "" {
			m = mode
		}
		data, err := core.BuildDashboard(root, m)
		if err != nil {
			http.Error(w, "bad mode", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(data)
	})

	mux.Handle("/events", sseHandler(root, ""))

	return mux
}
