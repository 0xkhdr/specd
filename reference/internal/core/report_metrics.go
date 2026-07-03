package core

import (
	"fmt"
	"sort"
	"strings"
)

// RenderPrometheusMetrics emits a deterministic Prometheus textfile view of the
// spec report telemetry. It is format-only: no network endpoint, no exporter,
// no runtime dependency.
func RenderPrometheusMetrics(data ReportData) string {
	roll := RollupTelemetry(data.State)
	spec := data.State.Spec
	var b strings.Builder
	b.WriteString("# HELP specd_task_total Number of tasks in the spec.\n")
	b.WriteString("# TYPE specd_task_total gauge\n")
	fmt.Fprintf(&b, "specd_task_total{spec=%q} %d\n", spec, len(data.State.Tasks))
	b.WriteString("# HELP specd_telemetry_duration_ms Host-reported task duration roll-up in milliseconds.\n")
	b.WriteString("# TYPE specd_telemetry_duration_ms gauge\n")
	fmt.Fprintf(&b, "specd_telemetry_duration_ms{spec=%q} %d\n", spec, roll.DurationMs)
	b.WriteString("# HELP specd_telemetry_verify_duration_ms Verification duration roll-up in milliseconds.\n")
	b.WriteString("# TYPE specd_telemetry_verify_duration_ms gauge\n")
	fmt.Fprintf(&b, "specd_telemetry_verify_duration_ms{spec=%q} %d\n", spec, roll.VerifyDurationMs)
	b.WriteString("# HELP specd_telemetry_tokens Host-reported token roll-up.\n")
	b.WriteString("# TYPE specd_telemetry_tokens gauge\n")
	fmt.Fprintf(&b, "specd_telemetry_tokens{spec=%q} %d\n", spec, roll.Tokens)
	b.WriteString("# HELP specd_telemetry_cost_usd Host-reported cost roll-up in USD.\n")
	b.WriteString("# TYPE specd_telemetry_cost_usd gauge\n")
	fmt.Fprintf(&b, "specd_telemetry_cost_usd{spec=%q,annotated=%q} %.6f\n", spec, boolLabel(roll.CostAnnotated), roll.Cost)
	b.WriteString("# HELP specd_telemetry_retries Retry roll-up.\n")
	b.WriteString("# TYPE specd_telemetry_retries gauge\n")
	fmt.Fprintf(&b, "specd_telemetry_retries{spec=%q} %d\n", spec, roll.Retries)

	waves := append([]WaveTelemetry{}, roll.Waves...)
	sort.Slice(waves, func(i, j int) bool { return waves[i].Wave < waves[j].Wave })
	for _, w := range waves {
		fmt.Fprintf(&b, "specd_wave_task_total{spec=%q,wave=%q} %d\n", spec, fmt.Sprint(w.Wave), w.Tasks)
		fmt.Fprintf(&b, "specd_wave_telemetry_duration_ms{spec=%q,wave=%q} %d\n", spec, fmt.Sprint(w.Wave), w.DurationMs)
		fmt.Fprintf(&b, "specd_wave_telemetry_verify_duration_ms{spec=%q,wave=%q} %d\n", spec, fmt.Sprint(w.Wave), w.VerifyDurationMs)
		fmt.Fprintf(&b, "specd_wave_telemetry_tokens{spec=%q,wave=%q} %d\n", spec, fmt.Sprint(w.Wave), w.Tokens)
		fmt.Fprintf(&b, "specd_wave_telemetry_cost_usd{spec=%q,wave=%q,annotated=%q} %.6f\n", spec, fmt.Sprint(w.Wave), boolLabel(w.CostAnnotated), w.Cost)
		fmt.Fprintf(&b, "specd_wave_telemetry_retries{spec=%q,wave=%q} %d\n", spec, fmt.Sprint(w.Wave), w.Retries)
	}
	return b.String()
}

func boolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
