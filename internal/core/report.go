package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Badge struct {
	Label string
	Color string
}

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

type ReportData struct {
	State        *State
	Requirements *string
	Design       *string
	Tasks        *string
	Decisions    *string
	Memory       *string
	MidReqs      *string
}

func ExtractSection(md *string, heading string) *string {
	if md == nil {
		return nil
	}
	lines := splitLines(*md)
	re := regexp.MustCompile(`(?i)^##\s+` + regexp.QuoteMeta(heading))
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
	if len(d.State.Blockers) > 0 {
		var lines []string
		for _, b := range d.State.Blockers {
			lines = append(lines, fmt.Sprintf("- **%s** — %s _(since %s)_", b.Task, b.Reason, b.Since))
		}
		s = append(s, reportSection{"🚧", "Blockers", strings.Join(lines, "\n")})
	}
	return s
}

func RenderMarkdown(d ReportData) string {
	b := GetBadge(d.State.Status)
	var out []string
	out = append(out, fmt.Sprintf("# %s — [%s]", d.State.Title, b.Label))
	out = append(out, "")
	out = append(out, fmt.Sprintf("> Spec: `%s` · Status: **%s** · Phase: **%s** · Turn: %d",
		d.State.Spec, d.State.Status, d.State.Phase, d.State.Turn))
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
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">%s
  <title>%s — %s</title>
  <style>
    body { font: 15px/1.55 system-ui, sans-serif; max-width: 920px; margin: 2rem auto; padding: 0 1rem; color: #c9d1d9; background: #0d1117; }
    h1 { font-size: 1.6rem; }
    h2 { border-bottom: 1px solid #30363d; padding-bottom: .3rem; margin-top: 2rem; }
    .badge { display: inline-block; padding: .15rem .6rem; border-radius: 1rem; color: #fff; font-size: .85rem; background: %s; }
    .meta { color: #8b949e; font-size: .9rem; }
    pre { white-space: pre-wrap; background: #161b22; padding: 1rem; border-radius: 6px; overflow-x: auto; }
    section { margin-bottom: 1rem; }
  </style>
</head>
<body>
  <h1>%s <span class="badge">%s</span></h1>
  <p class="meta">Spec: <code>%s</code> · Status: %s · Phase: %s · Turn: %d</p>
%s
</body>
</html>
`, refresh, esc(d.State.Title), b.Label, b.Color, esc(d.State.Title), b.Label,
		esc(d.State.Spec), esc(string(d.State.Status)), esc(string(d.State.Phase)), d.State.Turn,
		strings.Join(secHTML, "\n"))
}
