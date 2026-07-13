package adapter

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// ExportNeutralEvents maps Domain 07's canonical local schema to an
// OpenTelemetry-compatible projection. It performs no I/O; transport remains
// an external adapter concern. Correlation and privacy audit fields survive as
// bounded attributes, while raw content cannot enter because EventV1 has no
// such field.
func ExportNeutralEvents(events []core.EventV1) ([]OTelSpan, error) {
	spans := make([]OTelSpan, 0, len(events))
	for _, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("otel event export: %w", err)
		}
		attributes := map[string]string{"specd.spec_id": event.SpecID, "specd.kind": string(event.Kind), "specd.status": event.Status}
		for key, value := range map[string]string{"specd.run_id": event.RunID, "specd.parent_span_id": event.ParentSpanID, "specd.task_id": event.TaskID, "specd.telemetry_source": event.TelemetrySource, "specd.attestation_ref": event.AttestationRef, "specd.evidence_ref": event.EvidenceRef} {
			if value != "" {
				attributes[key] = value
			}
		}
		if len(event.Redactions) > 0 {
			attributes["specd.redactions"] = strings.Join(event.Redactions, ",")
		}
		spans = append(spans, OTelSpan{TraceID: event.RunID, SpanID: event.SpanID, Name: string(event.Kind), Timestamp: event.Timestamp, Attributes: attributes})
	}
	return spans, nil
}

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
