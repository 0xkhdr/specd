package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Badge is a display label and hex color used to render a spec's status in
// reports.
type Badge struct {
	Label string
	Color string
}

// GetBadge maps a spec status to its display Badge, falling back to an
// "Unknown" badge for any status it does not recognize.
func GetBadge(status SpecStatus) Badge {
	switch status {
	case StatusRequirements, StatusDesign, StatusTasks:
		return Badge{"Planning", "#a371f7"}
	case StatusExecuting:
		return Badge{"Implementing", "#d29922"}
	case StatusVerifying:
		return Badge{"Verifying", "#58a6ff"}
	case StatusComplete:
		return Badge{"Complete", "#3fb950"}
	case StatusBlocked:
		return Badge{"Blocked", "#f85149"}
	}
	return Badge{"Unknown", "#888"}
}

// ReportData bundles a spec's State with the raw markdown of its planning and
// supporting artifacts, ready for rendering by RenderMarkdown or RenderHTML.
type ReportData struct {
	State        *State
	Requirements *string
	Design       *string
	Tasks        *string
	Decisions    *string
	Memory       *string
	MidReqs      *string
}

// sectionRECache memoizes the per-heading section regex. Headings come from a
// fixed set ("Introduction", "Overview", ...), so the cache stays bounded and
// avoids recompiling the same regex on every ExtractSection call.
var (
	sectionREMu    sync.Mutex
	sectionRECache = map[string]*regexp.Regexp{}
)

func sectionRE(heading string) *regexp.Regexp {
	sectionREMu.Lock()
	defer sectionREMu.Unlock()
	if re, ok := sectionRECache[heading]; ok {
		return re
	}
	re := regexp.MustCompile(`(?i)^##\s+` + regexp.QuoteMeta(heading))
	sectionRECache[heading] = re
	return re
}

// ExtractSection returns the body text of the first "## heading" section in
// md (case-insensitive), stopping at the next "## " heading or end of
// document. It returns nil if md is nil, the heading is not found, or the
// extracted body is empty after trimming.
func ExtractSection(md *string, heading string) *string {
	if md == nil {
		return nil
	}
	lines := splitLines(*md)
	re := sectionRE(heading)
	start := -1
	for i, l := range lines {
		if re.MatchString(l) {
			start = i
			break
		}
	}
	if start == -1 {
		return nil
	}
	var body []string
	for i := start + 1; i < len(lines); i++ {
		if len(lines[i]) >= 3 && lines[i][:3] == "## " {
			break
		}
		body = append(body, lines[i])
	}
	s := strings.TrimSpace(strings.Join(body, "\n"))
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

func execSummary(d ReportData) string {
	if s := ExtractSection(d.Requirements, "Introduction"); s != nil {
		return *s
	}
	if s := ExtractSection(d.Design, "Overview"); s != nil {
		return *s
	}
	return "_No summary provided._"
}

func progressOverview(state *State) string {
	c := CountTasks(state)
	cards := fmt.Sprintf("**%d** complete · **%d** running · **%d** pending · **%d** blocked · **%d** total",
		c.Complete, c.Running, c.Pending, c.Blocked, c.Total)
	return cards + "\n\n```\n" + WaveGraph(state) + "\n```"
}

type reportSection struct {
	Icon  string
	Title string
	Body  string
}

func buildSections(d ReportData) []reportSection {
	s := []reportSection{
		{"📝", "Executive Summary", execSummary(d)},
		{"📊", "Progress Overview", progressOverview(d.State)},
		{"📋", "Requirements", deref(d.Requirements, "_None._")},
		{"🗺️", "Plan / Design", deref(d.Design, "_None._")},
		{"✅", "Tasks", deref(d.Tasks, "_None._")},
		{"🔄", "Mid-Requirements", deref(d.MidReqs, "_None._")},
		{"🧠", "Build Knowledge", deref(d.Memory, "_None._")},
		{"📓", "Decisions", deref(d.Decisions, "_None._")},
	}
	if body := tokenEconomics(d.State); body != "" {
		s = append(s, reportSection{"💸", "Token Economics", body})
	}
	if len(d.State.Acceptance) > 0 {
		keys := make([]string, 0, len(d.State.Acceptance))
		for k := range d.State.Acceptance {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		var lines []string
		for _, k := range keys {
			rec := d.State.Acceptance[k]
			icon := "✅"
			if rec.Status != "pass" {
				icon = "❌"
			}
			lines = append(lines, fmt.Sprintf("- %s **%s** _(req %d)_ — %s", icon, k, rec.Requirement, rec.Evidence))
		}
		s = append(s, reportSection{"🧪", "Acceptance Criteria", strings.Join(lines, "\n")})
	}
	if body := verificationEvidence(d.State); body != "" {
		s = append(s, reportSection{"🔬", "Verification Evidence", body})
	}
	if body := telemetrySection(d.State); body != "" {
		s = append(s, reportSection{"⏱️", "Telemetry", body})
	}
	if len(d.State.Blockers) > 0 {
		var lines []string
		for _, b := range d.State.Blockers {
			lines = append(lines, fmt.Sprintf("- **%s** — %s _(since %s)_", b.Task, b.Reason, b.Since))
		}
		s = append(s, reportSection{"🚧", "Blockers", strings.Join(lines, "\n")})
	}
	return s
}

// verificationEvidence renders the per-task verify evidence (changed-file count
// and coverage) captured by `specd verify`. It is evidence reporting only —
// coverage is shown as data, never as a pass/fail floor. Tasks without a verify
// record are omitted; an empty body suppresses the whole section.
func tokenEconomics(state *State) string {
	if state == nil || len(state.Routing) == 0 {
		return ""
	}
	e := RoutingEconomicsFromState(state)
	if len(e.ByTier) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Total: **%s USD** · **%d** tokens\n\n", e.TotalCostUSD, e.TotalTokens))
	b.WriteString("| Tier | Tasks | Tokens | Cost USD |\n")
	b.WriteString("|---|---:|---:|---:|\n")
	tiers := make([]string, 0, len(e.ByTier))
	for tier := range e.ByTier {
		tiers = append(tiers, tier)
	}
	sort.Strings(tiers)
	for _, tier := range tiers {
		row := e.ByTier[tier]
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %s |\n", tier, row.Tasks, row.Tokens, row.CostUSD))
	}
	return strings.TrimRight(b.String(), "\n")
}

func verificationEvidence(state *State) string {
	ids := make([]string, 0, len(state.Tasks))
	for id, t := range state.Tasks {
		if t.Verification != nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return ""
	}
	sort.Slice(ids, func(i, j int) bool { return ordinal(ids[i]) < ordinal(ids[j]) })
	lines := []string{"| Task | Verified | Coverage | Changed files |", "|------|----------|----------|---------------|"}
	for _, id := range ids {
		v := state.Tasks[id].Verification
		mark := "✅"
		if !v.Verified {
			mark = "❌"
		}
		cov := v.Coverage
		if cov == "" {
			cov = "—"
		}
		sandbox := ""
		if v.Sandbox != "" {
			sandbox = " _(sandbox: " + v.Sandbox + ")_"
		}
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %d%s |", id, mark, cov, len(v.ChangedFiles), sandbox))
	}
	return strings.Join(lines, "\n")
}

// telemetrySection renders the per-wave and per-spec telemetry roll-up. Returns
// "" when no task carried telemetry, so specs without telemetry stay unchanged.
func telemetrySection(state *State) string {
	roll := RollupTelemetry(state)
	if !roll.HasData() {
		return ""
	}
	lines := []string{"| Wave | Tasks | Duration | Verify | Retries | Tokens | Cost |", "|------|-------|----------|--------|---------|--------|------|"}
	for _, w := range roll.Waves {
		lines = append(lines, fmt.Sprintf("| %d | %d | %s | %s | %d | %s | %s |",
			w.Wave, w.Tasks, humanMs(w.DurationMs), humanMs(w.VerifyDurationMs), w.Retries, tokensStr(w.Tokens), costStr(w.Cost, w.CostAnnotated)))
	}
	lines = append(lines, fmt.Sprintf("| **total** | — | **%s** | **%s** | **%d** | **%s** | **%s** |",
		humanMs(roll.DurationMs), humanMs(roll.VerifyDurationMs), roll.Retries, tokensStr(roll.Tokens), costStr(roll.Cost, roll.CostAnnotated)))
	return strings.Join(lines, "\n")
}

func humanMs(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func tokensStr(n int) string {
	if n <= 0 {
		return "—"
	}
	return fmt.Sprintf("%d", n)
}

func costStr(c float64, annotated bool) string {
	if !annotated {
		return "—"
	}
	return fmt.Sprintf("%.2f", c)
}

// RenderMarkdown renders d as a complete Markdown report: a title with status
// badge, the spec/status/phase/turn summary line, the effective mode, and
// every section returned by buildSections.
func RenderMarkdown(d ReportData) string {
	b := GetBadge(d.State.Status)
	var out []string
	out = append(out, fmt.Sprintf("# %s — [%s]", d.State.Title, b.Label))
	out = append(out, "")
	out = append(out, fmt.Sprintf("> Spec: `%s` · Status: **%s** · Phase: **%s** · Turn: %d",
		d.State.Spec, d.State.Status, d.State.Phase, d.State.Turn))
	out = append(out, "")
	modeOrigin := d.State.ModeOrigin
	if modeOrigin == "" {
		modeOrigin = OriginDefault
	}
	out = append(out, fmt.Sprintf("> Mode: **%s** (origin %s)", d.State.EffectiveMode(), modeOrigin))
	out = append(out, "")
	for _, sec := range buildSections(d) {
		out = append(out, fmt.Sprintf("## %s %s", sec.Icon, sec.Title))
		out = append(out, "")
		out = append(out, sec.Body)
		out = append(out, "")
	}
	result := strings.Join(out, "\n")
	result = strings.TrimRight(result, "\n") + "\n"
	return result
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// RenderHTML renders d as a complete, self-contained HTML page: a styled
// header with status badge, every section returned by buildSections, and an
// inline live-update script that re-renders the page on matching /events SSE
// deltas. autoRefreshSeconds, when positive, adds a meta refresh tag as a
// fallback for clients without EventSource support.
func RenderHTML(d ReportData, autoRefreshSeconds int) string {
	b := GetBadge(d.State.Status)
	refresh := ""
	if autoRefreshSeconds > 0 {
		refresh = fmt.Sprintf("\n  <meta http-equiv=\"refresh\" content=\"%d\">", autoRefreshSeconds)
	}
	var secHTML []string
	for _, sec := range buildSections(d) {
		secHTML = append(secHTML, fmt.Sprintf("  <section>\n    <h2>%s %s</h2>\n    <pre>%s</pre>\n  </section>",
			sec.Icon, esc(sec.Title), esc(sec.Body)))
	}
	// Live-update client: subscribe to the reused /events SSE stream and, on a
	// frontier delta for this spec, re-fetch and re-render this report in place —
	// no polling, no full-page reload, no LLM call. Self-contained inline script
	// with no external asset fetch (R4, R6). Degrades to a static page where
	// EventSource is unavailable.
	liveScript := fmt.Sprintf(`  <script>
  (function () {
    var slug = %q;
    if (typeof EventSource === "undefined") return;
    var es = new EventSource("/events");
    es.onmessage = function (e) {
      var ev;
      try { ev = JSON.parse(e.data); } catch (_) { return; }
      if (ev.spec !== slug) return;
      fetch("/api/report?spec=" + encodeURIComponent(slug))
        .then(function (r) { return r.ok ? r.text() : Promise.reject(); })
        .then(function () { return fetch("/s/" + encodeURIComponent(slug)); })
        .then(function (r) { return r.text(); })
        .then(function (html) {
          var doc = new DOMParser().parseFromString(html, "text/html");
          if (doc.body) document.body.innerHTML = doc.body.innerHTML;
        })
        .catch(function () {});
    };
  })();
  </script>`, d.State.Spec)
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">%s
  <title>%s — %s</title>
  <style>
    body { font: 15px/1.55 system-ui, sans-serif; max-width: 920px; margin: 2rem auto; padding: 0 1rem; color: #c9d1d9; background: #0d1117; }
    h1 { font-size: 1.6rem; }
    h2 { border-bottom: 1px solid #30363d; padding-bottom: .3rem; margin-top: 2rem; }
    .badge { display: inline-block; padding: .15rem .6rem; border-radius: 1rem; color: #fff; font-size: .85rem; background: %s; }
    .meta { color: #8b949e; font-size: .9rem; }
    pre { white-space: pre-wrap; background: #161b22; padding: 1rem; border-radius: 6px; overflow-x: auto; }
    section { margin-bottom: 1rem; }
    @media (max-width: 600px) {
      body { margin: 1rem auto; font-size: 14px; }
      h1 { font-size: 1.3rem; }
      pre { padding: .75rem; }
    }
  </style>
</head>
<body>
  <h1>%s <span class="badge">%s</span></h1>
  <p class="meta">Spec: <code>%s</code> · Status: %s · Phase: %s · Turn: %d</p>
%s
%s
</body>
</html>
`, refresh, esc(d.State.Title), b.Label, b.Color, esc(d.State.Title), b.Label,
		esc(d.State.Spec), esc(string(d.State.Status)), esc(string(d.State.Phase)), d.State.Turn,
		strings.Join(secHTML, "\n"), liveScript)
}
