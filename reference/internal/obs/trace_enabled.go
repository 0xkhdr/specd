//go:build specd_trace

package obs

import (
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"
)

// EndSpan completes a tracing span.
type EndSpan func()

type traceEvent struct {
	Name  string `json:"name"`
	Phase string `json:"ph"`
	TS    int64  `json:"ts"`
	Dur   int64  `json:"dur"`
	PID   int    `json:"pid"`
	TID   int    `json:"tid"`
}

var traceState = struct {
	sync.Mutex
	path   string
	events []traceEvent
}{path: os.Getenv("SPECD_TRACE_FILE")}

// StartSpan records Chrome trace compatible complete events when built with
// -tags specd_trace. Set SPECD_TRACE_FILE to choose the output path; otherwise
// spans are logged as structured events on stderr when SPECD_LOG enables info.
func StartSpan(name string) EndSpan {
	if name == "" {
		return func() {}
	}
	start := time.Now()
	return func() {
		dur := time.Since(start)
		ev := traceEvent{Name: name, Phase: "X", TS: start.UnixMicro(), Dur: dur.Microseconds(), PID: os.Getpid(), TID: 0}
		if traceState.path == "" {
			if ParseLevel(os.Getenv("SPECD_LOG")) <= slog.LevelInfo {
				NewLogger().Info("trace span", "event", "trace.span", "span", name, "duration_us", strconv.FormatInt(ev.Dur, 10))
			}
			return
		}
		traceState.Lock()
		traceState.events = append(traceState.events, ev)
		raw, err := json.Marshal(struct {
			TraceEvents []traceEvent `json:"traceEvents"`
		}{TraceEvents: traceState.events})
		traceState.Unlock()
		if err != nil {
			NewLogger().Warn("trace marshal failed", "event", "trace.error", "error", err.Error())
			return
		}
		if err := os.WriteFile(traceState.path, raw, 0o644); err != nil { //nolint:gosec // local developer trace file, non-secret diagnostics.
			NewLogger().Warn("trace write failed", "event", "trace.error", "path", traceState.path, "error", err.Error())
		}
	}
}
