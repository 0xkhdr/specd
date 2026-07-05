package core

import (
	"fmt"
	"sort"
	"strings"
)

// PrometheusMetrics is the pure, on-disk-derived snapshot rendered as a
// Prometheus textfile exposition (spec 13 R4). It is assembled by the caller
// from the same state.json + ledgers the rest of `report` reads; RenderMetrics
// here adds no I/O and no gate logic.
//
// Metric names are an API: renaming one breaks every dashboard built against it,
// so the contract is written down in docs/command-reference.md and must not
// churn. All names carry the `specd_` prefix, snake_case, with `_total` on
// monotonic counters and `_seconds` on durations, per Prometheus conventions.
type PrometheusMetrics struct {
	Slug            string
	TasksByStatus   map[string]int
	VerifyAttempts  int
	VerifyFailures  int
	CriteriaPassing int
	CriteriaTotal   int
	EscalatedTasks  int
	Tokens          int
	Cost            string // exact decimal string; "" renders as 0
	DurationMs      int
}

// RenderPrometheus emits node_exporter textfile-collector-compatible output:
// each metric family preceded by its HELP and TYPE lines, every series labelled
// with the spec slug, values in a deterministic order so repeated runs are
// byte-identical and no series is duplicated (R4, R5).
func RenderPrometheus(m PrometheusMetrics) string {
	var b strings.Builder
	spec := promLabel("spec", m.Slug)

	// Tasks by status — a gauge (a status count rises and falls as work moves).
	b.WriteString("# HELP specd_tasks Number of tasks in each status.\n")
	b.WriteString("# TYPE specd_tasks gauge\n")
	for _, status := range sortedKeys(m.TasksByStatus) {
		fmt.Fprintf(&b, "specd_tasks{%s,%s} %d\n", spec, promLabel("status", status), m.TasksByStatus[status])
	}

	writeCounter(&b, "specd_verify_attempts_total", "Total verify attempts recorded in the evidence ledger.", spec, m.VerifyAttempts)
	writeCounter(&b, "specd_verify_failures_total", "Verify attempts that exited non-zero.", spec, m.VerifyFailures)

	// Acceptance-criterion coverage (spec 04) as a gauge with a verdict label.
	b.WriteString("# HELP specd_criteria Acceptance criteria by verdict (passing vs. total declared).\n")
	b.WriteString("# TYPE specd_criteria gauge\n")
	fmt.Fprintf(&b, "specd_criteria{%s,%s} %d\n", spec, promLabel("verdict", "passing"), m.CriteriaPassing)
	fmt.Fprintf(&b, "specd_criteria{%s,%s} %d\n", spec, promLabel("verdict", "total"), m.CriteriaTotal)

	// Escalated task count (spec 06). Until escalation lands this is a
	// well-formed zero — a natural total, not placeholder spam.
	b.WriteString("# HELP specd_escalated_tasks Tasks currently blocked awaiting human override.\n")
	b.WriteString("# TYPE specd_escalated_tasks gauge\n")
	fmt.Fprintf(&b, "specd_escalated_tasks{%s} %d\n", spec, m.EscalatedTasks)

	// Worker-reported telemetry totals (spec 10), stored verbatim and summed
	// with exact-decimal math upstream; a spec with no telemetry shows zeros.
	writeCounter(&b, "specd_worker_tokens_total", "Worker-reported tokens summed across verify attempts.", spec, m.Tokens)
	b.WriteString("# HELP specd_worker_cost_total Worker-reported cost summed across verify attempts.\n")
	b.WriteString("# TYPE specd_worker_cost_total counter\n")
	fmt.Fprintf(&b, "specd_worker_cost_total{%s} %s\n", spec, promDecimal(m.Cost))
	b.WriteString("# HELP specd_worker_duration_seconds_total Worker-reported wall-clock seconds summed across verify attempts.\n")
	b.WriteString("# TYPE specd_worker_duration_seconds_total counter\n")
	fmt.Fprintf(&b, "specd_worker_duration_seconds_total{%s} %s\n", spec, msToSeconds(m.DurationMs))

	return b.String()
}

func writeCounter(b *strings.Builder, name, help, spec string, value int) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s counter\n", name)
	fmt.Fprintf(b, "%s{%s} %d\n", name, spec, value)
}

// promLabel renders name="value" with the value escaped per the exposition
// format: backslash, double-quote, and newline are the three escape sequences.
func promLabel(name, value string) string {
	esc := value
	esc = strings.ReplaceAll(esc, `\`, `\\`)
	esc = strings.ReplaceAll(esc, `"`, `\"`)
	esc = strings.ReplaceAll(esc, "\n", `\n`)
	return name + `="` + esc + `"`
}

// promDecimal renders a stored cost string; an empty (unreported) cost is 0.
func promDecimal(cost string) string {
	if cost == "" {
		return "0"
	}
	return cost
}

// msToSeconds converts integer milliseconds to a fixed 3-decimal second value
// without float rounding, keeping the exact-decimal discipline of spec 10.
func msToSeconds(ms int) string {
	return fmt.Sprintf("%d.%03d", ms/1000, ms%1000)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
