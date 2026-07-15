package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

// errProgramStateNotFound signals that no program-state.json exists yet for a
// parent session (a program that never took a driver step, or a non-program
// session). Callers treat it as "nothing to resume", distinct from a corrupt
// file, which fails closed with a decode/validation error.
var errProgramStateNotFound = errors.New("program orchestration: program state not found")

// ProgramState is the authoritative on-disk projection of a program run's
// frontier (cross-spec recovery). It captures, for one parent session, the child
// session each spec is using, the dispatch keys in flight at the last driver
// step, and each child's last-known status — enough to reconstruct the program
// DAG frontier after a host crash without re-deriving it from scattered child
// sessions. The child session remains authoritative on resume; this file is a
// crash-coherent hint, written atomically on every program driver step.
type ProgramState struct {
	Version         int                   `json:"version"`
	ParentSessionID string                `json:"parentSessionId"`
	ChildSessions   map[string]string     `json:"childSessions"`
	InflightKeys    []string              `json:"inflightKeys"`
	ChildStatus     map[string]SpecStatus `json:"childStatus"`
	UpdatedAt       string                `json:"updatedAt"`
}

// CompleteChildCount returns how many children are in a terminal-done status.
func (s ProgramState) CompleteChildCount() int {
	n := 0
	for _, status := range s.ChildStatus {
		if status == StatusComplete {
			n++
		}
	}
	return n
}

// canonical returns the byte-stable encoding of the state: map keys are sorted
// by encoding/json and the inflight-key slice is sorted here, so an unchanged
// frontier always serializes identically (round-trip stable).
func (s ProgramState) canonical() ([]byte, error) {
	normalized := s
	normalized.InflightKeys = append([]string(nil), s.InflightKeys...)
	sort.Strings(normalized.InflightKeys)
	if normalized.ChildSessions == nil {
		normalized.ChildSessions = map[string]string{}
	}
	if normalized.ChildStatus == nil {
		normalized.ChildStatus = map[string]SpecStatus{}
	}
	if normalized.InflightKeys == nil {
		normalized.InflightKeys = []string{}
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("program orchestration: encode program state: %w", err)
	}
	return append(raw, '\n'), nil
}

// ValidateProgramState fails closed on any structurally invalid frontier so a
// corrupt file never drives a partial resume.
func ValidateProgramState(state ProgramState) error {
	if state.Version != OrchestrationModelVersion {
		return fmt.Errorf("program orchestration: unsupported program state version %d", state.Version)
	}
	if err := validateACPOpaqueID("program session ID", state.ParentSessionID); err != nil {
		return err
	}
	for slug, childSessionID := range state.ChildSessions {
		if err := ValidateSlug(slug); err != nil {
			return fmt.Errorf("program orchestration: invalid child slug %q: %w", slug, err)
		}
		if err := validateACPOpaqueID("child session ID", childSessionID); err != nil {
			return err
		}
	}
	for slug, status := range state.ChildStatus {
		if err := ValidateSlug(slug); err != nil {
			return fmt.Errorf("program orchestration: invalid child slug %q: %w", slug, err)
		}
		if !validSpecStatus(status) {
			return fmt.Errorf("program orchestration: invalid child status %q for %q", status, slug)
		}
	}
	for _, key := range state.InflightKeys {
		if key == "" {
			return fmt.Errorf("program orchestration: empty inflight key")
		}
	}
	if _, err := parseACPTime("program state updatedAt", state.UpdatedAt); err != nil {
		return err
	}
	return nil
}

// SaveProgramState writes the frontier atomically (temp + rename) so a crash
// always leaves a coherent latest file rather than a torn write. The program
// driver is the single writer (one step at a time), so the atomic rename is the
// CAS discipline here — exactly as saveProgramSession persists the parent
// session beside it.
func SaveProgramState(root string, state ProgramState) error {
	if state.UpdatedAt == "" {
		state.UpdatedAt = Clock().UTC().Format(time.RFC3339Nano)
	}
	if err := ValidateProgramState(state); err != nil {
		return err
	}
	raw, err := state.canonical()
	if err != nil {
		return err
	}
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	path, err := paths.ProgramStatePath(state.ParentSessionID)
	if err != nil {
		return err
	}
	if err := atomicWritePrivate(path, raw); err != nil {
		return fmt.Errorf("program orchestration: save program state: %w", err)
	}
	return nil
}

// persistProgramFrontier projects one program step's result plus the driver's
// in-flight dispatch keys into a ProgramState and writes it. Child sessions and
// statuses come from the step's leases and snapshot; in-flight keys come from the
// driver. It is called at the per-step commit point of DriveProgramOrchestration.
func persistProgramFrontier(root, parentSessionID string, res ProgramStepResult, inflightKeys map[string]bool) error {
	childSessions := map[string]string{}
	childStatus := map[string]SpecStatus{}
	for _, child := range res.Snapshot.Children {
		childStatus[child.Slug] = child.Status
		if child.ChildSessionID != "" {
			childSessions[child.Slug] = child.ChildSessionID
		}
	}
	for _, lease := range res.Leases {
		if lease.ChildSessionID != "" {
			childSessions[lease.Slug] = lease.ChildSessionID
		}
	}
	keys := make([]string, 0, len(inflightKeys))
	for key := range inflightKeys {
		keys = append(keys, key)
	}
	return SaveProgramState(root, ProgramState{
		Version:         OrchestrationModelVersion,
		ParentSessionID: parentSessionID,
		ChildSessions:   childSessions,
		InflightKeys:    keys,
		ChildStatus:     childStatus,
		UpdatedAt:       Clock().UTC().Format(time.RFC3339Nano),
	})
}

// LoadProgramState reads and validates the frontier for a parent session. A
// missing file yields errProgramStateNotFound; a present-but-corrupt file fails
// closed with a decode/validation error rather than a partial resume.
func LoadProgramState(root, parentSessionID string) (ProgramState, error) {
	if err := validateACPOpaqueID("program session ID", parentSessionID); err != nil {
		return ProgramState{}, err
	}
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return ProgramState{}, err
	}
	path, err := paths.ProgramStatePath(parentSessionID)
	if err != nil {
		return ProgramState{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ProgramState{}, fmt.Errorf("%w: %s", errProgramStateNotFound, parentSessionID)
	}
	if err != nil {
		return ProgramState{}, fmt.Errorf("program orchestration: read program state: %w", err)
	}
	var state ProgramState
	if err := decodeACPStrict(raw, &state); err != nil {
		return ProgramState{}, fmt.Errorf("program orchestration: corrupt program state: %w", err)
	}
	if err := ValidateProgramState(state); err != nil {
		return ProgramState{}, fmt.Errorf("program orchestration: corrupt program state: %w", err)
	}
	return state, nil
}
