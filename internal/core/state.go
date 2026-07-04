package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const StateSchemaVersion = 1

var ErrRevisionConflict = errors.New("state revision conflict")

type Mode string

const (
	ModeDefault Mode = "default"
	ModeAgent   Mode = "agent"
)

type State struct {
	SchemaVersion int                        `json:"schema_version"`
	Slug          string                     `json:"slug"`
	Mode          Mode                       `json:"mode"`
	Status        Status                     `json:"status"`
	Phase         Phase                      `json:"phase"`
	Revision      int64                      `json:"revision"`
	Records       map[string]json.RawMessage `json:"records,omitempty"`
	Extra         map[string]json.RawMessage `json:"extra,omitempty"`
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
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode state: %w", err)
	}
	if err := state.Validate(); err != nil {
		return State{}, err
	}
	if state.Records == nil {
		state.Records = map[string]json.RawMessage{}
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
