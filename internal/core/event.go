package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// EventSchemaV1 identifies specd's provider-neutral, metadata-only local event
// contract. External adapters may map this data to OpenTelemetry; core only
// validates and renders it and therefore stays offline and dependency-free.
const EventSchemaV1 = "event/v1"

// EventV1 is the stable bridge between local run spans and external telemetry
// adapters. It deliberately has no prompt, response, file-content, raw-output,
// or arbitrary attributes field. Privacy decisions remain explicit metadata.
type EventV1 struct {
	SchemaVersion   string   `json:"schema_version"`
	EventID         string   `json:"event_id"`
	RunID           string   `json:"run_id,omitempty"`
	SpanID          string   `json:"span_id,omitempty"`
	ParentSpanID    string   `json:"parent_span_id,omitempty"`
	SpecID          string   `json:"spec_id"`
	TaskID          string   `json:"task_id,omitempty"`
	Attempt         int      `json:"attempt,omitempty"`
	Kind            SpanKind `json:"kind"`
	Timestamp       string   `json:"timestamp,omitempty"`
	Status          string   `json:"status,omitempty"`
	GitHead         string   `json:"git_head,omitempty"`
	TelemetrySource string   `json:"telemetry_source,omitempty"`
	AttestationRef  string   `json:"attestation_ref,omitempty"`
	EvidenceRef     string   `json:"evidence_ref,omitempty"`
	Redactions      []string `json:"redactions,omitempty"`
}

func (e EventV1) Validate() error {
	if e.SchemaVersion != EventSchemaV1 {
		return fmt.Errorf("unknown event schema version %q", e.SchemaVersion)
	}
	if e.EventID == "" || e.SpecID == "" {
		return errors.New("event requires event_id and spec_id")
	}
	if _, err := ParseSpanKind(string(e.Kind)); err != nil {
		return err
	}
	if e.Attempt < 0 {
		return errors.New("event attempt must be non-negative")
	}
	if e.Kind.ClaimsCodeEffect() && e.GitHead == "" {
		return fmt.Errorf("%s event %q claims code effects but carries no git_head", e.Kind, e.EventID)
	}
	if e.TelemetrySource != "" {
		switch e.TelemetrySource {
		case TelemetrySourceWorker, TelemetrySourceAdapter, TelemetrySourceOperator:
		default:
			return fmt.Errorf("unknown telemetry_source %q", e.TelemetrySource)
		}
	}
	if e.EvidenceRef != "" {
		if err := validateEvidenceRef(e.EvidenceRef); err != nil {
			return err
		}
	}
	if e.AttestationRef != "" {
		if err := validateEvidenceRef(e.AttestationRef); err != nil {
			return err
		}
	}
	if !sort.StringsAreSorted(e.Redactions) {
		return errors.New("event redactions must be sorted")
	}
	for _, r := range e.Redactions {
		if strings.TrimSpace(r) == "" {
			return errors.New("event redaction must be non-empty")
		}
	}
	return nil
}

// EventFromSpan projects the existing correlated local span without inventing
// identity or outcome. Timestamp remains informational.
func EventFromSpan(s RunSpan) EventV1 {
	return EventV1{SchemaVersion: EventSchemaV1, EventID: s.SpanID, RunID: s.RunID, SpanID: s.SpanID, ParentSpanID: s.ParentSpanID, SpecID: s.SpecID, TaskID: s.TaskID, Attempt: s.Attempt, Kind: s.Kind, Timestamp: s.StartedAt, Status: s.Status, GitHead: s.GitHead}
}

// RenderEventsJSON emits canonical JSON Lines in caller-provided deterministic
// order. Struct field order and encoding/json's stable encoding make repeated
// renders byte-identical.
func RenderEventsJSON(events []EventV1) (string, error) {
	var b strings.Builder
	seen := make(map[string]bool, len(events))
	for _, e := range events {
		if err := e.Validate(); err != nil {
			return "", err
		}
		if seen[e.EventID] {
			return "", fmt.Errorf("duplicate event_id %q", e.EventID)
		}
		seen[e.EventID] = true
		raw, err := json.Marshal(e)
		if err != nil {
			return "", err
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String(), nil
}
