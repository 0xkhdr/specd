package obs

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const metricsEventDuration = "metric.duration"

type durationMetric struct {
	Count int64
	Sum   time.Duration
	Last  time.Duration
	Max   time.Duration
}

var metricsState = struct {
	sync.Mutex
	values map[string]durationMetric
}{values: map[string]durationMetric{}}

var metricsEndpointState = struct {
	sync.Once
	addr string
	err  error
}{}

// RecordDuration emits one structured duration metric through specd's existing
// stderr logger and stores it for the optional Prometheus text endpoint.
func RecordDuration(name string, d time.Duration) {
	if name == "" {
		return
	}
	if metricsEndpointConfigured() {
		recordDurationSample(name, d)
		startMetricsEndpoint()
	}
	if ParseLevel(os.Getenv("SPECD_LOG")) <= slog.LevelInfo {
		NewLogger().Info("duration metric", "event", metricsEventDuration, "metric", name, slog.Duration(name, d), "duration_ms", d.Milliseconds())
	}
}

func metricsEndpointConfigured() bool {
	return strings.TrimSpace(os.Getenv("SPECD_METRICS_ENDPOINT")) != ""
}

func recordDurationSample(name string, d time.Duration) {
	metricsState.Lock()
	defer metricsState.Unlock()
	m := metricsState.values[name]
	m.Count++
	m.Sum += d
	m.Last = d
	if d > m.Max {
		m.Max = d
	}
	metricsState.values[name] = m
}

func startMetricsEndpoint() {
	metricsEndpointState.Once.Do(func() {
		addr := strings.TrimSpace(os.Getenv("SPECD_METRICS_ENDPOINT"))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			metricsEndpointState.err = err
			NewLogger().Warn("metrics endpoint unavailable", "event", "metrics.endpoint.error", "addr", addr, "error", err.Error())
			return
		}
		metricsEndpointState.addr = ln.Addr().String()
		mux := http.NewServeMux()
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
			_, _ = w.Write([]byte(RenderPrometheusMetrics()))
		})
		server := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second}
		go func() {
			if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
				NewLogger().Warn("metrics endpoint stopped", "event", "metrics.endpoint.error", "addr", metricsEndpointState.addr, "error", err.Error())
			}
		}()
	})
}

// MetricsEndpointAddr returns the active metrics listener address, if enabled.
func MetricsEndpointAddr() string { return metricsEndpointState.addr }

func resetMetricsForTest() {
	metricsState.Lock()
	metricsState.values = map[string]durationMetric{}
	metricsState.Unlock()
	metricsEndpointState = struct {
		sync.Once
		addr string
		err  error
	}{}
}

// RenderPrometheusMetrics renders the in-memory duration metrics in Prometheus
// text exposition format without requiring a Prometheus client dependency.
func RenderPrometheusMetrics() string {
	metricsState.Lock()
	snapshot := make(map[string]durationMetric, len(metricsState.values))
	for k, v := range metricsState.values {
		snapshot[k] = v
	}
	metricsState.Unlock()

	names := make([]string, 0, len(snapshot))
	for name := range snapshot {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("# HELP specd_duration_seconds Duration samples recorded by specd.\n")
	b.WriteString("# TYPE specd_duration_seconds summary\n")
	for _, name := range names {
		m := snapshot[name]
		fmt.Fprintf(&b, "specd_duration_seconds_count{name=%q} %d\n", name, m.Count)
		fmt.Fprintf(&b, "specd_duration_seconds_sum{name=%q} %.9f\n", name, m.Sum.Seconds())
		fmt.Fprintf(&b, "specd_duration_seconds_last{name=%q} %.9f\n", name, m.Last.Seconds())
		fmt.Fprintf(&b, "specd_duration_seconds_max{name=%q} %.9f\n", name, m.Max.Seconds())
	}
	return b.String()
}
