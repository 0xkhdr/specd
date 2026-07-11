package core

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// metricNamePattern is the Prometheus metric-name grammar. lintPrometheus uses
// it to enforce R5 (valid names) locally instead of shipping promtool.
var metricNamePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// lintPrometheus applies promtool-style structural rules to an exposition:
// valid metric names, HELP/TYPE present per family, no duplicate series, and
// properly escaped label values. It returns the first violation found.
func lintPrometheus(t *testing.T, text string) {
	t.Helper()
	seenSeries := map[string]bool{}
	haveHelp := map[string]bool{}
	haveType := map[string]bool{}
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# HELP ") || strings.HasPrefix(line, "# TYPE ") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				t.Fatalf("malformed comment line: %q", line)
			}
			name := fields[2]
			if !metricNamePattern.MatchString(name) {
				t.Fatalf("invalid metric name %q", name)
			}
			if fields[1] == "HELP" {
				haveHelp[name] = true
			} else {
				haveType[name] = true
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		// A sample line: name{labels} value  OR  name value.
		series := line
		if sp := strings.LastIndex(line, " "); sp >= 0 {
			series = line[:sp]
		}
		name := series
		if brace := strings.Index(series, "{"); brace >= 0 {
			name = series[:brace]
			if !strings.HasSuffix(series, "}") {
				t.Fatalf("unterminated label set: %q", line)
			}
		}
		if !metricNamePattern.MatchString(name) {
			t.Fatalf("invalid metric name in sample %q", line)
		}
		if !haveHelp[name] || !haveType[name] {
			t.Fatalf("sample %q emitted before its HELP/TYPE", name)
		}
		if seenSeries[series] {
			t.Fatalf("duplicate series: %q", series)
		}
		seenSeries[series] = true
	}
}

func TestRenderPrometheusLintsClean(t *testing.T) {
	m := PrometheusMetrics{
		Slug: `pay"ments`, // embed a quote to exercise label escaping
		TasksByStatus: map[string]int{
			"complete": 2,
			"pending":  1,
		},
		VerifyAttempts:  5,
		VerifyFailures:  2,
		CriteriaPassing: 3,
		CriteriaTotal:   4,
		Tokens:          1200,
		Cost:            "0.34",
		DurationMs:      2500,
	}
	out := RenderPrometheus(m)
	lintPrometheus(t, out)

	// The embedded quote must be escaped, never emitted raw.
	if !strings.Contains(out, `spec="pay\"ments"`) {
		t.Fatalf("label value not escaped:\n%s", out)
	}
	// Spot-check a few values carry through correctly.
	for _, want := range []string{
		`specd_verify_attempts_total{spec="pay\"ments"} 5`,
		`specd_verify_failures_total{spec="pay\"ments"} 2`,
		`specd_worker_duration_seconds_total{spec="pay\"ments"} 2.500`,
		`specd_worker_cost_total{spec="pay\"ments"} 0.34`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing metric line %q in:\n%s", want, out)
		}
	}
}

func TestRenderPrometheusEmptyCostIsZero(t *testing.T) {
	out := RenderPrometheus(PrometheusMetrics{Slug: "demo", TasksByStatus: map[string]int{}})
	lintPrometheus(t, out)
	if !strings.Contains(out, `specd_worker_cost_total{spec="demo"} 0`) {
		t.Fatalf("unreported cost should render 0:\n%s", out)
	}
}

// TestPrometheusTaskLabelsAreUnbounded characterizes the W0 gap W5/W8 closes:
// per-task telemetry is rendered as a distinct series carrying a task="…"
// label, so label cardinality grows one-for-one with task count with no bound.
// A spec with N telemetried tasks emits N task-labeled series.
func TestPrometheusTaskLabelsAreUnbounded(t *testing.T) {
	report := TelemetryReport{}
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("T%d", i)
		report.Tasks = append(report.Tasks, TaskTelemetry{TaskID: id, HasTelemetry: true, Tokens: 1, Attempts: []Annotations{{Tokens: 1}}})
	}
	out := RenderTelemetry("demo", report)
	if n := strings.Count(out, `specd_task_cost{spec="demo",task="`); n != 50 {
		t.Fatalf("expected one unbounded task series per task, got %d", n)
	}
}
