package orchestration

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// ACP event kinds. claim/report carry the worker-rigor fields (spec 10 R3);
// dispatch is the brain's own record.
const (
	ACPKindDispatch = "dispatch"
	ACPKindClaim    = "claim"
	ACPKindReport   = "report"
)

type ACPEvent struct {
	Seq     int       `json:"seq"`
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	TaskID  string    `json:"task_id,omitempty"`
	Payload string    `json:"payload,omitempty"`
	// MissionID is the deterministic dispatch identifier (session/step/task, spec
	// 07 R3). Optional so bare dispatch events and pre-spec-07 ledgers stay valid.
	// It is the key the resume path and the duplicate guard match on.
	MissionID string `json:"mission_id,omitempty"`

	// Worker-rigor fields (spec 10 R3), all optional so a bare dispatch event and
	// pre-telemetry ledgers stay valid. Attempt is monotonic per task (see
	// NextAttempt); GitHead pins the claim/report to a commit; ChangedFiles is the
	// worker-reported diff surface; VerifyRef points at the verify evidence
	// backing a report; Telemetry is verbatim worker cost.
	Attempt      int               `json:"attempt,omitempty"`
	GitHead      string            `json:"git_head,omitempty"`
	ChangedFiles []string          `json:"changed_files,omitempty"`
	VerifyRef    string            `json:"verify_ref,omitempty"`
	Telemetry    *core.Annotations `json:"telemetry,omitempty"`
}

// NextAttempt is the attempt number for a new claim on taskID: the count of
// prior claim events for that task plus one. It is a countable fact derived from
// the ledger under the spec lock, never a stored counter that can skew (spec 10
// R3). It feeds spec 06's escalation counting.
func NextAttempt(events []ACPEvent, taskID string) int {
	n := 0
	for _, e := range events {
		if e.Kind == ACPKindClaim && e.TaskID == taskID {
			n++
		}
	}
	return n + 1
}

// AppendClaim appends a claim event, filling the attempt number from the current
// ledger when the caller left it zero. The read-then-append happens under the
// caller's spec lock so the derived attempt is race-free.
func AppendClaim(path string, event ACPEvent) error {
	event.Kind = ACPKindClaim
	if event.Attempt == 0 {
		events, err := ReadACP(path)
		if err != nil {
			return err
		}
		event.Attempt = NextAttempt(events, event.TaskID)
	}
	return AppendACP(path, event)
}

// ErrDuplicateMission is returned when a dispatch with a mission id already in
// the ledger is appended again. It is the invariant that makes crash-recovery
// re-issue idempotent: the same deterministic mission id can appear at most once.
var ErrDuplicateMission = errors.New("duplicate mission id")

// HasMission reports whether the ledger already carries an event with missionID.
func HasMission(events []ACPEvent, missionID string) bool {
	if missionID == "" {
		return false
	}
	for _, e := range events {
		if e.MissionID == missionID {
			return true
		}
	}
	return false
}

// MissionEvent returns the first event carrying missionID, or false.
func MissionEvent(events []ACPEvent, missionID string) (ACPEvent, bool) {
	for _, e := range events {
		if e.MissionID != "" && e.MissionID == missionID {
			return e, true
		}
	}
	return ACPEvent{}, false
}

// AppendDispatch appends a dispatch event, refusing a duplicate mission id
// (spec 07 R3). The read-then-append runs under the caller's spec lock so the
// duplicate check is race-free.
func AppendDispatch(path string, event ACPEvent) error {
	event.Kind = ACPKindDispatch
	if event.MissionID != "" {
		events, err := ReadACP(path)
		if err != nil {
			return err
		}
		if HasMission(events, event.MissionID) {
			return fmt.Errorf("%w: %s", ErrDuplicateMission, event.MissionID)
		}
	}
	return AppendACP(path, event)
}

func AppendACP(path string, event ACPEvent) error {
	events, err := ReadACP(path)
	if err != nil {
		return err
	}
	event.Seq = len(events) + 1
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode acp event: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir acp: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open acp: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append acp: %w", err)
	}
	return file.Sync()
}

func ReadACP(path string) ([]ACPEvent, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open acp: %w", err)
	}
	defer file.Close()

	var events []ACPEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event ACPEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode acp: %w", err)
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}
