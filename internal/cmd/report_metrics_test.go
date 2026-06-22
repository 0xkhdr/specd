package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

func TestReportPrometheusMetricsGolden(t *testing.T) {
	rawGolden, err := os.ReadFile("testdata/report_prometheus.golden")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := strings.ReplaceAll(string(rawGolden), "\r\n", "\n")

	h := testharness.New(t)
	slug := h.Spec("metrics-spec").
		Req("metrics", "As an operator I want metrics.", "THE SYSTEM SHALL emit metrics.").
		FullDesign().
		Status(core.StatusExecuting).
		AddTask(testharness.TaskSpec{ID: "T1", Wave: 1}).
		AddTask(testharness.TaskSpec{ID: "T2", Wave: 2}).
		Build()

	state, err := core.LoadState(h.Root, slug)
	if err != nil || state == nil {
		t.Fatalf("LoadState: %v", err)
	}
	t1 := state.Tasks["T1"]
	t1.Telemetry = &core.Telemetry{DurationMs: 1000, VerifyDurationMs: 200, Retries: 1, Tokens: 50, Cost: "0.25"}
	state.Tasks["T1"] = t1
	t2 := state.Tasks["T2"]
	t2.Telemetry = &core.Telemetry{DurationMs: 3000, VerifyDurationMs: 400, Tokens: 70, Cost: "$0.75"}
	state.Tasks["T2"] = t2
	if err := core.SaveState(h.Root, slug, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	res := h.RunExpect(core.ExitOK, "report", slug, "--format", "prometheus")
	if res.Stdout != want {
		t.Fatalf("prometheus output mismatch\nwant:\n%s\ngot:\n%s", want, res.Stdout)
	}
	res2 := h.RunExpect(core.ExitOK, "report", slug, "--format", "prometheus")
	if res2.Stdout != res.Stdout {
		t.Fatalf("prometheus output not deterministic\nfirst:\n%s\nsecond:\n%s", res.Stdout, res2.Stdout)
	}
}
