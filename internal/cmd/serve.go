package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// Dashboard/watch HTTP server timeout bounds (A1). Static report responses are
// safe to bound with a WriteTimeout; the long-lived /events SSE stream clears
// its own write deadline (see sseHandler) so the bound never severs it.
const (
	serveReadHeaderTimeout = 10 * time.Second
	serveWriteTimeout      = 60 * time.Second
	serveIdleTimeout       = 60 * time.Second
)

// NewServeHandler builds the read-only, browser-native dashboard handler for a
// project. It is multi-spec: GET `/` renders an index of every spec under the
// project, GET `/s/<slug>` renders that spec's live report HTML (the same markup
// as `specd report --format html`), GET `/api/report?spec=<slug>` returns the
// JSON ReportData, and `/events` mounts the reused frontier SSE stream for live
// updates. The `slug` argument is the default spec used when no `?spec=` query is
// supplied (and the default link target on the index). Every response is rebuilt
// from disk per request so the view always reflects current state, and the
// handler exposes no mutating routes: non-GET methods get 405 and a missing spec
// yields 404. It is the unit of behaviour the serve tests exercise without
// binding a port.
func NewServeHandler(root, slug string) http.Handler {
	mux := http.NewServeMux()

	// GET / — spec index listing every spec under the project (R2.1).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, renderIndexHTML(root))
	})

	// GET /s/<slug> — live report HTML for one spec (R2.2).
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
		io.WriteString(w, core.RenderHTML(data, cfg.Report.AutoRefreshSeconds))
	})

	// GET /api/report?spec=<slug> — JSON ReportData (R2/R4); defaults to slug.
	mux.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		want := r.URL.Query().Get("spec")
		if want == "" {
			want = slug
		}
		data, ok := serveReportData(w, root, want)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(data)
	})

	// /events — reused frontier SSE stream for live updates (R4). The stream is
	// read-only and project-wide; the page filters to the viewed spec.
	mux.Handle("/events", sseHandler(root, ""))

	return mux
}

// methodNotAllowed writes the shared 405 response for the read-only dashboard.
func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// renderIndexHTML builds a self-contained spec index page from disk. Each spec's
// status/phase is read from its state.json so the index always reflects current
// state; specs with unreadable state are still listed by slug.
func renderIndexHTML(root string) string {
	var rows strings.Builder
	specs := core.ListSpecs(root)
	if len(specs) == 0 {
		rows.WriteString(`    <p class="meta">No specs found under .specd/specs/.</p>` + "\n")
	} else {
		rows.WriteString("    <ul>\n")
		for _, s := range specs {
			title, status, phase := s, "", ""
			if st, err := core.LoadState(root, s); err == nil && st != nil {
				title = st.Title
				status = string(st.Status)
				phase = string(st.Phase)
			}
			meta := ""
			if status != "" {
				meta = fmt.Sprintf(` <span class="meta">— %s · %s</span>`, esc(status), esc(phase))
			}
			fmt.Fprintf(&rows, "      <li><a href=\"/s/%s\">%s</a>%s</li>\n",
				esc(s), esc(title), meta)
		}
		rows.WriteString("    </ul>\n")
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>specd dashboard</title>
  <style>
    body { font: 15px/1.55 system-ui, sans-serif; max-width: 920px; margin: 2rem auto; padding: 0 1rem; color: #c9d1d9; background: #0d1117; }
    h1 { font-size: 1.6rem; }
    a { color: #58a6ff; text-decoration: none; }
    a:hover { text-decoration: underline; }
    ul { list-style: none; padding: 0; }
    li { padding: .5rem .75rem; margin: .3rem 0; background: #161b22; border-radius: 6px; }
    .meta { color: #8b949e; font-size: .9rem; }
    @media (max-width: 600px) { body { margin: 1rem auto; } }
  </style>
</head>
<body>
  <h1>specd dashboard</h1>
%s</body>
</html>
`, rows.String())
}

// esc is the package-local HTML escaper shared with core's report renderer.
func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
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

// runServe starts the read-only dashboard server bound to loopback. It blocks
// serving until interrupted. The default address is 127.0.0.1:8765; override
// with --addr (loopback host recommended — the server is read-only but exposes
// spec contents).
func runServe(args cli.Args) int {
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
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: serveReadHeaderTimeout,
		WriteTimeout:      serveWriteTimeout,
		IdleTimeout:       serveIdleTimeout,
	}
	if err := srv.ListenAndServe(); err != nil {
		core.Error(err.Error())
		return core.ExitGate
	}
	return core.ExitOK
}
