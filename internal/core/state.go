package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/0xkhdr/specd/internal/spec"
)

// SchemaVersion is the current state.json schema version this build of specd
// writes and understands. LoadState refuses to read a state.json whose
// schemaVersion is newer than this value.
const SchemaVersion = 6

// Execution mode for a spec. Simple is the plain spec-driven lifecycle the host
// agent drives itself; Orchestrated lets the Brain/Pinky multi-agent layer drive.
// Mode is per-spec and recorded in state.json (the single source of truth), never
// inferred from project-wide config — capability permits, mode selects.
const (
	ModeSimple       = "simple"
	ModeOrchestrated = "orchestrated"
	ModeConductor    = "conductor"
)

// Origin records how a spec's execution mode was chosen, for audit via replay.
const (
	OriginDefault     = "default"              // never opted in; default Simple
	OriginUser        = "user"                 // explicit user opt-in (flag or --set)
	OriginRecommended = "recommended-accepted" // user accepted a harness recommendation
)

// SpecStatus and Phase (with their consts) live in internal/spec so both core
// and the context engine can share them without an import cycle. These aliases
// and const re-declarations keep every existing core.SpecStatus / core.Status* /
// core.Phase / core.Phase* call site compiling unchanged.
type SpecStatus = spec.SpecStatus

// StatusRequirements through StatusBlocked re-export the spec lifecycle
// statuses from internal/spec under their core.Status* names, so existing
// call sites keep compiling unchanged (see the SpecStatus alias comment).
const (
	StatusRequirements = spec.StatusRequirements
	StatusDesign       = spec.StatusDesign
	StatusTasks        = spec.StatusTasks
	StatusExecuting    = spec.StatusExecuting
	StatusVerifying    = spec.StatusVerifying
	StatusComplete     = spec.StatusComplete
	StatusBlocked      = spec.StatusBlocked
)

// Phase is a re-export of spec.Phase; see the SpecStatus alias comment above
// for why core re-declares it.
type Phase = spec.Phase

// PhasePerceive through PhaseReflect re-export the perceive-analyze-plan-
// execute-verify-reflect loop phases from internal/spec under their core.Phase*
// names.
const (
	PhasePerceive = spec.PhasePerceive
	PhaseAnalyze  = spec.PhaseAnalyze
	PhasePlan     = spec.PhasePlan
	PhaseExecute  = spec.PhaseExecute
	PhaseVerify   = spec.PhaseVerify
	PhaseReflect  = spec.PhaseReflect
)

// Gate represents whether a spec is blocked awaiting an explicit approval
// (e.g. `specd approve`) before it can proceed.
type Gate string

// GateNone and GateAwaitingApproval are the two valid Gate values: no pending
// approval, or blocked until the operator approves.
const (
	GateNone             Gate = "none"
	GateAwaitingApproval Gate = "awaiting-approval"
)

// TaskStatus is the lifecycle status of a single task within a spec's DAG.
type TaskStatus string

// TaskPending through TaskBlocked are the valid TaskStatus values a task can
// hold.
const (
	TaskPending  TaskStatus = "pending"
	TaskRunning  TaskStatus = "running"
	TaskComplete TaskStatus = "complete"
	TaskBlocked  TaskStatus = "blocked"
)

// VerificationRecord is the durable evidence captured when a task's verify
// command runs: exit status, captured output tails, timing, and optional
// scope/coverage/sandbox/revert metadata.
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

// CriterionRecord is the recorded pass/fail evidence for one acceptance
// criterion (requirement.criterion) of a spec.
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

// TaskState is the persisted state of a single task within a spec: its
// metadata, current status, timestamps, evidence, verification record,
// blocker (if any), and telemetry.
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

// Blocker records why a task is blocked: which task, the reason, and when it
// became blocked.
type EvalSummary struct {
	Suite    string  `json:"suite"`
	Score    float64 `json:"score"`
	MinScore float64 `json:"minScore"`
	Pass     bool    `json:"pass"`
	Seq      int     `json:"seq"`
	Time     string  `json:"time"`
}

type RoutingStamp struct {
	Tier      string  `json:"tier"`
	BudgetUSD float64 `json:"budgetUSD"`
	RuleIndex int     `json:"ruleIndex"`
}

type ConductorSession struct {
	SessionID string `json:"sessionID"`
	Task      string `json:"task"`
	Micro     string `json:"micro"`
	StartedAt string `json:"startedAt"`
}

type EscalationRecord struct {
	Task  string `json:"task"`
	Rule  string `json:"rule"`
	Facts string `json:"facts"`
	Time  string `json:"time"`
}

type Blocker struct {
	Task   string `json:"task"`
	Reason string `json:"reason"`
	Since  string `json:"since"`
}

// State is the full on-disk representation of a spec's state.json: schema and
// revision bookkeeping, lifecycle status/phase/gate, its tasks, blockers,
// acceptance evidence, and execution-mode metadata.
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
	Evals         map[string]EvalSummary     `json:"evals,omitempty"`
	Routing       map[string]RoutingStamp    `json:"routing,omitempty"`
	Conductor     *ConductorSession          `json:"conductor,omitempty"`
	Escalation    *EscalationRecord          `json:"escalation,omitempty"`
	// Prompt is the optional originating `specd new --from` text. omitempty keeps
	// state.json byte-identical for specs created without --from.
	Prompt string `json:"prompt,omitempty"`
	// ExecutionMode is the per-spec execution mode ("simple" | "orchestrated").
	// Empty means Simple (see EffectiveMode); omitempty so Simple specs keep
	// byte-identical state.json and pre-mode specs migrate without data change.
	ExecutionMode string `json:"executionMode,omitempty"`
	// ModeOrigin records how ExecutionMode was set ("default" | "user" |
	// "recommended-accepted"), for the replay audit trail. omitempty for the
	// same byte-stability reason as ExecutionMode.
	ModeOrigin string `json:"modeOrigin,omitempty"`
}

// EffectiveMode returns the spec's resolved execution mode, treating an empty
// ExecutionMode (the omitempty Simple default) as ModeSimple. This is the single
// place that maps the stored-or-absent field to a concrete mode, so callers
// never branch on the empty string.
func (s State) EffectiveMode() string {
	if s.ExecutionMode == "" {
		return ModeSimple
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

// NowISO returns the current time, via Clock, formatted as RFC3339Nano UTC —
// the timestamp format used throughout state.json.
func NowISO() string {
	return Clock().UTC().Format(time.RFC3339Nano)
}

// InitialState builds the freshly-created State for a new spec: schema
// version stamped, revision 0, status/phase set to the start of the
// requirements-analysis lifecycle, and empty task/blocker collections.
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
	if err := validateExecutionMode(s.ExecutionMode); err != nil {
		return State{}, err
	}
	return s, nil
}

func validateExecutionMode(mode string) error {
	switch mode {
	case "", ModeSimple, ModeOrchestrated, ModeConductor:
		return nil
	default:
		return GateError(fmt.Sprintf("state.json executionMode %q is unknown", mode))
	}
}

func statePath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "state.json")
}

// LoadState reads and returns the spec's state.json from
// .specd/specs/<slug>/state.json under root, migrating it to SchemaVersion
// and back-filling nil Tasks/Blockers as needed. It returns (nil, nil) if no
// state.json exists yet (a not-yet-initialized spec), and a GateError for any
// corrupt, malformed, or invalid-status state — LoadState never silently
// coerces bad on-disk state into something runnable. Callers that intend to
// mutate the result must hold the spec lock and persist via SaveState, whose
// compare-and-swap (CAS) on Revision is what actually protects state.json
// from concurrent writers; LoadState itself performs a single, lock-free read.
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
	// Fail loud on a corrupt/hand-edited status rather than silently coercing it.
	// A resume (e.g. cross-spec-recovery walking child state.json) must refuse an
	// impossible status, naming the spec and the offending value, so the operator
	// is never unknowingly running on coerced state.
	if !s.Status.IsValid() {
		return nil, GateError(fmt.Sprintf("invalid status %q in state.json for spec '%s' — refusing to resume on corrupt state; fix or recreate the spec", s.Status, slug))
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

// IsSpecdError reports whether err is (or wraps) a *SpecdError, returning the
// unwrapped error alongside the boolean for callers that need its structured
// fields.
func IsSpecdError(err error) (*SpecdError, bool) {
	var se *SpecdError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}
