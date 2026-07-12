package orchestration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// ACP event kinds. claim/report carry the worker-rigor fields (spec 10 R3);
// dispatch is the brain's own record.
const (
	ACPKindDispatch = "dispatch"
	ACPKindClaim    = "claim"
	ACPKindReport   = "report"
	ACPKindCancel   = "cancel"
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
	Observation  *ObservationV1    `json:"observation,omitempty"`

	// TraceDigest pins the normalized observable trace (TraceDigest) backing a
	// report, linking the ledger to the trajectory evidence a trajectory eval
	// scores (spec 04 R4.2/R4.3). Optional so bare dispatch/claim events and
	// pre-trace ledgers stay valid.
	TraceDigest string `json:"trace_digest,omitempty"`

	// Sanitized mission audit identity. AuditID is strictly increasing within a
	// run/mission/task/policy stream; AuditKind follows constitutional stage order.
	AuditID        int    `json:"audit_id,omitempty"`
	AuditKind      string `json:"audit_kind,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	PolicyDigest   string `json:"policy_digest,omitempty"`
	DispatchDigest string `json:"dispatch_digest,omitempty"`
}

var auditStage = map[string]int{"authority": 1, "tools": 2, "diff": 3, "scans": 4, "verify": 5, "review": 6, "exceptions": 7, "submit": 8}

func validateAuditEvent(e ACPEvent) error {
	if e.AuditID == 0 && e.AuditKind == "" {
		return nil
	}
	if e.AuditID < 1 || e.RunID == "" || e.MissionID == "" || e.TaskID == "" || e.PolicyDigest == "" {
		return errors.New("audit event missing identity")
	}
	if _, ok := auditStage[e.AuditKind]; !ok {
		return fmt.Errorf("unknown audit kind %q", e.AuditKind)
	}
	lower := strings.ToLower(e.Payload)
	for _, marker := range []string{"--token=", "password=", "secret=", "chain-of-thought", "hidden reasoning"} {
		if strings.Contains(lower, marker) {
			return errors.New("audit payload contains sensitive or hidden data")
		}
	}
	return nil
}

// HarnessAffectedPaths is the harness-observed set of paths a worker touched.
// It is audit evidence, not an authoritative diff: Domain 06 owns changed-file
// authority (spec 04 R4.3). Paths are de-duplicated and sorted.
func HarnessAffectedPaths(events []ObservableEvent) []string {
	seen := map[string]bool{}
	for _, ev := range events {
		for _, p := range ev.Paths {
			if p != "" {
				seen[p] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
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

// AppendACP appends one event to the ledger. It is O(1): Seq is a read-time
// projection (ReadACP numbers events by position), so a write never re-reads the
// whole file — open, append, fsync. ponytail: the semantic appends
// (AppendClaim's attempt count, AppendDispatch's duplicate-mission guard) still
// scan the ledger, but that read is their actual job and the per-spec ledger is
// bounded by task count × attempts. Maintain an on-disk index only if a ledger
// ever grows past a few thousand events.
func AppendACP(path string, event ACPEvent) error {
	if err := validateAuditEvent(event); err != nil {
		return err
	}
	if event.Observation != nil {
		normalized, err := NormalizeObservation(*event.Observation)
		if err != nil {
			return err
		}
		event.Observation = &normalized
	}
	if err := core.ValidateAnnotations(event.Telemetry); err != nil {
		return fmt.Errorf("acp telemetry: %w", err)
	}
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

// ReadACP loads the ledger and numbers each event by position (Seq = 1-based
// index). It reads the whole file at once — no per-line size cap, so a large
// event (ChangedFiles/Payload/Telemetry) never trips the old 64KB bufio.Scanner
// limit.
func ReadACP(path string) ([]ACPEvent, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open acp: %w", err)
	}
	var events []ACPEvent
	lastID := map[string]int{}
	lastStage := map[string]int{}
	lines := bytes.Split(data, []byte{'\n'})
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event ACPEvent
		if err := json.Unmarshal(line, &event); err != nil {
			if i == len(lines)-1 {
				// A torn *final* line is the signature of a crash mid-append
				// (AppendACP writes the record and its newline in one fsynced
				// write): drop it and keep the prior complete events, so recovery
				// converges on the prior complete ledger rather than failing to
				// read (spec 07 R2.4). Corruption anywhere else is a real error.
				break
			}
			return nil, fmt.Errorf("decode acp line %d: %w", i+1, err)
		}
		if err := core.ValidateAnnotations(event.Telemetry); err != nil {
			return nil, fmt.Errorf("acp telemetry: %w", err)
		}
		if err := validateAuditEvent(event); err != nil {
			return nil, fmt.Errorf("acp audit line %d: %w", i+1, err)
		}
		if event.AuditID > 0 {
			key := event.RunID + "\x00" + event.MissionID + "\x00" + event.TaskID + "\x00" + event.PolicyDigest
			if event.AuditID <= lastID[key] {
				return nil, fmt.Errorf("audit id duplicate or out of order: %d", event.AuditID)
			}
			stage := auditStage[event.AuditKind]
			if stage < lastStage[key] {
				return nil, fmt.Errorf("audit stage out of order: %s", event.AuditKind)
			}
			lastID[key], lastStage[key] = event.AuditID, stage
		}
		event.Seq = len(events) + 1
		events = append(events, event)
	}
	return events, nil
}
