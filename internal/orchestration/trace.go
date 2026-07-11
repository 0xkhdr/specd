package orchestration

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
)

// ObservableEvent is one run-scoped, externally observable step in a worker
// trajectory (spec 04 R4.1). It records what was done — tool/action identity,
// sanitized argument/result *class* (never raw bodies), affected paths, when,
// who, and a correlation id — and nothing about how the agent reasoned. There
// is deliberately no field for prompts, chain-of-thought, or secrets: hidden
// reasoning is rejected at the parse boundary, not masked (R4.1).
type ObservableEvent struct {
	RunID       string   `json:"run_id"`
	EventID     string   `json:"event_id"`
	Seq         int      `json:"seq"`
	Tool        string   `json:"tool"`
	ArgClass    string   `json:"arg_class,omitempty"`
	ResultClass string   `json:"result_class,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	Time        string   `json:"time"`
	Actor       string   `json:"actor"`
	Correlation string   `json:"correlation,omitempty"`
}

// forbiddenTraceKeys are JSON keys a normalized trace must never carry: raw
// reasoning, prompts, or secret material. Their presence fails the parse rather
// than being silently stripped — vague masking is not trusted (R4.1, R4.2).
var forbiddenTraceKeys = []string{
	"reasoning", "thinking", "chain_of_thought", "cot",
	"prompt", "system_prompt", "messages",
	"secret", "secrets", "token", "api_key", "raw",
}

var allowedTraceKeys = map[string]bool{
	"run_id": true, "event_id": true, "seq": true, "tool": true,
	"arg_class": true, "result_class": true, "paths": true, "time": true,
	"actor": true, "correlation": true,
}

// ParseTrace reads a JSONL trace, rejecting any line that carries a forbidden
// field, then normalizes the events. It returns a stable ordered error naming
// the first offending line/field, or the normalized events.
func ParseTrace(raw []byte) ([]ObservableEvent, error) {
	var events []ObservableEvent
	for i, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(line, &probe); err != nil {
			return nil, fmt.Errorf("TRACE_MALFORMED: line %d: %w", i+1, err)
		}
		for _, k := range forbiddenTraceKeys {
			if _, ok := probe[k]; ok {
				return nil, fmt.Errorf("TRACE_FORBIDDEN_FIELD: line %d carries %q", i+1, k)
			}
		}
		for k := range probe {
			if !allowedTraceKeys[k] {
				return nil, fmt.Errorf("TRACE_UNKNOWN_FIELD: line %d carries %q", i+1, k)
			}
		}
		var ev ObservableEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("TRACE_MALFORMED: line %d: %w", i+1, err)
		}
		events = append(events, ev)
	}
	if err := NormalizeTrace(events); err != nil {
		return nil, err
	}
	return events, nil
}

// NormalizeTrace validates a decoded trace: every event names its run/event id,
// tool, time and actor; all events share one run id; event ids are unique; and
// the sequence is strictly increasing (spec 04 R4.2 — duplicate/non-monotonic
// fail closed). It mutates nothing; a bad trace is refused, not repaired.
func NormalizeTrace(events []ObservableEvent) error {
	seenID := map[string]bool{}
	runID := ""
	prevSeq := 0
	for i, ev := range events {
		if ev.RunID == "" || ev.EventID == "" || ev.Tool == "" || ev.Time == "" || ev.Actor == "" {
			return fmt.Errorf("TRACE_REQUIRED_FIELD: event %d missing run/event/tool/time/actor", i+1)
		}
		if runID == "" {
			runID = ev.RunID
		} else if ev.RunID != runID {
			return fmt.Errorf("TRACE_RUN_MIXED: event %d run_id %q != %q", i+1, ev.RunID, runID)
		}
		if seenID[ev.EventID] {
			return fmt.Errorf("TRACE_DUPLICATE_EVENT: %q", ev.EventID)
		}
		if ev.Seq < 1 {
			return fmt.Errorf("TRACE_SEQUENCE_INVALID: event %d seq %d", i+1, ev.Seq)
		}
		seenID[ev.EventID] = true
		if i > 0 && ev.Seq <= prevSeq {
			return fmt.Errorf("TRACE_SEQUENCE_NONMONOTONIC: event %d seq %d <= %d", i+1, ev.Seq, prevSeq)
		}
		prevSeq = ev.Seq
	}
	return nil
}

// TraceDigest is the content address of a normalized trace: the SHA-256 of its
// canonical JSON encoding. A trajectory eval record pins this so completion can
// confirm the score was produced against exactly these events (spec 04 R4.2).
func TraceDigest(events []ObservableEvent) string {
	raw, _ := json.Marshal(events)
	return core.Digest(raw)
}
