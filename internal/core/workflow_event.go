package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const WorkflowEventSchemaVersion = 1

// WorkflowEventV1 is durable provenance for one state transition. Projection
// contains only governed state; inputs are represented by identities and
// digests so prompts, command output, and secrets never enter the ledger.
type WorkflowEventV1 struct {
	SchemaVersion       int               `json:"schema_version"`
	ID                  string            `json:"id"`
	EntityKind          string            `json:"entity_kind"`
	EntityID            string            `json:"entity_id"`
	BeforeEntityVersion int64             `json:"before_entity_version"`
	AfterEntityVersion  int64             `json:"after_entity_version"`
	ExpectedRevision    int64             `json:"expected_revision"`
	ResultingRevision   int64             `json:"resulting_revision"`
	Transition          string            `json:"transition"`
	Actor               string            `json:"actor"`
	AuthorityDigest     string            `json:"authority_digest"`
	Reason              string            `json:"reason"`
	InputDigests        map[string]string `json:"input_digests,omitempty"`
	ImpactedEntities    []string          `json:"impacted_entities,omitempty"`
	GitHead             string            `json:"git_head,omitempty"`
	Timestamp           string            `json:"timestamp"`
	Projection          State             `json:"projection"`
}

func WorkflowEventPath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "workflow-events.jsonl")
}

// CanonicalWorkflowEventID returns the content address of an event. The ID and
// projection checkpoint are excluded to avoid a circular digest dependency.
func CanonicalWorkflowEventID(event WorkflowEventV1) (string, error) {
	event.ID = ""
	event.Projection.LastEventID = ""
	raw, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("encode workflow event: %w", err)
	}
	return Digest(raw), nil
}

func (event WorkflowEventV1) Validate() error {
	if event.SchemaVersion != WorkflowEventSchemaVersion {
		return fmt.Errorf("unsupported workflow event schema %d", event.SchemaVersion)
	}
	for name, value := range map[string]string{
		"id": event.ID, "entity_kind": event.EntityKind, "entity_id": event.EntityID,
		"transition": event.Transition, "actor": event.Actor, "reason": event.Reason,
		"timestamp": event.Timestamp,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("workflow event %s is required", name)
		}
	}
	if event.ResultingRevision != event.ExpectedRevision+1 {
		return errors.New("workflow event resulting revision must follow expected revision")
	}
	if event.AfterEntityVersion != event.BeforeEntityVersion+1 {
		return errors.New("workflow event entity version must advance by one")
	}
	if event.Projection.Revision != event.ResultingRevision {
		return errors.New("workflow event projection revision mismatch")
	}
	if event.Projection.LastEventID != event.ID {
		return errors.New("workflow event projection checkpoint mismatch")
	}
	for name, digest := range event.InputDigests {
		if strings.TrimSpace(name) == "" || !validDigest(digest) {
			return fmt.Errorf("workflow event input digest %q is invalid", name)
		}
	}
	if !validDigest(event.AuthorityDigest) {
		return errors.New("workflow event authority_digest is invalid")
	}
	want, err := CanonicalWorkflowEventID(event)
	if err != nil {
		return err
	}
	if event.ID != want {
		return errors.New("workflow event id does not match canonical digest")
	}
	return event.Projection.Validate()
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

// NewWorkflowEvent fills the deterministic identity and projection checkpoint.
func NewWorkflowEvent(event WorkflowEventV1) (WorkflowEventV1, error) {
	event.SchemaVersion = WorkflowEventSchemaVersion
	event.ResultingRevision = event.ExpectedRevision + 1
	event.Projection.Revision = event.ResultingRevision
	event.Projection.LastEventID = ""
	id, err := CanonicalWorkflowEventID(event)
	if err != nil {
		return WorkflowEventV1{}, err
	}
	event.ID = id
	event.Projection.LastEventID = id
	return event, event.Validate()
}

func AppendWorkflowEvent(path string, event WorkflowEventV1) error {
	if err := event.Validate(); err != nil {
		return err
	}
	events, err := ReadWorkflowEvents(path)
	if err != nil {
		return err
	}
	for _, prior := range events {
		if prior.ID == event.ID {
			return fmt.Errorf("duplicate workflow event id %q", event.ID)
		}
	}
	if len(events) > 0 && events[len(events)-1].ResultingRevision != event.ExpectedRevision {
		return fmt.Errorf("workflow event revision chain: expected %d after %d", event.ExpectedRevision, events[len(events)-1].ResultingRevision)
	}
	if raw, readErr := os.ReadFile(path); readErr == nil && len(raw) > 0 && !bytes.HasSuffix(raw, []byte("\n")) {
		if err := os.Truncate(path, int64(bytes.LastIndexByte(raw, '\n')+1)); err != nil {
			return fmt.Errorf("discard torn workflow event tail: %w", err)
		}
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return AppendFile(path, string(raw)+"\n")
}

// ReadWorkflowEvents accepts only one recoverable corruption: an incomplete
// final line without a newline. Every complete malformed line fails closed.
func ReadWorkflowEvents(path string) ([]WorkflowEventV1, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 && !bytes.HasSuffix(raw, []byte("\n")) {
		raw = raw[:bytes.LastIndexByte(raw, '\n')+1]
	}
	var events []WorkflowEventV1
	for _, line := range bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var event WorkflowEventV1
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, fmt.Errorf("decode workflow event: %w", err)
		}
		if err := event.Validate(); err != nil {
			return nil, err
		}
		if len(events) > 0 && events[len(events)-1].ResultingRevision != event.ExpectedRevision {
			return nil, errors.New("workflow event revision chain mismatch")
		}
		events = append(events, event)
	}
	return events, nil
}

// TransitionCommit performs the event-first half-transaction followed by the
// projection CAS. A crash between them is completed by RecoverWorkflowState.
type TransitionCommit struct {
	StatePath string
	EventPath string
	Event     WorkflowEventV1
}

func CommitWorkflowTransition(commit TransitionCommit) error {
	state, err := RecoverWorkflowState(commit.StatePath, commit.EventPath)
	if err != nil {
		return err
	}
	if state.Revision != commit.Event.ExpectedRevision {
		return fmt.Errorf("%w: expected %d, got %d", ErrRevisionConflict, commit.Event.ExpectedRevision, state.Revision)
	}
	if err := AppendWorkflowEvent(commit.EventPath, commit.Event); err != nil {
		return err
	}
	return SaveStateCAS(commit.StatePath, state.Revision, commit.Event.Projection)
}

func ReplayWorkflowEvents(baseline State, events []WorkflowEventV1) (State, error) {
	state := baseline
	for _, event := range events {
		if err := event.Validate(); err != nil {
			return State{}, err
		}
		if state.Revision != event.ExpectedRevision {
			return State{}, fmt.Errorf("workflow replay revision conflict: state %d, event expects %d", state.Revision, event.ExpectedRevision)
		}
		state = event.Projection
	}
	return state, nil
}

// RecoverWorkflowState applies durable events after the state's checkpoint.
func RecoverWorkflowState(statePath, eventPath string) (State, error) {
	state, err := LoadState(statePath)
	if err != nil {
		return State{}, err
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return State{}, err
	}
	start := 0
	if state.LastEventID != "" {
		start = -1
		for i, event := range events {
			if event.ID == state.LastEventID {
				if event.ResultingRevision != state.Revision {
					return State{}, errors.New("workflow projection checkpoint revision mismatch")
				}
				start = i + 1
				break
			}
		}
		if start < 0 {
			return State{}, errors.New("workflow projection checkpoint is absent from ledger")
		}
	}
	for _, event := range events[start:] {
		if state.Revision != event.ExpectedRevision {
			return State{}, errors.New("workflow projection is ahead of or diverges from ledger")
		}
		next := event.Projection
		if err := SaveStateCAS(statePath, state.Revision, next); err != nil {
			return State{}, err
		}
		state = next
	}
	return state, nil
}
