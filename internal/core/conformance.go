package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
)

// Conformance event kinds: the eight protocol violations R7.1 enumerates.
//
// Each names something an agent did that the protocol says it should not have.
// Recording them answers a question no gate can: not "was this work valid?" —
// the gates already settle that — but "did hosts actually enter the protocol, or
// are they routing around it?" A harness whose gates all pass while these events
// accumulate is a harness being worked around.
const (
	ConformanceWorkWithoutBootstrap  = "work_without_bootstrap"
	ConformanceStaleActionReplayed   = "stale_action_replayed"
	ConformanceActedWithoutAuthority = "acted_without_authority"
	ConformanceUndeclaredPathTouched = "undeclared_path_touched"
	ConformanceHumanOnlyInvoked      = "human_only_invoked"
	ConformanceContextAckSkipped     = "context_ack_skipped"
	ConformancePrematureCompletion   = "premature_completion"
	ConformanceDirectSpecdMutation   = "direct_specd_mutation"
)

// ConformanceEventKinds is the complete set, in stable order. Exported so a
// reporter can enumerate kinds without hard-coding them, and so a test can
// assert every kind is recordable.
var ConformanceEventKinds = []string{
	ConformanceActedWithoutAuthority,
	ConformanceContextAckSkipped,
	ConformanceDirectSpecdMutation,
	ConformanceHumanOnlyInvoked,
	ConformancePrematureCompletion,
	ConformanceStaleActionReplayed,
	ConformanceUndeclaredPathTouched,
	ConformanceWorkWithoutBootstrap,
}

// ConformanceEvent is one observed protocol violation.
//
// Observational by construction (R7.2). Nothing in internal/core/gates reads
// this file, no lifecycle transition consults it, and deleting it changes no
// outcome — TestConformanceEventsRemovingLogChangesNoGateOutcome pins exactly
// that. The moment a gate branches on one of these, the record stops being an
// honest observation and becomes a second enforcement path with none of the
// evidence guarantees behind it.
type ConformanceEvent struct {
	Kind      string `json:"kind"`
	Slug      string `json:"slug"`
	TaskID    string `json:"task_id,omitempty"`
	Actor     string `json:"actor,omitempty"`
	Operation string `json:"operation,omitempty"`

	// Detail is a short human-readable note. Redacted like every other ledger
	// field: an observation is not a reason to leak a secret or an absolute
	// home path into a file that may be committed.
	Detail    string `json:"detail,omitempty"`
	Timestamp string `json:"timestamp"`
}

func ConformancePath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "conformance.jsonl")
}

// IsConformanceKind reports whether kind is one of the R7.1 events.
func IsConformanceKind(kind string) bool {
	for _, known := range ConformanceEventKinds {
		if known == kind {
			return true
		}
	}
	return false
}

// RecordConformanceEvent appends one observation.
//
// It returns an error only for a malformed event, never for a failure to
// observe: a caller must not be able to change control flow by whether the
// observation succeeded, or the log becomes load-bearing by the back door.
// Callers are expected to ignore the returned error in enforcement paths.
func RecordConformanceEvent(root, slug string, event ConformanceEvent) error {
	if !IsConformanceKind(event.Kind) {
		return Refusef("CONFORMANCE_KIND_UNKNOWN", "%q is not a recorded protocol event", event.Kind)
	}
	if slug == "" {
		return Refuse("CONFORMANCE_SLUG_REQUIRED", "a conformance event must name the spec it was observed against")
	}
	event.Slug = slug
	if event.Timestamp == "" {
		event.Timestamp = Clock().Format(time.RFC3339)
	}
	if event.Actor == "" {
		event.Actor = recordActor()
	}
	redactor := verifyexec.NewRedactor(nil)
	event.Detail = redactor.String(event.Detail)
	event.Operation = redactor.String(event.Operation)

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return AppendFile(ConformancePath(root, slug), string(data)+"\n")
}

// LoadConformanceEvents reads the log in write order. A missing log is an empty
// history, not an error: a spec no host ever violated has nothing to read.
//
// A malformed line is skipped rather than fatal. The log is observational, so a
// corrupt entry must not break the reporter that reads it — and refusing to
// report anything because one line is bad would lose the observations that are
// intact.
func LoadConformanceEvents(path string) ([]ConformanceEvent, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []ConformanceEvent
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event ConformanceEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// ConformanceSummary counts observations by kind, including kinds never seen so
// a report distinguishes "zero" from "not tracked".
func ConformanceSummary(events []ConformanceEvent) map[string]int {
	summary := make(map[string]int, len(ConformanceEventKinds))
	for _, kind := range ConformanceEventKinds {
		summary[kind] = 0
	}
	for _, event := range events {
		if _, ok := summary[event.Kind]; ok {
			summary[event.Kind]++
		}
	}
	return summary
}
