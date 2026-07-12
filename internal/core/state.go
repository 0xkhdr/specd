package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

const StateSchemaVersion = 2

// Clock is the injectable time source for record timestamps. Production uses
// wall-clock UTC; tests swap it for determinism. All record timestamps flow
// through here — never call time.Now directly in a record path.
var Clock = func() time.Time { return time.Now().UTC() }

// Record is a stamped ledger entry stored in State.Records. Content fields
// (Text/Scope/Gate/ApprovedRevision) are per-kind; StampRecord fills the
// provenance triple (Timestamp/GitHead/Actor) that ADR-6 observability and
// PROJECT.md §3 evidence integrity require on every record.
type Record struct {
	Kind             string `json:"kind"`
	Text             string `json:"text,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Gate             string `json:"gate,omitempty"`
	ApprovedRevision int64  `json:"approved_revision,omitempty"`
	// SourceDigest and CriteriaIDs are the compact, validated Domain 01
	// structured-intent metadata (spec 01 R1/R5): SourceDigest pins the approved
	// requirements/design source bytes via core.Digest so a later amendment can
	// detect drift; CriteriaIDs records which criterion IDs the record covers.
	// Both are additive and omitempty — records written by an older specd load
	// unchanged (backward compatible, no schema bump required).
	SourceDigest string   `json:"source_digest,omitempty"`
	CriteriaIDs  []string `json:"criteria_ids,omitempty"`
	Timestamp    string   `json:"timestamp"`
	GitHead      string   `json:"git_head"`
	Actor        string   `json:"actor"`
}

// StampRecord fills the provenance triple on rec: an RFC 3339 timestamp from
// the injectable Clock, the caller-resolved git HEAD, and the host actor. The
// actor is host-reported and stored verbatim — never trusted as proof.
func StampRecord(rec Record, gitHead string) Record {
	rec.Timestamp = Clock().Format(time.RFC3339)
	rec.GitHead = gitHead
	rec.Actor = recordActor()
	return rec
}

// recordActor resolves the acting identity: $SPECD_ACTOR (already in the
// scrubbed-env allowlist) if set, else the OS user, else "unknown".
func recordActor() string {
	if actor := os.Getenv("SPECD_ACTOR"); actor != "" {
		return actor
	}
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "unknown"
}

var ErrRevisionConflict = errors.New("state revision conflict")

type Mode string

const (
	ModeDefault      Mode = "default"
	ModeAgent        Mode = "agent"
	ModeOrchestrated Mode = "orchestrated"
)

func ValidMode(mode Mode) bool {
	switch mode {
	case ModeDefault, ModeAgent, ModeOrchestrated:
		return true
	default:
		return false
	}
}

type State struct {
	SchemaVersion int                        `json:"schema_version"`
	Slug          string                     `json:"slug"`
	Mode          Mode                       `json:"mode"`
	Status        Status                     `json:"status"`
	Phase         Phase                      `json:"phase"`
	Revision      int64                      `json:"revision"`
	Records       map[string]json.RawMessage `json:"records,omitempty"`
	// TaskStatus is the machine truth for per-task run status (ADR-1: status
	// lives in state.json, tasks.md stays clean Markdown). The Sync gate
	// enforces that tasks.md markers agree with this map.
	TaskStatus map[string]TaskRunStatus   `json:"task_status,omitempty"`
	Extra      map[string]json.RawMessage `json:"extra,omitempty"`
}

func InitialState(slug string) State {
	return State{
		SchemaVersion: StateSchemaVersion,
		Slug:          slug,
		Mode:          ModeDefault,
		Status:        StatusRequirements,
		Phase:         PhaseForStatus(StatusRequirements),
		Revision:      0,
		Records:       map[string]json.RawMessage{},
	}
}

func StatePath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "state.json")
}

func LoadState(path string) (State, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var state State
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode %s: %w", path, err)
	}
	state, err = MigrateState(state)
	if err != nil {
		return State{}, err
	}
	if state.Records == nil {
		state.Records = map[string]json.RawMessage{}
	}
	return state, state.Validate()
}

func MigrateState(state State) (State, error) {
	switch state.SchemaVersion {
	case 0:
		state.SchemaVersion = 1
		return MigrateState(state)
	case 1:
		state.SchemaVersion = StateSchemaVersion
	case StateSchemaVersion:
	default:
		return State{}, fmt.Errorf("unsupported state schema %d", state.SchemaVersion)
	}
	return state, nil
}

func SaveState(path string, state State) error {
	if err := state.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')
	return AtomicWrite(path, string(data))
}

func SaveStateCAS(path string, expectedRevision int64, state State) error {
	current, err := LoadState(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if expectedRevision != 0 {
			return fmt.Errorf("%w: expected %d, file absent", ErrRevisionConflict, expectedRevision)
		}
	} else if current.Revision != expectedRevision {
		return fmt.Errorf("%w: expected %d, got %d", ErrRevisionConflict, expectedRevision, current.Revision)
	}
	state.Revision = expectedRevision + 1
	return SaveState(path, state)
}

func (s State) Validate() error {
	if s.SchemaVersion != StateSchemaVersion {
		return fmt.Errorf("unsupported state schema %d", s.SchemaVersion)
	}
	if s.Slug == "" {
		return errors.New("state slug is required")
	}
	if !ValidMode(s.Mode) {
		return fmt.Errorf("invalid state mode %q", s.Mode)
	}
	if !ValidStatus(s.Status) {
		return fmt.Errorf("invalid state status %q", s.Status)
	}
	if !ValidPhase(s.Phase) {
		return fmt.Errorf("invalid state phase %q", s.Phase)
	}
	if s.Phase != PhaseForStatus(s.Status) && s.Status != StatusBlocked {
		return fmt.Errorf("state phase %q does not match status %q", s.Phase, s.Status)
	}
	return nil
}
