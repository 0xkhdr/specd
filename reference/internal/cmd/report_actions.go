package cmd

// report_actions.go holds the sub-actions dispatched by RunReport in report.go:
// the read-only replay/diff renderers and the serve/watch/SSE live-view servers.
// They were consolidated here (from the former replay.go, diff.go, serve.go,
// watch.go, watch_sse.go) so the report cluster lives in one file while
// report.go keeps only the dispatcher and report-data assembly.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// runReplay prints a spec's deterministic, audit-derived event timeline. It is
// strictly read-only: it loads state and renders, never mutating anything. Text
// by default; a typed JSON array under SPECD_JSON.
func runReplay(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd replay <slug>")
	if !ok {
		return code
	}
	if sessionID := args.Str("acp-session"); sessionID != "" {
		store, err := core.NewACPStore(root)
		if err != nil {
			return specdExit(err)
		}
		events, err := store.ReplaySessionEvents(sessionID)
		if err != nil {
			return specdExit(err)
		}
		timeline := core.ReplaySessionTimeline(events)
		if core.IsJSONMode() {
			if timeline == nil {
				timeline = []core.SessionTimelineEvent{}
			}
			if err := core.PrintJSON(timeline); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		fmt.Printf("acp replay — %s (%d event%s)\n", sessionID, len(timeline), plural(len(timeline)))
		for _, event := range timeline {
			fmt.Printf(" %s\n", core.FormatSessionTimelineEvent(event))
		}
		return core.ExitOK
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	state, err := core.LoadState(root, slug)
	if err != nil {
		return specdExit(err)
	}
	if state == nil {
		return specdExit(core.NotFoundError(fmt.Sprintf("no state for spec '%s'", slug)))
	}

	events := core.ReplayTimeline(state)

	if core.IsJSONMode() {
		if events == nil {
			events = []core.TimelineEvent{}
		}
		if err := core.PrintJSON(events); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	fmt.Printf("replay — %s (%d event%s)\n", slug, len(events), plural(len(events)))
	for _, e := range events {
		at := e.At
		if at == "" {
			at = "(no timestamp)"
		}
		task := ""
		if e.Task != "" {
			task = " " + e.Task
		}
		fmt.Printf("  %s  %-14s%s  %s\n", at, e.Kind, task, e.Detail)
	}
	return core.ExitOK
}

// ArtifactChange is one artifact file that changed between two git refs.
type ArtifactChange struct {
	Path   string `json:"path"`   // repo-relative path
	Status string `json:"status"` // "added" | "modified" | "deleted" | "renamed"
}

// runDiff shows how a spec's on-disk artifacts changed between two git refs. It
// is strictly read-only — a thin, deterministic wrapper over `git diff
// --name-status` scoped to the spec directory. `--from` is required; `--to`
// defaults to the working tree. Bad refs or a non-git repo are reported as
// errors, never panics, and a spec with no changes is an empty (not failing)
// result. Text by default; a typed JSON object under SPECD_JSON.
func runDiff(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd diff <slug> --from <ref> [--to <ref>]")
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	from := strings.TrimSpace(args.Str("from"))
	if from == "" {
		return usageExit("usage: specd diff <slug> --from <ref> [--to <ref>]")
	}
	to := strings.TrimSpace(args.Str("to"))

	// Scope the diff to the spec's artifact directory (repo-relative pathspec).
	pathspec := ".specd/specs/" + slug

	gitArgs := []string{"-C", root, "diff", "--name-status", "--no-color", from}
	if to != "" {
		gitArgs = append(gitArgs, to)
	}
	gitArgs = append(gitArgs, "--", pathspec)

	out, err := exec.Command("git", gitArgs...).CombinedOutput() //nolint:gosec // git is a fixed binary; args are specd-built refs/pathspecs, not a shell string (see SECURITY.md)
	if err != nil {
		return specdExit(core.GateError(fmt.Sprintf("git diff failed (is this a git repo, and are %q/%q valid refs?): %s", from, to, strings.TrimSpace(string(out)))))
	}

	changes := parseNameStatus(string(out))

	if core.IsJSONMode() {
		if changes == nil {
			changes = []ArtifactChange{}
		}
		if err := core.PrintJSON(struct {
			Spec    string           `json:"spec"`
			From    string           `json:"from"`
			To      string           `json:"to"`
			Changes []ArtifactChange `json:"changes"`
		}{slug, from, toOrWorkingTree(to), changes}); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	fmt.Printf("diff — %s  %s..%s  (%d artifact change%s)\n", slug, from, toOrWorkingTree(to), len(changes), plural(len(changes)))
	for _, c := range changes {
		fmt.Printf("  %-9s %s\n", c.Status, c.Path)
	}
	return core.ExitOK
}

func toOrWorkingTree(to string) string {
	if to == "" {
		return "(working tree)"
	}
	return to
}

// parseNameStatus turns `git diff --name-status` output into a sorted, stable
// slice of ArtifactChange. Unknown status letters are passed through verbatim so
// the output never silently drops a change. Output is sorted by path for
// determinism regardless of git's internal ordering.
func parseNameStatus(raw string) []ArtifactChange {
	var changes []ArtifactChange
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		code := fields[0]
		// Renames/copies are "R100"/"C75" with old and new paths; report the new path.
		path := fields[len(fields)-1]
		changes = append(changes, ArtifactChange{Path: path, Status: statusWord(code)})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

func statusWord(code string) string {
	if code == "" {
		return "changed"
	}
	switch code[0] {
	case 'A':
		return "added"
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	default:
		return strings.ToLower(code)
	}
}

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
	return serveHandler(addr, handler)
}

// serveHandler binds handler to addr with the shared read-only-server timeout
// bounds and blocks serving until interrupted. It is the single listen path for
// both `serve` and the unified `dashboard`.
func serveHandler(addr string, handler http.Handler) int {
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
