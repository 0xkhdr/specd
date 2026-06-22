package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

const SchemaVersion = 5

// Execution mode for a spec. Base is the plain spec-driven lifecycle the host
// agent drives itself; Orchestrated lets the Brain/Pinky multi-agent layer drive.
// Mode is per-spec and recorded in state.json (the single source of truth), never
// inferred from project-wide config — capability permits, mode selects.
const (
	ModeBase         = "base"
	ModeOrchestrated = "orchestrated"
)

// Origin records how a spec's execution mode was chosen, for audit via replay.
const (
	OriginDefault     = "default"              // never opted in; default Base
	OriginUser        = "user"                 // explicit user opt-in (flag or --set)
	OriginRecommended = "recommended-accepted" // user accepted a harness recommendation
)

type SpecStatus string

const (
	StatusRequirements SpecStatus = "requirements"
	StatusDesign       SpecStatus = "design"
	StatusTasks        SpecStatus = "tasks"
	StatusExecuting    SpecStatus = "executing"
	StatusVerifying    SpecStatus = "verifying"
	StatusComplete     SpecStatus = "complete"
	StatusBlocked      SpecStatus = "blocked"
)

type Phase string

const (
	PhasePerceive Phase = "perceive"
	PhaseAnalyze  Phase = "analyze"
	PhasePlan     Phase = "plan"
	PhaseExecute  Phase = "execute"
	PhaseVerify   Phase = "verify"
	PhaseReflect  Phase = "reflect"
)

type Gate string

const (
	GateNone             Gate = "none"
	GateAwaitingApproval Gate = "awaiting-approval"
)

type TaskStatus string

const (
	TaskPending  TaskStatus = "pending"
	TaskRunning  TaskStatus = "running"
	TaskComplete TaskStatus = "complete"
	TaskBlocked  TaskStatus = "blocked"
)

type VerificationRecord struct {
	Command    string  `json:"command"`
	ExitCode   int     `json:"exitCode"`
	Verified   bool    `json:"verified"`
	TimedOut   bool    `json:"timedOut"`
	StdoutTail string  `json:"stdoutTail"`
	StderrTail string  `json:"stderrTail"`
	DurationMs int64   `json:"durationMs"`
	RanAt      string  `json:"ranAt"`
	GitHead    *string `json:"gitHead,omitempty"`
	// ChangedFiles is the set of working-tree paths changed at verify time
	// (git diff --name-only vs HEAD). Evidence for the scope gate; omitempty so
	// records written before this field still parse byte-for-byte.
	ChangedFiles []string `json:"changedFiles,omitempty"`
	// Coverage is the parsed total coverage at verify time (e.g. "84.2%") or
	// "unavailable" when no coverage signal was found. It is evidence only —
	// coverage capture never fails a verify. omitempty for back-compat.
	Coverage string `json:"coverage,omitempty"`
	// Sandbox names the isolation backend the command ran under ("bwrap",
	// "container"). Empty/omitted means the default unsandboxed shell runner, so
	// pre-sandbox records and `--sandbox none` runs stay byte-identical.
	Sandbox string `json:"sandbox,omitempty"`
	// Reverted is true when a failed verify stashed the working tree under
	// --revert-on-fail. StashRef carries the recoverable git stash reference so
	// the change can be restored with `git stash apply <ref>`. Both omitempty so
	// passing/default runs stay byte-identical.
	Reverted bool   `json:"reverted,omitempty"`
	StashRef string `json:"stashRef,omitempty"`
}

type CriterionRecord struct {
	Requirement int    `json:"requirement"`
	Criterion   int    `json:"criterion"`
	Status      string `json:"status"` // "pass" | "fail"
	Evidence    string `json:"evidence"`
	RanAt       string `json:"ranAt"`
}

// Telemetry is per-task cost/timing evidence. Durations are measured via the
// injectable Clock (deterministic under the test clock); tokens/cost are
// operator-annotated values, never computed by specd (no pricing API). Every
// field is omitempty so tasks without telemetry stay byte-identical.
type Telemetry struct {
	DurationMs       int64  `json:"durationMs,omitempty"`       // running → complete elapsed
	VerifyDurationMs int64  `json:"verifyDurationMs,omitempty"` // most recent verify run
	Retries          int    `json:"retries,omitempty"`          // verify re-runs for this task
	Tokens           int    `json:"tokens,omitempty"`           // annotated, not computed
	Cost             string `json:"cost,omitempty"`             // annotated (e.g. "0.42"), not computed
}

type TaskState struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	Role         string              `json:"role"`
	Wave         int                 `json:"wave"`
	Depends      []string            `json:"depends"`
	Requirements []int               `json:"requirements"`
	Status       TaskStatus          `json:"status"`
	StartedAt    *string             `json:"startedAt,omitempty"`
	FinishedAt   *string             `json:"finishedAt,omitempty"`
	Evidence     *string             `json:"evidence,omitempty"`
	Verification *VerificationRecord `json:"verification,omitempty"`
	Blocker      *string             `json:"blocker,omitempty"`
	Telemetry    *Telemetry          `json:"telemetry,omitempty"`
}

type Blocker struct {
	Task   string `json:"task"`
	Reason string `json:"reason"`
	Since  string `json:"since"`
}

type State struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Revision      int                        `json:"revision"`
	Spec          string                     `json:"spec"`
	Title         string                     `json:"title"`
	Status        SpecStatus                 `json:"status"`
	Phase         Phase                      `json:"phase"`
	Gate          Gate                       `json:"gate"`
	Turn          int                        `json:"turn"`
	CreatedAt     string                     `json:"createdAt"`
	UpdatedAt     string                     `json:"updatedAt"`
	Tasks         map[string]TaskState       `json:"tasks"`
	Blockers      []Blocker                  `json:"blockers"`
	Acceptance    map[string]CriterionRecord `json:"acceptance,omitempty"`
	// Prompt is the optional originating `specd new --from` text. omitempty keeps
	// state.json byte-identical for specs created without --from.
	Prompt string `json:"prompt,omitempty"`
	// ExecutionMode is the per-spec execution mode ("base" | "orchestrated").
	// Empty means Base (see EffectiveMode); omitempty so Base specs keep
	// byte-identical state.json and pre-mode specs migrate without data change.
	ExecutionMode string `json:"executionMode,omitempty"`
	// ModeOrigin records how ExecutionMode was set ("default" | "user" |
	// "recommended-accepted"), for the replay audit trail. omitempty for the
	// same byte-stability reason as ExecutionMode.
	ModeOrigin string `json:"modeOrigin,omitempty"`
}

// EffectiveMode returns the spec's resolved execution mode, treating an empty
// ExecutionMode (the omitempty Base default) as ModeBase. This is the single
// place that maps the stored-or-absent field to a concrete mode, so callers
// never branch on the empty string.
func (s State) EffectiveMode() string {
	if s.ExecutionMode == "" {
		return ModeBase
	}
	return s.ExecutionMode
}

// Clock is the time source for all spec-state timestamps and human-readable
// date stamps. Production uses the real wall clock; tests override it (see
// internal/testharness.FakeClock) for deterministic, golden-comparable output.
//
// Note: the advisory-lock staleness logic in lock.go deliberately does NOT use
// Clock — lock reclamation needs real elapsed wall-clock time and must stay
// immune to a frozen test clock.
var Clock = time.Now

func NowISO() string {
	return Clock().UTC().Format(time.RFC3339Nano)
}

func InitialState(spec, title string) State {
	ts := NowISO()
	return State{
		SchemaVersion: SchemaVersion,
		Revision:      0,
		Spec:          spec,
		Title:         title,
		Status:        StatusRequirements,
		Phase:         PhaseAnalyze,
		Gate:          GateNone,
		Turn:          0,
		CreatedAt:     ts,
		UpdatedAt:     ts,
		Tasks:         map[string]TaskState{},
		Blockers:      []Blocker{},
	}
}

func migrate(raw map[string]json.RawMessage) (State, error) {
	var sv int
	if v, ok := raw["schemaVersion"]; ok {
		if err := json.Unmarshal(v, &sv); err != nil {
			return State{}, GateError("corrupt schemaVersion in state.json")
		}
	}
	if sv == 0 {
		sv = 1
	}
	if sv > SchemaVersion {
		return State{}, GateError(fmt.Sprintf("state.json schemaVersion %d is newer than this specd (%d) — upgrade specd", sv, SchemaVersion))
	}
	// All migrations are shape-compatible; just stamp the current version.
	raw["schemaVersion"] = json.RawMessage(fmt.Sprintf("%d", SchemaVersion))
	if _, ok := raw["revision"]; !ok {
		raw["revision"] = json.RawMessage("0")
	}

	// Re-marshal to canonical form and unmarshal into State.
	b, err := json.Marshal(raw)
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, err
	}
	return s, nil
}

func statePath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "state.json")
}

func LoadState(root, slug string) (*State, error) {
	raw := ReadOrNull(statePath(root, slug))
	if raw == nil {
		return nil, nil
	}
	var partial map[string]json.RawMessage
	if err := json.Unmarshal([]byte(*raw), &partial); err != nil {
		return nil, GateError(fmt.Sprintf("corrupt state.json for spec '%s': %v", slug, err))
	}
	if partial == nil || partial["spec"] == nil {
		return nil, GateError(fmt.Sprintf("malformed state.json for spec '%s': missing required fields", slug))
	}
	s, err := migrate(partial)
	if err != nil {
		return nil, err
	}
	if s.Tasks == nil {
		s.Tasks = map[string]TaskState{}
	}
	if s.Blockers == nil {
		s.Blockers = []Blocker{}
	}
	return &s, nil
}

// assertLocked, when true, makes SaveState panic if it is called without the
// caller holding the spec lock. Tests flip it on to catch the footgun; prod
// leaves it false so the check costs nothing.
var assertLocked = false

// SaveState performs a compare-and-swap commit of state to disk: it verifies
// the on-disk revision still matches state.Revision, then bumps the revision
// and atomically writes.
//
// INVARIANT: SaveState MUST be called inside WithSpecLock(root, slug, …) for
// the same (root, slug). The read-then-write CAS is not atomic on its own; the
// advisory lock is what serializes it against concurrent writers. Calling
// SaveState without the lock silently reintroduces the lost-update race. Test
// builds set assertLocked to panic on violations.
func SaveState(root, slug string, state *State) error {
	if assertLocked && !lockHeldBy(root, slug) {
		panic("SaveState called without spec lock: " + slug)
	}
	path := statePath(root, slug)
	disk := ReadOrNull(path)
	if disk != nil {
		var onDisk struct {
			Revision int `json:"revision"`
		}
		if err := json.Unmarshal([]byte(*disk), &onDisk); err == nil {
			if onDisk.Revision != state.Revision {
				return GateError(fmt.Sprintf("state.json for '%s' changed underfoot (on-disk revision %d ≠ expected %d) — concurrent write detected, reload and retry", slug, onDisk.Revision, state.Revision))
			}
		}
	} else if state.Revision > 0 {
		// We hold a state that was previously persisted (revision > 0) but the
		// file is gone now: a concurrent delete (or delete+recreate) happened
		// mid-session. Treat as a conflict rather than silently recreating it.
		return GateError(fmt.Sprintf("state.json for '%s' disappeared mid-session — concurrent delete detected, reload and retry", slug))
	}
	state.Revision++
	state.UpdatedAt = NowISO()
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, string(b)+"\n")
}

func IsSpecdError(err error) (*SpecdError, bool) {
	var se *SpecdError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}
