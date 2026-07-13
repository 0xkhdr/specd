package adapter

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/orchestration"
)

// OTelSpan is a dependency-free OpenTelemetry-compatible span projection.
// Attributes contain bounded classifications and correlation only: never raw
// source paths, source content, prompts, tool output, or secrets.
type OTelSpan struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Name       string            `json:"name"`
	Timestamp  string            `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ExportOTel validates local observable events then projects safe span data.
func ExportOTel(events []orchestration.ObservableEvent) ([]OTelSpan, error) {
	if err := orchestration.NormalizeTrace(events); err != nil {
		return nil, fmt.Errorf("otel export: %w", err)
	}
	spans := make([]OTelSpan, 0, len(events))
	for _, event := range events {
		attributes := map[string]string{
			"specd.actor":        event.Actor,
			"specd.arg_class":    event.ArgClass,
			"specd.result_class": event.ResultClass,
		}
		if event.Correlation != "" {
			attributes["specd.correlation_id"] = event.Correlation
		}
		spans = append(spans, OTelSpan{TraceID: event.RunID, SpanID: event.EventID, Name: event.Tool, Timestamp: event.Time, Attributes: attributes})
	}
	return spans, nil
}
