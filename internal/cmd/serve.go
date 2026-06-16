package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// NewServeHandler builds the read-only HTTP handler that serves a spec's live
// report. Every response is rebuilt from disk per request (the dashboard reflects
// the latest state) and the handler exposes no mutating routes: GET `/` renders
// the same HTML as `specd report --format html`, GET `/api/report` returns the
// JSON ReportData, non-GET methods get 405, and a missing spec/root yields 404.
// It is the unit of behaviour the serve tests exercise without binding a port.
func NewServeHandler(root, slug string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, ok := serveReportData(w, root, slug)
		if !ok {
			return
		}
		cfg := core.LoadConfig(root)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, core.RenderHTML(data, cfg.Report.AutoRefreshSeconds))
	})

	mux.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, ok := serveReportData(w, root, slug)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(data)
	})

	return mux
}

// serveReportData loads the report data for a request, writing a 404 and
// returning ok=false when the spec or its state is missing. It never panics on a
// missing spec/root — the error path is a clean HTTP status.
func serveReportData(w http.ResponseWriter, root, slug string) (core.ReportData, bool) {
	if err := core.RequireSpec(root, slug); err != nil {
		http.Error(w, "spec not found", http.StatusNotFound)
		return core.ReportData{}, false
	}
	state, err := core.LoadState(root, slug)
	if err != nil || state == nil {
		http.Error(w, "spec state not found", http.StatusNotFound)
		return core.ReportData{}, false
	}
	return loadReportData(root, slug, state), true
}

// RunServe starts the read-only dashboard server bound to loopback. It blocks
// serving until interrupted. The default address is 127.0.0.1:8765; override
// with --addr (loopback host recommended — the server is read-only but exposes
// spec contents).
func RunServe(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd serve <slug> [--addr 127.0.0.1:8765]")
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	addr := args.Str("addr")
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	handler := NewServeHandler(root, slug)
	fmt.Printf("specd serve: read-only dashboard for '%s' on http://%s/ (Ctrl-C to stop)\n", slug, addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		core.Error(err.Error())
		return core.ExitGate
	}
	return core.ExitOK
}
