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

const (
	StateSchemaVersion = 2
	// LegacyStateSchemaVersion is the v1 on-disk shape. It is still readable
	// during the compatibility window: loading projects it onto the canonical
	// schema-2 fields in memory. Only `specd` migration writes v2 durably.
	LegacyStateSchemaVersion = 1
)

// PreflightStateSchema checks compatibility without decoding or mutating state.
// Installers use it before replacing a binary so future state cannot be opened
// by an older binary and silently downgraded.
func PreflightStateSchema(raw []byte) error {
	var header struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return fmt.Errorf("state schema preflight: %w", err)
	}
	if header.SchemaVersion == nil {
		return errors.New("state schema preflight: schema_version is required")
	}
	if *header.SchemaVersion > StateSchemaVersion {
		return fmt.Errorf("state schema preflight: unsafe downgrade from schema %d to %d", *header.SchemaVersion, StateSchemaVersion)
	}
	if *header.SchemaVersion < 0 {
		return fmt.Errorf("state schema preflight: invalid schema %d", *header.SchemaVersion)
	}
	return nil
}

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
	// Both are omitempty: records without structured-intent metadata are valid.
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
	SchemaVersion int    `json:"schema_version"`
	Slug          string `json:"slug"`
	Mode          Mode   `json:"mode"`
	// Status is the schema-1 compatibility projection of Stage/Condition. It
	// stays on disk for legacy readers; ProjectStatus owns its value.
	Status Status `json:"status"`
	Phase  Phase  `json:"phase"`
	// Cycle, Stage, Condition, and CurrentRequest are the canonical schema-2
	// lifecycle facts (spec 03 R2.1). Cycle is 1 for every migrated v1 spec.
	Cycle          int                        `json:"cycle,omitempty"`
	Stage          Stage                      `json:"stage,omitempty"`
	Condition      Condition                  `json:"condition,omitempty"`
	CurrentRequest string                     `json:"current_request,omitempty"`
	Revision       int64                      `json:"revision"`
	LastEventID    string                     `json:"last_event_id,omitempty"`
	Records        map[string]json.RawMessage `json:"records,omitempty"`
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
		Cycle:         1,
		Stage:         StageRequirements,
		Condition:     ConditionActive,
		Revision:      0,
		Records:       map[string]json.RawMessage{},
	}
}

func StatePath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "state.json")
}

func LoadState(path string) (State, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	state, err := DecodeState(raw)
	if err != nil {
		return State{}, fmt.Errorf("decode %s: %w", path, err)
	}
	// A v1 file is read through the compatibility projection so existing
	// projects keep working before migration commits (spec 03 R6.1).
	state.SchemaVersion = StateSchemaVersion
	state.projectCanonical()
	return state, state.Validate()
}

// DecodeState decodes on-disk state bytes without projecting or validating.
// Schema newer than this binary fails here, before any mutation (spec 03 R1.4).
func DecodeState(raw []byte) (State, error) {
	var state State
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&state); err != nil {
		return State{}, err
	}
	if state.SchemaVersion > StateSchemaVersion || state.SchemaVersion < LegacyStateSchemaVersion {
		return State{}, fmt.Errorf("unsupported state schema %d (this binary supports %d and %d)", state.SchemaVersion, LegacyStateSchemaVersion, StateSchemaVersion)
	}
	if state.Records == nil {
		state.Records = map[string]json.RawMessage{}
	}
	return state, nil
}

// projectCanonical derives the canonical pair for a state that a schema-1
// mutator wrote. During the compatibility window the legacy status is still the
// mutation surface, so stage is adopted from it and the unremarkable conditions
// (absent, active, complete, stale block) follow the stage. A deliberate
// condition — paused, waiting, cancelled — is never repaired: it must reach
// ValidateStageCondition and be rejected there if the combination is illegal.
func (s *State) projectCanonical() {
	if s.Cycle == 0 {
		s.Cycle = 1
	}
	if s.Status == StatusBlocked {
		// The stage a legacy blocked state was blocked in is unrecoverable;
		// Validate refuses it with the repair diagnostic (spec 03 R6.2).
		s.Condition = ConditionBlocked
		return
	}
	if !ValidStatus(s.Status) {
		return
	}
	s.Stage = Stage(s.Status)
	switch s.Condition {
	case "", ConditionActive, ConditionComplete, ConditionBlocked:
		s.Condition = ConditionActive
		if s.Stage == StageComplete {
			s.Condition = ConditionComplete
		}
	}
}

// MarshalJSON projects the canonical fields on the way out so every serialized
// state — file bytes, event projection, report — carries the same derived pair
// and replay stays byte-for-field equivalent (spec 03 R1.2, R2.3).
func (s State) MarshalJSON() ([]byte, error) {
	type stateFields State
	s.projectCanonical()
	return json.Marshal(stateFields(s))
}

func SaveState(path string, state State) error {
	state.SchemaVersion = StateSchemaVersion
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
	s.projectCanonical()
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
	if s.Cycle < 1 {
		return fmt.Errorf("invalid state cycle %d", s.Cycle)
	}
	if s.Stage == "" && s.Status == StatusBlocked {
		return errors.New("legacy blocked state does not reveal its prior stage: repair it by recording the stage the spec was blocked in, then migrate")
	}
	sc := StageCondition{Stage: s.Stage, Condition: s.Condition, CurrentRequest: s.CurrentRequest}
	if err := ValidateStageCondition(sc); err != nil {
		return err
	}
	if projected := ProjectStatus(sc); projected != s.Status {
		return fmt.Errorf("state status %q is not the projection %q of stage %q and condition %q", s.Status, projected, s.Stage, s.Condition)
	}
	if !ValidPhase(s.Phase) {
		return fmt.Errorf("invalid state phase %q", s.Phase)
	}
	if s.Phase != PhaseForStatus(s.Status) && s.Status != StatusBlocked {
		return fmt.Errorf("state phase %q does not match status %q", s.Phase, s.Status)
	}
	if _, err := s.Amendments(); err != nil {
		return fmt.Errorf("invalid amendment record: %w", err)
	}
	if _, err := s.Spikes(); err != nil {
		return fmt.Errorf("invalid spike record: %w", err)
	}
	// A stage waiting on approval must name a request that can still be
	// answered; a closed request leaves the wait unresolvable (spec 03 R5.2).
	requests, err := s.ApprovalRequests()
	if err != nil {
		return fmt.Errorf("invalid approval request record: %w", err)
	}
	if s.CurrentRequest != "" && !ApprovalRequestPending(requests, s.CurrentRequest) {
		return fmt.Errorf("current approval request %q is not an open request", s.CurrentRequest)
	}
	return nil
}
