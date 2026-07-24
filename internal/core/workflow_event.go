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

const (
	transitionArtifactBeforeDigest = "transition_artifact_before"
	transitionArtifactAfterDigest  = "transition_artifact_after"
)

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
	Artifact  *TransitionArtifact
}

// TransitionArtifact extends the event-first transaction to one derived
// artifact. Before and After are held in a durable sidecar and bound by digest
// into the event, so recovery writes only content authenticated by the ledger.
type TransitionArtifact struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type transitionJournal struct {
	EventID  string             `json:"event_id"`
	Event    WorkflowEventV1    `json:"event"`
	Artifact TransitionArtifact `json:"artifact"`
}

func transitionJournalPath(eventPath string) string {
	return eventPath + ".transaction.json"
}

func transitionArtifactPath(statePath, eventPath string) (string, error) {
	statePath, err := filepath.Abs(statePath)
	if err != nil {
		return "", err
	}
	eventPath, err = filepath.Abs(eventPath)
	if err != nil {
		return "", err
	}
	if filepath.Base(statePath) != "state.json" || filepath.Base(eventPath) != "workflow-events.jsonl" ||
		filepath.Dir(statePath) != filepath.Dir(eventPath) {
		return "", errors.New("transition state and event paths do not identify one spec")
	}
	return filepath.Join(filepath.Dir(eventPath), "tasks.md"), nil
}

func bindTransitionArtifact(event WorkflowEventV1, artifact TransitionArtifact) (WorkflowEventV1, error) {
	inputs := make(map[string]string, len(event.InputDigests)+2)
	for name, digest := range event.InputDigests {
		inputs[name] = digest
	}
	inputs[transitionArtifactBeforeDigest] = Digest([]byte(artifact.Before))
	inputs[transitionArtifactAfterDigest] = Digest([]byte(artifact.After))
	event.ID = ""
	event.Projection.LastEventID = ""
	event.InputDigests = inputs
	return NewWorkflowEvent(event)
}

func validateTransitionArtifact(event WorkflowEventV1, artifact TransitionArtifact) error {
	if event.InputDigests[transitionArtifactBeforeDigest] != Digest([]byte(artifact.Before)) ||
		event.InputDigests[transitionArtifactAfterDigest] != Digest([]byte(artifact.After)) {
		return errors.New("transition journal artifact content does not match event")
	}
	return nil
}

func CommitWorkflowTransition(commit TransitionCommit) error {
	state, err := RecoverWorkflowState(commit.StatePath, commit.EventPath)
	if err != nil {
		return err
	}
	if state.Revision != commit.Event.ExpectedRevision {
		return fmt.Errorf("%w: expected %d, got %d", ErrRevisionConflict, commit.Event.ExpectedRevision, state.Revision)
	}
	if err := commit.Event.Validate(); err != nil {
		return err
	}
	journalPath := transitionJournalPath(commit.EventPath)
	var artifactPath string
	if commit.Artifact != nil {
		artifactPath, err = transitionArtifactPath(commit.StatePath, commit.EventPath)
		if err != nil {
			return err
		}
		providedPath, err := filepath.Abs(commit.Artifact.Path)
		if err != nil || providedPath != artifactPath {
			return errors.New("transition artifact must be the spec tasks.md")
		}
		current, err := os.ReadFile(artifactPath)
		if err != nil {
			return err
		}
		if string(current) != commit.Artifact.Before {
			return errors.New("transition artifact changed since preview")
		}
		if err := validateTransitionArtifact(commit.Event, *commit.Artifact); err != nil {
			return err
		}
		artifact := *commit.Artifact
		artifact.Path = artifactPath
		raw, err := json.Marshal(transitionJournal{EventID: commit.Event.ID, Event: commit.Event, Artifact: artifact})
		if err != nil {
			return err
		}
		if err := AtomicWrite(journalPath, string(raw)+"\n"); err != nil {
			return err
		}
	}
	if err := AppendWorkflowEvent(commit.EventPath, commit.Event); err != nil {
		// An append can report a late fsync error after the bytes reached the
		// ledger. Re-read before recovering so a durable event rolls forward
		// rather than being paired with a rolled-back artifact.
		if events, readErr := ReadWorkflowEvents(commit.EventPath); readErr == nil {
			_, _ = recoverTransitionArtifact(commit.StatePath, commit.EventPath, state, events)
		}
		return err
	}
	if commit.Artifact != nil {
		if err := AtomicWrite(artifactPath, commit.Artifact.After); err != nil {
			return err
		}
	}
	if err := SaveStateCAS(commit.StatePath, state.Revision, commit.Event.Projection); err != nil {
		return err
	}
	if commit.Artifact != nil {
		return RemoveFileDurable(journalPath)
	}
	return nil
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
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return State{}, err
	}
	state, err := LoadState(statePath)
	if err != nil {
		return State{}, err
	}
	journal, err := recoverTransitionArtifact(statePath, eventPath, state, events)
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
	if journal {
		if err := RemoveFileDurable(transitionJournalPath(eventPath)); err != nil {
			return State{}, err
		}
	}
	return state, nil
}

// recoverTransitionArtifact returns whether a committed-event journal remains
// to be removed after state replay. With no durable event, recovery removes the
// journal only while the artifact is still unchanged; legacy sidecars that
// would require an unauthenticated rollback fail closed.
func recoverTransitionArtifact(statePath, eventPath string, state State, events []WorkflowEventV1) (bool, error) {
	path := transitionJournalPath(eventPath)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var journal transitionJournal
	if err := json.Unmarshal(raw, &journal); err != nil {
		return false, fmt.Errorf("decode transition journal: %w", err)
	}
	if journal.EventID == "" || journal.Artifact.Path == "" {
		return false, errors.New("transition journal is incomplete")
	}
	if err := journal.Event.Validate(); err != nil {
		return false, fmt.Errorf("validate transition journal event: %w", err)
	}
	if journal.EventID != journal.Event.ID {
		return false, errors.New("transition journal event identity mismatch")
	}
	artifactPath, err := transitionArtifactPath(statePath, eventPath)
	if err != nil {
		return false, err
	}
	journalArtifactPath, err := filepath.Abs(journal.Artifact.Path)
	if err != nil || journalArtifactPath != artifactPath {
		return false, errors.New("transition journal artifact must be the spec tasks.md")
	}
	var committedEvent *WorkflowEventV1
	for i, event := range events {
		if event.ID == journal.EventID {
			if i != len(events)-1 {
				return false, errors.New("transition journal event is not the ledger tip")
			}
			committedEvent = &events[i]
			break
		}
	}
	if committedEvent != nil {
		if state.Revision != journal.Event.ExpectedRevision && state.Revision != journal.Event.ResultingRevision {
			return false, errors.New("transition journal event does not match state revision")
		}
	} else {
		if state.Revision != journal.Event.ExpectedRevision {
			return false, errors.New("transition journal event does not follow state revision")
		}
		if len(events) > 0 && events[len(events)-1].ResultingRevision != journal.Event.ExpectedRevision {
			return false, errors.New("transition journal event does not follow ledger revision")
		}
	}
	current, err := os.ReadFile(artifactPath)
	if err != nil {
		return false, err
	}
	if committedEvent == nil {
		if string(current) != journal.Artifact.Before {
			return false, errors.New("uncommitted transition journal cannot restore artifact content")
		}
		return false, RemoveFileDurable(path)
	}
	if err := validateTransitionArtifact(*committedEvent, journal.Artifact); err != nil {
		return false, err
	}
	if string(current) != journal.Artifact.Before && string(current) != journal.Artifact.After {
		return false, errors.New("transition journal artifact content mismatch")
	}
	if err := AtomicWrite(artifactPath, journal.Artifact.After); err != nil {
		return false, err
	}
	return true, nil
}
