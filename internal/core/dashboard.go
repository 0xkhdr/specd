package core

import (
	"fmt"
	"sort"
	"strings"
)

// The unified dashboard (V11/P6.2) projects the whole project — every spec's
// status, orchestrator waves, conductor sessions, eval trends, cost, and
// escalations, plus the shared harness bundle — into a single read-only view.
// DashboardData is that projection: it is assembled purely from local
// state.json files, ledgers, and the harness manifest, with zero outbound
// network, so RenderDashboardHTML is a pure function of on-disk bytes and can be
// snapshot-tested from fixtures. The binary never perceives or generates prose:
// the render is deterministic templating over already-recorded state.

// dashboardModes are the panel filters accepted by `--mode`. "all" renders every
// panel; the others scope the view to one concern.
const (
	DashboardModeAll          = "all"
	DashboardModeConductor    = "conductor"
	DashboardModeOrchestrator = "orchestrator"
	DashboardModeCost         = "cost"
	DashboardModeEval         = "eval"
)

// DashboardData is the deterministic, project-wide projection the unified
// dashboard renders.
type DashboardData struct {
	Mode         string           `json:"mode"`
	Harness      *HarnessManifest `json:"harness,omitempty"`
	Quarantined  []string         `json:"quarantined,omitempty"`
	Specs        []DashboardSpec  `json:"specs"`
	TotalCostUSD string           `json:"totalCostUSD,omitempty"`
	TotalTokens  int64            `json:"totalTokens,omitempty"`
}

// DashboardSpec is one spec's row in the unified dashboard.
type DashboardSpec struct {
	Slug       string            `json:"slug"`
	Title      string            `json:"title"`
	Status     string            `json:"status"`
	Phase      string            `json:"phase"`
	Mode       string            `json:"mode"`
	TasksDone  int               `json:"tasksDone"`
	TasksTotal int               `json:"tasksTotal"`
	Waves      []DashboardWave   `json:"waves,omitempty"`
	CostUSD    string            `json:"costUSD,omitempty"`
	Tokens     int64             `json:"tokens,omitempty"`
	Conductor  *ConductorSession `json:"conductor,omitempty"`
	Escalation *EscalationRecord `json:"escalation,omitempty"`
	Evals      []EvalSummary     `json:"evals,omitempty"`
}

// DashboardWave summarises one orchestrator wave: how many of its tasks are
// complete. Waves are derived from each task's recorded Wave index.
type DashboardWave struct {
	Wave  int `json:"wave"`
	Done  int `json:"done"`
	Total int `json:"total"`
}

// NormalizeDashboardMode maps a raw --mode value to a known panel filter,
// defaulting to "all". An unknown value is reported so the caller can reject it.
func NormalizeDashboardMode(mode string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "", DashboardModeAll:
		return DashboardModeAll, true
	case DashboardModeConductor:
		return DashboardModeConductor, true
	case DashboardModeOrchestrator:
		return DashboardModeOrchestrator, true
	case DashboardModeCost:
		return DashboardModeCost, true
	case DashboardModeEval:
		return DashboardModeEval, true
	default:
		return DashboardModeAll, false
	}
}

// dashboardShows reports whether a panel is rendered under the active mode.
func (d DashboardData) dashboardShows(panel string) bool {
	return d.Mode == DashboardModeAll || d.Mode == panel
}

// BuildDashboard assembles the project-wide dashboard projection from disk. It
// reads every spec's state, the harness bundle if present, and sums cost/tokens
// across specs. It never touches the network and never mutates state.
func BuildDashboard(root, mode string) (DashboardData, error) {
	norm, ok := NormalizeDashboardMode(mode)
	if !ok {
		return DashboardData{}, UsageError(fmt.Sprintf("dashboard: unknown --mode %q (want all|conductor|orchestrator|cost|eval)", mode))
	}
	d := DashboardData{Mode: norm}
	if m, err := LoadHarnessManifest(root); err == nil {
		d.Harness = &m
		d.Quarantined = HarnessQuarantined(root)
	}
	var totalCost float64
	var haveCost bool
	for _, slug := range ListSpecs(root) {
		st, err := LoadState(root, slug)
		if err != nil || st == nil {
			continue
		}
		ds := DashboardSpec{
			Slug:       slug,
			Title:      st.Title,
			Status:     string(st.Status),
			Phase:      string(st.Phase),
			Mode:       st.EffectiveMode(),
			Conductor:  st.Conductor,
			Escalation: st.Escalation,
		}
		ds.TasksTotal = len(st.Tasks)
		waves := map[int]*DashboardWave{}
		for _, t := range st.Tasks {
			if t.Status == TaskComplete {
				ds.TasksDone++
			}
			w := waves[t.Wave]
			if w == nil {
				w = &DashboardWave{Wave: t.Wave}
				waves[t.Wave] = w
			}
			w.Total++
			if t.Status == TaskComplete {
				w.Done++
			}
		}
		for _, w := range waves {
			ds.Waves = append(ds.Waves, *w)
		}
		sort.Slice(ds.Waves, func(i, j int) bool { return ds.Waves[i].Wave < ds.Waves[j].Wave })

		econ := RoutingEconomicsFromState(st)
		ds.CostUSD = econ.TotalCostUSD
		ds.Tokens = econ.TotalTokens
		if c, ok := parseCostStr(econ.TotalCostUSD); ok {
			totalCost += c
			haveCost = true
		}
		d.TotalTokens += econ.TotalTokens

		for _, e := range st.Evals {
			ds.Evals = append(ds.Evals, e)
		}
		sort.Slice(ds.Evals, func(i, j int) bool { return ds.Evals[i].Suite < ds.Evals[j].Suite })

		d.Specs = append(d.Specs, ds)
	}
	sort.Slice(d.Specs, func(i, j int) bool { return d.Specs[i].Slug < d.Specs[j].Slug })
	if haveCost {
		d.TotalCostUSD = fmt.Sprintf("%.4f", totalCost)
	}
	return d, nil
}

// RenderDashboardHTML renders the unified dashboard as a self-contained,
// dependency-free HTML page. autoRefreshSeconds, when > 0, mounts the shared SSE
// live-update hook; 0 renders a static page. The markup is deterministic given
// DashboardData so it can be byte-compared in tests.
func RenderDashboardHTML(d DashboardData, autoRefreshSeconds int) string {
	var b strings.Builder
	b.WriteString(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>specd unified dashboard</title>
  <style>
    body { font: 15px/1.55 system-ui, sans-serif; max-width: 1040px; margin: 2rem auto; padding: 0 1rem; color: #c9d1d9; background: #0d1117; }
    h1 { font-size: 1.6rem; } h2 { font-size: 1.15rem; margin-top: 1.6rem; border-bottom: 1px solid #21262d; padding-bottom: .3rem; }
    a { color: #58a6ff; text-decoration: none; } a:hover { text-decoration: underline; }
    table { border-collapse: collapse; width: 100%; margin: .5rem 0; }
    th, td { text-align: left; padding: .4rem .6rem; border-bottom: 1px solid #21262d; font-size: .92rem; }
    th { color: #8b949e; font-weight: 600; }
    .meta { color: #8b949e; font-size: .9rem; } .warn { color: #f0883e; }
    .cards { display: flex; flex-wrap: wrap; gap: .75rem; margin: .6rem 0; }
    .card { background: #161b22; border-radius: 8px; padding: .7rem 1rem; min-width: 9rem; }
    .card .n { font-size: 1.4rem; font-weight: 700; } .card .l { color: #8b949e; font-size: .8rem; text-transform: uppercase; letter-spacing: .04em; }
    .scroll { overflow-x: auto; }
  </style>
</head>
<body>
  <h1>specd unified dashboard <span class="meta">— mode: `)
	b.WriteString(esc(d.Mode))
	b.WriteString("</span></h1>\n")

	// Summary cards (always shown).
	fmt.Fprintf(&b, `  <div class="cards">
    <div class="card"><div class="n">%d</div><div class="l">specs</div></div>
    <div class="card"><div class="n">$%s</div><div class="l">total cost</div></div>
    <div class="card"><div class="n">%d</div><div class="l">tokens</div></div>
  </div>
`, len(d.Specs), dashCostOrZero(d.TotalCostUSD), d.TotalTokens)

	if d.Harness != nil {
		fmt.Fprintf(&b, "  <h2>Harness bundle</h2>\n  <p class=\"meta\">%s v%d — %d artifact(s), provenance %s</p>\n",
			esc(d.Harness.Name), d.Harness.Version, len(d.Harness.Files), esc(d.Harness.Provenance))
		if len(d.Quarantined) > 0 {
			b.WriteString("  <p class=\"warn\">⚠ quarantined (awaiting enable): ")
			b.WriteString(esc(strings.Join(d.Quarantined, ", ")))
			b.WriteString("</p>\n")
		}
	}

	if d.dashboardShows(DashboardModeOrchestrator) {
		renderDashSpecsTable(&b, d.Specs)
	}
	if d.dashboardShows(DashboardModeConductor) {
		renderDashConductor(&b, d.Specs)
	}
	if d.dashboardShows(DashboardModeCost) {
		renderDashCost(&b, d.Specs)
	}
	if d.dashboardShows(DashboardModeEval) {
		renderDashEvals(&b, d.Specs)
	}
	renderDashEscalations(&b, d.Specs)

	if autoRefreshSeconds > 0 {
		fmt.Fprintf(&b, `  <script>
    (function(){ var es = new EventSource('/events'); es.onmessage = function(){ location.reload(); }; })();
  </script>
`)
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

func renderDashSpecsTable(b *strings.Builder, specs []DashboardSpec) {
	b.WriteString("  <h2>Specs &amp; waves</h2>\n  <div class=\"scroll\"><table>\n    <tr><th>Spec</th><th>Status</th><th>Phase</th><th>Mode</th><th>Tasks</th><th>Waves</th></tr>\n")
	for _, s := range specs {
		var waves []string
		for _, w := range s.Waves {
			waves = append(waves, fmt.Sprintf("w%d %d/%d", w.Wave, w.Done, w.Total))
		}
		fmt.Fprintf(b, "    <tr><td><a href=\"/s/%s\">%s</a></td><td>%s</td><td>%s</td><td>%s</td><td>%d/%d</td><td class=\"meta\">%s</td></tr>\n",
			esc(s.Slug), esc(dashTitle(s)), esc(s.Status), esc(s.Phase), esc(s.Mode),
			s.TasksDone, s.TasksTotal, esc(strings.Join(waves, " · ")))
	}
	b.WriteString("  </table></div>\n")
}

func renderDashConductor(b *strings.Builder, specs []DashboardSpec) {
	var rows []DashboardSpec
	for _, s := range specs {
		if s.Conductor != nil {
			rows = append(rows, s)
		}
	}
	if len(rows) == 0 {
		return
	}
	b.WriteString("  <h2>Conductor sessions</h2>\n  <div class=\"scroll\"><table>\n    <tr><th>Spec</th><th>Session</th><th>Task</th></tr>\n")
	for _, s := range rows {
		fmt.Fprintf(b, "    <tr><td>%s</td><td class=\"meta\">%s</td><td>%s</td></tr>\n",
			esc(s.Slug), esc(s.Conductor.SessionID), esc(s.Conductor.Task))
	}
	b.WriteString("  </table></div>\n")
}

func renderDashCost(b *strings.Builder, specs []DashboardSpec) {
	b.WriteString("  <h2>Cost attribution</h2>\n  <div class=\"scroll\"><table>\n    <tr><th>Spec</th><th>Cost (USD)</th><th>Tokens</th></tr>\n")
	for _, s := range specs {
		fmt.Fprintf(b, "    <tr><td>%s</td><td>$%s</td><td>%d</td></tr>\n",
			esc(s.Slug), dashCostOrZero(s.CostUSD), s.Tokens)
	}
	b.WriteString("  </table></div>\n")
}

func renderDashEvals(b *strings.Builder, specs []DashboardSpec) {
	var any bool
	for _, s := range specs {
		if len(s.Evals) > 0 {
			any = true
			break
		}
	}
	if !any {
		return
	}
	b.WriteString("  <h2>Eval trends</h2>\n  <div class=\"scroll\"><table>\n    <tr><th>Spec</th><th>Suite</th><th>Score</th><th>Min</th></tr>\n")
	for _, s := range specs {
		for _, e := range s.Evals {
			pass := ""
			if e.Score < e.MinScore {
				pass = ` class="warn"`
			}
			fmt.Fprintf(b, "    <tr><td>%s</td><td>%s</td><td%s>%.2f</td><td class=\"meta\">%.2f</td></tr>\n",
				esc(s.Slug), esc(e.Suite), pass, e.Score, e.MinScore)
		}
	}
	b.WriteString("  </table></div>\n")
}

func renderDashEscalations(b *strings.Builder, specs []DashboardSpec) {
	var rows []DashboardSpec
	for _, s := range specs {
		if s.Escalation != nil {
			rows = append(rows, s)
		}
	}
	if len(rows) == 0 {
		return
	}
	b.WriteString("  <h2 class=\"warn\">Escalations</h2>\n  <div class=\"scroll\"><table>\n    <tr><th>Spec</th><th>Task</th><th>When</th></tr>\n")
	for _, s := range rows {
		fmt.Fprintf(b, "    <tr><td>%s</td><td>%s</td><td class=\"meta\">%s</td></tr>\n",
			esc(s.Slug), esc(s.Escalation.Task), esc(s.Escalation.Time))
	}
	b.WriteString("  </table></div>\n")
}

func dashTitle(s DashboardSpec) string {
	if strings.TrimSpace(s.Title) != "" {
		return s.Title
	}
	return s.Slug
}

func dashCostOrZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0.0000"
	}
	return s
}
