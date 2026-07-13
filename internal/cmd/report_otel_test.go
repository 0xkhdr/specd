package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// seedTrace writes one run's observable-event trace JSONL under the spec's
// evals/traces store — the on-disk source `report --format otel` projects.
func seedTrace(t *testing.T, root, slug, runID string, lines ...string) {
	t.Helper()
	path := core.EvalTracePath(root, slug, runID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir traces: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}
}

// TestTraceExportOTelPreservesCorrelationAndOmitsRawData pins spec 10 R10.2 end
// to end: `report --format otel` maps a local observable-event trace to
// OpenTelemetry-compatible spans, preserving run/event correlation while
// carrying only bounded classes — never raw source paths, content, or prompts.
// A second export over the same tree is byte-identical.
func TestTraceExportOTelPreservesCorrelationAndOmitsRawData(t *testing.T) {
	root := newDemoSpec(t)
	seedTrace(t, root, "demo", "run1",
		`{"run_id":"run1","event_id":"e1","seq":1,"tool":"edit","arg_class":"spec-text","result_class":"ok","time":"2026-07-05T09:00:00Z","actor":"craftsman","correlation":"T1"}`,
		`{"run_id":"run1","event_id":"e2","seq":2,"tool":"verify","arg_class":"public-metadata","result_class":"pass","time":"2026-07-05T09:01:00Z","actor":"validator","correlation":"T1"}`,
	)

	first, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"format": "otel"})
	})
	if err != nil {
		t.Fatalf("report --format otel: %v", err)
	}

	var trace, corr int
	for _, line := range strings.Split(strings.TrimRight(first, "\n"), "\n") {
		if line == "" {
			continue
		}
		var span struct {
			TraceID    string            `json:"trace_id"`
			SpanID     string            `json:"span_id"`
			Name       string            `json:"name"`
			Attributes map[string]string `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(line), &span); err != nil {
			t.Fatalf("otel line %q not JSON: %v", line, err)
		}
		if span.TraceID == "run1" {
			trace++
		}
		if span.Attributes["specd.correlation_id"] == "T1" {
			corr++
		}
	}
	if trace != 2 {
		t.Fatalf("expected 2 spans correlated to run1, got %d:\n%s", trace, first)
	}
	if corr != 2 {
		t.Fatalf("expected correlation preserved on both spans, got %d:\n%s", corr, first)
	}

	// R10.2: raw source paths/content/prompts must be absent by construction.
	for _, forbidden := range []string{"prompt", "reasoning", "source-content", "/var/www"} {
		if strings.Contains(first, forbidden) {
			t.Fatalf("otel export leaked raw data %q:\n%s", forbidden, first)
		}
	}

	second, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"format": "otel"})
	})
	if err != nil {
		t.Fatalf("second report --format otel: %v", err)
	}
	if first != second {
		t.Fatalf("otel export not byte-identical:\n--- 1 ---\n%s\n--- 2 ---\n%s", first, second)
	}
}

// TestTraceExportOTelRejectsForbiddenField confirms the export refuses a trace
// carrying reasoning/prompt/secret material rather than silently masking it,
// and that a spec with no traces exports nothing (graceful degradation).
func TestTraceExportOTelRejectsForbiddenField(t *testing.T) {
	root := newDemoSpec(t)

	// No traces yet: export is empty, not an error.
	out, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"format": "otel"})
	})
	if err != nil {
		t.Fatalf("empty otel export: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty export with no traces, got:\n%s", out)
	}

	seedTrace(t, root, "demo", "bad",
		`{"run_id":"bad","event_id":"e1","seq":1,"tool":"edit","time":"2026-07-05T09:00:00Z","actor":"craftsman","prompt":"leak me"}`,
	)
	if err := Run(root, "report", []string{"demo"}, map[string]string{"format": "otel"}); err == nil {
		t.Fatal("expected forbidden-field trace to fail export, got nil")
	}
}
