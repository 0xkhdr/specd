package obs

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRecordDurationLogsStructuredMetric(t *testing.T) {
	resetMetricsForTest()
	t.Setenv("SPECD_LOG", "info")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	RecordDuration("unit_duration", 1500*time.Millisecond)
	_ = w.Close()
	os.Stderr = old
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	out := string(raw)
	if !strings.Contains(out, `"event":"metric.duration"`) || !strings.Contains(out, `"metric":"unit_duration"`) || !strings.Contains(out, `"duration_ms":1500`) {
		t.Fatalf("metric log missing fields: %s", out)
	}
}

func TestMetricsEndpointIsOptInAndRendersPrometheus(t *testing.T) {
	resetMetricsForTest()
	RecordDuration("no_endpoint_duration", time.Millisecond)
	if got := MetricsEndpointAddr(); got != "" {
		t.Fatalf("endpoint started while env unset: %q", got)
	}
	if body := RenderPrometheusMetrics(); strings.Contains(body, "no_endpoint_duration") {
		t.Fatalf("disabled endpoint should not retain samples, got: %s", body)
	}

	resetMetricsForTest()
	t.Setenv("SPECD_METRICS_ENDPOINT", "127.0.0.1:0")
	RecordDuration("http_duration", 2*time.Millisecond)
	addr := MetricsEndpointAddr()
	if addr == "" {
		t.Fatal("metrics endpoint did not start")
	}
	resp, err := http.Get("http://" + addr + "/metrics") //nolint:gosec,noctx // local test endpoint.
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		"# TYPE specd_duration_seconds summary",
		`specd_duration_seconds_count{name="http_duration"} 1`,
		`specd_duration_seconds_sum{name="http_duration"} 0.002000000`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
}
